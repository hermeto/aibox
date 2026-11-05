package profile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/hermeto/aibox/schemas"
)

// EnvProfilesDir overrides the directory used to resolve profile names.
const EnvProfilesDir = "AIBOX_PROFILES_DIR"

// ErrNotFound is returned when a profile name or path does not exist.
var ErrNotFound = errors.New("profile not found")

// ValidationError lists every schema violation with its JSON path.
type ValidationError struct {
	Path   string
	Issues []Issue
}

// Issue is one schema violation.
type Issue = schemas.Issue

func (e *ValidationError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "invalid profile %s:", e.Path)
	for _, i := range e.Issues {
		fmt.Fprintf(&b, "\n  %s: %s", i.Field, i.Reason)
	}
	return b.String()
}

// Resolve maps a --profile value to a file path. Values containing a path separator or
// ending in .yaml/.yml are paths; other values are names looked up in the profiles
// directory ($AIBOX_PROFILES_DIR, else <repo root>/profiles, else ./profiles).
func Resolve(ref string) (string, error) {
	if ref == "" {
		return "", fmt.Errorf("%w: empty profile reference", ErrNotFound)
	}
	if strings.ContainsRune(ref, filepath.Separator) || strings.HasSuffix(ref, ".yaml") || strings.HasSuffix(ref, ".yml") {
		if _, err := os.Stat(ref); err != nil {
			return "", fmt.Errorf("%w: %s", ErrNotFound, ref)
		}
		return ref, nil
	}
	dir := Dir()
	p := filepath.Join(dir, ref+".yaml")
	if _, err := os.Stat(p); err != nil {
		avail := Available(dir)
		return "", fmt.Errorf("%w: %q (available in %s: %s)", ErrNotFound, ref, dir, strings.Join(avail, ", "))
	}
	return p, nil
}

// Dir returns the directory used to resolve profile names.
func Dir() string {
	if d := os.Getenv(EnvProfilesDir); d != "" {
		return d
	}
	if root, err := RepoRoot(); err == nil {
		return filepath.Join(root, "profiles")
	}
	return "profiles"
}

// RepoRoot walks up from the working directory to the directory that contains go.mod or
// .git and a profiles/ directory.
func RepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for d := wd; ; d = filepath.Dir(d) {
		if fi, err := os.Stat(filepath.Join(d, "profiles")); err == nil && fi.IsDir() {
			for _, marker := range []string{"go.mod", ".git"} {
				if _, err := os.Stat(filepath.Join(d, marker)); err == nil {
					return d, nil
				}
			}
		}
		if parent := filepath.Dir(d); parent == d {
			return "", errors.New("repository root not found")
		}
	}
}

// Available lists profile names in dir.
func Available(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			names = append(names, strings.TrimSuffix(e.Name(), ".yaml"))
		}
	}
	sort.Strings(names)
	return names
}

// Load resolves, reads and validates a profile.
func Load(ref string) (*Profile, error) {
	path, err := Resolve(ref)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path) //nolint:gosec // path comes from the operator
	if err != nil {
		return nil, err
	}
	p, err := Parse(path, data)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// Parse validates YAML bytes against the schema and decodes them strictly.
func Parse(path string, data []byte) (*Profile, error) {
	jsonData, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, &ValidationError{Path: path, Issues: []Issue{{Field: "/", Reason: "not valid YAML: " + err.Error()}}}
	}
	if err := Validate(path, jsonData); err != nil {
		return nil, err
	}
	var p Profile
	dec := json.NewDecoder(bytes.NewReader(jsonData))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return nil, &ValidationError{Path: path, Issues: []Issue{{Field: "/", Reason: err.Error()}}}
	}
	p.Path = path
	if err := semanticChecks(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Validate checks JSON bytes against the profile schema.
func Validate(path string, jsonData []byte) error {
	issues, err := schemas.Validate(schemas.Profile, jsonData)
	if err != nil {
		return err
	}
	if len(issues) > 0 {
		return &ValidationError{Path: path, Issues: issues}
	}
	return nil
}

func semanticChecks(p *Profile) error {
	var issues []Issue
	if base := strings.TrimSuffix(filepath.Base(p.Path), filepath.Ext(p.Path)); base != p.Metadata.Name && filepath.Dir(p.Path) == Dir() {
		issues = append(issues, Issue{Field: "/metadata/name", Reason: fmt.Sprintf("must match file name %q", base)})
	}
	if d, err := p.Spec.Guardrails.TTLDuration(); err != nil || d < 30*time.Minute || d > 24*time.Hour {
		issues = append(issues, Issue{Field: "/spec/guardrails/ttl", Reason: "must be a duration between 30m and 24h"})
	}
	if l := p.Spec.Local; l != nil && l.RecommendedMemoryGiB < l.MinMemoryGiB {
		issues = append(issues, Issue{Field: "/spec/local/recommendedMemoryGiB", Reason: "must be >= minMemoryGiB"})
	}
	if len(issues) > 0 {
		return &ValidationError{Path: p.Path, Issues: issues}
	}
	return nil
}
