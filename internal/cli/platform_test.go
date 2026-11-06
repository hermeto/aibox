package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	"github.com/hermeto/aibox/internal/cost"
	"github.com/hermeto/aibox/internal/doctor"
	"github.com/hermeto/aibox/internal/exec"
	"github.com/hermeto/aibox/internal/kube"
	"github.com/hermeto/aibox/internal/models"
	"github.com/hermeto/aibox/internal/platform"
)

type fakeKube struct{}

func (fakeKube) ServerVersion() (string, error) { return "v1.37.0", nil }
func (fakeKube) Apply(context.Context, schema.GroupVersionResource, *unstructured.Unstructured) error {
	return nil
}
func (fakeKube) ListApplications(context.Context, string) ([]kube.AppStatus, error) {
	var apps []kube.AppStatus
	for _, n := range []string{platform.RootAppName, "serving", "karpenter", "gpu", "secrets", "models", "cert-manager", "kserve"} {
		apps = append(apps, kube.AppStatus{Name: n, Sync: "Synced", Health: "Healthy"})
	}
	return apps, nil
}
func (fakeKube) NotReadyWorkloads(context.Context, string) ([]string, int, error) { return nil, 1, nil }
func (fakeKube) PodSummary(context.Context, string) ([]string, error)             { return nil, nil }

func platformRunner() *exec.Fake {
	return fakeRunner("17179869184").
		On("git remote get-url origin", exec.Result{Stdout: "https://github.com/hermeto/aibox.git"}, nil).
		On("git rev-parse --abbrev-ref HEAD", exec.Result{Stdout: "main"}, nil).
		On("git rev-list --count @{u}..HEAD", exec.Result{Stdout: "0"}, nil).
		On("kind get clusters", exec.Result{Stdout: "aibox-local\n"}, nil).
		OnPrefix("helm upgrade --install", exec.Result{}, nil).
		On("kind delete cluster --name aibox-local", exec.Result{}, nil)
}

func kubeFactory(string) (platform.KubeAPI, error) { return fakeKube{}, nil }

func TestUpAndDownLocal(t *testing.T) {
	setProfiles(t)
	r := runCLI(t, &App{Runner: platformRunner(), Kube: kubeFactory}, "up", "--profile", "local", "-o", "json")
	if r.code != 0 {
		t.Fatalf("up exit %d: %s", r.code, r.stderr)
	}
	var res platform.UpResult
	if err := json.Unmarshal([]byte(r.stdout), &res); err != nil || res.Mode != "git" || len(res.Endpoints) != 2 {
		t.Fatalf("%q %v", r.stdout, err)
	}
	r = runCLI(t, &App{Runner: platformRunner(), Kube: kubeFactory}, "down")
	if r.code != 0 || !strings.Contains(r.stdout, "deleted") {
		t.Fatalf("down: %d %q %q", r.code, r.stdout, r.stderr)
	}
}

func TestUpDoctorFailureStops(t *testing.T) {
	setProfiles(t)
	r := runCLI(t, &App{Runner: fakeRunner("1073741824"), Kube: kubeFactory}, "up")
	if r.code != ExitFailure || !strings.Contains(r.stderr, "preflight checks failed") || !strings.Contains(r.stderr, "docker memory") {
		t.Fatalf("%d %q", r.code, r.stderr)
	}
}

func TestSmokeCloudNotImplementedYet(t *testing.T) {
	setProfiles(t)
	r := runCLI(t, &App{}, "smoke", "--profile", "aws-dev")
	if r.code != ExitNotImplemented || !strings.Contains(r.stderr, "aws-dev") {
		t.Fatalf("%d %q", r.code, r.stderr)
	}
}

type okAWS struct{}

func (okAWS) CallerIdentity(context.Context) (string, string, error) {
	return "000000000000", "arn:aws:iam::000000000000:user/test", nil
}
func (okAWS) InstanceTypesOffered(_ context.Context, types []string) (map[string]bool, error) {
	m := map[string]bool{}
	for _, t := range types {
		m[t] = true
	}
	return m, nil
}
func (okAWS) InstanceVCPUs(_ context.Context, types []string) (map[string]int, error) {
	m := map[string]int{}
	for _, t := range types {
		m[t] = 4
	}
	return m, nil
}
func (okAWS) EC2Quotas(context.Context) (map[string]float64, error) {
	return map[string]float64{"All G and VT Spot Instance Requests": 64, "Running On-Demand G and VT instances": 64}, nil
}

func awsApp(t *testing.T) *App {
	t.Helper()
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	tf := "tofu -chdir=" + filepath.Join(root, platform.TerraformDir)
	g := "tofu -chdir=" + filepath.Join(root, platform.GuardrailsDir)
	r := exec.NewFake().WithPath("tofu").WithPath("kubectl").WithPath("helm").
		OnPrefix(g+" init", exec.Result{}, nil).
		OnPrefix(g+" plan", exec.Result{}, nil).
		OnPrefix(g+" apply", exec.Result{}, nil).
		OnPrefix(g+" destroy", exec.Result{}, nil).
		OnPrefix("kubectl --kubeconfig", exec.Result{}, nil).
		On("tofu version", exec.Result{Stdout: "OpenTofu v1.13.8"}, nil).
		On("kubectl version --client", exec.Result{Stdout: "Client Version: v1.37.2"}, nil).
		On("helm version --short", exec.Result{Stdout: "v4.3.1"}, nil).
		On("git remote get-url origin", exec.Result{Stdout: "https://github.com/hermeto/aibox.git"}, nil).
		On("git rev-parse --abbrev-ref HEAD", exec.Result{Stdout: "main"}, nil).
		On("git rev-list --count @{u}..HEAD", exec.Result{Stdout: "0"}, nil).
		On("git config user.email", exec.Result{Stdout: "dev@example.com"}, nil).
		OnPrefix(tf+" init", exec.Result{}, nil).
		OnPrefix(tf+" plan", exec.Result{}, nil).
		OnPrefix(tf+" apply", exec.Result{}, nil).
		OnPrefix(tf+" destroy", exec.Result{}, nil).
		On(tf+" output -json", exec.Result{Stdout: `{"cluster_name":{"value":"aibox-dev"},"cluster_endpoint":{"value":"https://example.invalid"},"cluster_certificate_authority_data":{"value":"Y2E="},"region":{"value":"us-east-1"},"weights_bucket":{"value":"aibox-weights-aibox-dev-000000000000"}}`}, nil).
		OnPrefix("helm upgrade --install argocd", exec.Result{}, nil)
	stateDir := t.TempDir()
	return &App{
		Runner:   r,
		AWS:      func(context.Context, string) (doctor.AWSClient, error) { return okAWS{}, nil },
		Account:  func(context.Context, string) (string, error) { return "000000000000", nil },
		EgressIP: func(context.Context) (string, error) { return "203.0.113.10", nil },
		KubeFile: func(string, string) (platform.KubeAPI, error) { return fakeKube{}, nil },
		Jobs: func(string, string) (models.JobAPI, error) {
			return &fakeJobs{jobs: map[string]kube.JobState{}}, nil
		},
		CostClients: func(context.Context, string) (cost.EC2API, cost.ELBAPI, error) {
			return noResources{}, noResources{}, nil
		},
		Getenv: func(k string) string {
			if k == "AIBOX_STATE_DIR" {
				return stateDir
			}
			return ""
		},
	}
}

func TestUpAndDownAWSWithFakes(t *testing.T) {
	setProfiles(t)
	app := awsApp(t)
	r := runCLI(t, app, "up", "--profile", "aws-dev", "--yes")
	if r.code != 1 || !strings.Contains(r.stderr, "budget is required") {
		t.Fatalf("up without budget: %d %s", r.code, r.stderr)
	}
	r = runCLI(t, app, "up", "--profile", "aws-dev", "--yes", "--budget", "40")
	if r.code != 1 || !strings.Contains(r.stderr, "alert e-mail is required") {
		t.Fatalf("up without e-mail: %d %s", r.code, r.stderr)
	}
	r = runCLI(t, app, "up", "--profile", "aws-dev", "--yes", "--budget", "40", "--notify-email", "ops@example.com", "-o", "json")
	if r.code != 0 {
		t.Fatalf("up: %d %s", r.code, r.stderr)
	}
	if !strings.Contains(r.stderr, "cost estimate") && !strings.Contains(r.stdout, "budgetUSD") {
		t.Fatalf("missing budget in output: %s", r.stdout)
	}
	var res platform.AWSUpResult
	if err := json.Unmarshal([]byte(r.stdout), &res); err != nil || !strings.HasPrefix(res.RunID, "aws-dev-") {
		t.Fatalf("%q %v", r.stdout, err)
	}
	// Weights are mirrored before the model servers are deployed.
	if !strings.Contains(r.stderr, "Mirroring 1 model(s) to s3://aibox-weights-aibox-dev-000000000000") || !strings.Contains(r.stderr, "Weights mirrored; deploying the model servers") {
		t.Fatalf("model sync before serving missing:\n%s", r.stderr)
	}
	app.In = strings.NewReader("no\n")
	r = runCLI(t, app, "down", "--profile", "aws-dev")
	if r.code != ExitFailure || !strings.Contains(r.stderr, "not confirmed") {
		t.Fatalf("declined down: %d %q", r.code, r.stderr)
	}
	r = runCLI(t, app, "ttl", "show", "--profile", "aws-dev")
	if r.code != 0 || !strings.Contains(r.stdout, "teardown at") {
		t.Fatalf("ttl show: %d %q %q", r.code, r.stdout, r.stderr)
	}
	r = runCLI(t, app, "ttl", "extend", "2h", "--profile", "aws-dev", "-o", "json")
	if r.code != 0 || !strings.Contains(r.stdout, "expiresAt") {
		t.Fatalf("ttl extend: %d %q %q", r.code, r.stdout, r.stderr)
	}
	r = runCLI(t, app, "ttl", "extend", "soon", "--profile", "aws-dev")
	if r.code != ExitUsage {
		t.Fatalf("bad duration: %d", r.code)
	}
	r = runCLI(t, app, "cost", "--profile", "aws-dev")
	if r.code != 0 || !strings.Contains(r.stdout, "Total: USD") {
		t.Fatalf("cost: %d %q %q", r.code, r.stdout, r.stderr)
	}
	r = runCLI(t, app, "down", "--profile", "aws-dev", "--orphans-only")
	if r.code != 0 || !strings.Contains(r.stdout, "0 orphan(s)") {
		t.Fatalf("orphans-only: %d %q %q", r.code, r.stdout, r.stderr)
	}
	app.In = strings.NewReader("yes\n")
	r = runCLI(t, app, "down", "--profile", "aws-dev")
	if r.code != 0 || !strings.Contains(r.stdout, "destroyed") || !strings.Contains(r.stdout, "0 orphan(s)") {
		t.Fatalf("down: %d %q %q", r.code, r.stdout, r.stderr)
	}
	r = runCLI(t, app, "cost", "--profile", "aws-dev")
	if r.code != 1 || !strings.Contains(r.stderr, "no active run") {
		t.Fatalf("cost without run: %d %q", r.code, r.stderr)
	}
	r = runCLI(t, app, "ttl", "show", "--profile", "local")
	if r.code != 1 || !strings.Contains(r.stderr, "cloud profiles only") {
		t.Fatalf("ttl local: %d %q", r.code, r.stderr)
	}
}

func TestUpAWSStopsOnSkippedDoctor(t *testing.T) {
	setProfiles(t)
	app := awsApp(t)
	app.AWS = func(context.Context, string) (doctor.AWSClient, error) { return nil, doctor.ErrNoCredentials }
	r := runCLI(t, app, "up", "--profile", "aws-dev", "--yes", "--budget", "10")
	if r.code != ExitFailure || !strings.Contains(r.stderr, "no credentials") {
		t.Fatalf("%d %q", r.code, r.stderr)
	}
}

type noResources struct{ cost.EC2API }

func (noResources) DescribeInstances(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{}, nil
}
func (noResources) DescribeVolumes(context.Context, *ec2.DescribeVolumesInput, ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	return &ec2.DescribeVolumesOutput{}, nil
}
func (noResources) DescribeSnapshots(context.Context, *ec2.DescribeSnapshotsInput, ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	return &ec2.DescribeSnapshotsOutput{}, nil
}
func (noResources) DescribeNetworkInterfaces(context.Context, *ec2.DescribeNetworkInterfacesInput, ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	return &ec2.DescribeNetworkInterfacesOutput{}, nil
}
func (noResources) DescribeAddresses(context.Context, *ec2.DescribeAddressesInput, ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
	return &ec2.DescribeAddressesOutput{}, nil
}
func (noResources) DescribeNatGateways(context.Context, *ec2.DescribeNatGatewaysInput, ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
	return &ec2.DescribeNatGatewaysOutput{}, nil
}
func (noResources) DescribeLoadBalancers(context.Context, *elbv2.DescribeLoadBalancersInput, ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancersOutput, error) {
	return &elbv2.DescribeLoadBalancersOutput{}, nil
}
func (noResources) DescribeTags(context.Context, *elbv2.DescribeTagsInput, ...func(*elbv2.Options)) (*elbv2.DescribeTagsOutput, error) {
	return &elbv2.DescribeTagsOutput{}, nil
}

func TestBudgetFromEnvironment(t *testing.T) {
	app := &App{Getenv: func(k string) string {
		if k == "BUDGET_USD" {
			return "not-a-number"
		}
		return ""
	}}
	if _, err := app.budget(0); err == nil {
		t.Fatal("invalid BUDGET_USD must fail")
	}
	app.Getenv = func(string) string { return "25" }
	if b, err := app.budget(0); err != nil || b != 25 {
		t.Fatalf("%v %v", b, err)
	}
	if b, _ := app.budget(5); b != 5 {
		t.Fatal("flag wins")
	}
}

func TestConfirmWithoutInput(t *testing.T) {
	app := &App{In: strings.NewReader(""), Err: io.Discard}
	if ok, err := app.confirm("go?"); err == nil || ok {
		t.Fatalf("%v %v", ok, err)
	}
}

func TestSmokeAgainstFakeServer(t *testing.T) {
	setProfiles(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":[{"id":"sim-llama-8b"}]}`)
	})
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"choices":[{"index":0}],"usage":{"completion_tokens":2}}`)
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "vllm:num_requests_running 0\nvllm:num_requests_waiting 0\nvllm:kv_cache_usage_perc 0\nvllm:time_to_first_token_seconds_bucket{le=\"1\"} 1\n")
	})
	s := httptest.NewServer(mux)
	defer s.Close()
	r := runCLI(t, &App{}, "smoke", "--endpoint", s.URL+"/v1", "-o", "json")
	if r.code != 0 {
		t.Fatalf("%d %s %s", r.code, r.stdout, r.stderr)
	}
	r = runCLI(t, &App{}, "smoke", "--endpoint", s.URL+"/v1", "--expect-model", "missing")
	if r.code != ExitFailure || !strings.Contains(r.stdout, "FAILED") {
		t.Fatalf("%d %s", r.code, r.stdout)
	}
}
