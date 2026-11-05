package platform

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"github.com/hermeto/aibox/internal/exec"
	"github.com/hermeto/aibox/internal/kube"
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

// Component is one child Application of the app-of-apps.
type Component struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	SyncWave  int    `json:"syncWave"`
	Path      string `json:"path,omitempty"`
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

// KubeContext is the kubeconfig context kind creates for the profile's cluster.
func (l *Local) KubeContext() string { return "kind-" + l.Profile.Spec.Cluster.Name }

func (l *Local) logf(format string, args ...any) {
	if l.Log != nil {
		_, _ = fmt.Fprintf(l.Log, format+"\n", args...)
	}
}

func (l *Local) poll() time.Duration {
	if l.PollInterval > 0 {
		return l.PollInterval
	}
	return 5 * time.Second
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

	if err := l.installArgoCD(wctx); err != nil {
		return nil, err
	}
	if l.DryRun {
		if res.Source != nil {
			obj := RootApplication(*res.Source, l.Profile.Metadata.Name)
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
	if err := l.waitWorkloads(wctx, k, ArgoCDNamespace); err != nil {
		return nil, fmt.Errorf("argo cd not ready: %w", err)
	}
	if opts.FromWorktree {
		if err := l.applyWorktree(wctx, k, comps); err != nil {
			return nil, err
		}
		return res, nil
	}
	l.logf("Applying root Application %s", RootAppName)
	if err := k.Apply(wctx, kube.Applications, RootApplication(*res.Source, l.Profile.Metadata.Name)); err != nil {
		return nil, err
	}
	if err := l.waitApplications(wctx, k, comps); err != nil {
		l.diagnostics(ctx, k, comps)
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

func (l *Local) installArgoCD(ctx context.Context) error {
	values := filepath.Join(l.Root, "platform", "bootstrap", "argocd", "values-"+l.Profile.Metadata.Name+".yaml")
	l.logf("Installing Argo CD (chart %s %s)", ArgoCDChart, ArgoCDChartVersion)
	_, err := l.Runner.Run(ctx, exec.Cmd{Name: "helm", Args: []string{
		"upgrade", "--install", ArgoCDRelease, ArgoCDChart,
		"--repo", ArgoCDChartRepo, "--version", ArgoCDChartVersion,
		"--namespace", ArgoCDNamespace, "--create-namespace",
		"--kube-context", l.KubeContext(),
		"--values", values,
	}})
	if err != nil {
		return fmt.Errorf("install argo cd: %w", err)
	}
	return nil
}

// RootApplication renders the app-of-apps root for a profile.
func RootApplication(src GitSource, profileName string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata": map[string]any{
			"name":      RootAppName,
			"namespace": ArgoCDNamespace,
			"labels":    map[string]any{"app.kubernetes.io/part-of": "aibox", "aibox.dev/profile": profileName},
		},
		"spec": map[string]any{
			"project": "default",
			"source": map[string]any{
				"repoURL":        src.RepoURL,
				"targetRevision": src.Revision,
				"path":           "platform/bootstrap",
				"helm": map[string]any{
					"releaseName": RootAppName,
					"valueFiles":  []any{"values-" + profileName + ".yaml"},
					"parameters": []any{
						map[string]any{"name": "repoURL", "value": src.RepoURL},
						map[string]any{"name": "targetRevision", "value": src.Revision},
					},
				},
			},
			"destination": map[string]any{"server": "https://kubernetes.default.svc", "namespace": ArgoCDNamespace},
			"syncPolicy": map[string]any{
				"automated": map[string]any{"prune": true, "selfHeal": true},
			},
		},
	}}
}

// Components reads the app-of-apps components for the profile, sorted by sync wave.
func (l *Local) Components() ([]Component, error) {
	path := filepath.Join(l.Root, "platform", "bootstrap", "values-"+l.Profile.Metadata.Name+".yaml")
	data, err := os.ReadFile(path) //nolint:gosec // repository file
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var v struct {
		Apps []Component `json:"apps"`
	}
	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for i := range v.Apps {
		if v.Apps[i].Path == "" {
			v.Apps[i].Path = filepath.ToSlash(filepath.Join("platform", v.Apps[i].Name, "overlays", l.Profile.Metadata.Name))
		}
	}
	sort.SliceStable(v.Apps, func(i, j int) bool { return v.Apps[i].SyncWave < v.Apps[j].SyncWave })
	return v.Apps, nil
}

func (l *Local) waitWorkloads(ctx context.Context, k KubeAPI, ns string) error {
	l.logf("Waiting for workloads in namespace %s", ns)
	var last []string
	for {
		notReady, total, err := k.NotReadyWorkloads(ctx, ns)
		if err == nil && total > 0 && len(notReady) == 0 {
			return nil
		}
		if err == nil {
			last = notReady
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for %s (not ready: %s): %w", ns, strings.Join(last, ", "), ctx.Err())
		case <-time.After(l.poll()):
		}
	}
}

func (l *Local) waitApplications(ctx context.Context, k KubeAPI, comps []Component) error {
	want := map[string]bool{RootAppName: true}
	for _, c := range comps {
		want[c.Name] = true
	}
	l.logf("Waiting for %d Argo CD Application(s) to be Synced and Healthy", len(want))
	var notReady []string
	for {
		apps, err := k.ListApplications(ctx, ArgoCDNamespace)
		if err == nil {
			notReady = notReady[:0]
			seen := map[string]bool{}
			for _, a := range apps {
				if !want[a.Name] {
					continue
				}
				seen[a.Name] = true
				if !a.Ready() {
					notReady = append(notReady, a.String())
				}
			}
			for name := range want {
				if !seen[name] {
					notReady = append(notReady, name+": not created yet")
				}
			}
			sort.Strings(notReady)
			if len(notReady) == 0 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for Argo CD Applications:\n  %s", strings.Join(notReady, "\n  "))
		case <-time.After(l.poll()):
		}
	}
}

func (l *Local) applyWorktree(ctx context.Context, k KubeAPI, comps []Component) error {
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
		if err := l.waitWorkloads(ctx, k, c.Namespace); err != nil {
			return err
		}
	}
	return nil
}

func (l *Local) diagnostics(ctx context.Context, k KubeAPI, comps []Component) {
	dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	l.logf("--- diagnostics ---")
	if apps, err := k.ListApplications(dctx, ArgoCDNamespace); err == nil {
		for _, a := range apps {
			l.logf("application %s", a)
		}
	}
	namespaces := []string{ArgoCDNamespace}
	for _, c := range comps {
		namespaces = append(namespaces, c.Namespace)
	}
	for _, ns := range namespaces {
		if pods, err := k.PodSummary(dctx, ns); err == nil {
			for _, p := range pods {
				l.logf("pod %s", p)
			}
		}
	}
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
