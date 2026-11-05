// Package contract checks the runtime metrics AIBox depends on (metrics/runtime-metrics.yaml):
// against metric samples of the pinned vLLM and SGLang versions, and against a live model
// server when AIBOX_METRICS_URL is set (CI runs it against the simulator after e2e-local):
//
//	AIBOX_METRICS_URL=http://127.0.0.1:30082/metrics go test ./test/contract/... -run TestRuntimeMetrics
//
// AIBOX_METRICS_RUNTIME selects the runtime names of the live server (default vllm).
package contract

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"sigs.k8s.io/yaml"
)

type mapping struct {
	APIVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Runtimes   map[string]string `json:"runtimes"`
	Metrics    []metric          `json:"metrics"`
}

type metric struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Help   string `json:"help"`
	VLLM   string `json:"vllm"`
	SGLang string `json:"sglang"`
}

func (m metric) runtimeName(runtime string) string {
	if runtime == "sglang" {
		return m.SGLang
	}
	return m.VLLM
}

var types = map[string]dto.MetricType{
	"counter":   dto.MetricType_COUNTER,
	"gauge":     dto.MetricType_GAUGE,
	"histogram": dto.MetricType_HISTOGRAM,
}

func load(t *testing.T) mapping {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "metrics", "runtime-metrics.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var m mapping
	if err := yaml.UnmarshalStrict(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func families(r io.Reader) (map[string]*dto.MetricFamily, error) {
	p := expfmt.NewTextParser(model.UTF8Validation)
	return p.TextToMetricFamilies(r)
}

// check returns one message per required metric that is missing or has the wrong type.
func check(runtime string, fams map[string]*dto.MetricFamily, m mapping) []string {
	var problems []string
	for _, e := range m.Metrics {
		name := e.runtimeName(runtime)
		f, ok := fams[name]
		switch {
		case name == "":
			problems = append(problems, fmt.Sprintf("%s: no %s name in metrics/runtime-metrics.yaml", e.Name, runtime))
		case !ok:
			problems = append(problems, fmt.Sprintf("%s: %s metric %s is missing", e.Name, runtime, name))
		case f.GetType() != types[e.Type]:
			problems = append(problems, fmt.Sprintf("%s: %s metric %s is a %s, want %s", e.Name, runtime, name, strings.ToLower(f.GetType().String()), e.Type))
		}
	}
	return problems
}

func TestMappingIsComplete(t *testing.T) {
	m := load(t)
	if m.Runtimes["vllm"] == "" || m.Runtimes["sglang"] == "" || len(m.Metrics) == 0 {
		t.Fatalf("mapping lacks runtime versions or metrics: %+v", m)
	}
	seen := map[string]bool{}
	for _, e := range m.Metrics {
		if _, ok := types[e.Type]; !ok || e.Help == "" || seen[e.Name] {
			t.Errorf("%s: bad type %q, missing help or duplicate name", e.Name, e.Type)
		}
		seen[e.Name] = true
	}
	for _, required := range []string{"requests_waiting", "requests_running", "kv_cache_usage_ratio", "time_to_first_token_seconds", "inter_token_latency_seconds", "e2e_request_latency_seconds", "prompt_tokens_total", "generation_tokens_total"} {
		if !seen[required] {
			t.Errorf("canonical metric %s missing from the mapping", required)
		}
	}
}

func TestRuntimeMetrics(t *testing.T) {
	m := load(t)
	for _, runtime := range []string{"vllm", "sglang"} {
		t.Run(runtime+"-"+m.Runtimes[runtime], func(t *testing.T) {
			f, err := os.Open(filepath.Join("testdata", runtime+".prom")) //nolint:gosec // test fixture
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = f.Close() }()
			fams, err := families(f)
			if err != nil {
				t.Fatal(err)
			}
			if problems := check(runtime, fams, m); len(problems) > 0 {
				t.Fatalf("metric contract broken:\n  %s", strings.Join(problems, "\n  "))
			}
		})
	}
	url := os.Getenv("AIBOX_METRICS_URL")
	if url == "" {
		return
	}
	runtime := os.Getenv("AIBOX_METRICS_RUNTIME")
	if runtime == "" {
		runtime = "vllm"
	}
	t.Run("live-"+runtime, func(t *testing.T) {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(url) //nolint:noctx // test against an operator-provided URL
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s: %s", url, resp.Status)
		}
		fams, err := families(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if problems := check(runtime, fams, m); len(problems) > 0 {
			t.Fatalf("metric contract broken at %s:\n  %s", url, strings.Join(problems, "\n  "))
		}
	})
}

func TestCheckNamesTheBrokenMetric(t *testing.T) {
	m := load(t)
	fams := map[string]*dto.MetricFamily{}
	gauge := dto.MetricType_GAUGE
	for _, e := range m.Metrics {
		mt := types[e.Type]
		fams[e.VLLM] = &dto.MetricFamily{Type: &mt}
	}
	delete(fams, "vllm:kv_cache_usage_perc")
	fams["vllm:time_to_first_token_seconds"] = &dto.MetricFamily{Type: &gauge}
	problems := strings.Join(check("vllm", fams, m), "\n")
	for _, want := range []string{
		"kv_cache_usage_ratio: vllm metric vllm:kv_cache_usage_perc is missing",
		"time_to_first_token_seconds: vllm metric vllm:time_to_first_token_seconds is a gauge, want histogram",
	} {
		if !strings.Contains(problems, want) {
			t.Errorf("missing %q in:\n%s", want, problems)
		}
	}
}
