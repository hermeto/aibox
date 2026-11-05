package doctor

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hermeto/aibox/internal/profile"
)

// ErrNoCredentials signals that the AWS credential chain resolved nothing.
var ErrNoCredentials = errors.New("no AWS credentials")

// AWSClient is the read-only subset of AWS used by doctor.
type AWSClient interface {
	// CallerIdentity calls sts:GetCallerIdentity.
	CallerIdentity(ctx context.Context) (account, arn string, err error)
	// InstanceTypesOffered reports, per instance type, whether the region offers it.
	InstanceTypesOffered(ctx context.Context, types []string) (map[string]bool, error)
	// InstanceVCPUs returns the default vCPU count per instance type.
	InstanceVCPUs(ctx context.Context, types []string) (map[string]int, error)
	// EC2Quotas returns applied EC2 service quota values keyed by quota name.
	EC2Quotas(ctx context.Context) (map[string]float64, error)
}

// AWSFactory builds a client for a region; it returns ErrNoCredentials (wrapped) when no
// credentials can be resolved, without calling any AWS API.
type AWSFactory func(ctx context.Context, region string) (AWSClient, error)

const quotaConsole = "https://console.aws.amazon.com/servicequotas/home/services/ec2/quotas"

func awsChecks(ctx context.Context, env Env, cfg *profile.AWS) []Check {
	if cfg == nil {
		return nil
	}
	region := Check{ID: "aws.region", Name: "AWS region", Status: Pass, Message: cfg.Region}
	if envRegion := firstNonEmpty(env.getenv("AWS_REGION"), env.getenv("AWS_DEFAULT_REGION")); envRegion != "" && envRegion != cfg.Region {
		region.Status = Warn
		region.Message = fmt.Sprintf("profile region %s differs from AWS_REGION=%s; the profile wins", cfg.Region, envRegion)
		region.Remediation = "unset AWS_REGION or align it with the profile to avoid confusion"
	}

	skipAll := func(reason string) []Check {
		ids := [][2]string{{"aws.credentials", "AWS credentials"}, {"aws.instance-types", "GPU instance types offered"}, {"aws.quotas", "GPU vCPU quotas"}}
		out := []Check{region}
		for _, id := range ids {
			out = append(out, Check{ID: id[0], Name: id[1], Status: Skip, Message: "skipped: " + reason})
		}
		return out
	}
	if env.AWS == nil {
		return skipAll("AWS checks disabled")
	}
	client, err := env.AWS(ctx, cfg.Region)
	if errors.Is(err, ErrNoCredentials) {
		return skipAll("no credentials")
	}
	creds := Check{ID: "aws.credentials", Name: "AWS credentials"}
	if err == nil {
		var account, arn string
		account, arn, err = client.CallerIdentity(ctx)
		if err == nil {
			creds.Status = Pass
			creds.Message = "authenticated as " + arn
			creds.Details = map[string]string{"account": account}
		}
	}
	if err != nil {
		creds.Status = Fail
		creds.Message = "credentials present but not usable: " + err.Error()
		creds.Remediation = "refresh credentials (e.g. `aws sso login`) and re-run"
		out := []Check{region, creds}
		out = append(out, Check{ID: "aws.instance-types", Name: "GPU instance types offered", Status: Skip, Message: "skipped: credentials not usable"},
			Check{ID: "aws.quotas", Name: "GPU vCPU quotas", Status: Skip, Message: "skipped: credentials not usable"})
		return out
	}
	out := []Check{region, creds, offeringCheck(ctx, client, cfg)}
	return append(out, quotaChecks(ctx, client, cfg)...)
}

func allInstanceTypes(cfg *profile.AWS) []string {
	set := map[string]bool{}
	for _, c := range cfg.GPUClasses {
		for _, t := range c.InstanceTypes {
			set[t] = true
		}
	}
	types := make([]string, 0, len(set))
	for t := range set {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}

func offeringCheck(ctx context.Context, client AWSClient, cfg *profile.AWS) Check {
	c := Check{ID: "aws.instance-types", Name: "GPU instance types offered"}
	types := allInstanceTypes(cfg)
	offered, err := client.InstanceTypesOffered(ctx, types)
	if err != nil {
		c.Status = Fail
		c.Message = "could not list instance type offerings: " + err.Error()
		c.Remediation = "grant ec2:DescribeInstanceTypeOfferings (read-only) to your identity"
		return c
	}
	var missing []string
	for _, t := range types {
		if !offered[t] {
			missing = append(missing, t)
		}
	}
	if len(missing) > 0 {
		c.Status = Fail
		c.Message = fmt.Sprintf("not offered in %s: %s", cfg.Region, strings.Join(missing, ", "))
		c.Remediation = "choose a region that offers these instance types (see docs/runbooks/aws-first-run.md) or change the profile"
		return c
	}
	c.Status = Pass
	c.Message = fmt.Sprintf("%s offered in %s", strings.Join(types, ", "), cfg.Region)
	return c
}

// quotaKey identifies an EC2 vCPU quota.
type quotaKey struct{ family, capacity string }

func (k quotaKey) name() string {
	fam := "G and VT"
	if k.family == "p" {
		fam = "P"
	}
	if k.capacity == profile.CapacitySpot {
		return fmt.Sprintf("All %s Spot Instance Requests", fam)
	}
	return fmt.Sprintf("Running On-Demand %s instances", fam)
}

func (k quotaKey) id() string { return "aws.quota." + k.family + "-" + k.capacity }

func family(instanceType string) string {
	if strings.HasPrefix(instanceType, "p") {
		return "p"
	}
	return "g"
}

func quotaChecks(ctx context.Context, client AWSClient, cfg *profile.AWS) []Check {
	vcpus, err := client.InstanceVCPUs(ctx, allInstanceTypes(cfg))
	if err == nil {
		var quotas map[string]float64
		if quotas, err = client.EC2Quotas(ctx); err == nil {
			return evaluateQuotas(cfg, vcpus, quotas)
		}
	}
	return []Check{{ID: "aws.quotas", Name: "GPU vCPU quotas", Status: Fail,
		Message:     "could not read quotas: " + err.Error(),
		Remediation: "grant servicequotas:ListServiceQuotas and ec2:DescribeInstanceTypes (read-only)"}}
}

func evaluateQuotas(cfg *profile.AWS, vcpus map[string]int, quotas map[string]float64) []Check {
	required := map[quotaKey]int{}
	var out []Check
	for _, c := range cfg.GPUClasses {
		if c.CapacityType == profile.CapacityCapacityBlock {
			out = append(out, Check{ID: "aws.quota." + c.Name, Name: "GPU quota " + c.Name, Status: Pass,
				Message: "uses EC2 Capacity Block " + c.CapacityReservationID + " (reserved capacity, no vCPU quota needed)"})
			continue
		}
		largest := 0
		for _, t := range c.InstanceTypes {
			if vcpus[t] > largest {
				largest = vcpus[t]
			}
		}
		required[quotaKey{family(c.InstanceTypes[0]), c.CapacityType}] += largest * c.MaxNodes
	}
	keys := make([]quotaKey, 0, len(required))
	for k := range required {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].id() < keys[j].id() })
	for _, k := range keys {
		need := required[k]
		have, ok := quotas[k.name()]
		c := Check{ID: k.id(), Name: "GPU quota " + k.name(), Details: map[string]string{
			"quota": k.name(), "required": strconv.Itoa(need), "current": strconv.FormatFloat(have, 'f', 0, 64),
		}}
		switch {
		case !ok:
			c.Status = Fail
			c.Message = "quota not found: " + k.name()
			c.Remediation = "check the quota in " + quotaConsole
		case have < float64(need):
			c.Status = Fail
			c.Message = fmt.Sprintf("%s is %.0f vCPUs, profile needs %d", k.name(), have, need)
			c.Remediation = fmt.Sprintf("request an increase to at least %d vCPUs at %s (approval can take days)", need, quotaConsole)
		default:
			c.Status = Pass
			c.Message = fmt.Sprintf("%.0f vCPUs available, %d needed", have, need)
		}
		out = append(out, c)
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
