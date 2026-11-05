package platform

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hermeto/aibox/internal/exec"
)

func TestNormalizeRepoURL(t *testing.T) {
	cases := map[string]string{
		"git@github.com:hermeto/aibox.git":            "https://github.com/hermeto/aibox.git",
		"git@github.com:hermeto/aibox":                "https://github.com/hermeto/aibox.git",
		"ssh://git@github.com/hermeto/aibox.git":      "https://github.com/hermeto/aibox.git",
		"https://github.com/hermeto/aibox":            "https://github.com/hermeto/aibox.git",
		"https://token@github.com/hermeto/aibox.git/": "https://github.com/hermeto/aibox.git",
		"http://127.0.0.1:8080/git/hermeto/aibox":     "https://127.0.0.1:8080/git/hermeto/aibox.git",
	}
	for in, want := range cases {
		got, err := NormalizeRepoURL(in)
		if err != nil || got != want {
			t.Fatalf("%q → %q, %v (want %q)", in, got, err, want)
		}
	}
	if _, err := NormalizeRepoURL("file:///tmp/repo"); err == nil {
		t.Fatal("expected error for file URL")
	}
}

func TestDetectGitSourceDetachedAndNoUpstream(t *testing.T) {
	sha := "0123456789abcdef0123456789abcdef01234567"
	f := exec.NewFake().
		On("git remote get-url origin", exec.Result{Stdout: "https://github.com/hermeto/aibox.git"}, nil).
		On("git rev-parse --abbrev-ref HEAD", exec.Result{Stdout: "HEAD"}, nil).
		On("git rev-parse HEAD", exec.Result{Stdout: sha}, nil).
		On("git rev-list --count @{u}..HEAD", exec.Result{}, errors.New("no upstream"))
	src, err := DetectGitSource(context.Background(), f, ".", "", "")
	if err != nil || src.Revision != sha || src.Warning != "" || src.Unpushed != -1 {
		t.Fatalf("%+v %v", src, err)
	}
	f = exec.NewFake().
		On("git remote get-url origin", exec.Result{Stdout: "git@github.com:o/r.git"}, nil).
		On("git rev-parse --abbrev-ref HEAD", exec.Result{Stdout: "feature"}, nil).
		On("git rev-list --count @{u}..HEAD", exec.Result{}, errors.New("no upstream"))
	src, err = DetectGitSource(context.Background(), f, ".", "", "")
	if err != nil || !strings.Contains(src.Warning, "no upstream") {
		t.Fatalf("%+v %v", src, err)
	}
	if _, err := DetectGitSource(context.Background(), exec.NewFake(), ".", "", ""); err == nil {
		t.Fatal("expected remote error")
	}
	f = exec.NewFake().On("git remote get-url origin", exec.Result{Stdout: "file:///x"}, nil)
	if _, err := DetectGitSource(context.Background(), f, ".", "", ""); err == nil {
		t.Fatal("expected unsupported remote")
	}
	f = exec.NewFake()
	if _, err := DetectGitSource(context.Background(), f, ".", "https://x/y.git", ""); err == nil {
		t.Fatal("expected branch error")
	}
}
