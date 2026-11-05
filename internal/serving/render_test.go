package serving

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"

	"github.com/hermeto/aibox/internal/models"
	"github.com/hermeto/aibox/internal/profile"
)

var update = flag.Bool("update", false, "rewrite golden files")

const root = "../.."

func loadProfile(t *testing.T, name string) *profile.Profile {
	t.Helper()
	t.Setenv(profile.EnvProfilesDir, filepath.Join(root, "profiles"))
	p, err := profile.Load(name)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func catalog(t *testing.T) *models.Catalog {
	t.Helper()
	c, err := models.Load(filepath.Join(root, "models", "catalog.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func ptr[T any](v T) *T { return &v }

func model(id, runtime, load string, tp int) *models.Model {
	return &models.Model{
		ID: id, Enabled: true, Profiles: []string{"aws-bench"},
		Source:  models.Source{Type: models.SourceHuggingFace, Repo: "org/" + id, Revision: "0123456789abcdef0123456789abcdef01234567"},
		License: "apache-2.0", Quantization: "fp8", TensorParallelSize: tp, MaxModelLen: 8192, MinGPUClass: "gpu-l4",
		Runtime: runtime, LoadMethod: load,
		RuntimeFlags: &models.RuntimeFlags{
			EnablePrefixCaching: ptr(false), EnableChunkedPrefill: ptr(false), GPUMemoryUtilization: 0.9,
			MaxNumSeqs: 64, KVCacheDtype: "fp8", ExtraArgs: []string{"--seed", "1"},
		},
		ShutdownGracePeriodSeconds: 300,
		SLO:                        models.SLO{TTFTP99Ms: 1000, ITLP99Ms: 100},
	}
}

func golden(t *testing.T, name string, obj any) {
	t.Helper()
	got, err := yaml.Marshal(obj)
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
		t.Fatalf("%v (run go test ./internal/serving -update)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s differs (run go test ./internal/serving -update):\n%s", path, got)
	}
}

func TestServersGolden(t *testing.T) {
	bench := loadProfile(t, "aws-bench")
	for name, m := range map[string]*models.Model{
		"vllm-s3-stream-tp2.golden.yaml": model("vllm-s3", "vllm", "s3-stream", 2),
		"sglang-s3-stream.golden.yaml":   model("sglang-s3", "sglang", "s3-stream", 1),
		"vllm-nvme-cache.golden.yaml":    model("vllm-nvme", "vllm", "nvme-cache", 1),
		"vllm-hf-download.golden.yaml":   model("vllm-hf", "vllm", "hf-download", 1),
	} {
		obj, err := Service(bench, m)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		golden(t, name, obj)
	}
	sim, err := Service(loadProfile(t, "local"), catalog(t).Get("sim-llama-8b"))
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "simulator.golden.yaml", sim)
}

func TestRuntimeFlagMapping(t *testing.T) {
	bench := loadProfile(t, "aws-bench")
	args := func(m *models.Model) string {
		obj, err := Service(bench, m)
		if err != nil {
			t.Fatal(err)
		}
		c := obj["spec"].(map[string]any)["template"].(map[string]any)["containers"].([]any)[0].(map[string]any)
		var parts []string
		for _, a := range c["args"].([]any) {
			parts = append(parts, a.(string))
		}
		return strings.Join(parts, " ")
	}
	v := args(model("v", "vllm", "s3-stream", 2))
	for _, want := range []string{"s3://$(AIBOX_WEIGHTS_BUCKET)/models/v/", "--tensor-parallel-size 2", "--kv-cache-dtype fp8",
		"--no-enable-prefix-caching", "--no-enable-chunked-prefill", "--quantization fp8", "--load-format runai_streamer",
		"--shutdown-timeout 280", "--gpu-memory-utilization 0.9", "--max-num-seqs 64", "--seed 1"} {
		if !strings.Contains(v, want) {
			t.Errorf("vllm args lack %q: %s", want, v)
		}
	}
	s := args(model("s", "sglang", "s3-stream", 1))
	for _, want := range []string{"-m sglang.launch_server --model-path s3://", "--tp-size 1", "--kv-cache-dtype fp8_e4m3",
		"--disable-radix-cache", "--chunked-prefill-size -1", "--mem-fraction-static 0.9", "--max-running-requests 64", "--enable-metrics"} {
		if !strings.Contains(s, want) {
			t.Errorf("sglang args lack %q: %s", want, s)
		}
	}
	awq := model("a", "vllm", "hf-download", 1)
	awq.Quantization = "awq"
	if a := args(awq); !strings.Contains(a, "--quantization awq_marlin") || !strings.Contains(a, "--revision 0123456789abcdef") || !strings.HasPrefix(a, "org/a ") {
		t.Errorf("awq/hf-download args: %s", a)
	}
}

func TestPlacementErrors(t *testing.T) {
	dev := loadProfile(t, "aws-dev")
	if _, err := Service(dev, model("big", "vllm", "s3-stream", 2)); err == nil || !strings.Contains(err.Error(), "needs 2 GPUs") {
		t.Fatalf("TP > GPUs: %v", err)
	}
	h100 := model("h", "vllm", "s3-stream", 1)
	h100.MinGPUClass = "gpu-h100"
	if _, err := Service(dev, h100); err == nil || !strings.Contains(err.Error(), "does not define") {
		t.Fatalf("missing class: %v", err)
	}
	if _, err := Service(loadProfile(t, "local"), model("l", "vllm", "s3-stream", 1)); err == nil || !strings.Contains(err.Error(), "fake gpu-sim") {
		t.Fatalf("local GPU model: %v", err)
	}
	none := model("n", "vllm", "none", 1)
	if _, err := Service(loadProfile(t, "aws-bench"), none); err == nil || !strings.Contains(err.Error(), "needs weights") {
		t.Fatalf("load method none: %v", err)
	}
	bad := model("b", "tgi", "s3-stream", 1)
	if _, err := Service(loadProfile(t, "aws-bench"), bad); err == nil || !strings.Contains(err.Error(), "unknown runtime") {
		t.Fatalf("unknown runtime: %v", err)
	}
}

func TestNameAndRenderSkipsUnservedModels(t *testing.T) {
	if Name("qwen2.5-72b-awq") != "qwen2-5-72b-awq" {
		t.Fatal(Name("qwen2.5-72b-awq"))
	}
	out, err := Render(loadProfile(t, "local"), catalog(t))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "tiny-random-llama") || !strings.Contains(string(out), "name: sim-llama-8b") {
		t.Fatalf("local render must serve only the simulator:\n%s", out)
	}
	c := catalog(t)
	c.Get("qwen3-8b").LoadMethod = "nvme-cache"
	out, err = Render(loadProfile(t, "aws-dev"), c)
	if err != nil || !strings.Contains(string(out), "name: "+FetchConfigMap) || !strings.Contains(string(out), "mirror.py fetch") {
		t.Fatalf("nvme-cache needs the fetch scripts ConfigMap: %v\n%s", err, out)
	}
}

// TestCommittedManifestsMatchCatalog fails when platform/serving/overlays/*/models.yaml
// drifts from the catalog or the profiles. Fix with `make generate`.
func TestCommittedManifestsMatchCatalog(t *testing.T) {
	c := catalog(t)
	for _, name := range []string{"local", "aws-dev", "aws-bench"} {
		want, err := Render(loadProfile(t, name), c)
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(root, "platform", "serving", "overlays", name, "models.yaml")) //nolint:gosec // repository file
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("platform/serving/overlays/%s/models.yaml is stale: run `make generate`", name)
		}
	}
}

func TestGatewayObjects(t *testing.T) {
	local := loadProfile(t, "local")
	out, err := Render(local, catalog(t))
	if err != nil {
		t.Fatal(err)
	}
	docs := strings.Split(string(out), "\n---\n")
	byKind := map[string]map[string]any{}
	for _, d := range docs {
		var obj map[string]any
		if err := yaml.Unmarshal([]byte(d), &obj); err == nil && obj != nil {
			byKind[obj["kind"].(string)] = obj
		}
	}
	for _, kind := range []string{"Service", "Backend", "AIServiceBackend", "AIGatewayRoute", "BackendTrafficPolicy"} {
		if byKind[kind] == nil {
			t.Fatalf("missing %s in:\n%s", kind, out)
		}
	}
	if spec := byKind["Service"]["spec"].(map[string]any); spec["clusterIP"] != "None" {
		t.Fatalf("round-robin backend must be a headless Service: %v", spec)
	}
	rules := byKind["AIGatewayRoute"]["spec"].(map[string]any)["rules"].([]any)
	match := rules[0].(map[string]any)["matches"].([]any)[0].(map[string]any)["headers"].([]any)[0].(map[string]any)
	if match["name"] != ModelHeader || match["value"] != "sim-llama-8b" {
		t.Fatalf("route rule %v", match)
	}
	// Per-client limits come first; the default rule excludes those clients.
	rl := byKind["BackendTrafficPolicy"]["spec"].(map[string]any)["rateLimit"].(map[string]any)["global"].(map[string]any)["rules"].([]any)
	if len(rl) != 6 {
		t.Fatalf("2 clients + default, requests and tokens each: %d rules", len(rl))
	}
	first := rl[0].(map[string]any)
	if v := first["clientSelectors"].([]any)[0].(map[string]any)["headers"].([]any)[0].(map[string]any)["value"]; v != "e2e-requests" {
		t.Fatalf("first rule client %v", v)
	}
	if first["limit"].(map[string]any)["requests"] != float64(3) {
		t.Fatalf("e2e-requests limit %v", first["limit"])
	}
	tokens := rl[3].(map[string]any)
	if tokens["limit"].(map[string]any)["requests"] != float64(40) || tokens["cost"] == nil {
		t.Fatalf("e2e-tokens token rule %v", tokens)
	}
	def := rl[4].(map[string]any)["clientSelectors"].([]any)[0].(map[string]any)["headers"].([]any)
	if len(def) != 2 || def[0].(map[string]any)["type"] != "Distinct" || def[1].(map[string]any)["invert"] != true {
		t.Fatalf("default rule selectors %v", def)
	}
	// Without profile limits, only the default rules exist.
	if rules := rateLimits(loadProfile(t, "aws-dev"))["spec"].(map[string]any)["rateLimit"].(map[string]any)["global"].(map[string]any)["rules"].([]any); len(rules) != 2 {
		t.Fatalf("default limits: %d rules", len(rules))
	}
}
