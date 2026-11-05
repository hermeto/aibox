package platform

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

	"github.com/hermeto/aibox/internal/exec"
	"github.com/hermeto/aibox/internal/profile"
)

const outputsJSON = `{"cluster_name":{"value":"aibox-dev"},"cluster_endpoint":{"value":"https://example.invalid"},"cluster_certificate_authority_data":{"value":"Y2E="},"region":{"value":"us-east-1"}}`

func awsProfile(t *testing.T, name string) *profile.Profile {
	t.Helper()
	t.Setenv(profile.EnvProfilesDir, filepath.Join(repoRoot(t), "profiles"))
	p, err := profile.Load(name)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func tfPrefix(root string) string { return "tofu -chdir=" + filepath.Join(root, TerraformDir) }

func awsRunner(root string) *exec.Fake {
	tf := tfPrefix(root)
	return exec.NewFake().WithPath("tofu").
		On("git remote get-url origin", exec.Result{Stdout: "https://github.com/hermeto/aibox.git"}, nil).
		On("git rev-parse --abbrev-ref HEAD", exec.Result{Stdout: "main"}, nil).
		On("git rev-list --count @{u}..HEAD", exec.Result{Stdout: "0"}, nil).
		On("git config user.email", exec.Result{Stdout: "dev@example.com\n"}, nil).
		OnPrefix(tf+" init", exec.Result{}, nil).
		OnPrefix(tf+" plan", exec.Result{}, nil).
		OnPrefix(tf+" apply", exec.Result{}, nil).
		On(tf+" output -json", exec.Result{Stdout: outputsJSON}, nil).
		OnPrefix(tf+" destroy", exec.Result{}, nil).
		OnPrefix("helm upgrade --install argocd", exec.Result{}, nil)
}

func newAWS(t *testing.T, r exec.Runner) (*AWS, *bytes.Buffer) {
	t.Helper()
	var log bytes.Buffer
	k := &fakeKube{apps: readyApps()}
	return &AWS{
		Profile: awsProfile(t, "aws-dev"), Runner: r, Root: repoRoot(t), StateDir: t.TempDir(), Log: &log, Yes: true,
		Account:      func(context.Context) (string, error) { return "000000000000", nil },
		EgressIP:     func(context.Context) (string, error) { return "203.0.113.10\n", nil },
		Kube:         func(string, string) (KubeAPI, error) { return k, nil },
		Getenv:       func(string) string { return "" },
		Now:          func() time.Time { return time.Date(2026, 11, 5, 1, 2, 0, 0, time.UTC) },
		PollInterval: time.Millisecond,
	}, &log
}

func commandIndex(lines []string, prefix string) int {
	for i, l := range lines {
		if strings.HasPrefix(l, prefix) {
			return i
		}
	}
	return -1
}

func TestUpAWSRunsTerraformSequence(t *testing.T) {
	r := awsRunner(repoRoot(t))
	a, log := newAWS(t, r)
	res, err := a.Up(context.Background(), AWSUpOptions{Timeout: time.Second})
	if err != nil {
		t.Fatalf("Up: %v\n%s", err, log)
	}
	if !strings.HasPrefix(res.RunID, "aws-dev-202611050102-") || res.Kubeconfig == "" {
		t.Fatalf("result %+v", res)
	}
	lines := r.CommandLines()
	tf := tfPrefix(repoRoot(t))
	order := []string{tf + " init", tf + " plan", tf + " apply", tf + " output -json", "helm upgrade --install argocd"}
	last := -1
	for _, p := range order {
		i := commandIndex(lines, p)
		if i <= last {
			t.Fatalf("%q out of order in %v", p, lines)
		}
		last = i
	}
	init := lines[commandIndex(lines, tf+" init")]
	for _, want := range []string{"-backend-config=bucket=aibox-tfstate-000000000000-us-east-1", "-backend-config=key=aws-dev/aibox-dev.tfstate", "-backend-config=region=us-east-1"} {
		if !strings.Contains(init, want) {
			t.Fatalf("init %q lacks %q", init, want)
		}
	}
	vars := map[string]any{}
	data, _ := os.ReadFile(filepath.Join(a.StateDir, "terraform.tfvars.json"))
	if err := json.Unmarshal(data, &vars); err != nil {
		t.Fatal(err)
	}
	if vars["owner"] != "dev@example.com" || vars["kubernetes_version"] != "1.35" || vars["admin_cidrs"].([]any)[0] != "203.0.113.10/32" {
		t.Fatalf("tfvars %v", vars)
	}
	kc, _ := os.ReadFile(res.Kubeconfig)
	if !strings.Contains(string(kc), "- --cluster-name\n") || !strings.Contains(string(kc), "- aibox-dev\n") || !strings.Contains(string(kc), "current-context: aibox-aws-dev") {
		t.Fatalf("kubeconfig:\n%s", kc)
	}
	helm := lines[commandIndex(lines, "helm upgrade")]
	if !strings.Contains(helm, "--kubeconfig "+res.Kubeconfig) || !strings.Contains(helm, "values-aws-dev.yaml") {
		t.Fatalf("helm %q", helm)
	}
	var txt bytes.Buffer
	if err := res.RenderText(&txt); err != nil || !strings.Contains(txt.String(), "port-forward") {
		t.Fatalf("render %q", txt.String())
	}

	// Second up reuses the run ID.
	r2 := awsRunner(repoRoot(t))
	a.Runner = r2
	res2, err := a.Up(context.Background(), AWSUpOptions{Timeout: time.Second, AdminCIDRs: []string{"198.51.100.0/24"}, Owner: "x"})
	if err != nil || res2.RunID != res.RunID {
		t.Fatalf("run ID not reused: %v %s vs %s", err, res2.RunID, res.RunID)
	}
}

func TestUpAWSConfirmation(t *testing.T) {
	r := awsRunner(repoRoot(t))
	a, _ := newAWS(t, r)
	a.Yes = false
	if _, err := a.Up(context.Background(), AWSUpOptions{}); !errors.Is(err, ErrNotConfirmed) {
		t.Fatalf("want ErrNotConfirmed, got %v", err)
	}
	if commandIndex(r.CommandLines(), tfPrefix(repoRoot(t))+" apply") >= 0 {
		t.Fatal("apply must not run without confirmation")
	}
	a.Confirm = func(string) (bool, error) { return false, nil }
	if _, err := a.Up(context.Background(), AWSUpOptions{}); !errors.Is(err, ErrNotConfirmed) {
		t.Fatalf("declined: %v", err)
	}
	a.Confirm = func(string) (bool, error) { return false, errors.New("no tty") }
	if _, err := a.Up(context.Background(), AWSUpOptions{}); err == nil || !strings.Contains(err.Error(), "no tty") {
		t.Fatalf("confirm error: %v", err)
	}
	a.Runner = awsRunner(repoRoot(t))
	a.Confirm = func(string) (bool, error) { return true, nil }
	if _, err := a.Up(context.Background(), AWSUpOptions{Timeout: time.Second}); err != nil {
		t.Fatalf("confirmed: %v", err)
	}
}

func TestUpAWSDryRunAppliesNothing(t *testing.T) {
	inner := awsRunner(repoRoot(t))
	var printed bytes.Buffer
	r := exec.DryRun{Inner: inner, Out: &printed, ReadOnly: exec.ReadOnly}
	a, _ := newAWS(t, r)
	a.DryRun = true
	a.Yes = false
	a.Kube = func(string, string) (KubeAPI, error) { t.Fatal("dry-run must not connect"); return nil, nil }
	if _, err := a.Up(context.Background(), AWSUpOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, c := range inner.CommandLines() {
		if strings.Contains(c, " apply") || strings.Contains(c, " init") {
			t.Fatalf("dry-run executed %q", c)
		}
	}
	if !strings.Contains(printed.String(), "[dry-run] tofu") || !strings.Contains(printed.String(), " apply -input=false") {
		t.Fatalf("printed %q", printed.String())
	}
}

func TestUpAWSFailures(t *testing.T) {
	root := repoRoot(t)
	a, _ := newAWS(t, exec.NewFake())
	if _, err := a.Up(context.Background(), AWSUpOptions{}); err == nil || !strings.Contains(err.Error(), "neither tofu nor terraform") {
		t.Fatalf("engine: %v", err)
	}
	r := exec.NewFake().WithPath("terraform").
		On("git remote get-url origin", exec.Result{Stdout: "https://github.com/hermeto/aibox.git"}, nil).
		On("git rev-parse --abbrev-ref HEAD", exec.Result{Stdout: "main"}, nil).
		On("git rev-list --count @{u}..HEAD", exec.Result{Stdout: "0"}, nil).
		On("git config user.email", exec.Result{}, errors.New("unset")).
		OnPrefix("terraform -chdir="+filepath.Join(root, TerraformDir)+" init", exec.Result{}, errors.New("bucket does not exist"))
	a, _ = newAWS(t, r)
	a.Getenv = func(k string) string {
		if k == "USER" {
			return "operator"
		}
		return ""
	}
	_, err := a.Up(context.Background(), AWSUpOptions{})
	if err == nil || !strings.Contains(err.Error(), "aws-first-run.md") {
		t.Fatalf("init hint: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(a.StateDir, "run.json"))
	if !strings.Contains(string(data), `"owner": "operator"`) {
		t.Fatalf("owner fallback: %s", data)
	}
	a, _ = newAWS(t, awsRunner(root))
	a.EgressIP = func(context.Context) (string, error) { return "not-an-ip", nil }
	if _, err := a.Up(context.Background(), AWSUpOptions{}); err == nil || !strings.Contains(err.Error(), "--admin-cidr") {
		t.Fatalf("egress: %v", err)
	}
	a, _ = newAWS(t, awsRunner(root))
	a.Account = func(context.Context) (string, error) { return "", errors.New("expired token") }
	if _, err := a.Up(context.Background(), AWSUpOptions{}); err == nil || !strings.Contains(err.Error(), "expired token") {
		t.Fatalf("account: %v", err)
	}
	a, _ = newAWS(t, awsRunner(root))
	a.Getenv = func(k string) string {
		if k == "AIBOX_TF_STATE_BUCKET" {
			return "my-state"
		}
		return ""
	}
	a.Account = nil
	if _, err := a.Up(context.Background(), AWSUpOptions{Timeout: time.Second}); err != nil {
		t.Fatalf("explicit bucket: %v", err)
	}
	a.Profile = localProfile(t)
	if _, err := a.Up(context.Background(), AWSUpOptions{}); err == nil {
		t.Fatal("expected non-AWS profile error")
	}
}

func TestDownAWS(t *testing.T) {
	r := awsRunner(repoRoot(t))
	a, log := newAWS(t, r)
	if deleted, err := a.Down(context.Background()); err != nil || deleted || !strings.Contains(log.String(), "nothing to destroy") {
		t.Fatalf("no state: %v %v", deleted, err)
	}
	if _, err := a.Up(context.Background(), AWSUpOptions{Timeout: time.Second}); err != nil {
		t.Fatal(err)
	}
	a.Yes = false
	if _, err := a.Down(context.Background()); !errors.Is(err, ErrNotConfirmed) {
		t.Fatalf("down must need confirmation: %v", err)
	}
	a.Yes = true
	deleted, err := a.Down(context.Background())
	if err != nil || !deleted {
		t.Fatalf("down: %v %v", deleted, err)
	}
	destroy := r.CommandLines()[commandIndex(r.CommandLines(), tfPrefix(repoRoot(t))+" destroy")]
	if !strings.Contains(destroy, "-auto-approve") || !strings.Contains(destroy, "terraform.tfvars.json") {
		t.Fatalf("destroy %q", destroy)
	}
	if _, err := os.Stat(filepath.Join(a.StateDir, "run.json")); !os.IsNotExist(err) {
		t.Fatal("run state must be removed after down")
	}
}

func TestWriteKubeconfigErrors(t *testing.T) {
	a, _ := newAWS(t, exec.NewFake())
	if _, err := a.writeKubeconfig("not json"); err == nil {
		t.Fatal("expected parse error")
	}
	if _, err := a.writeKubeconfig(`{"cluster_name":{"value":"x"}}`); err == nil {
		t.Fatal("expected missing outputs error")
	}
	if id := NewRunID("aws-dev", time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)); !strings.HasPrefix(id, "aws-dev-202601020304-") || len(id) != len("aws-dev-202601020304-")+4 {
		t.Fatalf("run id %q", id)
	}
}
