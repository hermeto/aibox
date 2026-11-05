package platform

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"github.com/hermeto/aibox/internal/exec"
	"github.com/hermeto/aibox/internal/kube"
	"github.com/hermeto/aibox/internal/models"
	"github.com/hermeto/aibox/internal/profile"
)

// Pinned Argo CD installation (see specs/003-local-platform/research.md).
const (
	ArgoCDNamespace    = "argocd"
	ArgoCDRelease      = "argocd"
	ArgoCDChart        = "argo-cd"
	ArgoCDChartRepo    = "https://argoproj.github.io/argo-helm"
	ArgoCDChartVersion = "10.12.1"
	RootAppName        = "aibox-root"
)

// KubeAPI is the subset of internal/kube used by the orchestration (faked in tests).
type KubeAPI interface {
	ServerVersion() (string, error)
	Apply(ctx context.Context, gvr schema.GroupVersionResource, obj *unstructured.Unstructured) error
	ListApplications(ctx context.Context, ns string) ([]kube.AppStatus, error)
	NotReadyWorkloads(ctx context.Context, ns string) ([]string, int, error)
	PodSummary(ctx context.Context, ns string) ([]string, error)
}

// Endpoint is a URL printed after bring-up.
type Endpoint struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// LocalEndpoints are fixed by infra/local/kind-config.yaml port mappings.
var LocalEndpoints = []Endpoint{
	{Name: "inference", URL: "http://127.0.0.1:30080/v1"},
	{Name: "argocd", URL: "http://127.0.0.1:30081"},
}

// LocalMetricsURL serves the local model server's Prometheus metrics (NodePort 30082); the
// gateway on 30080 exposes only the OpenAI API.
const LocalMetricsURL = "http://127.0.0.1:30082/metrics"

// Component is one child Application of the app-of-apps.
type Component struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	SyncWave  int    `json:"syncWave"`
	Path      string `json:"path,omitempty"`
	// RequiresModels components deploy only after the profile's weights are mirrored.
	RequiresModels bool `json:"requiresModels,omitempty"`
}

// Local brings the local (kind) profile up and down.
type Local struct {
	Profile *profile.Profile
	Runner  exec.Runner
	// Root is the repository root (manifests are read from here).
	Root string
	// Kube connects to a kubeconfig context.
	Kube func(kubeContext string) (KubeAPI, error)
	// Log receives progress lines (stderr in the CLI).
	Log io.Writer
	// DryRun prints actions instead of changing the cluster.
	DryRun bool
	// PollInterval between readiness checks (default 5s).
	PollInterval time.Duration
	// StateDir keeps the CLI's gateway key (default <Root>/.aibox/<profile>).
	StateDir string
}

// StateDirectory returns where the profile's local state (the gateway key) lives.
func (l *Local) StateDirectory() string {
	if l.StateDir != "" {
		return l.StateDir
	}
	return filepath.Join(l.Root, ".aibox", l.Profile.Metadata.Name)
}

// UpOptions configure a bring-up.
type UpOptions struct {
	RepoURL      string
	Revision     string
	FromWorktree bool
	Timeout      time.Duration
}

// UpResult is printed after a successful bring-up.
type UpResult struct {
	Profile        string     `json:"profile"`
	Cluster        string     `json:"cluster"`
	ClusterCreated bool       `json:"clusterCreated"`
	Mode           string     `json:"mode"`
	Source         *GitSource `json:"source,omitempty"`
	Endpoints      []Endpoint `json:"endpoints"`
}

// RenderText prints the endpoints.
func (r *UpResult) RenderText(w io.Writer) error {
	var b strings.Builder
	fmt.Fprintf(&b, "\nAIBox is up (profile %s, cluster %s, mode %s).\n\nEndpoints:\n", r.Profile, r.Cluster, r.Mode)
	for _, e := range r.Endpoints {
		fmt.Fprintf(&b, "  %-10s %s\n", e.Name, e.URL)
	}
	b.WriteString("\nTry: curl -s http://127.0.0.1:30080/v1/chat/completions -H 'Content-Type: application/json' \\\n" +
		"       -d '{\"model\":\"sim-llama-8b\",\"messages\":[{\"role\":\"user\",\"content\":\"Hello\"}]}'\n")
	b.WriteString("Argo CD login: admin / `kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d`\n")
	_, err := io.WriteString(w, b.String())
	return err
}

func (l *Local) gitops() *gitops {
	return &gitops{Runner: l.Runner, Root: l.Root, Log: l.Log, ProfileName: l.Profile.Metadata.Name, KubeContext: l.KubeContext(), PollInterval: l.PollInterval,
		WeightsBucket: models.LocalTarget().Bucket, Region: models.LocalTarget().Region}
}

// Components reads the app-of-apps components for the profile, sorted by sync wave.
func (l *Local) Components() ([]Component, error) {
	return LoadComponents(l.Root, l.Profile.Metadata.Name)
}

// KubeContext is the kubeconfig context kind creates for the profile's cluster.
func (l *Local) KubeContext() string { return "kind-" + l.Profile.Spec.Cluster.Name }

func (l *Local) logf(format string, args ...any) {
	if l.Log != nil {
		_, _ = fmt.Fprintf(l.Log, format+"\n", args...)
	}
}

// Up brings the platform up. It is idempotent.
func (l *Local) Up(ctx context.Context, opts UpOptions) (*UpResult, error) {
	if l.Profile.Spec.Provider != profile.ProviderKind {
		return nil, fmt.Errorf("profile %s is not a kind profile", l.Profile.Metadata.Name)
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Minute
	}
	kindConfig := filepath.Join(l.Root, "infra", "local", "kind-config.yaml")
	if err := checkKubernetesVersion(kindConfig, l.Profile.Spec.Cluster.KubernetesVersion); err != nil {
		return nil, err
	}
	comps, err := l.Components()
	if err != nil {
		return nil, err
	}
	res := &UpResult{Profile: l.Profile.Metadata.Name, Cluster: l.Profile.Spec.Cluster.Name, Endpoints: LocalEndpoints, Mode: "git"}
	if opts.FromWorktree {
		res.Mode = "worktree"
	} else {
		src, err := DetectGitSource(ctx, l.Runner, l.Root, opts.RepoURL, opts.Revision)
		if err != nil {
			return nil, err
		}
		if src.Warning != "" {
			l.logf("warning: %s", src.Warning)
		}
		res.Source = &src
		l.logf("Argo CD will track %s at %s", src.RepoURL, src.Revision)
	}

	created, err := l.ensureCluster(ctx, kindConfig)
	if err != nil {
		return nil, err
	}
	res.ClusterCreated = created

	deadline := time.Now().Add(opts.Timeout)
	wctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	g := l.gitops()
	if !l.DryRun {
		if g.APIKey, err = LoadOrCreateAPIKey(l.StateDirectory()); err != nil {
			return nil, err
		}
	}
	if err := g.installArgoCD(wctx); err != nil {
		return nil, err
	}
	if l.DryRun {
		if res.Source != nil {
			obj := RootApplication(*res.Source, l.Profile.Metadata.Name, RootParams{})
			out, _ := yaml.Marshal(obj.Object)
			l.logf("[dry-run] would apply:\n%s", out)
		} else {
			l.logf("[dry-run] would render and apply %d component(s) from the working tree", len(comps))
		}
		return res, nil
	}
	k, err := l.Kube(l.KubeContext())
	if err != nil {
		return nil, err
	}
	if err := g.waitWorkloads(wctx, k, ArgoCDNamespace); err != nil {
		return nil, fmt.Errorf("argo cd not ready: %w", err)
	}
	if opts.FromWorktree {
		if err := l.applyWorktree(wctx, g, k, comps); err != nil {
			return nil, err
		}
		return res, nil
	}
	if err := g.bootstrap(wctx, k, *res.Source, comps); err != nil {
		return nil, err
	}
	return res, nil
}

// Down deletes the kind cluster. It reports whether a cluster was deleted.
func (l *Local) Down(ctx context.Context) (bool, error) {
	exists, err := l.clusterExists(ctx)
	if err != nil {
		return false, err
	}
	if !exists {
		l.logf("Cluster %s does not exist: nothing to delete", l.Profile.Spec.Cluster.Name)
		return false, nil
	}
	l.logf("Deleting kind cluster %s", l.Profile.Spec.Cluster.Name)
	_, err = l.Runner.Run(ctx, exec.Cmd{Name: "kind", Args: []string{"delete", "cluster", "--name", l.Profile.Spec.Cluster.Name}, Stream: l.Log})
	return err == nil, err
}

func (l *Local) clusterExists(ctx context.Context) (bool, error) {
	res, err := l.Runner.Run(ctx, exec.Cmd{Name: "kind", Args: []string{"get", "clusters"}})
	if err != nil {
		return false, fmt.Errorf("kind get clusters: %w", err)
	}
	for _, line := range strings.Split(res.Stdout, "\n") {
		if strings.TrimSpace(line) == l.Profile.Spec.Cluster.Name {
			return true, nil
		}
	}
	return false, nil
}

func (l *Local) ensureCluster(ctx context.Context, kindConfig string) (bool, error) {
	exists, err := l.clusterExists(ctx)
	if err != nil {
		return false, err
	}
	name := l.Profile.Spec.Cluster.Name
	if exists {
		l.logf("Reusing existing kind cluster %s", name)
		if l.DryRun {
			return false, nil
		}
		k, err := l.Kube(l.KubeContext())
		if err == nil {
			_, err = k.ServerVersion()
		}
		if err != nil {
			return false, fmt.Errorf("cluster %s exists but its API server is unreachable (run `aibox down --profile %s` first): %w", name, l.Profile.Metadata.Name, err)
		}
		return false, nil
	}
	l.logf("Creating kind cluster %s", name)
	_, err = l.Runner.Run(ctx, exec.Cmd{Name: "kind", Args: []string{"create", "cluster", "--name", name, "--config", kindConfig, "--wait", "120s"}, Stream: l.Log})
	if err != nil {
		return false, fmt.Errorf("create kind cluster: %w", err)
	}
	return true, nil
}

func (l *Local) applyWorktree(ctx context.Context, g *gitops, k KubeAPI, comps []Component) error {
	for _, c := range comps {
		l.logf("Applying %s from the working tree (%s → namespace %s)", c.Name, c.Path, c.Namespace)
		rendered, err := l.Runner.Run(ctx, exec.Cmd{Name: "kubectl", Args: []string{"kustomize", "--enable-helm", filepath.Join(l.Root, filepath.FromSlash(c.Path))}})
		if err != nil {
			return fmt.Errorf("render %s: %w", c.Name, err)
		}
		var manifest bytes.Buffer
		fmt.Fprintf(&manifest, "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: %s\n---\n%s", c.Namespace, rendered.Stdout)
		_, err = l.Runner.Run(ctx, exec.Cmd{Name: "kubectl", Args: []string{
			"--context", l.KubeContext(), "apply", "--server-side", "--force-conflicts", "--field-manager=" + kube.FieldManager,
			"--namespace", c.Namespace, "-f", "-",
		}, Stdin: &manifest})
		if err != nil {
			return fmt.Errorf("apply %s: %w", c.Name, err)
		}
		if err := g.waitWorkloads(ctx, k, c.Namespace); err != nil {
			return err
		}
	}
	return nil
}

// checkKubernetesVersion verifies that every node image in the kind config matches the
// profile's MAJOR.MINOR Kubernetes version.
func checkKubernetesVersion(kindConfig, want string) error {
	data, err := os.ReadFile(kindConfig) //nolint:gosec // repository file
	if err != nil {
		return fmt.Errorf("read kind config: %w", err)
	}
	var cfg struct {
		Nodes []struct {
			Image string `json:"image"`
		} `json:"nodes"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse kind config: %w", err)
	}
	for _, n := range cfg.Nodes {
		tag := n.Image
		if i := strings.LastIndex(tag, ":v"); i >= 0 {
			tag = tag[i+2:]
		}
		if !strings.HasPrefix(tag, want+".") {
			return errors.New("kind node image " + n.Image + " does not match profile kubernetesVersion " + want)
		}
	}
	return nil
}
