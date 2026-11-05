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
}
