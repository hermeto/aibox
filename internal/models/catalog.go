// Package models loads the model catalog (models/catalog.yaml), validates it against
// schemas/catalog.schema.json, and mirrors and verifies model weights with in-cluster Jobs.
package models

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"sigs.k8s.io/yaml"

	"github.com/hermeto/aibox/internal/profile"
	"github.com/hermeto/aibox/schemas"
)

// EnvCatalog overrides the catalog file path.
const EnvCatalog = "AIBOX_CATALOG"

// Source types.
const (
	SourceHuggingFace = "huggingface"
	SourceSimulator   = "simulator"
)

// PlaceholderRevision marks a revision that CI (hack/hfcheck) has not pinned yet.
const PlaceholderRevision = "0000000000000000000000000000000000000000"

// Catalog is the typed form of models/catalog.yaml.
type Catalog struct {
	APIVersion string  `json:"apiVersion"`
	Kind       string  `json:"kind"`
	Models     []Model `json:"models"`

	// Path is the file the catalog was loaded from (not part of the file).
	Path string `json:"-"`
}

// Model is one catalog entry.
type Model struct {
	ID                 string        `json:"id"`
	Description        string        `json:"description,omitempty"`
	Enabled            bool          `json:"enabled"`
	Serve              *bool         `json:"serve,omitempty"`
	Profiles           []string      `json:"profiles"`
	Source             Source        `json:"source"`
	License            string        `json:"license"`
	Gated              bool          `json:"gated"`
	Quantization       string        `json:"quantization"`
	TensorParallelSize int           `json:"tensorParallelSize"`
	MaxModelLen        int           `json:"maxModelLen"`
	MinGPUClass        string        `json:"minGPUClass"`
	Runtime            string        `json:"runtime"`
	RuntimeFlags       *RuntimeFlags `json:"runtimeFlags,omitempty"`
	LoadMethod         string        `json:"loadMethod"`
	SLO                SLO           `json:"slo"`
	// Routing selects how the gateway spreads requests across servers (default round-robin).
	Routing string `json:"routing,omitempty"`
	// StartupTimeoutSeconds bounds weight loading plus the warm-up request (default 1800).
	StartupTimeoutSeconds int `json:"startupTimeoutSeconds,omitempty"`
	// ShutdownGracePeriodSeconds bounds draining on shutdown (default 120).
	ShutdownGracePeriodSeconds int `json:"shutdownGracePeriodSeconds,omitempty"`
}

// Defaults for the optional serving timings.
const (
	DefaultStartupTimeoutSeconds      = 1800
	DefaultShutdownGracePeriodSeconds = 120
)

// RoutingMode returns the model's routing mode.
func (m *Model) RoutingMode() string {
	if m.Routing != "" {
		return m.Routing
	}
	return "round-robin"
}

// StartupTimeout returns the load budget in seconds.
func (m *Model) StartupTimeout() int {
	if m.StartupTimeoutSeconds > 0 {
		return m.StartupTimeoutSeconds
	}
	return DefaultStartupTimeoutSeconds
}

// GracePeriod returns the drain budget in seconds.
func (m *Model) GracePeriod() int {
	if m.ShutdownGracePeriodSeconds > 0 {
		return m.ShutdownGracePeriodSeconds
	}
	return DefaultShutdownGracePeriodSeconds
}

// Source is where the weights come from.
type Source struct {
	Type     string `json:"type"`
	Repo     string `json:"repo,omitempty"`
	Revision string `json:"revision,omitempty"`
}

// RuntimeFlags tune the inference runtime (feature 008 renders them).
type RuntimeFlags struct {
	EnablePrefixCaching  *bool    `json:"enablePrefixCaching,omitempty"`
	EnableChunkedPrefill *bool    `json:"enableChunkedPrefill,omitempty"`
	GPUMemoryUtilization float64  `json:"gpuMemoryUtilization,omitempty"`
	MaxNumSeqs           int      `json:"maxNumSeqs,omitempty"`
	KVCacheDtype         string   `json:"kvCacheDtype,omitempty"`
	ExtraArgs            []string `json:"extraArgs,omitempty"`
}

// SLO holds the latency targets the benchmarks check.
type SLO struct {
	TTFTP99Ms int `json:"ttftP99Ms"`
	ITLP99Ms  int `json:"itlP99Ms"`
}

// Mirrorable reports whether the model has weights to mirror.
func (m *Model) Mirrorable() bool { return m.Source.Type == SourceHuggingFace }

// Pinned reports whether the revision is a real commit (not the CI placeholder).
func (m *Model) Pinned() bool { return m.Source.Revision != PlaceholderRevision }

// Served reports whether the platform serves the model (default true; false for models
// that exist only to test the weight lifecycle).
func (m *Model) Served() bool { return m.Serve == nil || *m.Serve }

// Prefix is the object-storage prefix of the model's mirror, ending in "/".
func (m *Model) Prefix() string { return "models/" + m.ID + "/" + m.Source.Revision + "/" }

// For reports whether the model is declared for the profile.
func (m *Model) For(profileName string) bool {
	for _, p := range m.Profiles {
		if p == profileName {
			return true
		}
	}
	return false
}

// ValidationError lists every catalog violation with its JSON path.
type ValidationError struct {
	Path   string
	Issues []schemas.Issue
}

func (e *ValidationError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "invalid model catalog %s:", e.Path)
	for _, i := range e.Issues {
		fmt.Fprintf(&b, "\n  %s: %s", i.Field, i.Reason)
	}
	return b.String()
}

// DefaultPath returns $AIBOX_CATALOG, else <repo root>/models/catalog.yaml.
func DefaultPath() string {
	if p := os.Getenv(EnvCatalog); p != "" {
		return p
	}
	if root, err := profile.RepoRoot(); err == nil {
		return filepath.Join(root, "models", "catalog.yaml")
	}
	return filepath.Join("models", "catalog.yaml")
}

// Load reads and validates the catalog at path.
func Load(path string) (*Catalog, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path comes from the operator
	if err != nil {
		return nil, err
	}
	return Parse(path, data)
}

// Parse validates YAML bytes against the schema and the semantic rules, and decodes them.
func Parse(path string, data []byte) (*Catalog, error) {
	jsonData, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, &ValidationError{Path: path, Issues: []schemas.Issue{{Field: "/", Reason: "not valid YAML: " + err.Error()}}}
	}
	issues, err := schemas.Validate(schemas.Catalog, jsonData)
	if err != nil {
		return nil, err
	}
	if len(issues) > 0 {
		return nil, &ValidationError{Path: path, Issues: issues}
	}
	var c Catalog
	dec := json.NewDecoder(bytes.NewReader(jsonData))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return nil, &ValidationError{Path: path, Issues: []schemas.Issue{{Field: "/", Reason: err.Error()}}}
	}
	c.Path = path
	if issues := semanticChecks(&c); len(issues) > 0 {
		return nil, &ValidationError{Path: path, Issues: issues}
	}
	return &c, nil
}

func semanticChecks(c *Catalog) []schemas.Issue {
	var issues []schemas.Issue
	seen := map[string]bool{}
	for i := range c.Models {
		m := &c.Models[i]
		at := func(field string) string { return fmt.Sprintf("/models/%d/%s", i, field) }
		if seen[m.ID] {
			issues = append(issues, schemas.Issue{Field: at("id"), Reason: fmt.Sprintf("duplicate id %q", m.ID)})
		}
		seen[m.ID] = true
		if m.Source.Type == SourceSimulator && (m.Runtime != "simulator" || m.LoadMethod != "none") {
			issues = append(issues, schemas.Issue{Field: at("source/type"), Reason: "simulator sources need runtime simulator and loadMethod none"})
		}
		if m.Source.Type != SourceSimulator && m.LoadMethod == "none" {
			issues = append(issues, schemas.Issue{Field: at("loadMethod"), Reason: "models with weights need a loading method"})
		}
	}
	return issues
}

// Select returns the models for a profile ("" means every profile): the enabled ones, or
// all of them with all set. Explicit ids select exactly those models (even disabled ones)
// and must be declared for the profile.
func (c *Catalog) Select(profileName string, all bool, ids []string) ([]Model, error) {
	if len(ids) > 0 {
		out := make([]Model, 0, len(ids))
		for _, id := range ids {
			m := c.Get(id)
			if m == nil {
				return nil, fmt.Errorf("unknown model %q (see aibox models list --all)", id)
			}
			if profileName != "" && !m.For(profileName) {
				return nil, fmt.Errorf("model %s is not declared for profile %s (profiles: %s)", id, profileName, strings.Join(m.Profiles, ", "))
			}
			out = append(out, *m)
		}
		return out, nil
	}
	var out []Model
	for _, m := range c.Models {
		if (profileName == "" || m.For(profileName)) && (all || m.Enabled) {
			out = append(out, m)
		}
	}
	return out, nil
}

// Get returns the model with the given id, or nil.
func (c *Catalog) Get(id string) *Model {
	for i := range c.Models {
		if c.Models[i].ID == id {
			return &c.Models[i]
		}
	}
	return nil
}

// List is the output of `aibox models list`.
type List struct {
	Profile string  `json:"profile,omitempty"`
	Models  []Model `json:"models"`
}

// RenderText prints one line per model.
func (l List) RenderText(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tENABLED\tRUNTIME\tGPU CLASS\tTP\tQUANT\tLOAD\tSLO TTFT/ITL p99\tSOURCE")
	for _, m := range l.Models {
		src := m.Source.Type
		if m.Mirrorable() {
			src = m.Source.Repo + "@" + shortRev(m.Source.Revision)
		}
		_, _ = fmt.Fprintf(tw, "%s\t%t\t%s\t%s\t%d\t%s\t%s\t%dms/%dms\t%s\n", m.ID, m.Enabled, m.Runtime, m.MinGPUClass,
			m.TensorParallelSize, m.Quantization, m.LoadMethod, m.SLO.TTFTP99Ms, m.SLO.ITLP99Ms, src)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if len(l.Models) == 0 {
		_, err := fmt.Fprintf(w, "No models for profile %s (use --all to include disabled ones).\n", l.Profile)
		return err
	}
	return nil
}

func shortRev(rev string) string {
	if rev == PlaceholderRevision {
		return "unpinned"
	}
	if len(rev) > 12 {
		return rev[:12]
	}
	return rev
}
