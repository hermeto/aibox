package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hermeto/aibox/internal/doctor"
	"github.com/hermeto/aibox/internal/exec"
	"github.com/hermeto/aibox/internal/profile"
	"github.com/hermeto/aibox/internal/version"
)

var update = flag.Bool("update", false, "update golden files")

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s mismatch (go test ./internal/cli -update)\n--- got\n%s\n--- want\n%s", name, got, want)
	}
}

type result struct {
	code           int
	stdout, stderr string
}

func runCLI(t *testing.T, app *App, args ...string) result {
	t.Helper()
	var out, errb bytes.Buffer
	app.Out, app.Err = &out, &errb
	code := run(context.Background(), app, args)
	return result{code, out.String(), errb.String()}
}

func setProfiles(t *testing.T) {
	t.Helper()
	dir, _ := filepath.Abs(filepath.Join("..", "..", "profiles"))
	t.Setenv(profile.EnvProfilesDir, dir)
}

func TestHelpGolden(t *testing.T) {
	r := runCLI(t, &App{}, "--help")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	for _, c := range []string{"up", "down", "models", "smoke", "bench", "report", "cost", "doctor", "version"} {
		if !strings.Contains(r.stdout, "\n  "+c+" ") {
			t.Fatalf("help lacks %q", c)
		}
	}
	for _, f := range []string{"--profile", "--output", "--verbose", "--dry-run", "--yes"} {
		if !strings.Contains(r.stdout, f) {
			t.Fatalf("help lacks %q", f)
		}
	}
	golden(t, "help.golden", []byte(r.stdout))
}

func TestNotImplementedExit2(t *testing.T) {
	for _, c := range notImplemented {
		r := runCLI(t, &App{}, c.use, "--profile", "local")
		if r.code != ExitNotImplemented {
			t.Fatalf("%s: exit %d", c.use, r.code)
		}
		if !strings.Contains(r.stderr, "not implemented yet") || !strings.Contains(r.stderr, "roadmap") || !strings.Contains(r.stderr, c.feature) {
			t.Fatalf("%s: stderr %q", c.use, r.stderr)
		}
	}
}

func TestNotImplementedJSONError(t *testing.T) {
	r := runCLI(t, &App{}, "up", "-o", "json")
	var got map[string]any
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON: %q", r.stdout)
	}
	if got["code"].(float64) != 2 || !strings.Contains(got["error"].(string), "not implemented") {
		t.Fatalf("unexpected %v", got)
	}
}

func TestUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"--nope"}, {"bogus"}, {"version", "-o", "yaml"}, {"version", "extra"}} {
		r := runCLI(t, &App{}, args...)
		if r.code != ExitUsage {
			t.Fatalf("%v: exit %d (%s)", args, r.code, r.stderr)
		}
	}
}

func TestVersion(t *testing.T) {
	old := version.Version
	version.Version = "v0.1.0"
	t.Cleanup(func() { version.Version = old })
	r := runCLI(t, &App{}, "version")
	if r.code != 0 || !strings.HasPrefix(r.stdout, "aibox v0.1.0 (commit") {
		t.Fatalf("text: %d %q", r.code, r.stdout)
	}
	r = runCLI(t, &App{}, "version", "--output", "json", "--verbose")
	var info version.Info
	if err := json.Unmarshal([]byte(r.stdout), &info); err != nil || info.Version != "v0.1.0" {
		t.Fatalf("json: %q %v", r.stdout, err)
	}
	if !strings.Contains(r.stderr, "starting") {
		t.Fatalf("verbose log missing: %q", r.stderr)
	}
}

func fakeRunner(memTotal string) *exec.Fake {
	return exec.NewFake().
		WithPath("docker").WithPath("kind").WithPath("kubectl").WithPath("helm").WithPath("tofu").
		On("docker --version", exec.Result{Stdout: "Docker version 29.0.1"}, nil).
		On("kind version", exec.Result{Stdout: "kind v0.33.0"}, nil).
		On("kubectl version --client", exec.Result{Stdout: "Client Version: v1.35.0"}, nil).
		On("helm version --short", exec.Result{Stdout: "v3.19.0"}, nil).
		On("tofu version", exec.Result{Stdout: "OpenTofu v1.10.6"}, nil).
		On(`docker info --format "{{json .}}"`, exec.Result{Stdout: `{"NCPU":4,"MemTotal":` + memTotal + `,"ServerVersion":"29.0.1","Architecture":"x86_64"}`}, nil)
}

func TestDoctorLocal(t *testing.T) {
	setProfiles(t)
	r := runCLI(t, &App{Runner: fakeRunner("17179869184")}, "doctor", "--profile", "local", "--output", "json")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	var rep doctor.Report
	if err := json.Unmarshal([]byte(r.stdout), &rep); err != nil || !rep.OK || rep.Provider != "kind" {
		t.Fatalf("report %q %v", r.stdout, err)
	}
	r = runCLI(t, &App{Runner: fakeRunner("1073741824")}, "doctor", "--dry-run")
	if r.code != ExitFailure || !strings.Contains(r.stdout, "FAILED") || strings.Contains(r.stderr, "Error:") {
		t.Fatalf("failing doctor: %d %q %q", r.code, r.stdout, r.stderr)
	}
}

func TestDoctorAWSNoCredentials(t *testing.T) {
	setProfiles(t)
	noCreds := func(context.Context, string) (doctor.AWSClient, error) { return nil, doctor.ErrNoCredentials }
	r := runCLI(t, &App{Runner: fakeRunner("17179869184"), AWS: noCreds, Getenv: func(string) string { return "" }}, "doctor", "--profile", "aws-dev")
	if r.code != 0 || !strings.Contains(r.stdout, "skipped: no credentials") {
		t.Fatalf("exit %d: %s", r.code, r.stdout)
	}
}

func TestDoctorProfileErrors(t *testing.T) {
	setProfiles(t)
	r := runCLI(t, &App{}, "doctor", "--profile", "nope")
	if r.code != ExitFailure || !strings.Contains(r.stderr, "available in") {
		t.Fatalf("missing profile: %d %q", r.code, r.stderr)
	}
	bad := filepath.Join("..", "profile", "testdata", "invalid", "unknown-key.yaml")
	r = runCLI(t, &App{}, "doctor", "--profile", bad, "-o", "json")
	if r.code != ExitFailure || !strings.Contains(r.stderr, "/spec") || !strings.Contains(r.stdout, `"code": 1`) {
		t.Fatalf("invalid profile: %d %q %q", r.code, r.stdout, r.stderr)
	}
}

func TestExitCodeMapping(t *testing.T) {
	if ExitCode(nil) != 0 || ExitCode(errors.New("boom")) != 1 || ExitCode(errors.New("unknown command \"x\"")) != ExitUsage {
		t.Fatal("mapping")
	}
	e := &ExitError{Code: 2}
	if e.Error() == "" || (&ExitError{Code: 1, Err: errors.New("x")}).Unwrap() == nil {
		t.Fatal("ExitError methods")
	}
}

func TestRunnerSelection(t *testing.T) {
	a := &App{}
	if _, ok := a.runner().(exec.Real); !ok {
		t.Fatal("default runner should be Real")
	}
	a.Opts.DryRun = true
	if _, ok := a.runner().(exec.DryRun); !ok {
		t.Fatal("dry-run runner expected")
	}
	if _, ok := a.readOnlyRunner().(exec.Real); !ok {
		t.Fatal("read-only runner ignores dry-run")
	}
	if a.getenv("PATH") == "" {
		t.Fatal("getenv")
	}
}

func TestExecute(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Execute(context.Background(), []string{"version"}, &out, &errb); code != 0 || out.Len() == 0 {
		t.Fatalf("Execute: %d %q", code, errb.String())
	}
}
