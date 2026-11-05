//go:build e2e

// Package e2e holds end-to-end tests that need Docker and network access. They run in the
// CI job `e2e-local`:
//
//	go test -tags e2e ./test/e2e/... -run TestLocalPlatform -timeout 30m -v
//
// Environment: AIBOX_E2E_REPO_URL and AIBOX_E2E_REVISION select what Argo CD tracks
// (default: origin remote and current branch); AIBOX_E2E_KEEP=1 keeps the cluster for
// diagnostics (CI tears it down in a later step).
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hermeto/aibox/internal/cli"
	"github.com/hermeto/aibox/internal/models"
	"github.com/hermeto/aibox/internal/profile"
	"github.com/hermeto/aibox/internal/smoke"
)

func aibox(t *testing.T, args ...string) (int, string) {
	t.Helper()
	var out bytes.Buffer
	t.Logf("$ aibox %v", args)
	code := cli.Execute(context.Background(), args, &out, os.Stderr)
	return code, out.String()
}

func TestLocalPlatform(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Setenv(profile.EnvProfilesDir, filepath.Join(root, "profiles"))

	upArgs := []string{"up", "--profile", "local", "--timeout", "10m"}
	if v := os.Getenv("AIBOX_E2E_REPO_URL"); v != "" {
		upArgs = append(upArgs, "--repo-url", v)
	}
	if v := os.Getenv("AIBOX_E2E_REVISION"); v != "" {
		upArgs = append(upArgs, "--revision", v)
	}
	if os.Getenv("AIBOX_E2E_KEEP") != "1" {
		t.Cleanup(func() { aibox(t, "down", "--profile", "local") })
	}

	start := time.Now()
	if code, out := aibox(t, upArgs...); code != 0 {
		t.Fatalf("up exited %d\n%s", code, out)
	}
	elapsed := time.Since(start)
	t.Logf("bring-up took %s", elapsed.Round(time.Second))
	if elapsed > 10*time.Minute {
		t.Errorf("bring-up took %s, want <= 10m (SC-002)", elapsed)
	}

	// Idempotency: a second up reuses the cluster and converges again.
	code, out := aibox(t, append(upArgs, "--output", "json")...)
	if code != 0 {
		t.Fatalf("second up exited %d\n%s", code, out)
	}
	var res struct {
		ClusterCreated bool `json:"clusterCreated"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil || res.ClusterCreated {
		t.Fatalf("second up should reuse the cluster: %v %s", err, out)
	}

	code, out = aibox(t, "smoke", "--profile", "local", "--output", "json")
	var rep smoke.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("smoke output: %v\n%s", err, out)
	}
	for _, c := range rep.Checks {
		t.Logf("smoke %-40s %s %s", c.Name, c.Status, c.Message)
	}
	if code != 0 || !rep.OK {
		t.Fatalf("smoke failed (exit %d)", code)
	}

	// Weight lifecycle (feature 007): mirror the tiny model into the in-cluster store, verify
	// it from a pod, and check that a second sync uploads nothing.
	for i, want := range []string{models.StatusSynced, models.StatusSkipped} {
		code, out = aibox(t, "models", "sync", "--profile", "local", "--model", "tiny-random-llama", "--timeout", "10m", "--output", "json")
		var sync models.Report
		if err := json.Unmarshal([]byte(out), &sync); err != nil || len(sync.Models) != 1 {
			t.Fatalf("sync %d output: %v\n%s", i+1, err, out)
		}
		m := sync.Models[0]
		t.Logf("sync %d: %s verified=%t %s", i+1, m.Status, m.Verified, m.Message)
		if code != 0 || m.Status != want || !m.Verified {
			t.Fatalf("sync %d: exit %d, status %s (want %s), verified %t", i+1, code, m.Status, want, m.Verified)
		}
	}
}
