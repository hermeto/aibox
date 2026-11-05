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

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/hermeto/aibox/internal/cost"
	"github.com/hermeto/aibox/internal/exec"
	"github.com/hermeto/aibox/internal/profile"
)

type emptyEC2 struct{ cost.EC2API }

func (emptyEC2) DescribeInstances(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{}, nil
}

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

func gPrefix(root string) string { return "tofu -chdir=" + filepath.Join(root, GuardrailsDir) }

var upOpts = AWSUpOptions{Timeout: time.Second, BudgetUSD: 50, NotifyEmail: "ops@example.com"}

func cleanOrphans(_ context.Context, region, runID string) (*cost.OrphanReport, error) {
	return &cost.OrphanReport{RunID: runID, Region: region, Orphans: []cost.Orphan{}, OK: true}, nil
}

func awsRunner(root string) *exec.Fake {
	tf := tfPrefix(root)
	g := gPrefix(root)
	return exec.NewFake().WithPath("tofu").
		OnPrefix(g+" init", exec.Result{}, nil).
		OnPrefix(g+" plan", exec.Result{}, nil).
		OnPrefix(g+" apply", exec.Result{}, nil).
		OnPrefix(g+" destroy", exec.Result{}, nil).
		OnPrefix("kubectl --kubeconfig", exec.Result{Stdout: "gateway aibox-gw\n"}, nil).
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
		Orphans:      cleanOrphans,
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
	res, err := a.Up(context.Background(), upOpts)
	if err != nil {
		t.Fatalf("Up: %v\n%s", err, log)
	}
	if !strings.HasPrefix(res.RunID, "aws-dev-202611050102-") || res.Kubeconfig == "" {
		t.Fatalf("result %+v", res)
	}
	lines := r.CommandLines()
	tf := tfPrefix(repoRoot(t))
	g := gPrefix(repoRoot(t))
	order := []string{g + " init", g + " plan", tf + " init", tf + " plan", g + " apply", tf + " apply", tf + " output -json", "helm upgrade --install argocd"}
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
	if vars["owner"] != "dev@example.com" || vars["kubernetes_version"] != "1.35" || vars["admin_cidrs"].([]any)[0] != "203.0.113.10/32" ||
		vars["teardown_role_arn"] != "arn:aws:iam::000000000000:role/aibox-"+res.RunID+"-teardown" {
		t.Fatalf("tfvars %v", vars)
	}
	gv := map[string]any{}
	data, _ = os.ReadFile(filepath.Join(a.StateDir, "guardrails.tfvars.json"))
	if err := json.Unmarshal(data, &gv); err != nil {
		t.Fatal(err)
	}
	if gv["budget_usd"] != 50.0 || gv["notify_email"] != "ops@example.com" || gv["ttl_expires_at"] != "2026-11-05T05:02:00Z" ||
		gv["guardrails_state_key"] != "aws-dev/aibox-dev-guardrails.tfstate" || gv["revision"] != "main" || !strings.Contains(gv["main_tfvars_json"].(string), `"run_id"`) {
		t.Fatalf("guardrail vars %v", gv)
	}
	if !res.ExpiresAt.Equal(time.Date(2026, 11, 5, 5, 2, 0, 0, time.UTC)) || res.BudgetUSD != 50 {
		t.Fatalf("result %+v", res)
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
	opts2 := upOpts
	opts2.AdminCIDRs, opts2.Owner = []string{"198.51.100.0/24"}, "x"
	res2, err := a.Up(context.Background(), opts2)
	if err != nil || res2.RunID != res.RunID {
		t.Fatalf("run ID not reused: %v %s vs %s", err, res2.RunID, res.RunID)
	}
}

func TestUpAWSConfirmation(t *testing.T) {
	r := awsRunner(repoRoot(t))
	a, _ := newAWS(t, r)
	a.Yes = false
	if _, err := a.Up(context.Background(), upOpts); !errors.Is(err, ErrNotConfirmed) {
		t.Fatalf("want ErrNotConfirmed, got %v", err)
	}
	if commandIndex(r.CommandLines(), tfPrefix(repoRoot(t))+" apply") >= 0 || commandIndex(r.CommandLines(), gPrefix(repoRoot(t))+" apply") >= 0 {
		t.Fatal("apply must not run without confirmation")
	}
	a.Confirm = func(string) (bool, error) { return false, nil }
	if _, err := a.Up(context.Background(), upOpts); !errors.Is(err, ErrNotConfirmed) {
		t.Fatalf("declined: %v", err)
	}
	a.Confirm = func(string) (bool, error) { return false, errors.New("no tty") }
	if _, err := a.Up(context.Background(), upOpts); err == nil || !strings.Contains(err.Error(), "no tty") {
		t.Fatalf("confirm error: %v", err)
	}
	a.Runner = awsRunner(repoRoot(t))
	a.Confirm = func(string) (bool, error) { return true, nil }
	if _, err := a.Up(context.Background(), upOpts); err != nil {
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
	if _, err := a.Up(context.Background(), upOpts); err != nil {
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
	if _, err := a.Up(context.Background(), AWSUpOptions{}); !errors.Is(err, ErrBudgetRequired) {
		t.Fatalf("budget: %v", err)
	}
	if _, err := a.Up(context.Background(), AWSUpOptions{BudgetUSD: 10}); !errors.Is(err, ErrEmailRequired) {
		t.Fatalf("email: %v", err)
	}
	if len(exec.NewFake().CommandLines()) != 0 {
		t.Fatal("no command may run before budget/e-mail checks")
	}
	if _, err := a.Up(context.Background(), AWSUpOptions{BudgetUSD: 10, NotifyEmail: "x@example.com", TTL: 48 * time.Hour}); err == nil {
		t.Fatal("ttl over 24h must fail")
	}
	if _, err := a.Up(context.Background(), upOpts); err == nil || !strings.Contains(err.Error(), "neither tofu nor terraform") {
		t.Fatalf("engine: %v", err)
	}
	r := exec.NewFake().WithPath("terraform").
		On("git remote get-url origin", exec.Result{Stdout: "https://github.com/hermeto/aibox.git"}, nil).
		On("git rev-parse --abbrev-ref HEAD", exec.Result{Stdout: "main"}, nil).
		On("git rev-list --count @{u}..HEAD", exec.Result{Stdout: "0"}, nil).
		On("git config user.email", exec.Result{}, errors.New("unset")).
		OnPrefix("terraform -chdir="+filepath.Join(root, GuardrailsDir)+" init", exec.Result{}, errors.New("bucket does not exist"))
	a, _ = newAWS(t, r)
	a.Getenv = func(k string) string {
		if k == "USER" {
			return "operator"
		}
		return ""
	}
	_, err := a.Up(context.Background(), upOpts)
	if err == nil || !strings.Contains(err.Error(), "aws-first-run.md") {
		t.Fatalf("init hint: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(a.StateDir, "run.json"))
	if !strings.Contains(string(data), `"owner": "operator"`) {
		t.Fatalf("owner fallback: %s", data)
	}
	a, _ = newAWS(t, awsRunner(root))
	a.EgressIP = func(context.Context) (string, error) { return "not-an-ip", nil }
	if _, err := a.Up(context.Background(), upOpts); err == nil || !strings.Contains(err.Error(), "--admin-cidr") {
		t.Fatalf("egress: %v", err)
	}
	a, _ = newAWS(t, awsRunner(root))
	a.Account = func(context.Context) (string, error) { return "", errors.New("expired token") }
	if _, err := a.Up(context.Background(), upOpts); err == nil || !strings.Contains(err.Error(), "expired token") {
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
	if _, err := a.Up(context.Background(), upOpts); err != nil {
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
	if res, err := a.Down(context.Background()); err != nil || res.Destroyed || !strings.Contains(log.String(), "nothing to destroy") {
		t.Fatalf("no state: %+v %v", res, err)
	}
	if _, err := a.Up(context.Background(), upOpts); err != nil {
		t.Fatal(err)
	}
	a.Yes = false
	if _, err := a.Down(context.Background()); !errors.Is(err, ErrNotConfirmed) {
		t.Fatalf("down must need confirmation: %v", err)
	}
	a.Yes = true
	res, err := a.Down(context.Background())
	if err != nil || !res.Destroyed || res.Orphans == nil || !res.Orphans.OK {
		t.Fatalf("down: %+v %v", res, err)
	}
	lines := r.CommandLines()
	tf, g := tfPrefix(repoRoot(t)), gPrefix(repoRoot(t))
	drain := commandIndex(lines, "kubectl --kubeconfig")
	mainDestroy := commandIndex(lines, tf+" destroy")
	gDestroy := commandIndex(lines, g+" destroy")
	if drain < 0 || mainDestroy < drain || gDestroy < mainDestroy {
		t.Fatalf("down order drain=%d main=%d guardrails=%d in %v", drain, mainDestroy, gDestroy, lines)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "-n gateway delete svc aibox-gw") {
		t.Fatalf("load balancer service not deleted: %v", lines)
	}
	var txt bytes.Buffer
	if err := res.RenderText(&txt); err != nil || !strings.Contains(txt.String(), "destroyed") {
		t.Fatal(txt.String())
	}
	if _, err := os.Stat(filepath.Join(a.StateDir, "run.json")); !os.IsNotExist(err) {
		t.Fatal("run state must be removed after a clean down")
	}
}

func TestDownAWSOrphansKeepState(t *testing.T) {
	a, _ := newAWS(t, awsRunner(repoRoot(t)))
	if _, err := a.Up(context.Background(), upOpts); err != nil {
		t.Fatal(err)
	}
	a.Orphans = func(_ context.Context, region, runID string) (*cost.OrphanReport, error) {
		return &cost.OrphanReport{RunID: runID, Region: region, Orphans: []cost.Orphan{{Type: "volume", ID: "vol-1"}}}, nil
	}
	res, err := a.Down(context.Background())
	if !errors.Is(err, ErrOrphans) || res == nil || res.Orphans == nil || len(res.Orphans.Orphans) != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	if _, err := os.Stat(filepath.Join(a.StateDir, "run.json")); err != nil {
		t.Fatal("run state must be kept while orphans remain")
	}
	if rep, err := a.CheckOrphans(context.Background()); !errors.Is(err, ErrOrphans) || len(rep.Orphans) != 1 {
		t.Fatalf("orphans-only: %+v %v", rep, err)
	}
	a.Orphans = func(context.Context, string, string) (*cost.OrphanReport, error) { return nil, errors.New("denied") }
	if _, err := a.CheckOrphans(context.Background()); err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("api error: %v", err)
	}
	a.Orphans = nil
	if _, err := a.CheckOrphans(context.Background()); err == nil {
		t.Fatal("unconfigured checker")
	}
}

func TestTTLExtendAndCost(t *testing.T) {
	r := awsRunner(repoRoot(t))
	a, _ := newAWS(t, r)
	if _, err := a.TTL(); !errors.Is(err, ErrNoActiveRun) {
		t.Fatalf("no run: %v", err)
	}
	if _, err := a.ExtendTTL(context.Background(), time.Hour); !errors.Is(err, ErrNoActiveRun) {
		t.Fatalf("no run: %v", err)
	}
	if _, err := a.Up(context.Background(), upOpts); err != nil {
		t.Fatal(err)
	}
	st, err := a.ExtendTTL(context.Background(), 2*time.Hour)
	if err != nil || !st.ExpiresAt.Equal(time.Date(2026, 11, 5, 7, 2, 0, 0, time.UTC)) {
		t.Fatalf("%+v %v", st, err)
	}
	if _, err := a.ExtendTTL(context.Background(), 20*time.Hour); err == nil {
		t.Fatal("beyond 24h must fail")
	}
	st, err = a.TTL()
	if err != nil || st.Remaining != 6*time.Hour {
		t.Fatalf("status %+v %v", st, err)
	}
	var txt bytes.Buffer
	_ = st.RenderText(&txt)
	if !strings.Contains(txt.String(), "6h0m0s left") {
		t.Fatal(txt.String())
	}
	applied := r.CommandLines()[commandIndex(r.CommandLines(), gPrefix(repoRoot(t))+" apply -input=false -auto-approve")]
	if !strings.Contains(applied, "guardrails.tfvars.json") {
		t.Fatal(applied)
	}
	a.Now = func() time.Time { return time.Date(2026, 11, 5, 3, 2, 0, 0, time.UTC) }
	rep, err := a.Cost(context.Background(), emptyEC2{})
	if err != nil || rep.RunID == "" || rep.Total != 0.29 {
		t.Fatalf("cost %+v %v", rep, err)
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
