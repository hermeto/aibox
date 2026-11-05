package platform

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/hermeto/aibox/internal/cost"
	"github.com/hermeto/aibox/internal/exec"
	"github.com/hermeto/aibox/internal/profile"
	"github.com/hermeto/aibox/internal/teardown"
)

// TerraformDir is the Terraform root for AWS profiles, relative to the repository root.
const TerraformDir = "infra/terraform/aws"

// Errors returned by the AWS orchestration.
var (
	// ErrNotConfirmed is returned when the operator declines (or cannot confirm) an apply.
	ErrNotConfirmed = errors.New("not confirmed (re-run with --yes to apply without a prompt)")
	// ErrBudgetRequired is returned when a cloud run has no budget.
	ErrBudgetRequired = errors.New("a budget is required for cloud runs: set BUDGET_USD or pass --budget (USD)")
	// ErrEmailRequired is returned when no alert e-mail is known.
	ErrEmailRequired = errors.New("an alert e-mail is required: pass --notify-email or set spec.guardrails.ownerEmail in the profile")
	// ErrNoActiveRun is returned by commands that need a running environment.
	ErrNoActiveRun = errors.New("no active run for this profile (nothing in .aibox/<profile>/run.json)")
	// ErrOrphans is returned when resources tagged with the run still exist after teardown.
	ErrOrphans = errors.New("orphaned resources remain after teardown")
)

// GuardrailsDir is the guardrails stack, relative to the repository root.
const GuardrailsDir = TerraformDir + "/guardrails"

// RunState persists per-run identity between `up` and `down` (.aibox/<profile>/run.json).
type RunState struct {
	RunID         string    `json:"runId"`
	Owner         string    `json:"owner"`
	CreatedAt     time.Time `json:"createdAt"`
	Region        string    `json:"region"`
	StateBucket   string    `json:"stateBucket"`
	StateKey      string    `json:"stateKey"`
	GuardrailsKey string    `json:"guardrailsStateKey"`
	BudgetUSD     float64   `json:"budgetUSD"`
	NotifyEmail   string    `json:"notifyEmail"`
	TTL           string    `json:"ttl"`
	ExpiresAt     time.Time `json:"expiresAt"`
	RepoURL       string    `json:"repoURL"`
	Revision      string    `json:"revision"`
	TeardownRole  string    `json:"teardownRoleArn,omitempty"`
}

// AWS brings AWS profiles up and down with Terraform/OpenTofu. Every command goes through
// Runner, so tests use exec.Fake and --dry-run prints instead of applying.
type AWS struct {
	Profile *profile.Profile
	Runner  exec.Runner
	Root    string
	// StateDir defaults to <Root>/.aibox/<profile>.
	StateDir string
	Log      io.Writer
	DryRun   bool
	// Yes skips confirmation prompts.
	Yes bool
	// Confirm asks the operator; nil means prompts are impossible (require --yes).
	Confirm func(prompt string) (bool, error)
	// Account returns the AWS account ID (sts:GetCallerIdentity).
	Account func(ctx context.Context) (string, error)
	// EgressIP returns the operator's public IPv4 address.
	EgressIP func(ctx context.Context) (string, error)
	// Kube connects to the cluster with an explicit kubeconfig.
	Kube func(kubeconfig, kubeContext string) (KubeAPI, error)
	// Orphans lists resources still tagged with the run (read-only).
	Orphans      func(ctx context.Context, region, runID string) (*cost.OrphanReport, error)
	Getenv       func(string) string
	Now          func() time.Time
	PollInterval time.Duration
}

// AWSUpOptions configure an AWS bring-up.
type AWSUpOptions struct {
	RepoURL     string
	Revision    string
	AdminCIDRs  []string
	Owner       string
	Timeout     time.Duration
	BudgetUSD   float64
	NotifyEmail string
	// TTL overrides the profile's guardrails.ttl.
	TTL time.Duration
}

// AWSUpResult is printed after an AWS bring-up.
type AWSUpResult struct {
	Profile    string          `json:"profile"`
	Cluster    string          `json:"cluster"`
	RunID      string          `json:"runId"`
	Region     string          `json:"region"`
	BudgetUSD  float64         `json:"budgetUSD"`
	ExpiresAt  time.Time       `json:"expiresAt"`
	Kubeconfig string          `json:"kubeconfig"`
	Outputs    string          `json:"outputsFile"`
	Source     *GitSource      `json:"source,omitempty"`
	Endpoints  []Endpoint      `json:"endpoints"`
	Raw        json.RawMessage `json:"-"`
}

// RenderText prints where to find the cluster.
func (r *AWSUpResult) RenderText(w io.Writer) error {
	_, err := fmt.Fprintf(w, "\nAIBox is up (profile %s, cluster %s, run %s, region %s).\n\nBudget:     USD %.2f (alerts at 50/80/100%%)\nTeardown:   %s (extend with: aibox ttl extend 2h --profile %s)\nKubeconfig: %s (context aibox-%s)\nOutputs:    %s\nArgo CD:    kubectl --kubeconfig %s -n argocd port-forward svc/argocd-server 8080:80\n",
		r.Profile, r.Cluster, r.RunID, r.Region, r.BudgetUSD, r.ExpiresAt.Format(time.RFC3339), r.Profile, r.Kubeconfig, r.Profile, r.Outputs, r.Kubeconfig)
	return err
}

func (a *AWS) logf(format string, args ...any) {
	if a.Log != nil {
		_, _ = fmt.Fprintf(a.Log, format+"\n", args...)
	}
}

func (a *AWS) getenv(k string) string {
	if a.Getenv != nil {
		return a.Getenv(k)
	}
	return os.Getenv(k)
}

func (a *AWS) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

func (a *AWS) stateDir() string {
	if a.StateDir != "" {
		return a.StateDir
	}
	return filepath.Join(a.Root, ".aibox", a.Profile.Metadata.Name)
}

// KubeContext is the context name written to the generated kubeconfig.
func (a *AWS) KubeContext() string { return "aibox-" + a.Profile.Metadata.Name }

func (a *AWS) engine() (string, error) {
	for _, e := range []string{"tofu", "terraform"} {
		if _, err := a.Runner.LookPath(e); err == nil {
			return e, nil
		}
	}
	return "", errors.New("neither tofu nor terraform found in PATH (see aibox doctor)")
}

func (a *AWS) tf(ctx context.Context, engine, dir string, args ...string) (exec.Result, error) {
	full := append([]string{"-chdir=" + filepath.Join(a.Root, dir)}, args...)
	return a.Runner.Run(ctx, exec.Cmd{Name: engine, Args: full, Stream: a.Log})
}

// loadRun returns the saved run state, or nil if none exists.
func (a *AWS) loadRun() (*RunState, error) {
	data, err := os.ReadFile(filepath.Join(a.stateDir(), "run.json")) //nolint:gosec // generated file
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rs RunState
	if err := json.Unmarshal(data, &rs); err != nil {
		return nil, fmt.Errorf("parse run state: %w", err)
	}
	return &rs, nil
}

func (a *AWS) writeJSON(name string, v any) (string, error) {
	if err := os.MkdirAll(a.stateDir(), 0o750); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(a.stateDir(), name)
	return path, os.WriteFile(path, append(data, '\n'), 0o600)
}

// NewRunID returns "<profile>-<UTC yyyymmddhhmm>-<4 hex>".
func NewRunID(profileName string, now time.Time) string {
	b := make([]byte, 2)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s-%s-%s", profileName, now.UTC().Format("200601021504"), hex.EncodeToString(b))
}

func (a *AWS) resolveOwner(ctx context.Context, explicit string) string {
	if explicit != "" {
		return explicit
	}
	if a.Profile.Spec.Guardrails.OwnerEmail != "" {
		return a.Profile.Spec.Guardrails.OwnerEmail
	}
	if res, err := a.Runner.Run(ctx, exec.Cmd{Name: "git", Args: []string{"config", "user.email"}, Dir: a.Root}); err == nil && strings.TrimSpace(res.Stdout) != "" {
		return strings.TrimSpace(res.Stdout)
	}
	if u := a.getenv("USER"); u != "" {
		return u
	}
	return "unknown"
}

// account resolves the AWS account ID ("" when unknown and AIBOX_TF_STATE_BUCKET is set).
func (a *AWS) account(ctx context.Context) (string, error) {
	if a.Account == nil {
		if a.getenv("AIBOX_TF_STATE_BUCKET") != "" {
			return "", nil
		}
		return "", errors.New("cannot determine the AWS account for the state bucket (set AIBOX_TF_STATE_BUCKET)")
	}
	acct, err := a.Account(ctx)
	if err != nil {
		return "", fmt.Errorf("determine AWS account: %w", err)
	}
	return acct, nil
}

func (a *AWS) stateBucket(acct string) string {
	if b := a.getenv("AIBOX_TF_STATE_BUCKET"); b != "" {
		return b
	}
	return fmt.Sprintf("aibox-tfstate-%s-%s", acct, a.Profile.Spec.AWS.Region)
}

func (a *AWS) adminCIDRs(ctx context.Context, explicit []string) ([]string, error) {
	if len(explicit) > 0 {
		return explicit, nil
	}
	if a.EgressIP == nil {
		return nil, errors.New("cannot detect your egress IP; pass --admin-cidr")
	}
	ip, err := a.EgressIP(ctx)
	if err != nil {
		return nil, fmt.Errorf("detect egress IP (pass --admin-cidr instead): %w", err)
	}
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil || parsed.To4() == nil {
		return nil, fmt.Errorf("unexpected egress IP %q (pass --admin-cidr)", ip)
	}
	return []string{parsed.String() + "/32"}, nil
}

func (a *AWS) confirm(prompt string) error {
	if a.Yes || a.DryRun {
		return nil
	}
	if a.Confirm == nil {
		return ErrNotConfirmed
	}
	ok, err := a.Confirm(prompt)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotConfirmed
	}
	return nil
}

func (a *AWS) tfVars(rs *RunState, cidrs []string) map[string]any {
	return map[string]any{
		"profile":            a.Profile.Metadata.Name,
		"run_id":             rs.RunID,
		"owner":              rs.Owner,
		"region":             a.Profile.Spec.AWS.Region,
		"cluster_name":       a.Profile.Spec.Cluster.Name,
		"kubernetes_version": a.Profile.Spec.Cluster.KubernetesVersion,
		"admin_cidrs":        cidrs,
		"teardown_role_arn":  rs.TeardownRole,
	}
}

func (a *AWS) guardrailVars(rs *RunState, mainVars map[string]any) (map[string]any, error) {
	mainJSON, err := json.Marshal(mainVars)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"profile":              a.Profile.Metadata.Name,
		"run_id":               rs.RunID,
		"owner":                rs.Owner,
		"region":               rs.Region,
		"cluster_name":         a.Profile.Spec.Cluster.Name,
		"budget_usd":           rs.BudgetUSD,
		"notify_email":         rs.NotifyEmail,
		"ttl_expires_at":       rs.ExpiresAt.UTC().Format(time.RFC3339),
		"main_tfvars_json":     string(mainJSON),
		"state_bucket":         rs.StateBucket,
		"main_state_key":       rs.StateKey,
		"guardrails_state_key": rs.GuardrailsKey,
		"repo_url":             rs.RepoURL,
		"revision":             rs.Revision,
	}, nil
}

func (a *AWS) init(ctx context.Context, engine, dir, key string, rs *RunState) error {
	_, err := a.tf(ctx, engine, dir, "init", "-input=false", "-reconfigure",
		"-backend-config=bucket="+rs.StateBucket,
		"-backend-config=key="+key,
		"-backend-config=region="+rs.Region)
	if err != nil {
		return fmt.Errorf("%s init failed — if the state bucket %s does not exist, create it once with the bootstrap stack (docs/runbooks/aws-first-run.md): %w", engine, rs.StateBucket, err)
	}
	return nil
}

// Up applies the run's guardrails (budget, TTL teardown), then the environment, then
// bootstraps GitOps. It refuses to start without a budget and an alert e-mail.
func (a *AWS) Up(ctx context.Context, opts AWSUpOptions) (*AWSUpResult, error) {
	if a.Profile.Spec.Provider != profile.ProviderAWS || a.Profile.Spec.AWS == nil {
		return nil, fmt.Errorf("profile %s is not an AWS profile", a.Profile.Metadata.Name)
	}
	if opts.BudgetUSD <= 0 {
		return nil, ErrBudgetRequired
	}
	email := firstNonEmptyString(opts.NotifyEmail, a.Profile.Spec.Guardrails.OwnerEmail)
	if email == "" {
		return nil, ErrEmailRequired
	}
	ttl := opts.TTL
	if ttl == 0 {
		d, err := a.Profile.Spec.Guardrails.TTLDuration()
		if err != nil {
			return nil, err
		}
		ttl = d
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 20 * time.Minute
	}
	engine, err := a.engine()
	if err != nil {
		return nil, err
	}
	rs, err := a.loadRun()
	if err != nil {
		return nil, err
	}
	src, err := DetectGitSource(ctx, a.Runner, a.Root, opts.RepoURL, opts.Revision)
	if err != nil {
		return nil, err
	}
	if src.Warning != "" {
		a.logf("warning: %s", src.Warning)
	}
	if rs == nil {
		acct, err := a.account(ctx)
		if err != nil {
			return nil, err
		}
		now := a.now().UTC()
		expires, err := teardown.Expiry(now, ttl)
		if err != nil {
			return nil, err
		}
		rs = &RunState{
			RunID:       NewRunID(a.Profile.Metadata.Name, now),
			Owner:       a.resolveOwner(ctx, opts.Owner),
			CreatedAt:   now,
			Region:      a.Profile.Spec.AWS.Region,
			StateBucket: a.stateBucket(acct),
			ExpiresAt:   expires,
			TTL:         ttl.String(),
		}
		rs.StateKey = a.Profile.Metadata.Name + "/" + a.Profile.Spec.Cluster.Name + ".tfstate"
		rs.GuardrailsKey = a.Profile.Metadata.Name + "/" + a.Profile.Spec.Cluster.Name + "-guardrails.tfstate"
		if acct != "" {
			rs.TeardownRole = fmt.Sprintf("arn:aws:iam::%s:role/aibox-%s-teardown", acct, rs.RunID)
		} else {
			a.logf("warning: AWS account unknown; the TTL teardown job will not get cluster access to drain load balancers and nodes first")
		}
	}
	rs.BudgetUSD, rs.NotifyEmail = opts.BudgetUSD, email
	rs.RepoURL, rs.Revision = src.RepoURL, src.Revision
	a.logf("Run %s (owner %s, budget USD %.2f, teardown at %s, state s3://%s/%s)", rs.RunID, rs.Owner, rs.BudgetUSD, rs.ExpiresAt.Format(time.RFC3339), rs.StateBucket, rs.StateKey)
	cidrs, err := a.adminCIDRs(ctx, opts.AdminCIDRs)
	if err != nil {
		return nil, err
	}
	if _, err := a.writeJSON("run.json", rs); err != nil {
		return nil, err
	}
	mainVars := a.tfVars(rs, cidrs)
	varsFile, err := a.writeJSON("terraform.tfvars.json", mainVars)
	if err != nil {
		return nil, err
	}
	gVars, err := a.guardrailVars(rs, mainVars)
	if err != nil {
		return nil, err
	}
	gVarsFile, err := a.writeJSON("guardrails.tfvars.json", gVars)
	if err != nil {
		return nil, err
	}

	// Plan both stacks, confirm once, then apply guardrails first so the budget and the TTL
	// exist even if the main apply fails half-way.
	gPlan := filepath.Join(a.stateDir(), "guardrails.tfplan")
	mPlan := filepath.Join(a.stateDir(), "plan.tfplan")
	if err := a.init(ctx, engine, GuardrailsDir, rs.GuardrailsKey, rs); err != nil {
		return nil, err
	}
	if _, err := a.tf(ctx, engine, GuardrailsDir, "plan", "-input=false", "-var-file="+gVarsFile, "-out="+gPlan); err != nil {
		return nil, fmt.Errorf("%s plan (guardrails): %w", engine, err)
	}
	if err := a.init(ctx, engine, TerraformDir, rs.StateKey, rs); err != nil {
		return nil, err
	}
	if _, err := a.tf(ctx, engine, TerraformDir, "plan", "-input=false", "-var-file="+varsFile, "-out="+mPlan); err != nil {
		return nil, fmt.Errorf("%s plan: %w", engine, err)
	}
	if err := a.confirm(fmt.Sprintf("Apply guardrails and environment for profile %s (budget USD %.2f, teardown at %s; resources cost money until `aibox down`)?",
		a.Profile.Metadata.Name, rs.BudgetUSD, rs.ExpiresAt.Format(time.RFC3339))); err != nil {
		return nil, err
	}
	if _, err := a.tf(ctx, engine, GuardrailsDir, "apply", "-input=false", gPlan); err != nil {
		return nil, fmt.Errorf("%s apply (guardrails): %w", engine, err)
	}
	if _, err := a.tf(ctx, engine, TerraformDir, "apply", "-input=false", mPlan); err != nil {
		return nil, fmt.Errorf("%s apply (re-run `aibox up` to resume, or `aibox down` to clean up): %w", engine, err)
	}
	out, err := a.Runner.Run(ctx, exec.Cmd{Name: engine, Args: []string{"-chdir=" + filepath.Join(a.Root, TerraformDir), "output", "-json"}})
	if err != nil {
		return nil, fmt.Errorf("%s output: %w", engine, err)
	}
	outputsFile := filepath.Join(a.stateDir(), "outputs.json")
	if !a.DryRun {
		if err := os.WriteFile(outputsFile, []byte(out.Stdout), 0o600); err != nil {
			return nil, err
		}
	}
	res := &AWSUpResult{Profile: a.Profile.Metadata.Name, Cluster: a.Profile.Spec.Cluster.Name, RunID: rs.RunID, Region: rs.Region,
		BudgetUSD: rs.BudgetUSD, ExpiresAt: rs.ExpiresAt, Outputs: outputsFile, Source: &src, Raw: json.RawMessage(out.Stdout)}
	if a.DryRun {
		a.logf("[dry-run] would write a kubeconfig, install Argo CD and apply the root Application")
		return res, nil
	}
	kubeconfig, err := a.writeKubeconfig(out.Stdout)
	if err != nil {
		return nil, err
	}
	res.Kubeconfig = kubeconfig
	if err := a.bootstrapGitOps(ctx, kubeconfig, src, rs.RunID, opts.Timeout); err != nil {
		return nil, err
	}
	return res, nil
}

func firstNonEmptyString(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func (a *AWS) bootstrapGitOps(ctx context.Context, kubeconfig string, src GitSource, runID string, timeout time.Duration) error {
	comps, err := LoadComponents(a.Root, a.Profile.Metadata.Name)
	if err != nil {
		return err
	}
	wctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	g := &gitops{Runner: a.Runner, Root: a.Root, Log: a.Log, ProfileName: a.Profile.Metadata.Name, KubeContext: a.KubeContext(), Kubeconfig: kubeconfig, RunID: runID, PollInterval: a.PollInterval}
	if err := g.installArgoCD(wctx); err != nil {
		return err
	}
	k, err := a.Kube(kubeconfig, a.KubeContext())
	if err != nil {
		return err
	}
	if err := g.waitWorkloads(wctx, k, ArgoCDNamespace); err != nil {
		return fmt.Errorf("argo cd not ready: %w", err)
	}
	return g.bootstrap(wctx, k, src, comps)
}

type tfOutput struct {
	Value any `json:"value"`
}

// writeKubeconfig renders a kubeconfig that authenticates with `aws eks get-token`.
func (a *AWS) writeKubeconfig(outputsJSON string) (string, error) {
	var outs map[string]tfOutput
	if err := json.Unmarshal([]byte(outputsJSON), &outs); err != nil {
		return "", fmt.Errorf("parse terraform outputs: %w", err)
	}
	get := func(k string) string {
		s, _ := outs[k].Value.(string)
		return s
	}
	endpoint, ca, name, region := get("cluster_endpoint"), get("cluster_certificate_authority_data"), get("cluster_name"), get("region")
	if endpoint == "" || ca == "" || name == "" {
		return "", errors.New("terraform outputs lack cluster_endpoint/cluster_certificate_authority_data/cluster_name")
	}
	ctxName := a.KubeContext()
	cfg, err := yaml.Marshal(map[string]any{
		"apiVersion": "v1",
		"kind":       "Config",
		"clusters": []any{map[string]any{"name": ctxName, "cluster": map[string]any{
			"server": endpoint, "certificate-authority-data": ca,
		}}},
		"users": []any{map[string]any{"name": ctxName, "user": map[string]any{"exec": map[string]any{
			"apiVersion":      "client.authentication.k8s.io/v1beta1",
			"command":         "aws",
			"args":            []string{"eks", "get-token", "--cluster-name", name, "--region", region},
			"interactiveMode": "Never",
		}}}},
		"contexts":        []any{map[string]any{"name": ctxName, "context": map[string]any{"cluster": ctxName, "user": ctxName}}},
		"current-context": ctxName,
	})
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(a.stateDir(), 0o750); err != nil {
		return "", err
	}
	path := filepath.Join(a.stateDir(), "kubeconfig")
	return path, os.WriteFile(path, cfg, 0o600)
}

// DownResult reports a teardown.
type DownResult struct {
	Profile   string             `json:"profile"`
	RunID     string             `json:"runId,omitempty"`
	Destroyed bool               `json:"destroyed"`
	Orphans   *cost.OrphanReport `json:"orphans,omitempty"`
}

// RenderText prints the teardown result and orphans.
func (d *DownResult) RenderText(w io.Writer) error {
	msg := fmt.Sprintf("Run %s (profile %s) destroyed.\n", d.RunID, d.Profile)
	if !d.Destroyed {
		msg = fmt.Sprintf("Nothing to destroy for profile %s.\n", d.Profile)
	}
	if _, err := io.WriteString(w, msg); err != nil {
		return err
	}
	if d.Orphans != nil {
		return d.Orphans.RenderText(w)
	}
	return nil
}

// Down drains the cluster, destroys the main stack and then the guardrails stack, and
// checks for orphans. Run state is kept while orphans remain.
func (a *AWS) Down(ctx context.Context) (*DownResult, error) {
	res := &DownResult{Profile: a.Profile.Metadata.Name}
	rs, err := a.loadRun()
	if err != nil {
		return nil, err
	}
	if rs == nil {
		a.logf("No run state in %s: nothing to destroy", a.stateDir())
		return res, nil
	}
	res.RunID = rs.RunID
	engine, err := a.engine()
	if err != nil {
		return nil, err
	}
	if err := a.confirm(fmt.Sprintf("Destroy every resource of run %s (profile %s)?", rs.RunID, a.Profile.Metadata.Name)); err != nil {
		return nil, err
	}
	a.drain(ctx)
	if err := a.init(ctx, engine, TerraformDir, rs.StateKey, rs); err != nil {
		return nil, err
	}
	varsFile := filepath.Join(a.stateDir(), "terraform.tfvars.json")
	if _, err := a.tf(ctx, engine, TerraformDir, "destroy", "-input=false", "-auto-approve", "-lock-timeout=5m", "-var-file="+varsFile); err != nil {
		return nil, fmt.Errorf("%s destroy (re-run `aibox down` to retry): %w", engine, err)
	}
	if err := a.init(ctx, engine, GuardrailsDir, rs.GuardrailsKey, rs); err != nil {
		return nil, err
	}
	gVarsFile := filepath.Join(a.stateDir(), "guardrails.tfvars.json")
	if _, err := a.tf(ctx, engine, GuardrailsDir, "destroy", "-input=false", "-auto-approve", "-lock-timeout=5m", "-var-file="+gVarsFile); err != nil {
		return nil, fmt.Errorf("%s destroy (guardrails): %w", engine, err)
	}
	res.Destroyed = true
	if a.DryRun {
		return res, nil
	}
	report, err := a.checkOrphans(ctx, rs)
	res.Orphans = report
	if err != nil {
		return res, err
	}
	for _, f := range []string{"run.json", "plan.tfplan", "guardrails.tfplan", "outputs.json", "kubeconfig"} {
		_ = os.Remove(filepath.Join(a.stateDir(), f))
	}
	return res, nil
}

// CheckOrphans runs only the orphan check for the current run.
func (a *AWS) CheckOrphans(ctx context.Context) (*cost.OrphanReport, error) {
	rs, err := a.loadRun()
	if err != nil {
		return nil, err
	}
	if rs == nil {
		return nil, ErrNoActiveRun
	}
	return a.checkOrphans(ctx, rs)
}

func (a *AWS) checkOrphans(ctx context.Context, rs *RunState) (*cost.OrphanReport, error) {
	if a.Orphans == nil {
		return nil, errors.New("orphan checker not configured")
	}
	report, err := a.Orphans(ctx, rs.Region, rs.RunID)
	if err != nil {
		return nil, fmt.Errorf("orphan check: %w", err)
	}
	if !report.OK {
		return report, fmt.Errorf("%w: %d resource(s) tagged run-id=%s", ErrOrphans, len(report.Orphans), rs.RunID)
	}
	return report, nil
}

// drain removes what controllers created outside Terraform (load balancers, Karpenter
// nodes) so the VPC can be deleted. Failures are logged, not fatal.
func (a *AWS) drain(ctx context.Context) {
	kubeconfig := filepath.Join(a.stateDir(), "kubeconfig")
	if _, err := os.Stat(kubeconfig); err != nil {
		a.logf("No kubeconfig for the run; skipping cluster drain")
		return
	}
	kc := []string{"--kubeconfig", kubeconfig, "--context", a.KubeContext()}
	run := func(args ...string) (exec.Result, error) {
		return a.Runner.Run(ctx, exec.Cmd{Name: "kubectl", Args: append(append([]string{}, kc...), args...)})
	}
	a.logf("Draining the cluster (Argo CD scaled to zero, Karpenter node pools and load balancers deleted)")
	if _, err := run("-n", ArgoCDNamespace, "scale", "deployment,statefulset", "--all", "--replicas=0"); err != nil {
		a.logf("warning: scale Argo CD down: %v", err)
	}
	if _, err := run("delete", "nodepools.karpenter.sh", "--all", "--ignore-not-found", "--wait=true", "--timeout=10m"); err != nil {
		a.logf("warning: delete Karpenter node pools: %v", err)
	}
	out, err := run("get", "svc", "-A", "-o", `jsonpath={range .items[?(@.spec.type=="LoadBalancer")]}{.metadata.namespace}{" "}{.metadata.name}{"\n"}{end}`)
	if err != nil {
		a.logf("warning: list load balancer services: %v", err)
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(out.Stdout), "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			continue
		}
		if _, err := run("-n", f[0], "delete", "svc", f[1], "--wait=true", "--timeout=5m"); err != nil {
			a.logf("warning: delete service %s/%s: %v", f[0], f[1], err)
		}
	}
}

// TTLStatus describes the current run's teardown time.
type TTLStatus struct {
	RunID     string        `json:"runId"`
	StartedAt time.Time     `json:"startedAt"`
	ExpiresAt time.Time     `json:"expiresAt"`
	Remaining time.Duration `json:"remaining"`
}

// RenderText prints the TTL status.
func (t *TTLStatus) RenderText(w io.Writer) error {
	_, err := fmt.Fprintf(w, "Run %s started %s; teardown at %s (%s left).\n", t.RunID, t.StartedAt.Format(time.RFC3339), t.ExpiresAt.Format(time.RFC3339), t.Remaining)
	return err
}

// TTL returns the current run's TTL status.
func (a *AWS) TTL() (*TTLStatus, error) {
	rs, err := a.loadRun()
	if err != nil {
		return nil, err
	}
	if rs == nil {
		return nil, ErrNoActiveRun
	}
	return &TTLStatus{RunID: rs.RunID, StartedAt: rs.CreatedAt, ExpiresAt: rs.ExpiresAt, Remaining: teardown.Remaining(rs.ExpiresAt, a.now())}, nil
}

// ExtendTTL moves the teardown by d (at most 24 h after the run start) by re-applying the
// guardrails stack.
func (a *AWS) ExtendTTL(ctx context.Context, d time.Duration) (*TTLStatus, error) {
	rs, err := a.loadRun()
	if err != nil {
		return nil, err
	}
	if rs == nil {
		return nil, ErrNoActiveRun
	}
	next, err := teardown.Extend(rs.CreatedAt, rs.ExpiresAt, d)
	if err != nil {
		return nil, err
	}
	engine, err := a.engine()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(a.stateDir(), "terraform.tfvars.json")) //nolint:gosec // generated file
	if err != nil {
		return nil, fmt.Errorf("read run inputs: %w", err)
	}
	var mainVars map[string]any
	if err := json.Unmarshal(data, &mainVars); err != nil {
		return nil, err
	}
	prev := rs.ExpiresAt
	rs.ExpiresAt = next
	gVars, err := a.guardrailVars(rs, mainVars)
	if err != nil {
		return nil, err
	}
	gVarsFile, err := a.writeJSON("guardrails.tfvars.json", gVars)
	if err != nil {
		return nil, err
	}
	if err := a.init(ctx, engine, GuardrailsDir, rs.GuardrailsKey, rs); err != nil {
		return nil, err
	}
	if _, err := a.tf(ctx, engine, GuardrailsDir, "apply", "-input=false", "-auto-approve", "-var-file="+gVarsFile); err != nil {
		rs.ExpiresAt = prev
		return nil, fmt.Errorf("%s apply (guardrails): %w", engine, err)
	}
	if !a.DryRun {
		if _, err := a.writeJSON("run.json", rs); err != nil {
			return nil, err
		}
	}
	a.logf("Teardown moved from %s to %s", prev.Format(time.RFC3339), next.Format(time.RFC3339))
	return &TTLStatus{RunID: rs.RunID, StartedAt: rs.CreatedAt, ExpiresAt: next, Remaining: teardown.Remaining(next, a.now())}, nil
}

// Cost prices the current run with the region's price table.
func (a *AWS) Cost(ctx context.Context, ec2c cost.EC2API) (*cost.Report, error) {
	rs, err := a.loadRun()
	if err != nil {
		return nil, err
	}
	if rs == nil {
		return nil, ErrNoActiveRun
	}
	table, err := cost.LoadTable(a.Root, rs.Region)
	if err != nil {
		return nil, err
	}
	return cost.RunCost(ctx, ec2c, table, rs.RunID, rs.CreatedAt, a.now())
}

// CheckIPEgress asks checkip.amazonaws.com for the caller's public IPv4 address.
func CheckIPEgress(client *http.Client) func(ctx context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://checkip.amazonaws.com", nil)
		if err != nil {
			return "", err
		}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer func() { _ = resp.Body.Close() }()
		b, err := io.ReadAll(io.LimitReader(resp.Body, 64))
		return strings.TrimSpace(string(b)), err
	}
}
