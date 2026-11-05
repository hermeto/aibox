package profile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func repoProfiles(t *testing.T) string {
	t.Helper()
	d, err := filepath.Abs(filepath.Join("..", "..", "profiles"))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestShippedProfilesAreValid(t *testing.T) {
	t.Setenv(EnvProfilesDir, repoProfiles(t))
	for _, name := range []string{"local", "aws-dev", "aws-bench"} {
		t.Run(name, func(t *testing.T) {
			p, err := Load(name)
			if err != nil {
				t.Fatalf("Load(%q): %v", name, err)
			}
			if p.Metadata.Name != name {
				t.Fatalf("name = %q", p.Metadata.Name)
			}
			if _, err := p.Spec.Guardrails.TTLDuration(); err != nil {
				t.Fatal(err)
			}
			if got, want := p.IsCloud(), name != "local"; got != want {
				t.Fatalf("IsCloud = %v", got)
			}
		})
	}
}

func TestInvalidProfiles(t *testing.T) {
	cases := map[string]struct{ field, reason string }{
		"unknown-key.yaml":                   {"/spec", "clustr"},
		"wrong-type.yaml":                    {"/spec/local/minCPUs", "integer"},
		"missing-required.yaml":              {"/spec", "aws"},
		"bad-provider.yaml":                  {"/spec/provider", "kind"},
		"capacity-block-no-reservation.yaml": {"/spec/aws/gpuClasses/0", "capacityReservationId"},
		"ttl-too-long.yaml":                  {"/spec/guardrails/ttl", "between 30m and 24h"},
		"not-yaml.yaml":                      {"/", "not valid YAML"},
		"recommended-below-min.yaml":         {"/spec/local/recommendedMemoryGiB", ">= minMemoryGiB"},
	}
	for file, want := range cases {
		t.Run(file, func(t *testing.T) {
			_, err := Load(filepath.Join("testdata", "invalid", file))
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("want ValidationError, got %v", err)
			}
			found := false
			for _, i := range ve.Issues {
				if i.Field == want.field && strings.Contains(i.Reason, want.reason) {
					found = true
				}
			}
			if !found {
				t.Fatalf("no issue %s ~ %q in:\n%s", want.field, want.reason, ve.Error())
			}
			if !strings.Contains(ve.Error(), want.field) {
				t.Fatalf("error text lacks field path: %s", ve.Error())
			}
		})
	}
}

func TestResolve(t *testing.T) {
	dir := repoProfiles(t)
	t.Setenv(EnvProfilesDir, dir)
	if p, err := Resolve("local"); err != nil || p != filepath.Join(dir, "local.yaml") {
		t.Fatalf("Resolve(local) = %q, %v", p, err)
	}
	_, err := Resolve("does-not-exist")
	if !errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "aws-bench, aws-dev, local") {
		t.Fatalf("want not-found listing available profiles, got %v", err)
	}
	if _, err := Resolve("./nope.yaml"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing path: %v", err)
	}
	if _, err := Resolve(""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("empty: %v", err)
	}
	if p, err := Resolve(filepath.Join(dir, "aws-dev.yaml")); err != nil || p == "" {
		t.Fatalf("path: %q %v", p, err)
	}
}

func TestDirFallsBackToRepoRoot(t *testing.T) {
	t.Setenv(EnvProfilesDir, "")
	wd, _ := os.Getwd()
	if got, want := Dir(), filepath.Join(wd, "..", "..", "profiles"); filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("Dir() = %q want %q", got, want)
	}
	root, err := RepoRoot()
	if err != nil || filepath.Base(root) == "" {
		t.Fatalf("RepoRoot: %q %v", root, err)
	}
	if got := Available(filepath.Join("testdata", "missing")); got != nil {
		t.Fatalf("Available(missing) = %v", got)
	}
}

func TestNameMustMatchFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvProfilesDir, dir)
	src, _ := os.ReadFile(filepath.Join(repoProfiles(t), "local.yaml"))
	if err := os.WriteFile(filepath.Join(dir, "other.yaml"), src, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load("other")
	var ve *ValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Error(), "/metadata/name") {
		t.Fatalf("want name mismatch, got %v", err)
	}
}
