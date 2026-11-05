package models

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"github.com/hermeto/aibox/internal/kube"
	"github.com/hermeto/aibox/internal/output"
)

var update = flag.Bool("update", false, "rewrite golden files")

func tinyModel() *Model {
	return &Model{ID: "tiny-random-llama", Enabled: true, Profiles: []string{"local"},
		Source:  Source{Type: SourceHuggingFace, Repo: "hf-internal-testing/tiny-random-LlamaForCausalLM", Revision: "0123456789abcdef0123456789abcdef01234567"},
		License: "mit", Quantization: "none", TensorParallelSize: 1, MaxModelLen: 512, MinGPUClass: "none", Runtime: "vllm", LoadMethod: "s3-stream"}
}

func checkGolden(t *testing.T, name string, obj *unstructured.Unstructured) {
	t.Helper()
	got, err := yaml.Marshal(obj.Object)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path) //nolint:gosec // test fixture
	if err != nil {
		t.Fatalf("%v (run go test ./internal/models -update)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s differs from the rendered object (run go test ./internal/models -update):\n%s", path, got)
	}
}

func TestJobsGolden(t *testing.T) {
	m := tinyModel()
	aws := Target{Bucket: "aibox-weights-aibox-dev-000000000000", Region: "us-east-1"}
	for name, job := range map[string]runtime.Object{
		"mirror-local.golden.yaml": MirrorJob(m, LocalTarget(), 30*time.Minute),
		"verify-local.golden.yaml": VerifyJob(m, LocalTarget(), 30*time.Minute),
		"mirror-aws.golden.yaml":   MirrorJob(m, aws, time.Hour),
	} {
		obj, err := ToUnstructured(job)
		if err != nil {
			t.Fatal(err)
		}
		checkGolden(t, name, obj)
	}
	cm, err := Scripts()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cm.Data["mirror.py"], "def mirror(") || !strings.Contains(cm.Data["requirements.txt"], "--hash=sha256:") {
		t.Fatal("scripts ConfigMap lacks the script or the hash-locked requirements")
	}
}

func TestJobNameAndLocalTarget(t *testing.T) {
	if got := JobName(KindMirror, "qwen2.5-72b-awq"); got != "mirror-qwen2-5-72b-awq" {
		t.Fatal(got)
	}
	if lt := LocalTarget(); !lt.Anonymous || !lt.CreateBucket || lt.Endpoint == "" {
		t.Fatalf("%+v", lt)
	}
}

// fakeAPI simulates the Job controller: an applied Job finishes with the next status read.
type fakeAPI struct {
	applied  []string
	deleted  []string
	jobs     map[string]kube.JobState
	marker   map[string]bool // mirrored prefixes (the .complete marker)
	failWith map[string]string
	applyErr error
	reads    int
}

func newFake() *fakeAPI {
	return &fakeAPI{jobs: map[string]kube.JobState{}, marker: map[string]bool{}, failWith: map[string]string{}}
}

func (f *fakeAPI) Apply(_ context.Context, gvr schema.GroupVersionResource, obj *unstructured.Unstructured) error {
	if f.applyErr != nil {
		return f.applyErr
	}
	f.applied = append(f.applied, gvr.Resource+"/"+obj.GetName())
	if gvr != kube.Jobs {
		return nil
	}
	name := obj.GetName()
	if msg, ok := f.failWith[name]; ok {
		f.jobs[name] = kube.JobState{Found: true, Failed: true, Message: msg}
		return nil
	}
	prefix := obj.GetAnnotations()["aibox.dev/target"]
	msg := "verified: 3 files"
	if strings.HasPrefix(name, KindMirror) {
		msg = "synced: 3 files, 3 uploaded (1234 bytes)"
		if f.marker[prefix] {
			msg = "skipped: already mirrored"
		}
		f.marker[prefix] = true
	}
	f.jobs[name] = kube.JobState{Found: true, Succeeded: true, Message: msg}
	return nil
}

func (f *fakeAPI) JobStatus(_ context.Context, _, name string) (kube.JobState, error) {
	f.reads++
	return f.jobs[name], nil
}

func (f *fakeAPI) DeleteJob(_ context.Context, _, name string) error {
	f.deleted = append(f.deleted, name)
	delete(f.jobs, name)
	return nil
}

func TestSyncIsIdempotent(t *testing.T) {
	api := newFake()
	var log bytes.Buffer
	s := &Syncer{API: api, Target: LocalTarget(), Verify: true, PollInterval: time.Millisecond, Log: &log}
	sim := Model{ID: "sim-llama-8b", Source: Source{Type: SourceSimulator}}

	rep := s.Sync(context.Background(), []Model{*tinyModel(), sim})
	if len(rep.Failed()) != 0 || rep.Models[0].Status != StatusSynced || !rep.Models[0].Verified || rep.Models[1].Status != StatusSkipped {
		t.Fatalf("first sync: %+v", rep.Models)
	}
	if api.applied[0] != "configmaps/"+ScriptsConfigMap || api.applied[1] != "jobs/mirror-tiny-random-llama" || api.applied[2] != "jobs/verify-tiny-random-llama" {
		t.Fatalf("apply order: %v", api.applied)
	}

	rep = s.Sync(context.Background(), []Model{*tinyModel()})
	if rep.Models[0].Status != StatusSkipped || !strings.HasPrefix(rep.Models[0].Message, "skipped") {
		t.Fatalf("second sync must skip (SC-003): %+v", rep.Models)
	}
	if strings.Join(api.deleted, ",") != "mirror-tiny-random-llama,verify-tiny-random-llama" {
		t.Fatalf("finished Jobs are replaced: %v", api.deleted)
	}
	if !strings.Contains(log.String(), "Mirroring tiny-random-llama") {
		t.Fatalf("log: %s", log.String())
	}

	var b bytes.Buffer
	rep.Profile = "local"
	if err := output.Print(&b, output.Text, rep); err != nil || !strings.Contains(b.String(), "s3://aibox-weights (http://seaweedfs") {
		t.Fatalf("render: %v\n%s", err, b.String())
	}
}

func TestSyncFailures(t *testing.T) {
	api := newFake()
	gated := tinyModel()
	gated.ID, gated.Gated, gated.Source.Repo = "llama", true, "meta-llama/Llama-3.1-8B-Instruct"
	api.failWith["mirror-llama"] = "gated: accept the license"
	unpinned := tinyModel()
	unpinned.ID, unpinned.Source.Revision = "unpinned", PlaceholderRevision
	badVerify := tinyModel()
	badVerify.ID = "corrupt"
	api.failWith["verify-corrupt"] = "config.json: SHA-256 mismatch"
	s := &Syncer{API: api, Target: LocalTarget(), Verify: true, PollInterval: time.Millisecond}

	rep := s.Sync(context.Background(), []Model{*gated, *unpinned, *badVerify})
	if got := strings.Join(rep.Failed(), ","); got != "llama,unpinned,corrupt" {
		t.Fatalf("failed: %s (%+v)", got, rep.Models)
	}
	if !strings.Contains(rep.Models[0].Message, "https://huggingface.co/meta-llama/Llama-3.1-8B-Instruct") || !strings.Contains(rep.Models[0].Message, "logs job/mirror-llama") {
		t.Fatalf("gated hint: %s", rep.Models[0].Message)
	}
	if !strings.Contains(rep.Models[1].Message, "hack/hfcheck") {
		t.Fatalf("placeholder: %s", rep.Models[1].Message)
	}
	if !strings.HasPrefix(rep.Models[2].Message, "load check: config.json") {
		t.Fatalf("verify failure: %s", rep.Models[2].Message)
	}

	api = newFake()
	api.applyErr = errors.New("forbidden")
	rep = (&Syncer{API: api, Target: LocalTarget()}).Sync(context.Background(), []Model{*tinyModel()})
	if rep.Models[0].Status != StatusFailed || !strings.Contains(rep.Models[0].Message, "sync scripts: forbidden") {
		t.Fatalf("scripts apply failure: %+v", rep.Models)
	}
}

// stuckAPI never finishes a Job.
type stuckAPI struct{ fakeAPI }

func (s *stuckAPI) JobStatus(context.Context, string, string) (kube.JobState, error) {
	return kube.JobState{Found: false}, nil
}

func TestSyncHonorsContext(t *testing.T) {
	api := &stuckAPI{fakeAPI: *newFake()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rep := (&Syncer{API: api, Target: LocalTarget(), PollInterval: time.Millisecond}).Sync(ctx, []Model{*tinyModel()})
	if rep.Models[0].Status != StatusFailed || !strings.Contains(rep.Models[0].Message, "context canceled") {
		t.Fatalf("%+v", rep.Models)
	}
}
