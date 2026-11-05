package gpu

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"

	"github.com/hermeto/aibox/internal/profile"
)

func load(t *testing.T, name string) *profile.Profile {
	t.Helper()
	t.Setenv(profile.EnvProfilesDir, filepath.Join("..", "..", "profiles"))
	p, err := profile.Load(name)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func docs(t *testing.T, b []byte) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, d := range bytes.Split(b, []byte("\n---\n")) {
		var m map[string]any
		if err := yaml.Unmarshal(d, &m); err != nil {
			t.Fatal(err)
		}
		if len(m) > 0 {
			out = append(out, m)
		}
	}
	return out
}

func get(m map[string]any, path ...string) any {
	var cur any = m
	for _, p := range path {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[p]
	}
	return cur
}

// TestCommittedNodePoolsUpToDate fails when platform/gpu/overlays/<profile>/nodepools.yaml
// drifts from the profile. Fix with `make generate`.
func TestCommittedNodePoolsUpToDate(t *testing.T) {
	for _, name := range []string{"aws-dev", "aws-bench"} {
		got, err := Render(load(t, name))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join("..", "..", "platform", "gpu", "overlays", name, "nodepools.yaml")
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s is out of date: run `make generate`", path)
		}
	}
}

func TestBenchPools(t *testing.T) {
	out, err := Render(load(t, "aws-bench"))
	if err != nil {
		t.Fatal(err)
	}
	pools := map[string]map[string]any{}
	classes := map[string]map[string]any{}
	for _, d := range docs(t, out) {
		name := get(d, "metadata", "name").(string)
		switch d["kind"] {
		case "NodePool":
			pools[name] = d
		case "EC2NodeClass":
			classes[name] = d
		}
	}
	if len(pools) != 3 || len(classes) != 3 {
		t.Fatalf("pools %d classes %d", len(pools), len(classes))
	}
	for name, p := range pools {
		if get(p, "spec", "disruption", "consolidationPolicy") != "WhenEmpty" || get(p, "spec", "disruption", "consolidateAfter") != DefaultConsolidateAfter {
			t.Fatalf("%s disruption", name)
		}
		taints := get(p, "spec", "template", "spec", "taints").([]any)
		if taints[0].(map[string]any)["key"] != GPUTaintKey {
			t.Fatalf("%s taint", name)
		}
		if get(p, "spec", "template", "metadata", "labels").(map[string]any)[ClassLabel] != name {
			t.Fatalf("%s label", name)
		}
	}
	h100 := pools["gpu-h100"]
	reqs := get(h100, "spec", "template", "spec", "requirements").([]any)
	if !strings.Contains(toString(reqs), "reserved") {
		t.Fatalf("h100 must require reserved capacity: %v", reqs)
	}
	if get(h100, "spec", "limits").(map[string]any)["nvidia.com/gpu"] != "8" {
		t.Fatalf("h100 limits %v", get(h100, "spec", "limits"))
	}
	if get(pools["gpu-l4"], "spec", "limits").(map[string]any)["cpu"] != "96" {
		t.Fatalf("l4 cpu limit %v", get(pools["gpu-l4"], "spec", "limits"))
	}
	if !strings.Contains(toString(get(classes["aibox-gpu-h100"], "spec", "capacityReservationSelectorTerms")), "cr-00000000000000000") {
		t.Fatal("h100 node class must select the profile's reservation")
	}
	if get(classes["aibox-gpu-l4"], "spec", "capacityReservationSelectorTerms") != nil {
		t.Fatal("on-demand classes must not select reservations")
	}
	if get(classes["aibox-gpu-l4"], "spec", "amiSelectorTerms").([]any)[0].(map[string]any)["alias"] != AMIAlias {
		t.Fatal("pinned AMI alias")
	}
}

func TestRenderErrors(t *testing.T) {
	if _, err := Render(load(t, "local")); err == nil {
		t.Fatal("local profile has no GPU classes")
	}
	p := load(t, "aws-dev")
	p.Spec.AWS.GPUClasses[0].InstanceTypes = []string{"z9.huge"}
	if _, err := Render(p); err == nil || !strings.Contains(err.Error(), "unknown instance type") {
		t.Fatalf("got %v", err)
	}
}

func toString(v any) string {
	b, _ := yaml.Marshal(v)
	return string(b)
}
