package doctor

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

	"github.com/hermeto/aibox/internal/exec"
	"github.com/hermeto/aibox/internal/profile"
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
		t.Fatalf("%s mismatch (run go test ./internal/doctor -update)\n--- got\n%s\n--- want\n%s", name, got, want)
	}
}

func loadProfile(t *testing.T, name string) *profile.Profile {
	t.Helper()
	dir, _ := filepath.Abs(filepath.Join("..", "..", "profiles"))
	t.Setenv(profile.EnvProfilesDir, dir)
	p, err := profile.Load(name)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

const dockerInfoOK = `{"NCPU":8,"MemTotal":17179869184,"ServerVersion":"29.0.1","Architecture":"x86_64"}`

func healthyLocal() *exec.Fake {
	return exec.NewFake().
		WithPath("docker").WithPath("kind").WithPath("kubectl").WithPath("helm").
		On("docker --version", exec.Result{Stdout: "Docker version 29.0.1, build abc"}, nil).
		On("kind version", exec.Result{Stdout: "kind v0.33.0 go1.27.3 linux/amd64"}, nil).
		On("kubectl version --client", exec.Result{Stdout: "Client Version: v1.35.0\nKustomize Version: v5.7.1"}, nil).
		On("helm version --short", exec.Result{Stdout: "v3.19.0+g3d8990f"}, nil)
}

func statuses(r *Report) map[string]Status {
	m := map[string]Status{}
	for _, c := range r.Checks {
		m[c.ID] = c.Status
	}
	return m
}

func TestLocalAllPassGolden(t *testing.T) {
	p := loadProfile(t, "local")
	f := healthyLocal().On(`docker info --format "{{json .}}"`, exec.Result{Stdout: dockerInfoOK}, nil)
	r := Run(context.Background(), p, Env{Runner: f, GOOS: "linux"})
	if !r.OK || r.Summary.Fail != 0 || r.Summary.Pass != 7 {
		t.Fatalf("want all pass, got %+v", r.Summary)
	}
	js, _ := json.MarshalIndent(r, "", "  ")
	golden(t, "local-pass.json", append(js, '\n'))
	var txt bytes.Buffer
	if err := r.RenderText(&txt); err != nil {
		t.Fatal(err)
	}
	golden(t, "local-pass.txt", txt.Bytes())
}

func TestLocalDaemonDown(t *testing.T) {
	p := loadProfile(t, "local")
	f := healthyLocal().On(`docker info --format "{{json .}}"`, exec.Result{ExitCode: 1, Stderr: "Cannot connect"}, errors.New("exit 1"))
	r := Run(context.Background(), p, Env{Runner: f, GOOS: "darwin"})
	s := statuses(r)
	if r.OK || s["docker.daemon"] != Fail || s["docker.cpus"] != Skip || s["docker.memory"] != Skip {
		t.Fatalf("unexpected %v", s)
	}
	var txt bytes.Buffer
	_ = r.RenderText(&txt)
	if !strings.Contains(txt.String(), "Remediation:") || !strings.Contains(txt.String(), "FAILED") {
		t.Fatalf("text lacks remediation/verdict:\n%s", txt.String())
	}
}

func TestLocalToolMissingAndOld(t *testing.T) {
	p := loadProfile(t, "local")
	f := exec.NewFake().WithPath("docker").WithPath("kind").WithPath("kubectl").
		On("docker --version", exec.Result{Stdout: "Docker version 20.10.1"}, nil).
		On("kind version", exec.Result{Stdout: "garbage"}, nil).
		On("kubectl version --client", exec.Result{Stdout: "Client Version: v1.35.0"}, nil).
		On(`docker info --format "{{json .}}"`, exec.Result{Stdout: dockerInfoOK}, nil)
	r := Run(context.Background(), p, Env{Runner: f, GOOS: "darwin"})
	s := statuses(r)
	if s["tool.docker"] != Fail || s["tool.kind"] != Warn || s["tool.helm"] != Fail || s["tool.kubectl"] != Pass {
		t.Fatalf("unexpected %v", s)
	}
	for _, c := range r.Checks {
		if c.ID == "tool.helm" && !strings.Contains(c.Remediation, "brew install helm") {
			t.Fatalf("darwin hint missing: %q", c.Remediation)
		}
		if c.ID == "tool.docker" && !strings.Contains(c.Message, "older than the minimum 24.0.0") {
			t.Fatalf("message: %q", c.Message)
		}
	}
}

func TestLocalLowResources(t *testing.T) {
	p := loadProfile(t, "local")
	low := `{"NCPU":2,"MemTotal":7516192768,"ServerVersion":"29.0.1","Architecture":"aarch64"}` // 7 GiB
	f := healthyLocal().On(`docker info --format "{{json .}}"`, exec.Result{Stdout: low}, nil)
	s := statuses(Run(context.Background(), p, Env{Runner: f}))
	if s["docker.cpus"] != Fail || s["docker.memory"] != Warn {
		t.Fatalf("unexpected %v", s)
	}
	tiny := `{"NCPU":8,"MemTotal":2147483648,"ServerVersion":"29.0.1","Architecture":"aarch64"}` // 2 GiB
	f = healthyLocal().On(`docker info --format "{{json .}}"`, exec.Result{Stdout: tiny}, nil)
	s = statuses(Run(context.Background(), p, Env{Runner: f}))
	if s["docker.memory"] != Fail {
		t.Fatalf("unexpected %v", s)
	}
}

func TestDockerChecksDefaultsWithoutLocalSection(t *testing.T) {
	f := exec.NewFake().On(`docker info --format "{{json .}}"`, exec.Result{Stdout: dockerInfoOK}, nil)
	cs := dockerChecks(context.Background(), Env{Runner: f}, nil)
	for _, c := range cs {
		if c.Status != Pass {
			t.Fatalf("%s = %s", c.ID, c.Status)
		}
	}
}

func TestParseVersion(t *testing.T) {
	cases := map[string]string{
		"OpenTofu v1.10.6\non linux_amd64": "1.10.6",
		"v3.19.0+g3d8990f":                 "3.19.0",
		"Client Version: v1.35":            "1.35.0",
	}
	for in, want := range cases {
		v, ok := parseVersion(in)
		if !ok || fmtVersion(v) != want {
			t.Fatalf("parseVersion(%q) = %v, %v", in, v, ok)
		}
	}
	if _, ok := parseVersion("none"); ok {
		t.Fatal("expected no version")
	}
	if !versionLess([3]int{1, 2, 3}, [3]int{1, 3, 0}) || versionLess([3]int{1, 2, 3}, [3]int{1, 2, 3}) {
		t.Fatal("versionLess")
	}
}
