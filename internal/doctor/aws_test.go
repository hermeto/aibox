package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hermeto/aibox/internal/exec"
)

type fakeAWS struct {
	identityErr error
	offered     map[string]bool
	offerErr    error
	vcpus       map[string]int
	quotas      map[string]float64
	quotaErr    error
}

func (f *fakeAWS) CallerIdentity(context.Context) (string, string, error) {
	if f.identityErr != nil {
		return "", "", f.identityErr
	}
	return "000000000000", "arn:aws:iam::000000000000:user/test", nil
}

func (f *fakeAWS) InstanceTypesOffered(_ context.Context, types []string) (map[string]bool, error) {
	if f.offerErr != nil {
		return nil, f.offerErr
	}
	out := map[string]bool{}
	for _, t := range types {
		out[t] = f.offered[t]
	}
	return out, nil
}

func (f *fakeAWS) InstanceVCPUs(context.Context, []string) (map[string]int, error) {
	return f.vcpus, nil
}

func (f *fakeAWS) EC2Quotas(context.Context) (map[string]float64, error) { return f.quotas, f.quotaErr }

func healthyAWS() *fakeAWS {
	return &fakeAWS{
		offered: map[string]bool{"g6.xlarge": true, "g6.2xlarge": true, "g6.12xlarge": true, "g5.xlarge": true, "g5.12xlarge": true, "p5.48xlarge": true},
		vcpus:   map[string]int{"g6.xlarge": 4, "g6.2xlarge": 8, "g6.12xlarge": 48, "g5.xlarge": 4, "g5.12xlarge": 48, "p5.48xlarge": 192},
		quotas: map[string]float64{
			"All G and VT Spot Instance Requests":  64,
			"Running On-Demand G and VT instances": 192,
			"All P Spot Instance Requests":         0,
			"Running On-Demand P instances":        0,
		},
	}
}

func fakeAWSTools() *exec.Fake {
	return exec.NewFake().WithPath("tofu").WithPath("kubectl").WithPath("helm").
		On("tofu version", exec.Result{Stdout: "OpenTofu v1.10.6"}, nil).
		On("kubectl version --client", exec.Result{Stdout: "Client Version: v1.35.0"}, nil).
		On("helm version --short", exec.Result{Stdout: "v3.19.0"}, nil)
}

func factory(c AWSClient, err error) AWSFactory {
	return func(context.Context, string) (AWSClient, error) { return c, err }
}

func TestAWSNoCredentialsSkipsGolden(t *testing.T) {
	p := loadProfile(t, "aws-dev")
	r := Run(context.Background(), p, Env{Runner: fakeAWSTools(), AWS: factory(nil, fmt.Errorf("resolve: %w", ErrNoCredentials)), Getenv: func(string) string { return "" }})
	if !r.OK || r.Summary.Skip != 3 {
		t.Fatalf("want 3 skips and OK, got %+v", r.Summary)
	}
	for _, c := range r.Checks {
		if c.Status == Skip && !strings.Contains(c.Message, "no credentials") {
			t.Fatalf("skip message %q", c.Message)
		}
	}
	js, _ := json.MarshalIndent(r, "", "  ")
	golden(t, "aws-no-credentials.json", append(js, '\n'))
}

func TestAWSHealthy(t *testing.T) {
	p := loadProfile(t, "aws-bench")
	r := Run(context.Background(), p, Env{Runner: fakeAWSTools(), AWS: factory(healthyAWS(), nil), Getenv: func(string) string { return "" }})
	s := statuses(r)
	if !r.OK {
		t.Fatalf("want OK, got %v", s)
	}
	// L4 (2×48) + A10G (2×48) on-demand = 192 vCPUs; H100 uses a Capacity Block.
	if s["aws.quota.g-on-demand"] != Pass || s["aws.quota.gpu-h100"] != Pass || s["aws.instance-types"] != Pass {
		t.Fatalf("unexpected %v", s)
	}
}

func TestAWSQuotaTooLowAndTypeNotOffered(t *testing.T) {
	p := loadProfile(t, "aws-dev")
	c := healthyAWS()
	c.quotas["All G and VT Spot Instance Requests"] = 8 // needs 2 × 8
	c.offered["g6.2xlarge"] = false
	r := Run(context.Background(), p, Env{Runner: fakeAWSTools(), AWS: factory(c, nil), Getenv: func(k string) string {
		if k == "AWS_REGION" {
			return "eu-west-1"
		}
		return ""
	}})
	s := statuses(r)
	if r.OK || s["aws.quota.g-spot"] != Fail || s["aws.instance-types"] != Fail || s["aws.region"] != Warn {
		t.Fatalf("unexpected %v", s)
	}
	for _, ch := range r.Checks {
		if ch.ID == "aws.quota.g-spot" {
			if ch.Details["required"] != "16" || ch.Details["current"] != "8" || !strings.Contains(ch.Remediation, "servicequotas") {
				t.Fatalf("quota details %+v", ch)
			}
		}
		if ch.ID == "aws.instance-types" && !strings.Contains(ch.Message, "g6.2xlarge") {
			t.Fatalf("offering message %q", ch.Message)
		}
	}
}

func TestAWSErrors(t *testing.T) {
	p := loadProfile(t, "aws-dev")
	env := func(c AWSClient, err error) Env {
		return Env{Runner: fakeAWSTools(), AWS: factory(c, err), Getenv: func(string) string { return "" }}
	}
	s := statuses(Run(context.Background(), p, env(&fakeAWS{identityErr: errors.New("expired")}, nil)))
	if s["aws.credentials"] != Fail || s["aws.quotas"] != Skip {
		t.Fatalf("identity error: %v", s)
	}
	s = statuses(Run(context.Background(), p, env(nil, errors.New("bad config"))))
	if s["aws.credentials"] != Fail {
		t.Fatalf("factory error: %v", s)
	}
	c := healthyAWS()
	c.offerErr = errors.New("denied")
	c.quotaErr = errors.New("denied")
	s = statuses(Run(context.Background(), p, env(c, nil)))
	if s["aws.instance-types"] != Fail || s["aws.quotas"] != Fail {
		t.Fatalf("api errors: %v", s)
	}
	c = healthyAWS()
	delete(c.quotas, "All G and VT Spot Instance Requests")
	s = statuses(Run(context.Background(), p, env(c, nil)))
	if s["aws.quota.g-spot"] != Fail {
		t.Fatalf("missing quota: %v", s)
	}
	s = statuses(Run(context.Background(), p, Env{Runner: fakeAWSTools()}))
	if s["aws.credentials"] != Skip {
		t.Fatalf("disabled: %v", s)
	}
}

func TestQuotaNames(t *testing.T) {
	cases := map[quotaKey]string{
		{"g", "spot"}:      "All G and VT Spot Instance Requests",
		{"g", "on-demand"}: "Running On-Demand G and VT instances",
		{"p", "spot"}:      "All P Spot Instance Requests",
		{"p", "on-demand"}: "Running On-Demand P instances",
	}
	for k, want := range cases {
		if k.name() != want {
			t.Fatalf("%v → %q", k, k.name())
		}
	}
	if family("p5.48xlarge") != "p" || family("g6.xlarge") != "g" {
		t.Fatal("family")
	}
	if firstNonEmpty("", "", "x") != "x" || firstNonEmpty() != "" {
		t.Fatal("firstNonEmpty")
	}
	if (Env{}).getenv("PATH") == "" {
		t.Fatal("getenv default")
	}
}
