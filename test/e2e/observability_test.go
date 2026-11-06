//go:build e2e

package e2e

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/hermeto/aibox/internal/kube"
	"github.com/hermeto/aibox/internal/platform"
)

const (
	prometheusURL = "http://127.0.0.1:30301"
	tempoURL      = "http://127.0.0.1:30302"
	grafanaURL    = "http://127.0.0.1:30300"
)

func httpGet(t *testing.T, url string, header map[string]string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return 0, nil
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// promQuery runs one instant query and returns the number of result series.
func promQuery(t *testing.T, query string) int {
	t.Helper()
	status, body := httpGet(t, prometheusURL+"/api/v1/query?query="+urlQueryEscape(query), nil)
	if status != http.StatusOK {
		return 0
	}
	var res struct {
		Data struct {
			Result []json.RawMessage `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return 0
	}
	return len(res.Data.Result)
}

func urlQueryEscape(s string) string {
	return strings.NewReplacer(
		" ", "%20", "\"", "%22", "{", "%7B", "}", "%7D", "=", "%3D",
		"(", "%28", ")", "%29", "+", "%2B",
	).Replace(s)
}

func waitFor(t *testing.T, what string, timeout time.Duration, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if ok() {
			t.Logf("%s: ok", what)
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: not ready after %s", what, timeout)
		}
		time.Sleep(5 * time.Second)
	}
}

// grafanaPassword reads the chart-generated admin password from the cluster.
func grafanaPassword(t *testing.T) string {
	t.Helper()
	c, err := kube.NewForContext("kind-aibox-local")
	if err != nil {
		t.Fatal(err)
	}
	secret, err := c.Dynamic.Resource(kube.Secrets).Namespace("observability").
		Get(context.Background(), "monitoring-grafana", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pw, _, _ := unstructured.NestedString(secret.Object, "data", "admin-password")
	decoded, err := base64.StdEncoding.DecodeString(pw)
	if err != nil {
		t.Fatal(err)
	}
	return string(decoded)
}

// checkObservability covers feature 010 (SC-001): simulator metrics in Prometheus, a gateway
// trace in Tempo and the five dashboards served by Grafana.
func checkObservability(t *testing.T, root string) {
	t.Helper()

	// One request through the gateway so there are rates and a trace to find.
	key, err := os.ReadFile(platform.APIKeyFile(filepath.Join(root, ".aibox", "local")))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		post(t, strings.TrimSpace(string(key)), "/chat/completions", chat("sim-llama-8b"))
	}

	waitFor(t, "simulator series in Prometheus", 4*time.Minute, func() bool {
		return promQuery(t, `vllm:num_requests_waiting{model_name="sim-llama-8b"}`) > 0
	})
	waitFor(t, "canonical runtime series", 3*time.Minute, func() bool {
		return promQuery(t, `aibox:kv_cache_usage_ratio{model="sim-llama-8b"}`) > 0
	})

	start := time.Now().Add(-15 * time.Minute).Unix()
	waitFor(t, "gateway trace in Tempo", 4*time.Minute, func() bool {
		status, body := httpGet(t, fmt.Sprintf("%s/api/search?start=%d&end=%d&limit=20", tempoURL, start, time.Now().Unix()), nil)
		if status != http.StatusOK {
			return false
		}
		var res struct {
			Traces []json.RawMessage `json:"traces"`
		}
		if err := json.Unmarshal(body, &res); err != nil {
			return false
		}
		return len(res.Traces) > 0
	})

	// Grafana: health is anonymous; the search needs the generated admin password.
	waitFor(t, "Grafana ready", 3*time.Minute, func() bool {
		status, _ := httpGet(t, grafanaURL+"/api/health", nil)
		return status == http.StatusOK
	})
	pw := grafanaPassword(t)
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:"+pw))
	waitFor(t, "five AIBox dashboards in Grafana", 4*time.Minute, func() bool {
		status, body := httpGet(t, grafanaURL+"/api/search?query=AIBox&type=dash-db", map[string]string{"Authorization": auth})
		if status != http.StatusOK {
			return false
		}
		var dashboards []struct {
			Title string `json:"title"`
		}
		if err := json.Unmarshal(body, &dashboards); err != nil {
			return false
		}
		return len(dashboards) >= 5
	})
}
