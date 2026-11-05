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

	"github.com/hermeto/aibox/internal/exec"
	"github.com/hermeto/aibox/internal/profile"
)

// TerraformDir is the Terraform root for AWS profiles, relative to the repository root.
const TerraformDir = "infra/terraform/aws"

// ErrNotConfirmed is returned when the operator declines (or cannot confirm) an apply.
var ErrNotConfirmed = errors.New("not confirmed (re-run with --yes to apply without a prompt)")

// RunState persists per-run identity between `up` and `down` (.aibox/<profile>/run.json).
type RunState struct {
	RunID       string    `json:"runId"`
	Owner       string    `json:"owner"`
	CreatedAt   time.Time `json:"createdAt"`
	Region      string    `json:"region"`
	StateBucket string    `json:"stateBucket"`
	StateKey    string    `json:"stateKey"`
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
	Kube         func(kubeconfig, kubeContext string) (KubeAPI, error)
	Getenv       func(string) string
	Now          func() time.Time
	PollInterval time.Duration
}

// AWSUpOptions configure an AWS bring-up.
type AWSUpOptions struct {
	RepoURL    string
	Revision   string
	AdminCIDRs []string
	Owner      string
	Timeout    time.Duration
}

// AWSUpResult is printed after an AWS bring-up.
type AWSUpResult struct {
	Profile    string          `json:"profile"`
	Cluster    string          `json:"cluster"`
	RunID      string          `json:"runId"`
	Region     string          `json:"region"`
	Kubeconfig string          `json:"kubeconfig"`
	Outputs    string          `json:"outputsFile"`
	Source     *GitSource      `json:"source,omitempty"`
	Endpoints  []Endpoint      `json:"endpoints"`
	Raw        json.RawMessage `json:"-"`
}

// RenderText prints where to find the cluster.
func (r *AWSUpResult) RenderText(w io.Writer) error {
	_, err := fmt.Fprintf(w, "\nAIBox is up (profile %s, cluster %s, run %s, region %s).\n\nKubeconfig: %s (context aibox-%s)\nOutputs:    %s\nArgo CD:    kubectl --kubeconfig %s -n argocd port-forward svc/argocd-server 8080:80\n",
		r.Profile, r.Cluster, r.RunID, r.Region, r.Kubeconfig, r.Profile, r.Outputs, r.Kubeconfig)
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

func (a *AWS) tf(ctx context.Context, engine string, args ...string) (exec.Result, error) {
	full := append([]string{"-chdir=" + filepath.Join(a.Root, TerraformDir)}, args...)
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

func (a *AWS) stateBucket(ctx context.Context) (string, error) {
	if b := a.getenv("AIBOX_TF_STATE_BUCKET"); b != "" {
		return b, nil
	}
	if a.Account == nil {
		return "", errors.New("cannot determine the AWS account for the state bucket (set AIBOX_TF_STATE_BUCKET)")
	}
	acct, err := a.Account(ctx)
	if err != nil {
		return "", fmt.Errorf("determine AWS account: %w", err)
	}
	return fmt.Sprintf("aibox-tfstate-%s-%s", acct, a.Profile.Spec.AWS.Region), nil
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
	}
}

func (a *AWS) init(ctx context.Context, engine string, rs *RunState) error {
	_, err := a.tf(ctx, engine, "init", "-input=false", "-reconfigure",
		"-backend-config=bucket="+rs.StateBucket,
		"-backend-config=key="+rs.StateKey,
		"-backend-config=region="+rs.Region)
	if err != nil {
		return fmt.Errorf("%s init failed — if the state bucket %s does not exist, create it once with the bootstrap stack (docs/runbooks/aws-first-run.md): %w", engine, rs.StateBucket, err)
	}
	return nil
}

// Up creates (or converges) the AWS environment and bootstraps GitOps.
func (a *AWS) Up(ctx context.Context, opts AWSUpOptions) (*AWSUpResult, error) {
	if a.Profile.Spec.Provider != profile.ProviderAWS || a.Profile.Spec.AWS == nil {
		return nil, fmt.Errorf("profile %s is not an AWS profile", a.Profile.Metadata.Name)
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
	if rs == nil {
		bucket, err := a.stateBucket(ctx)
		if err != nil {
			return nil, err
		}
		rs = &RunState{
			RunID:       NewRunID(a.Profile.Metadata.Name, a.now()),
			Owner:       a.resolveOwner(ctx, opts.Owner),
			CreatedAt:   a.now().UTC(),
			Region:      a.Profile.Spec.AWS.Region,
			StateBucket: bucket,
			StateKey:    a.Profile.Metadata.Name + "/" + a.Profile.Spec.Cluster.Name + ".tfstate",
		}
	}
	a.logf("Run %s (owner %s, state s3://%s/%s)", rs.RunID, rs.Owner, rs.StateBucket, rs.StateKey)
	cidrs, err := a.adminCIDRs(ctx, opts.AdminCIDRs)
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
	if _, err := a.writeJSON("run.json", rs); err != nil {
		return nil, err
	}
	varsFile, err := a.writeJSON("terraform.tfvars.json", a.tfVars(rs, cidrs))
	if err != nil {
		return nil, err
	}
	if err := a.init(ctx, engine, rs); err != nil {
		return nil, err
	}
	planFile := filepath.Join(a.stateDir(), "plan.tfplan")
	if _, err := a.tf(ctx, engine, "plan", "-input=false", "-var-file="+varsFile, "-out="+planFile); err != nil {
		return nil, fmt.Errorf("%s plan: %w", engine, err)
	}
	if err := a.confirm(fmt.Sprintf("Apply this plan to AWS account for profile %s (resources cost money until `aibox down`)?", a.Profile.Metadata.Name)); err != nil {
		return nil, err
	}
	if _, err := a.tf(ctx, engine, "apply", "-input=false", planFile); err != nil {
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
		Outputs: outputsFile, Source: &src, Raw: json.RawMessage(out.Stdout)}
	if a.DryRun {
		a.logf("[dry-run] would write a kubeconfig, install Argo CD and apply the root Application")
		return res, nil
	}
	kubeconfig, err := a.writeKubeconfig(out.Stdout)
	if err != nil {
		return nil, err
	}
	res.Kubeconfig = kubeconfig
	if err := a.bootstrapGitOps(ctx, kubeconfig, src, opts.Timeout); err != nil {
		return nil, err
	}
	return res, nil
}

func (a *AWS) bootstrapGitOps(ctx context.Context, kubeconfig string, src GitSource, timeout time.Duration) error {
	comps, err := LoadComponents(a.Root, a.Profile.Metadata.Name)
	if err != nil {
		return err
	}
	wctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	g := &gitops{Runner: a.Runner, Root: a.Root, Log: a.Log, ProfileName: a.Profile.Metadata.Name, KubeContext: a.KubeContext(), Kubeconfig: kubeconfig, PollInterval: a.PollInterval}
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

// Down destroys the AWS environment of the current run.
func (a *AWS) Down(ctx context.Context) (bool, error) {
	rs, err := a.loadRun()
	if err != nil {
		return false, err
	}
	if rs == nil {
		a.logf("No run state in %s: nothing to destroy", a.stateDir())
		return false, nil
	}
	engine, err := a.engine()
	if err != nil {
		return false, err
	}
	if err := a.confirm(fmt.Sprintf("Destroy every resource of run %s (profile %s)?", rs.RunID, a.Profile.Metadata.Name)); err != nil {
		return false, err
	}
	if err := a.init(ctx, engine, rs); err != nil {
		return false, err
	}
	varsFile := filepath.Join(a.stateDir(), "terraform.tfvars.json")
	if _, err := a.tf(ctx, engine, "destroy", "-input=false", "-auto-approve", "-var-file="+varsFile); err != nil {
		return false, fmt.Errorf("%s destroy (re-run `aibox down` to retry): %w", engine, err)
	}
	if !a.DryRun {
		for _, f := range []string{"run.json", "plan.tfplan", "outputs.json", "kubeconfig"} {
			_ = os.Remove(filepath.Join(a.stateDir(), f))
		}
	}
	return true, nil
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
