package platform

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/hermeto/aibox/internal/exec"
	"github.com/hermeto/aibox/internal/kube"
	"github.com/hermeto/aibox/internal/serving"
)

// gitops installs Argo CD and drives the app-of-apps for any profile.
type gitops struct {
	Runner      exec.Runner
	Root        string
	Log         io.Writer
	ProfileName string
	KubeContext string
	// RunID tags Karpenter-launched instances (cloud profiles only).
	RunID string
	// Kubeconfig is an explicit kubeconfig file ("" = default loading rules).
	Kubeconfig   string
	PollInterval time.Duration
	// ModelsReady deploys the components that need mirrored weights (cloud model servers).
	ModelsReady bool
	// WeightsBucket and Region are written to the run ConfigMap the model servers read.
	WeightsBucket string
	Region        string
	// APIKey is the CLI's gateway key, applied into the gateway's API key Secret.
	APIKey string
	// TLSCertificateArn and SourceRanges configure the cloud gateway load balancer.
	TLSCertificateArn string
	SourceRanges      []string
}

// active returns the components the root Application deploys in the current state.
func (g *gitops) active(comps []Component) []Component {
	var out []Component
	for _, c := range comps {
		if !c.RequiresModels || g.ModelsReady {
			out = append(out, c)
		}
	}
	return out
}

// applyRunConfig writes the aibox-run ConfigMap (run id, weights bucket, region) into the
// serving namespace; model servers read the bucket from it at runtime, so no account data
// ends up in git.
func (g *gitops) applyRunConfig(ctx context.Context, k KubeAPI) error {
	ns := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Namespace",
		"metadata": map[string]any{"name": serving.Namespace, "labels": map[string]any{"app.kubernetes.io/part-of": "aibox"}},
	}}
	if err := k.Apply(ctx, kube.Namespaces, ns); err != nil {
		return err
	}
	cm := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "ConfigMap",
		"metadata": map[string]any{"name": serving.RunConfigMap, "namespace": serving.Namespace,
			"labels": map[string]any{"app.kubernetes.io/part-of": "aibox"}},
		"data": map[string]any{
			"profile": g.ProfileName, "runId": g.RunID, "weightsBucket": g.WeightsBucket, "region": g.Region,
		},
	}}
	if err := k.Apply(ctx, kube.ConfigMaps, cm); err != nil {
		return err
	}
	if g.APIKey == "" {
		return nil
	}
	gwns := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Namespace",
		"metadata": map[string]any{"name": serving.GatewayNamespace, "labels": map[string]any{"app.kubernetes.io/part-of": "aibox"}},
	}}
	if err := k.Apply(ctx, kube.Namespaces, gwns); err != nil {
		return err
	}
	// Server-side apply owns only the CLI's entry; keys merged from the secret store stay.
	secret := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]any{"name": APIKeySecret, "namespace": serving.GatewayNamespace,
			"labels": map[string]any{"app.kubernetes.io/part-of": "aibox"}},
		"type": "Opaque",
		"data": map[string]any{CLIClientID: base64.StdEncoding.EncodeToString([]byte(g.APIKey))},
	}}
	return k.Apply(ctx, kube.Secrets, secret)
}

func (g *gitops) logf(format string, args ...any) {
	if g.Log != nil {
		_, _ = fmt.Fprintf(g.Log, format+"\n", args...)
	}
}

func (g *gitops) poll() time.Duration {
	if g.PollInterval > 0 {
		return g.PollInterval
	}
	return 5 * time.Second
}

// bootstrap applies the root Application and waits for every Application to be ready.
func (g *gitops) bootstrap(ctx context.Context, k KubeAPI, src GitSource, comps []Component) error {
	if err := g.applyRunConfig(ctx, k); err != nil {
		return fmt.Errorf("run config: %w", err)
	}
	g.logf("Applying root Application %s", RootAppName)
	if err := k.Apply(ctx, kube.Applications, RootApplication(src, g.ProfileName, RootParams{
		RunID: g.RunID, ModelsReady: g.ModelsReady, TLSCertificateArn: g.TLSCertificateArn, SourceRanges: g.SourceRanges,
	})); err != nil {
		return err
	}
	comps = g.active(comps)
	if err := g.waitApplications(ctx, k, comps); err != nil {
		g.diagnostics(ctx, k, comps)
		return err
	}
	return nil
}

func (g *gitops) installArgoCD(ctx context.Context) error {
	values := filepath.Join(g.Root, "platform", "bootstrap", "argocd", "values-"+g.ProfileName+".yaml")
	g.logf("Installing Argo CD (chart %s %s)", ArgoCDChart, ArgoCDChartVersion)
	args := []string{
		"upgrade", "--install", ArgoCDRelease, ArgoCDChart,
		"--repo", ArgoCDChartRepo, "--version", ArgoCDChartVersion,
		"--namespace", ArgoCDNamespace, "--create-namespace",
		"--kube-context", g.KubeContext,
		"--values", values,
	}
	if g.Kubeconfig != "" {
		args = append(args, "--kubeconfig", g.Kubeconfig)
	}
	_, err := g.Runner.Run(ctx, exec.Cmd{Name: "helm", Args: args})
	if err != nil {
		return fmt.Errorf("install argo cd: %w", err)
	}
	return nil
}

// RootParams are the run-specific bootstrap chart parameters.
type RootParams struct {
	// RunID (cloud profiles) tags Karpenter-launched instances.
	RunID string
	// ModelsReady deploys components that need mirrored weights.
	ModelsReady bool
	// TLSCertificateArn and SourceRanges configure the cloud gateway load balancer.
	TLSCertificateArn string
	SourceRanges      []string
}

// RootApplication renders the app-of-apps root for a profile.
func RootApplication(src GitSource, profileName string, rp RootParams) *unstructured.Unstructured {
	params := []any{
		map[string]any{"name": "repoURL", "value": src.RepoURL},
		map[string]any{"name": "targetRevision", "value": src.Revision},
	}
	if rp.RunID != "" {
		params = append(params, map[string]any{"name": "runId", "value": rp.RunID})
	}
	if rp.ModelsReady {
		params = append(params, map[string]any{"name": "modelsReady", "value": "true"})
	}
	if rp.TLSCertificateArn != "" {
		params = append(params, map[string]any{"name": "gateway.tlsCertificateArn", "value": rp.TLSCertificateArn})
	}
	for i, r := range rp.SourceRanges {
		params = append(params, map[string]any{"name": fmt.Sprintf("gateway.sourceRanges[%d]", i), "value": r})
	}
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
					"parameters":  params,
				},
			},
			"destination": map[string]any{"server": "https://kubernetes.default.svc", "namespace": ArgoCDNamespace},
			"syncPolicy": map[string]any{
				"automated": map[string]any{"prune": true, "selfHeal": true},
			},
		},
	}}
}

// LoadComponents reads the app-of-apps components for a profile, sorted by sync wave.
func LoadComponents(root, profileName string) ([]Component, error) {
	path := filepath.Join(root, "platform", "bootstrap", "values-"+profileName+".yaml")
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
			v.Apps[i].Path = filepath.ToSlash(filepath.Join("platform", v.Apps[i].Name, "overlays", profileName))
		}
	}
	sort.SliceStable(v.Apps, func(i, j int) bool { return v.Apps[i].SyncWave < v.Apps[j].SyncWave })
	return v.Apps, nil
}

func (g *gitops) waitWorkloads(ctx context.Context, k KubeAPI, ns string) error {
	g.logf("Waiting for workloads in namespace %s", ns)
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
		case <-time.After(g.poll()):
		}
	}
}

func (g *gitops) waitApplications(ctx context.Context, k KubeAPI, comps []Component) error {
	want := map[string]bool{RootAppName: true}
	for _, c := range comps {
		want[c.Name] = true
	}
	g.logf("Waiting for %d Argo CD Application(s) to be Synced and Healthy", len(want))
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
		case <-time.After(g.poll()):
		}
	}
}

func (g *gitops) diagnostics(ctx context.Context, k KubeAPI, comps []Component) {
	dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	g.logf("--- diagnostics ---")
	if apps, err := k.ListApplications(dctx, ArgoCDNamespace); err == nil {
		for _, a := range apps {
			g.logf("application %s", a)
		}
	}
	namespaces := []string{ArgoCDNamespace}
	for _, c := range comps {
		namespaces = append(namespaces, c.Namespace)
	}
	for _, ns := range namespaces {
		if pods, err := k.PodSummary(dctx, ns); err == nil {
			for _, p := range pods {
				g.logf("pod %s", p)
			}
		}
	}
}
