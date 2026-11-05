package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/hermeto/aibox/internal/kube"
	"github.com/hermeto/aibox/internal/models"
)

// fakeJobs finishes every applied Job successfully and records the Jobs' environment.
type fakeJobs struct {
	jobs    map[string]kube.JobState
	buckets []string
}

func (f *fakeJobs) Apply(_ context.Context, gvr schema.GroupVersionResource, obj *unstructured.Unstructured) error {
	if gvr != kube.Jobs {
		return nil
	}
	containers, _, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
	env, _ := containers[0].(map[string]any)["env"].([]any)
	for _, e := range env {
		if m := e.(map[string]any); m["name"] == "BUCKET" {
			f.buckets = append(f.buckets, m["value"].(string))
		}
	}
	f.jobs[obj.GetName()] = kube.JobState{Found: true, Succeeded: true, Message: "synced: 1 files, 1 uploaded (10 bytes)"}
	return nil
}

func (f *fakeJobs) JobStatus(_ context.Context, _, name string) (kube.JobState, error) {
	return f.jobs[name], nil
}

func (f *fakeJobs) DeleteJob(_ context.Context, _, name string) error {
	delete(f.jobs, name)
	return nil
}

// pinnedCatalog copies the shipped catalog with a real-looking revision for the tiny model.
func pinnedCatalog(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "models", "catalog.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "catalog.yaml")
	pinned := strings.ReplaceAll(string(data), models.PlaceholderRevision, "0123456789abcdef0123456789abcdef01234567")
	if err := os.WriteFile(path, []byte(pinned), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestModelsList(t *testing.T) {
	setProfiles(t)
	r := runCLI(t, &App{}, "models", "list", "-o", "json")
	var all models.List
	if r.code != 0 || json.Unmarshal([]byte(r.stdout), &all) != nil || len(all.Models) != 5 || all.Profile != "" {
		t.Fatalf("all enabled models: %d %s %s", r.code, r.stdout, r.stderr)
	}
	r = runCLI(t, &App{}, "models", "list", "--profile", "aws-bench")
	if r.code != 0 || !strings.Contains(r.stdout, "qwen2.5-72b-awq") || strings.Contains(r.stdout, "sim-llama-8b") || strings.Contains(r.stdout, "llama-3.1") {
		t.Fatalf("aws-bench: %d\n%s", r.code, r.stdout)
	}
	r = runCLI(t, &App{}, "models", "list", "--profile", "aws-dev", "--all")
	if r.code != 0 || !strings.Contains(r.stdout, "llama-3.1-8b-instruct") {
		t.Fatalf("--all: %d\n%s", r.code, r.stdout)
	}
	bad := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(bad, []byte("apiVersion: aibox.dev/v1alpha1\nkind: ModelCatalog\nmodels: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r = runCLI(t, &App{}, "models", "list", "--catalog", bad)
	if r.code != ExitFailure || !strings.Contains(r.stderr, "invalid model catalog") || !strings.Contains(r.stderr, "/models") {
		t.Fatalf("invalid catalog: %d %s", r.code, r.stderr)
	}
}

func TestModelsSyncLocal(t *testing.T) {
	setProfiles(t)
	cat := pinnedCatalog(t)
	jobs := &fakeJobs{jobs: map[string]kube.JobState{}}
	var gotContext string
	app := &App{Jobs: func(kubeconfig, kubeContext string) (models.JobAPI, error) {
		gotContext = kubeContext
		return jobs, nil
	}}
	r := runCLI(t, app, "models", "sync", "--profile", "local", "--catalog", cat, "-o", "json")
	var rep models.Report
	if r.code != 0 || json.Unmarshal([]byte(r.stdout), &rep) != nil {
		t.Fatalf("sync: %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
	if gotContext != "kind-aibox-local" || rep.Bucket != "aibox-weights" || len(rep.Models) != 2 {
		t.Fatalf("target: %s %+v", gotContext, rep)
	}
	if rep.Models[0].Status != models.StatusSkipped || rep.Models[1].ID != "tiny-random-llama" || rep.Models[1].Status != models.StatusSynced || !rep.Models[1].Verified {
		t.Fatalf("results: %+v", rep.Models)
	}

	r = runCLI(t, app, "models", "sync", "--profile", "local", "--model", "qwen3-8b", "--catalog", cat)
	if r.code != ExitUsage || !strings.Contains(r.stderr, "not declared for profile local") {
		t.Fatalf("model of another profile: %d %s", r.code, r.stderr)
	}
	r = runCLI(t, app, "models", "sync", "--profile", "local", "--dry-run", "--model", "tiny-random-llama", "--catalog", cat)
	if r.code != 0 || !strings.Contains(r.stdout, "[dry-run] would run Jobs mirror-tiny-random-llama and verify-tiny-random-llama") {
		t.Fatalf("dry run: %d\n%s", r.code, r.stdout)
	}
}

func TestModelsSyncAWS(t *testing.T) {
	setProfiles(t)
	state := t.TempDir()
	cat := pinnedCatalog(t)
	jobs := &fakeJobs{jobs: map[string]kube.JobState{}}
	var gotKubeconfig, gotContext string
	app := &App{
		Getenv: func(k string) string {
			if k == "AIBOX_STATE_DIR" {
				return state
			}
			return ""
		},
		Jobs: func(kubeconfig, kubeContext string) (models.JobAPI, error) {
			gotKubeconfig, gotContext = kubeconfig, kubeContext
			return jobs, nil
		},
	}
	r := runCLI(t, app, "models", "sync", "--profile", "aws-dev", "--catalog", cat)
	if r.code != ExitFailure || !strings.Contains(r.stderr, "no active run") {
		t.Fatalf("no run: %d %s", r.code, r.stderr)
	}

	dir := filepath.Join(state, "aws-dev")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"run.json":     `{"runId": "aws-dev-202611050000-abcd"}`,
		"outputs.json": `{"weights_bucket": {"value": "aibox-weights-aibox-dev-000000000000"}, "region": {"value": "us-east-1"}}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r = runCLI(t, app, "models", "sync", "--profile", "aws-dev", "--catalog", cat, "--no-verify")
	if r.code != 0 || gotContext != "aibox-aws-dev" || gotKubeconfig != filepath.Join(dir, "kubeconfig") {
		t.Fatalf("aws sync: %d %s %s\n%s", r.code, gotContext, gotKubeconfig, r.stderr)
	}
	if len(jobs.buckets) != 1 || jobs.buckets[0] != "aibox-weights-aibox-dev-000000000000" || !strings.Contains(r.stdout, "qwen3-8b") {
		t.Fatalf("jobs: %v\n%s", jobs.buckets, r.stdout)
	}

	if err := os.WriteFile(filepath.Join(dir, "outputs.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	r = runCLI(t, app, "models", "sync", "--profile", "aws-dev", "--catalog", cat)
	if r.code != ExitFailure || !strings.Contains(r.stderr, "lack weights_bucket") {
		t.Fatalf("missing bucket output: %d %s", r.code, r.stderr)
	}
}

func TestModelsSyncReportsFailures(t *testing.T) {
	setProfiles(t)
	data, err := os.ReadFile(pinnedCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	unpinned := regexp.MustCompile(`(tiny-random-LlamaForCausalLM, revision: )"[0-9a-f]{40}"`).ReplaceAllString(string(data), `$1"`+models.PlaceholderRevision+`"`)
	cat := filepath.Join(t.TempDir(), "catalog.yaml")
	if err := os.WriteFile(cat, []byte(unpinned), 0o600); err != nil {
		t.Fatal(err)
	}
	app := &App{Jobs: func(string, string) (models.JobAPI, error) { return &fakeJobs{jobs: map[string]kube.JobState{}}, nil }}
	r := runCLI(t, app, "models", "sync", "--profile", "local", "--model", "tiny-random-llama", "--catalog", cat)
	if r.code != ExitFailure || !strings.Contains(r.stdout, "revision not pinned") || !strings.Contains(r.stderr, "model sync failed for tiny-random-llama") {
		t.Fatalf("failure exit: %d\n%s\n%s", r.code, r.stdout, r.stderr)
	}
}
