package smoke

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const metricsOK = `# HELP vllm:num_requests_running x
vllm:num_requests_running{model_name="sim-llama-8b"} 0
vllm:num_requests_waiting{model_name="sim-llama-8b"} 0
vllm:kv_cache_usage_perc{model_name="sim-llama-8b"} 0.1
vllm:time_to_first_token_seconds_bucket{le="0.5",model_name="sim-llama-8b"} 1
`

func server(t *testing.T, chatStatus int, metricsBody string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("missing header")
		}
		_, _ = io.WriteString(w, `{"object":"list","data":[{"id":"sim-llama-8b"}]}`)
	})
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		if chatStatus != http.StatusOK {
			w.WriteHeader(chatStatus)
			_, _ = io.WriteString(w, `{"error":{"message":"boom"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"choices":[{"index":0,"message":{"role":"assistant","content":"hi"}}],"usage":{"completion_tokens":3}}`)
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, metricsBody) })
	return httptest.NewServer(mux)
}

func opts(s *httptest.Server) Options {
	return Options{Profile: "local", BaseURL: s.URL + "/v1", MetricsURL: s.URL + "/metrics", ExpectModels: []string{"sim-llama-8b"},
		Header: http.Header{"Authorization": []string{"Bearer k"}}}
}

func TestSmokeAllPass(t *testing.T) {
	s := server(t, http.StatusOK, metricsOK)
	defer s.Close()
	r := Run(context.Background(), opts(s))
	if !r.OK || len(r.Checks) != 3 {
		t.Fatalf("%+v", r)
	}
	var b bytes.Buffer
	if err := r.RenderText(&b); err != nil || !strings.Contains(b.String(), "OK") {
		t.Fatal(b.String())
	}
}

func TestSmokeFailures(t *testing.T) {
	s := server(t, http.StatusInternalServerError, "vllm:num_requests_running 0\n")
	defer s.Close()
	o := opts(s)
	o.ExpectModels = append(o.ExpectModels, "qwen")
	r := Run(context.Background(), o)
	if r.OK {
		t.Fatal("expected failure")
	}
	got := map[string]Check{}
	for _, c := range r.Checks {
		got[c.Name] = c
	}
	if got["models/list"].Status != Fail || !strings.Contains(got["models/list"].Message, "qwen not listed") {
		t.Fatalf("%+v", got["models/list"])
	}
	if got["sim-llama-8b/chat-completion"].Status != Fail || !strings.Contains(got["sim-llama-8b/chat-completion"].Message, "HTTP 500") {
		t.Fatalf("%+v", got)
	}
	if !strings.Contains(got["metrics/required-series"].Message, "vllm:kv_cache_usage_perc") {
		t.Fatalf("%+v", got["metrics/required-series"])
	}
	var b bytes.Buffer
	_ = r.RenderText(&b)
	if !strings.Contains(b.String(), "FAILED") {
		t.Fatal(b.String())
	}
}

func TestSmokeUnreachable(t *testing.T) {
	r := Run(context.Background(), Options{BaseURL: "http://127.0.0.1:1/v1", MetricsURL: "http://127.0.0.1:1/metrics", Models: []string{"m"}})
	if r.OK || len(r.Checks) != 3 {
		t.Fatalf("%+v", r)
	}
}

func TestSmokeBadPayloads(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, `not json`) })
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, `{"choices":[]}`) })
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })
	s := httptest.NewServer(mux)
	defer s.Close()
	r := Run(context.Background(), Options{BaseURL: s.URL + "/v1", MetricsURL: s.URL + "/metrics", Models: []string{"m"}})
	for _, c := range r.Checks {
		if c.Status != Fail {
			t.Fatalf("%+v should fail", c)
		}
	}
	mux2 := http.NewServeMux()
	mux2.HandleFunc("/v1/models", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
	s2 := httptest.NewServer(mux2)
	defer s2.Close()
	r = Run(context.Background(), Options{BaseURL: s2.URL + "/v1"})
	if r.OK || !strings.Contains(r.Checks[0].Message, "HTTP 401") {
		t.Fatalf("%+v", r)
	}
	mux3 := http.NewServeMux()
	mux3.HandleFunc("/v1/models", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, `{"data":[]}`) })
	s3 := httptest.NewServer(mux3)
	defer s3.Close()
	if r = Run(context.Background(), Options{BaseURL: s3.URL + "/v1"}); r.OK {
		t.Fatal("empty model list must fail")
	}
	if truncate(bytes.Repeat([]byte("x"), 300))[200:] != "…" {
		t.Fatal("truncate")
	}
}
