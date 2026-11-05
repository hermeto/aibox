package exec

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRealRunSuccessAndFailure(t *testing.T) {
	r := Real{}
	res, err := r.Run(context.Background(), Cmd{Name: "sh", Args: []string{"-c", "echo hello"}})
	if err != nil || strings.TrimSpace(res.Stdout) != "hello" {
		t.Fatalf("got %+v, %v", res, err)
	}
	res, err = r.Run(context.Background(), Cmd{Name: "sh", Args: []string{"-c", "echo oops >&2; exit 3"}})
	if !errors.Is(err, ErrExit) || res.ExitCode != 3 || !strings.Contains(err.Error(), "oops") {
		t.Fatalf("got %+v, %v", res, err)
	}
	if _, err := r.Run(context.Background(), Cmd{Name: "definitely-not-a-binary-aibox"}); err == nil {
		t.Fatal("expected error for missing binary")
	}
}

func TestRealRunStreamsAndEnv(t *testing.T) {
	var buf bytes.Buffer
	_, err := Real{}.Run(context.Background(), Cmd{Name: "sh", Args: []string{"-c", "echo $AIBOX_X"}, Env: []string{"AIBOX_X=42"}, Stream: &buf})
	if err != nil || strings.TrimSpace(buf.String()) != "42" {
		t.Fatalf("stream=%q err=%v", buf.String(), err)
	}
}

func TestDryRunPrintsAndDelegatesReadOnly(t *testing.T) {
	var out bytes.Buffer
	inner := NewFake().On("kind get clusters", Result{Stdout: "aibox-local\n"}, nil).WithPath("kind")
	d := DryRun{Inner: inner, Out: &out, ReadOnly: func(c Cmd) bool { return len(c.Args) > 0 && c.Args[0] == "get" }}
	res, err := d.Run(context.Background(), Cmd{Name: "kind", Args: []string{"get", "clusters"}})
	if err != nil || res.Stdout != "aibox-local\n" {
		t.Fatalf("read-only not delegated: %+v %v", res, err)
	}
	if _, err := d.Run(context.Background(), Cmd{Name: "kind", Args: []string{"create", "cluster", "--name", "a b"}}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "[dry-run] kind create cluster --name \"a b\"\n" {
		t.Fatalf("dry-run output %q", got)
	}
	if p, err := d.LookPath("kind"); err != nil || p == "" {
		t.Fatalf("LookPath: %q %v", p, err)
	}
	if p, _ := (DryRun{}).LookPath("x"); p != "/usr/bin/x" {
		t.Fatalf("LookPath without inner: %q", p)
	}
}

func TestFakeScriptsAndRecords(t *testing.T) {
	f := NewFake().
		On("a b", Result{Stdout: "1"}, nil).
		On("a b", Result{Stdout: "2"}, nil).
		OnPrefix("docker info", Result{Stdout: "{}"}, nil)
	ctx := context.Background()
	for _, want := range []string{"1", "2", "2"} {
		r, _ := f.Run(ctx, Cmd{Name: "a", Args: []string{"b"}})
		if r.Stdout != want {
			t.Fatalf("got %q want %q", r.Stdout, want)
		}
	}
	if r, err := f.Run(ctx, Cmd{Name: "docker", Args: []string{"info", "--format", "json"}}); err != nil || r.Stdout != "{}" {
		t.Fatalf("prefix: %+v %v", r, err)
	}
	if _, err := f.Run(ctx, Cmd{Name: "nope"}); err == nil {
		t.Fatal("expected unscripted error")
	}
	if _, err := f.LookPath("nope"); err == nil {
		t.Fatal("expected LookPath error")
	}
	if got := len(f.CommandLines()); got != 5 {
		t.Fatalf("recorded %d calls", got)
	}
}

func TestReadOnly(t *testing.T) {
	yes := []Cmd{{Name: "git", Args: []string{"status"}}, {Name: "kind", Args: []string{"get", "clusters"}}, {Name: "kubectl", Args: []string{"kustomize", "x"}},
		{Name: "helm", Args: []string{"template", "x"}}, {Name: "docker", Args: []string{"info"}}, {Name: "tofu", Args: []string{"version"}}}
	no := []Cmd{{Name: "kind", Args: []string{"create", "cluster"}}, {Name: "kubectl", Args: []string{"apply"}}, {Name: "helm", Args: []string{"upgrade"}},
		{Name: "docker", Args: []string{"run"}}, {Name: "terraform", Args: []string{"apply"}}, {Name: "rm"}}
	for _, c := range yes {
		if !ReadOnly(c) {
			t.Fatalf("%s should be read-only", c)
		}
	}
	for _, c := range no {
		if ReadOnly(c) {
			t.Fatalf("%s should not be read-only", c)
		}
	}
}
