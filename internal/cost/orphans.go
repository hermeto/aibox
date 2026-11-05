package cost

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
)

// RunTag is the tag key that identifies a run's resources.
const RunTag = "run-id"

// EC2API is the read-only subset of the EC2 client used by the orphan checker and cost.
type EC2API interface {
	DescribeInstances(ctx context.Context, in *ec2.DescribeInstancesInput, opts ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	DescribeVolumes(ctx context.Context, in *ec2.DescribeVolumesInput, opts ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
	DescribeSnapshots(ctx context.Context, in *ec2.DescribeSnapshotsInput, opts ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error)
	DescribeNetworkInterfaces(ctx context.Context, in *ec2.DescribeNetworkInterfacesInput, opts ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error)
	DescribeAddresses(ctx context.Context, in *ec2.DescribeAddressesInput, opts ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error)
	DescribeNatGateways(ctx context.Context, in *ec2.DescribeNatGatewaysInput, opts ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error)
}

// ELBAPI is the read-only subset of the Elastic Load Balancing v2 client.
type ELBAPI interface {
	DescribeLoadBalancers(ctx context.Context, in *elbv2.DescribeLoadBalancersInput, opts ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancersOutput, error)
	DescribeTags(ctx context.Context, in *elbv2.DescribeTagsInput, opts ...func(*elbv2.Options)) (*elbv2.DescribeTagsOutput, error)
}

// Orphan is a resource that still exists after teardown.
type Orphan struct {
	Type   string `json:"type"`
	ID     string `json:"id"`
	Region string `json:"region"`
	State  string `json:"state,omitempty"`
}

// OrphanReport is the result of an orphan check.
type OrphanReport struct {
	RunID   string   `json:"runId"`
	Region  string   `json:"region"`
	Orphans []Orphan `json:"orphans"`
	OK      bool     `json:"ok"`
}

// RenderText lists the orphans.
func (r *OrphanReport) RenderText(w io.Writer) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Orphan check for run %s in %s: %d orphan(s)\n", r.RunID, r.Region, len(r.Orphans))
	for _, o := range r.Orphans {
		fmt.Fprintf(&b, "  %-14s %-28s %s\n", o.Type, o.ID, o.State)
	}
	if !r.OK {
		b.WriteString("Delete them (see docs/runbooks/cost-guardrails.md) and re-run `aibox down --orphans-only`.\n")
	}
	_, err := w.Write([]byte(b.String()))
	return err
}

func tagFilter(runID string) []ec2types.Filter {
	return []ec2types.Filter{{Name: aws.String("tag:" + RunTag), Values: []string{runID}}}
}

// FindOrphans lists every resource tagged with runID that still exists. All calls are
// read-only Describe operations.
func FindOrphans(ctx context.Context, ec2c EC2API, elb ELBAPI, region, runID string) (*OrphanReport, error) {
	r := &OrphanReport{RunID: runID, Region: region, Orphans: []Orphan{}}
	add := func(typ, id, state string) {
		r.Orphans = append(r.Orphans, Orphan{Type: typ, ID: id, Region: region, State: state})
	}

	var token *string
	for {
		out, err := ec2c.DescribeInstances(ctx, &ec2.DescribeInstancesInput{Filters: tagFilter(runID), NextToken: token})
		if err != nil {
			return nil, fmt.Errorf("describe instances: %w", err)
		}
		for _, res := range out.Reservations {
			for _, in := range res.Instances {
				state := ""
				if in.State != nil {
					state = string(in.State.Name)
				}
				if state != string(ec2types.InstanceStateNameTerminated) {
					add("instance", aws.ToString(in.InstanceId), state)
				}
			}
		}
		if token = out.NextToken; token == nil {
			break
		}
	}
	vols, err := ec2c.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{Filters: tagFilter(runID)})
	if err != nil {
		return nil, fmt.Errorf("describe volumes: %w", err)
	}
	for _, v := range vols.Volumes {
		add("volume", aws.ToString(v.VolumeId), string(v.State))
	}
	snaps, err := ec2c.DescribeSnapshots(ctx, &ec2.DescribeSnapshotsInput{Filters: tagFilter(runID), OwnerIds: []string{"self"}})
	if err != nil {
		return nil, fmt.Errorf("describe snapshots: %w", err)
	}
	for _, s := range snaps.Snapshots {
		add("snapshot", aws.ToString(s.SnapshotId), string(s.State))
	}
	enis, err := ec2c.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{Filters: tagFilter(runID)})
	if err != nil {
		return nil, fmt.Errorf("describe network interfaces: %w", err)
	}
	for _, n := range enis.NetworkInterfaces {
		add("eni", aws.ToString(n.NetworkInterfaceId), string(n.Status))
	}
	eips, err := ec2c.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{Filters: tagFilter(runID)})
	if err != nil {
		return nil, fmt.Errorf("describe addresses: %w", err)
	}
	for _, a := range eips.Addresses {
		add("eip", aws.ToString(a.AllocationId), "")
	}
	nats, err := ec2c.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{Filter: tagFilter(runID)})
	if err != nil {
		return nil, fmt.Errorf("describe nat gateways: %w", err)
	}
	for _, n := range nats.NatGateways {
		if n.State != ec2types.NatGatewayStateDeleted {
			add("nat-gateway", aws.ToString(n.NatGatewayId), string(n.State))
		}
	}
	if err := loadBalancers(ctx, elb, runID, add); err != nil {
		return nil, err
	}
	sort.Slice(r.Orphans, func(i, j int) bool {
		if r.Orphans[i].Type != r.Orphans[j].Type {
			return r.Orphans[i].Type < r.Orphans[j].Type
		}
		return r.Orphans[i].ID < r.Orphans[j].ID
	})
	r.OK = len(r.Orphans) == 0
	return r, nil
}

func loadBalancers(ctx context.Context, elb ELBAPI, runID string, add func(typ, id, state string)) error {
	var marker *string
	for {
		out, err := elb.DescribeLoadBalancers(ctx, &elbv2.DescribeLoadBalancersInput{Marker: marker})
		if err != nil {
			return fmt.Errorf("describe load balancers: %w", err)
		}
		arns := make([]string, 0, len(out.LoadBalancers))
		for _, lb := range out.LoadBalancers {
			arns = append(arns, aws.ToString(lb.LoadBalancerArn))
		}
		for i := 0; i < len(arns); i += 20 { // DescribeTags accepts up to 20 ARNs
			end := min(i+20, len(arns))
			tags, err := elb.DescribeTags(ctx, &elbv2.DescribeTagsInput{ResourceArns: arns[i:end]})
			if err != nil {
				return fmt.Errorf("describe load balancer tags: %w", err)
			}
			for _, td := range tags.TagDescriptions {
				for _, t := range td.Tags {
					if aws.ToString(t.Key) == RunTag && aws.ToString(t.Value) == runID {
						add("load-balancer", aws.ToString(td.ResourceArn), "")
					}
				}
			}
		}
		if marker = out.NextMarker; marker == nil {
			return nil
		}
	}
}
