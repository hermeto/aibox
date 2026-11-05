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

	"github.com/hermeto/aibox/internal/doctor"
	"github.com/hermeto/aibox/internal/exec"
	"github.com/hermeto/aibox/internal/kube"
	"github.com/hermeto/aibox/internal/platform"
)

type fakeKube struct{}

func (fakeKube) ServerVersion() (string, error) { return "v1.37.0", nil }
func (fakeKube) Apply(context.Context, schema.GroupVersionResource, *unstructured.Unstructured) error {
	return nil
}
func (fakeKube) ListApplications(context.Context, string) ([]kube.AppStatus, error) {
	return []kube.AppStatus{{Name: platform.RootAppName, Sync: "Synced", Health: "Healthy"}, {Name: "serving", Sync: "Synced", Health: "Healthy"}}, nil
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
	r := exec.NewFake().WithPath("tofu").WithPath("kubectl").WithPath("helm").
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
		On(tf+" output -json", exec.Result{Stdout: `{"cluster_name":{"value":"aibox-dev"},"cluster_endpoint":{"value":"https://example.invalid"},"cluster_certificate_authority_data":{"value":"Y2E="},"region":{"value":"us-east-1"}}`}, nil).
		OnPrefix("helm upgrade --install argocd", exec.Result{}, nil)
	stateDir := t.TempDir()
	return &App{
		Runner:   r,
		AWS:      func(context.Context, string) (doctor.AWSClient, error) { return okAWS{}, nil },
		Account:  func(context.Context, string) (string, error) { return "000000000000", nil },
		EgressIP: func(context.Context) (string, error) { return "203.0.113.10", nil },
		KubeFile: func(string, string) (platform.KubeAPI, error) { return fakeKube{}, nil },
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
	r := runCLI(t, app, "up", "--profile", "aws-dev", "--yes", "-o", "json")
	if r.code != 0 {
		t.Fatalf("up: %d %s", r.code, r.stderr)
	}
	var res platform.AWSUpResult
	if err := json.Unmarshal([]byte(r.stdout), &res); err != nil || !strings.HasPrefix(res.RunID, "aws-dev-") {
		t.Fatalf("%q %v", r.stdout, err)
	}
	app.In = strings.NewReader("no\n")
	r = runCLI(t, app, "down", "--profile", "aws-dev")
	if r.code != ExitFailure || !strings.Contains(r.stderr, "not confirmed") {
		t.Fatalf("declined down: %d %q", r.code, r.stderr)
	}
	app.In = strings.NewReader("yes\n")
	r = runCLI(t, app, "down", "--profile", "aws-dev")
	if r.code != 0 || !strings.Contains(r.stdout, "deleted") {
		t.Fatalf("down: %d %q %q", r.code, r.stdout, r.stderr)
	}
}

func TestUpAWSStopsOnSkippedDoctor(t *testing.T) {
	setProfiles(t)
	app := awsApp(t)
	app.AWS = func(context.Context, string) (doctor.AWSClient, error) { return nil, doctor.ErrNoCredentials }
	r := runCLI(t, app, "up", "--profile", "aws-dev", "--yes")
	if r.code != ExitFailure || !strings.Contains(r.stderr, "no credentials") {
		t.Fatalf("%d %q", r.code, r.stderr)
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
