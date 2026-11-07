//go:build e2e

package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
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
	"github.com/hermeto/aibox/internal/serving"
)

const gatewayURL = "http://127.0.0.1:30080/v1"

// post sends an OpenAI request through the gateway and returns status and body.
func post(t *testing.T, key, path string, body map[string]any) (int, string) {
	t.Helper()
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, gatewayURL+path, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func chat(model string) map[string]any {
	return map[string]any{"model": model, "max_tokens": 20, "messages": []any{map[string]any{"role": "user", "content": "Hello"}}}
}

// addKeys adds test clients to the gateway's API key Secret with their own field manager,
// keeping the CLI's key, and waits until the gateway accepts them.
func addKeys(t *testing.T, keys map[string]string) {
	t.Helper()
	c, err := kube.NewForContext("kind-aibox-local")
	if err != nil {
		t.Fatal(err)
	}
	data := map[string]any{}
	for id, k := range keys {
		data[id] = base64.StdEncoding.EncodeToString([]byte(k))
	}
	secret := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]any{"name": platform.APIKeySecret, "namespace": serving.GatewayNamespace},
		"data":     data,
	}}
	_, err = c.Dynamic.Resource(kube.Secrets).Namespace(serving.GatewayNamespace).Apply(context.Background(), platform.APIKeySecret, secret,
		metav1.ApplyOptions{FieldManager: "aibox-e2e", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	for id, k := range keys {
		deadline := time.Now().Add(2 * time.Minute)
		for {
			status, _ := post(t, k, "/chat/completions", chat("sim-llama-8b"))
			if status == http.StatusOK || status == http.StatusTooManyRequests {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("key of %s not accepted by the gateway (last status %d)", id, status)
			}
			time.Sleep(3 * time.Second)
		}
	}
}

// untilStatus sends requests until one gets want, or fails after n requests.
func untilStatus(t *testing.T, key string, want, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if status, _ := post(t, key, "/chat/completions", chat("sim-llama-8b")); status == want {
			t.Logf("got %d after %d request(s)", want, i+1)
			return
		}
	}
	t.Fatalf("no %d after %d requests", want, n)
}

// checkGateway covers feature 009 through the local gateway (SC-001).
func checkGateway(t *testing.T, root string) {
	key, err := os.ReadFile(platform.APIKeyFile(filepath.Join(root, ".aibox", "local")))
	if err != nil {
		t.Fatal(err)
	}
	cli := strings.TrimSpace(string(key))

	if status, _ := post(t, "", "/chat/completions", chat("sim-llama-8b")); status != http.StatusUnauthorized {
		t.Errorf("no key: status %d, want 401", status)
	}
	if status, _ := post(t, "not-a-key", "/chat/completions", chat("sim-llama-8b")); status != http.StatusUnauthorized {
		t.Errorf("unknown key: status %d, want 401", status)
	}
	if status, body := post(t, cli, "/chat/completions", chat("sim-llama-8b")); status != http.StatusOK || !strings.Contains(body, "choices") {
		t.Errorf("chat completion: %d %s", status, body)
	}
	if status, body := post(t, cli, "/completions", map[string]any{"model": "sim-llama-8b", "prompt": "Hello", "max_tokens": 5}); status != http.StatusOK {
		t.Errorf("completion: %d %s", status, body)
	}
	if status, body := post(t, cli, "/chat/completions", chat("no-such-model")); status != http.StatusNotFound || !strings.Contains(body, "model_not_found") {
		t.Errorf("unknown model: %d %s", status, body)
	}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, gatewayURL+"/models", nil)
	req.Header.Set("Authorization", "Bearer "+cli)
	if resp, err := http.DefaultClient.Do(req); err != nil {
		t.Errorf("models: %v", err)
	} else {
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK || !strings.Contains(string(b), "sim-llama-8b") {
			t.Errorf("models: %d %s", resp.StatusCode, b)
		}
	}

	// Streaming: several server-sent events arrive through the gateway.
	body := chat("sim-llama-8b")
	body["stream"] = true
	data, _ := json.Marshal(body)
	sreq, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, gatewayURL+"/chat/completions", bytes.NewReader(data))
	sreq.Header.Set("Content-Type", "application/json")
	sreq.Header.Set("Authorization", "Bearer "+cli)
	if resp, err := http.DefaultClient.Do(sreq); err != nil {
		t.Errorf("stream: %v", err)
	} else {
		events := 0
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), "data:") {
				events++
			}
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK || events < 2 {
			t.Errorf("stream: status %d, %d events", resp.StatusCode, events)
		}
	}

	// Rate limits from profiles/local.yaml: e2e-requests (3 requests/min), e2e-tokens (40 tokens/min).
	addKeys(t, map[string]string{"e2e-requests": "aibox-e2e-requests-key", "e2e-tokens": "aibox-e2e-tokens-key"})
	untilStatus(t, "aibox-e2e-requests-key", http.StatusTooManyRequests, 10)
	untilStatus(t, "aibox-e2e-tokens-key", http.StatusTooManyRequests, 15)
	if status, _ := post(t, cli, "/chat/completions", chat("sim-llama-8b")); status != http.StatusOK {
		t.Errorf("other clients keep their own limits: status %d", status)
	}
}
