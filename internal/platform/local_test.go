package platform

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/hermeto/aibox/internal/exec"
	"github.com/hermeto/aibox/internal/kube"
	"github.com/hermeto/aibox/internal/profile"
)

type fakeKube struct {
	versionErr error
	applied    []*unstructured.Unstructured
	apps       []kube.AppStatus
	notReady   []string
}

func (f *fakeKube) ServerVersion() (string, error) { return "v1.37.0", f.versionErr }
func (f *fakeKube) Apply(_ context.Context, _ schema.GroupVersionResource, obj *unstructured.Unstructured) error {
	f.applied = append(f.applied, obj)
	return nil
}
func (f *fakeKube) ListApplications(context.Context, string) ([]kube.AppStatus, error) {
	return f.apps, nil
}
func (f *fakeKube) NotReadyWorkloads(context.Context, string) ([]string, int, error) {
	return f.notReady, 3, nil
}
func (f *fakeKube) PodSummary(context.Context, string) ([]string, error) {
	return []string{"argocd/argocd-server-0 Running"}, nil
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func localProfile(t *testing.T) *profile.Profile {
	t.Helper()
	t.Setenv(profile.EnvProfilesDir, filepath.Join(repoRoot(t), "profiles"))
	p, err := profile.Load("local")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func gitFake() *exec.Fake {
	f := exec.NewFake()
	f.On("git remote get-url origin", exec.Result{Stdout: "git@github.com:hermeto/aibox.git\n"}, nil)
	f.On("git rev-parse --abbrev-ref HEAD", exec.Result{Stdout: "003-local-platform\n"}, nil)
	f.On("git rev-list --count @{u}..HEAD", exec.Result{Stdout: "2\n"}, nil)
	f.OnPrefix("helm upgrade --install argocd argo-cd", exec.Result{}, nil)
	f.OnPrefix("kind create cluster --name aibox-local", exec.Result{}, nil)
	return f
}

func readyApps() []kube.AppStatus {
	var apps []kube.AppStatus
	for _, n := range []string{RootAppName, "serving", "karpenter", "gpu", "secrets", "models"} {
		apps = append(apps, kube.AppStatus{Name: n, Sync: "Synced", Health: "Healthy"})
	}
	return apps
}

func newLocal(t *testing.T, r exec.Runner, k *fakeKube) (*Local, *bytes.Buffer) {
	t.Helper()
	var log bytes.Buffer
	return &Local{
		Profile: localProfile(t), Runner: r, Root: repoRoot(t), Log: &log, PollInterval: time.Millisecond,
		Kube: func(string) (KubeAPI, error) { return k, nil },
	}, &log
}

func TestUpCreatesClusterAndAppliesRoot(t *testing.T) {
	r := gitFake().On("kind get clusters", exec.Result{Stdout: "other\n"}, nil)
	k := &fakeKube{apps: readyApps()}
	l, log := newLocal(t, r, k)
	res, err := l.Up(context.Background(), UpOptions{Timeout: time.Second})
	if err != nil {
		t.Fatalf("Up: %v\n%s", err, log)
	}
	if !res.ClusterCreated || res.Source.RepoURL != "https://github.com/hermeto/aibox.git" || res.Source.Revision != "003-local-platform" {
		t.Fatalf("result %+v %+v", res, res.Source)
	}
	if !strings.Contains(log.String(), "2 local commit(s) are not pushed") {
		t.Fatalf("missing unpushed warning:\n%s", log)
	}
	if len(k.applied) != 1 || k.applied[0].GetName() != RootAppName {
		t.Fatalf("applied %v", k.applied)
	}
	params, _, _ := unstructured.NestedSlice(k.applied[0].Object, "spec", "source", "helm", "parameters")
	if len(params) != 2 {
		t.Fatalf("helm parameters %v", params)
	}
	var helmCall string
	for _, c := range r.CommandLines() {
		if strings.HasPrefix(c, "helm ") {
			helmCall = c
		}
	}
	for _, want := range []string{"--version 10.12.1", "--kube-context kind-aibox-local", "values-local.yaml"} {
		if !strings.Contains(helmCall, want) {
			t.Fatalf("helm call %q lacks %q", helmCall, want)
		}
	}
	var txt bytes.Buffer
	if err := res.RenderText(&txt); err != nil || !strings.Contains(txt.String(), "http://127.0.0.1:30080/v1") {
		t.Fatalf("render: %q %v", txt.String(), err)
	}
}

func TestUpReusesClusterIdempotently(t *testing.T) {
	r := gitFake().On("kind get clusters", exec.Result{Stdout: "aibox-local\n"}, nil)
	k := &fakeKube{apps: readyApps()}
	l, log := newLocal(t, r, k)
	res, err := l.Up(context.Background(), UpOptions{Timeout: time.Second, RepoURL: "https://example.com/x/y.git", Revision: "abc"})
	if err != nil || res.ClusterCreated || !strings.Contains(log.String(), "Reusing existing kind cluster") {
		t.Fatalf("%v %+v\n%s", err, res, log)
	}
	for _, c := range r.CommandLines() {
		if strings.HasPrefix(c, "kind create") || strings.HasPrefix(c, "git ") {
			t.Fatalf("unexpected call %q", c)
		}
	}
}

func TestUpUnreachableExistingCluster(t *testing.T) {
	r := gitFake().On("kind get clusters", exec.Result{Stdout: "aibox-local\n"}, nil)
	l, _ := newLocal(t, r, &fakeKube{versionErr: errors.New("connection refused")})
	_, err := l.Up(context.Background(), UpOptions{Timeout: time.Second, RepoURL: "https://e/x/y.git", Revision: "r"})
	if err == nil || !strings.Contains(err.Error(), "aibox down --profile local") {
		t.Fatalf("got %v", err)
	}
}

func TestUpTimeoutNamesApplication(t *testing.T) {
	r := gitFake().On("kind get clusters", exec.Result{}, nil)
	k := &fakeKube{apps: []kube.AppStatus{{Name: RootAppName, Sync: "Synced", Health: "Progressing"}, {Name: "serving", Sync: "OutOfSync", Health: "Missing", Message: "repo not found"}}}
	l, log := newLocal(t, r, k)
	_, err := l.Up(context.Background(), UpOptions{Timeout: 50 * time.Millisecond, RepoURL: "https://e/x/y.git", Revision: "r"})
	if err == nil || !strings.Contains(err.Error(), "serving: sync=OutOfSync health=Missing (repo not found)") {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(log.String(), "--- diagnostics ---") || !strings.Contains(log.String(), "pod argocd/argocd-server-0 Running") {
		t.Fatalf("diagnostics missing:\n%s", log)
	}
}

func TestUpArgoCDNotReady(t *testing.T) {
	r := gitFake().On("kind get clusters", exec.Result{}, nil)
	l, _ := newLocal(t, r, &fakeKube{notReady: []string{"deployment/argocd-server"}})
	_, err := l.Up(context.Background(), UpOptions{Timeout: 30 * time.Millisecond, RepoURL: "https://e/x/y.git", Revision: "r"})
	if err == nil || !strings.Contains(err.Error(), "deployment/argocd-server") {
		t.Fatalf("got %v", err)
	}
}

func TestUpFromWorktree(t *testing.T) {
	r := gitFake().On("kind get clusters", exec.Result{}, nil).
		OnPrefix("kubectl kustomize --enable-helm", exec.Result{Stdout: "apiVersion: v1\nkind: Service\nmetadata:\n  name: x\n"}, nil).
		OnPrefix("kubectl --context kind-aibox-local apply --server-side", exec.Result{}, nil)
	k := &fakeKube{}
	l, _ := newLocal(t, r, k)
	res, err := l.Up(context.Background(), UpOptions{Timeout: time.Second, FromWorktree: true})
	if err != nil || res.Mode != "worktree" || res.Source != nil || len(k.applied) != 0 {
		t.Fatalf("%v %+v", err, res)
	}
	var stdin string
	for _, c := range r.Calls {
		if c.Stdin != nil {
			b := new(bytes.Buffer)
			_, _ = b.ReadFrom(c.Stdin)
			stdin = b.String()
		}
	}
	if !strings.Contains(stdin, "kind: Namespace") || !strings.Contains(stdin, "name: aibox-serving") {
		t.Fatalf("apply stdin %q", stdin)
	}
}

func TestUpDryRunAppliesNothing(t *testing.T) {
	inner := gitFake().On("kind get clusters", exec.Result{}, nil)
	var printed bytes.Buffer
	r := exec.DryRun{Inner: inner, Out: &printed, ReadOnly: exec.ReadOnly}
	l, log := newLocal(t, r, nil)
	l.DryRun = true
	l.Kube = func(string) (KubeAPI, error) { t.Fatal("dry-run must not connect"); return nil, nil }
	if _, err := l.Up(context.Background(), UpOptions{Timeout: time.Second}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(printed.String(), "[dry-run] kind create cluster") || !strings.Contains(printed.String(), "[dry-run] helm upgrade") {
		t.Fatalf("printed %q", printed.String())
	}
	if !strings.Contains(log.String(), "kind: Application") {
		t.Fatalf("root application not shown:\n%s", log)
	}
}

func TestDown(t *testing.T) {
	r := exec.NewFake().On("kind get clusters", exec.Result{Stdout: "aibox-local\n"}, nil).
		On("kind delete cluster --name aibox-local", exec.Result{}, nil)
	l, _ := newLocal(t, r, nil)
	if deleted, err := l.Down(context.Background()); err != nil || !deleted {
		t.Fatalf("%v %v", deleted, err)
	}
	r = exec.NewFake().On("kind get clusters", exec.Result{Stdout: "No kind clusters found.\n"}, nil)
	l, log := newLocal(t, r, nil)
	if deleted, err := l.Down(context.Background()); err != nil || deleted || !strings.Contains(log.String(), "nothing to delete") {
		t.Fatalf("%v %v", deleted, err)
	}
	r = exec.NewFake()
	l, _ = newLocal(t, r, nil)
	if _, err := l.Down(context.Background()); err == nil {
		t.Fatal("expected kind error")
	}
}

func TestComponentsAndVersionCheck(t *testing.T) {
	l, _ := newLocal(t, exec.NewFake(), nil)
	comps, err := l.Components()
	if err != nil || len(comps) != 3 || comps[0].Name != "secrets" || comps[2].Path != "platform/serving/overlays/local" {
		t.Fatalf("%v %v", comps, err)
	}
	cfg := filepath.Join(repoRoot(t), "infra", "local", "kind-config.yaml")
	if err := checkKubernetesVersion(cfg, "1.37"); err != nil {
		t.Fatal(err)
	}
	if err := checkKubernetesVersion(cfg, "1.36"); err == nil {
		t.Fatal("expected mismatch")
	}
	if err := checkKubernetesVersion(filepath.Join(t.TempDir(), "nope.yaml"), "1.37"); err == nil {
		t.Fatal("expected read error")
	}
	bad := filepath.Join(t.TempDir(), "bad.yaml")
	_ = os.WriteFile(bad, []byte("nodes: [unclosed"), 0o600)
	if err := checkKubernetesVersion(bad, "1.37"); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestNotAKindProfile(t *testing.T) {
	t.Setenv(profile.EnvProfilesDir, filepath.Join(repoRoot(t), "profiles"))
	p, _ := profile.Load("aws-dev")
	l := &Local{Profile: p}
	if _, err := l.Up(context.Background(), UpOptions{}); err == nil {
		t.Fatal("expected error")
	}
}
