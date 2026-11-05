package models

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hermeto/aibox/internal/output"
)

func shipped(t *testing.T) *Catalog {
	t.Helper()
	c, err := Load(filepath.Join("..", "..", "models", "catalog.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestShippedCatalogValidates(t *testing.T) {
	c := shipped(t)
	if len(c.Models) < 5 {
		t.Fatalf("expected the v1.0 models, got %d", len(c.Models))
	}
	for _, m := range c.Models {
		if m.Gated && m.Enabled {
			t.Errorf("%s: gated models ship disabled (NFR-11)", m.ID)
		}
		if m.Mirrorable() && len(m.Source.Revision) != 40 {
			t.Errorf("%s: revision must be a commit SHA", m.ID)
		}
	}
	tiny := c.Get("tiny-random-llama")
	if tiny == nil || tiny.Served() || !tiny.Mirrorable() || !tiny.For("local") {
		t.Fatalf("tiny-random-llama must be a local, mirror-only model: %+v", tiny)
	}
}

const valid = `apiVersion: aibox.dev/v1alpha1
kind: ModelCatalog
models:
  - id: m1
    enabled: true
    profiles: [local]
    source: {type: huggingface, repo: org/m1, revision: "0123456789abcdef0123456789abcdef01234567"}
    license: apache-2.0
    gated: false
    quantization: bf16
    tensorParallelSize: 1
    maxModelLen: 4096
    minGPUClass: gpu-l4
    runtime: vllm
    loadMethod: s3-stream
    slo: {ttftP99Ms: 1000, itlP99Ms: 100}
`

func TestParseErrorsCarryPaths(t *testing.T) {
	if _, err := Parse("ok.yaml", []byte(valid)); err != nil {
		t.Fatalf("valid catalog rejected: %v", err)
	}
	cases := map[string]struct {
		edit func(string) string
		want string
	}{
		"unknown field": {func(s string) string { return s + "    color: blue\n" }, "/models/0"},
		"short revision": {func(s string) string {
			return strings.Replace(s, "0123456789abcdef0123456789abcdef01234567", "main", 1)
		}, "/models/0/source/revision"},
		"tp out of range":   {func(s string) string { return strings.Replace(s, "tensorParallelSize: 1", "tensorParallelSize: 16", 1) }, "/models/0/tensorParallelSize"},
		"bad load method":   {func(s string) string { return strings.Replace(s, "s3-stream", "ftp", 1) }, "/models/0/loadMethod"},
		"repo on simulator": {func(s string) string { return strings.Replace(s, "type: huggingface", "type: simulator", 1) }, "/models/0/source"},
		"duplicate id": {func(s string) string {
			return s + strings.SplitN(s, "models:\n", 2)[1]
		}, "/models/1/id"},
		"weights without load method": {func(s string) string { return strings.Replace(s, "loadMethod: s3-stream", "loadMethod: none", 1) }, "/models/0/loadMethod"},
		"not yaml":                    {func(string) string { return "models: [" }, "/"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse("bad.yaml", []byte(tc.edit(valid)))
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("want ValidationError, got %v", err)
			}
			found := false
			for _, i := range ve.Issues {
				found = found || strings.HasPrefix(i.Field, tc.want)
			}
			if !found {
				t.Fatalf("no issue at %s: %v", tc.want, err)
			}
			if !strings.Contains(err.Error(), "bad.yaml") {
				t.Fatalf("error lacks the file name: %v", err)
			}
		})
	}
}

func TestSelect(t *testing.T) {
	c := shipped(t)
	ids := func(ms []Model) string {
		var out []string
		for _, m := range ms {
			out = append(out, m.ID)
		}
		return strings.Join(out, ",")
	}
	got, err := c.Select("aws-bench", false, nil)
	if err != nil || ids(got) != "qwen3-8b,qwen3-32b-fp8,qwen2.5-72b-awq" {
		t.Fatalf("aws-bench enabled: %s %v", ids(got), err)
	}
	if got, _ := c.Select("aws-dev", true, nil); !strings.Contains(ids(got), "llama-3.1-8b-instruct") {
		t.Fatalf("--all must include disabled models: %s", ids(got))
	}
	if got, _ := c.Select("local", false, nil); ids(got) != "sim-llama-8b,tiny-random-llama" {
		t.Fatalf("local: %s", ids(got))
	}
	if got, err := c.Select("local", false, []string{"tiny-random-llama"}); err != nil || ids(got) != "tiny-random-llama" {
		t.Fatalf("explicit id: %s %v", ids(got), err)
	}
	if _, err := c.Select("local", false, []string{"nope"}); err == nil || !strings.Contains(err.Error(), "unknown model") {
		t.Fatalf("unknown id: %v", err)
	}
	if _, err := c.Select("local", false, []string{"qwen3-8b"}); err == nil || !strings.Contains(err.Error(), "not declared for profile local") {
		t.Fatalf("wrong profile: %v", err)
	}
	if got, _ := c.Select("", true, nil); len(got) != len(c.Models) {
		t.Fatalf("all profiles: %d", len(got))
	}
}

func TestListRender(t *testing.T) {
	c := shipped(t)
	ms, _ := c.Select("local", false, nil)
	var b bytes.Buffer
	if err := output.Print(&b, output.Text, List{Profile: "local", Models: ms}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ID", "sim-llama-8b", "simulator", "tiny-random-llama", "hf-internal-testing/tiny-random-LlamaForCausalLM@"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("missing %q in:\n%s", want, b.String())
		}
	}
	b.Reset()
	if err := output.Print(&b, output.Text, List{Profile: "x"}); err != nil || !strings.Contains(b.String(), "No models for profile x") {
		t.Fatalf("empty list: %q %v", b.String(), err)
	}
	b.Reset()
	if err := output.Print(&b, output.JSON, List{Profile: "local", Models: ms}); err != nil || !strings.Contains(b.String(), `"tensorParallelSize": 1`) {
		t.Fatalf("json: %v\n%s", err, b.String())
	}
}

func TestShortRev(t *testing.T) {
	if shortRev(PlaceholderRevision) != "unpinned" || shortRev("0123456789abcdef0123") != "0123456789ab" || shortRev("abc") != "abc" {
		t.Fatal("shortRev")
	}
}

func TestDefaultPath(t *testing.T) {
	t.Setenv(EnvCatalog, "/x/catalog.yaml")
	if DefaultPath() != "/x/catalog.yaml" {
		t.Fatal(DefaultPath())
	}
	t.Setenv(EnvCatalog, "")
	if !strings.HasSuffix(DefaultPath(), filepath.Join("models", "catalog.yaml")) {
		t.Fatal(DefaultPath())
	}
}
