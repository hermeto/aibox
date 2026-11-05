package cost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	"github.com/hermeto/aibox/internal/profile"
)

// fixtures replays recorded API responses (testdata/<scenario>/<Operation>.json) decoded
// into the SDK output structs, so production filtering code runs on them.
type fixtures struct {
	t   *testing.T
	dir string
	err error
}

func load[T any](f fixtures, op string) (*T, error) {
	if f.err != nil {
		return nil, f.err
	}
	data, err := os.ReadFile(filepath.Join("testdata", f.dir, op+".json"))
	if err != nil {
		f.t.Fatalf("fixture %s/%s: %v", f.dir, op, err)
	}
	var out T
	if err := json.Unmarshal(data, &out); err != nil {
		f.t.Fatalf("decode %s/%s: %v", f.dir, op, err)
	}
	return &out, nil
}

func (f fixtures) DescribeInstances(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return load[ec2.DescribeInstancesOutput](f, "DescribeInstances")
}
func (f fixtures) DescribeVolumes(context.Context, *ec2.DescribeVolumesInput, ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	return load[ec2.DescribeVolumesOutput](f, "DescribeVolumes")
}
func (f fixtures) DescribeSnapshots(context.Context, *ec2.DescribeSnapshotsInput, ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	return load[ec2.DescribeSnapshotsOutput](f, "DescribeSnapshots")
}
func (f fixtures) DescribeNetworkInterfaces(context.Context, *ec2.DescribeNetworkInterfacesInput, ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	return load[ec2.DescribeNetworkInterfacesOutput](f, "DescribeNetworkInterfaces")
}
func (f fixtures) DescribeAddresses(context.Context, *ec2.DescribeAddressesInput, ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
	return load[ec2.DescribeAddressesOutput](f, "DescribeAddresses")
}
func (f fixtures) DescribeNatGateways(context.Context, *ec2.DescribeNatGatewaysInput, ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
	return load[ec2.DescribeNatGatewaysOutput](f, "DescribeNatGateways")
}
func (f fixtures) DescribeLoadBalancers(context.Context, *elbv2.DescribeLoadBalancersInput, ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancersOutput, error) {
	return load[elbv2.DescribeLoadBalancersOutput](f, "DescribeLoadBalancers")
}
func (f fixtures) DescribeTags(context.Context, *elbv2.DescribeTagsInput, ...func(*elbv2.Options)) (*elbv2.DescribeTagsOutput, error) {
	return load[elbv2.DescribeTagsOutput](f, "DescribeTags")
}

const runID = "aws-dev-202611050000-abcd"

func table(t *testing.T) *Table {
	t.Helper()
	tb, err := LoadTable(filepath.Join("..", ".."), "us-east-1")
	if err != nil {
		t.Fatal(err)
	}
	return tb
}

func TestOrphansClean(t *testing.T) {
	f := fixtures{t: t, dir: "clean"}
	r, err := FindOrphans(context.Background(), f, f, "us-east-1", runID)
	if err != nil || !r.OK || len(r.Orphans) != 0 {
		t.Fatalf("%+v %v", r, err)
	}
	var b bytes.Buffer
	_ = r.RenderText(&b)
	if !strings.Contains(b.String(), "0 orphan(s)") {
		t.Fatal(b.String())
	}
}

func TestOrphansLeftovers(t *testing.T) {
	f := fixtures{t: t, dir: "leftovers"}
	r, err := FindOrphans(context.Background(), f, f, "us-east-1", runID)
	if err != nil || r.OK {
		t.Fatalf("%+v %v", r, err)
	}
	got := map[string]int{}
	for _, o := range r.Orphans {
		got[o.Type]++
	}
	want := map[string]int{"instance": 3, "volume": 1, "snapshot": 1, "eni": 1, "eip": 1, "nat-gateway": 1, "load-balancer": 1}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%s: got %d want %d (%v)", k, got[k], v, r.Orphans)
		}
	}
	var b bytes.Buffer
	_ = r.RenderText(&b)
	if !strings.Contains(b.String(), "vol-0ccccccccccccccc1") || !strings.Contains(b.String(), "--orphans-only") {
		t.Fatal(b.String())
	}
}

func TestOrphansAPIError(t *testing.T) {
	f := fixtures{t: t, dir: "clean", err: errors.New("AccessDenied")}
	if _, err := FindOrphans(context.Background(), f, f, "us-east-1", runID); err == nil || !strings.Contains(err.Error(), "AccessDenied") {
		t.Fatalf("got %v", err)
	}
}

func TestRunCost(t *testing.T) {
	f := fixtures{t: t, dir: "leftovers"}
	start := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	now := start.Add(2 * time.Hour)
	r, err := RunCost(context.Background(), f, table(t), runID, start, now)
	if err != nil {
		t.Fatal(err)
	}
	// g6.xlarge spot 1.5 h × 0.32 = 0.48; m7i.xlarge 2 h × 0.2016 = 0.40; mystery unknown;
	// fixed: EKS 2 × 0.10 = 0.20, NAT 2 × 0.045 = 0.09 → 1.17.
	if r.Total != 1.17 || !r.LowerBound || len(r.Items) != 5 {
		t.Fatalf("total %.2f lower %v items %+v", r.Total, r.LowerBound, r.Items)
	}
	var b bytes.Buffer
	if err := r.RenderText(&b); err != nil || !strings.Contains(b.String(), "lags by up to 24 hours") || !strings.Contains(b.String(), "unknown") {
		t.Fatal(b.String())
	}
	if !strings.Contains(strings.Join(r.Notes, " "), "not verified") {
		t.Fatal("unverified table must be noted")
	}
	if hoursBetween(now, start) != 0 {
		t.Fatal("negative window")
	}
	if _, err := RunCost(context.Background(), fixtures{t: t, dir: "clean", err: errors.New("x")}, table(t), runID, start, now); err == nil {
		t.Fatal("expected error")
	}
}

func TestPriceTable(t *testing.T) {
	tb := table(t)
	if p, ok := tb.Hourly("g6.xlarge", "spot"); !ok || p >= 0.8048 {
		t.Fatalf("spot %v %v", p, ok)
	}
	if p, _ := tb.Hourly("p5.48xlarge", "capacity-block"); p != 31.464 {
		t.Fatalf("capacity block %v", p)
	}
	if p, _ := tb.Hourly("g5.xlarge", "on-demand"); p != 1.006 {
		t.Fatalf("on-demand %v", p)
	}
	if _, ok := tb.Hourly("nope.large", "spot"); ok {
		t.Fatal("unknown type")
	}
	if tb.Age(time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC)) <= MaxAge {
		t.Fatal("table should be stale in 2027-06")
	}
	bad := &Table{Instances: map[string]Price{"x": {OnDemand: 0}}, LastUpdated: "yesterday"}
	if err := bad.Validate(); err == nil || !strings.Contains(err.Error(), "onDemand") {
		t.Fatalf("validate: %v", err)
	}
	if (&Table{LastUpdated: "bad"}).Age(time.Now()) <= MaxAge {
		t.Fatal("unparsable date must count as stale")
	}
	if _, err := LoadTable(t.TempDir(), "us-east-1"); err == nil {
		t.Fatal("missing table")
	}
	noSpot := &Table{Instances: map[string]Price{"a": {OnDemand: 2}}}
	if p, _ := noSpot.Hourly("a", "spot"); p != 2 {
		t.Fatal("spot falls back to on-demand")
	}
	if p, _ := noSpot.Hourly("a", "capacity-block"); p != 2 {
		t.Fatal("capacity block falls back to on-demand")
	}
}

func TestEstimateRun(t *testing.T) {
	root := filepath.Join("..", "..")
	t.Setenv(profile.EnvProfilesDir, filepath.Join(root, "profiles"))
	p, err := profile.Load("aws-dev")
	if err != nil {
		t.Fatal(err)
	}
	e := EstimateRun(p, table(t), 4*time.Hour, time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC))
	// 2 × 0.2016 + 2 × 0.39 (g6.2xlarge spot) + 0.10 + 0.045 = 1.3282 → 1.33/h × 4 h = 5.32
	if e.PerHour != 1.33 || e.Total != 5.32 || e.StaleTable || e.Verified {
		t.Fatalf("%+v", e)
	}
	if !strings.Contains(e.Summary(), "USD 5.32") {
		t.Fatal(e.Summary())
	}
	p.Spec.AWS.GPUClasses[0].InstanceTypes = append(p.Spec.AWS.GPUClasses[0].InstanceTypes, "zz.unknown")
	e = EstimateRun(p, &Table{Region: "us-east-1", LastUpdated: "2026-11-05", Instances: map[string]Price{}}, time.Hour, time.Now())
	if len(e.Unknown) == 0 || !strings.Contains(e.Summary(), "unknown prices") {
		t.Fatalf("%+v", e)
	}
}
