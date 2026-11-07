// Package gateway checks the rendered serving overlays against Gateway API attachment rules
// that kustomize cannot enforce: a policy's targetRefs resolve only within the policy's own
// namespace, so a namespace override silently detaches it from the Gateway.
package gateway

import (
	"path/filepath"
	"testing"

	"github.com/hermeto/aibox/internal/serving"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

const root = "../.."

var profiles = []string{"local", "aws-dev", "aws-bench"}

func build(t *testing.T, dir string) []map[string]any {
	t.Helper()
	k := krusty.MakeKustomizer(krusty.MakeDefaultOptions())
	res, err := k.Run(filesys.MakeFsOnDisk(), filepath.Join(root, dir))
	if err != nil {
		t.Fatalf("kustomize build %s: %v", dir, err)
	}
	var out []map[string]any
	for _, r := range res.Resources() {
		m, err := r.Map()
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, m)
	}
	return out
}

func TestGatewayPoliciesLiveInTheGatewayNamespace(t *testing.T) {
	for _, p := range profiles {
		t.Run(p, func(t *testing.T) {
			found := 0
			for _, obj := range build(t, filepath.Join("platform", "serving", "overlays", p)) {
				if obj["kind"] != "BackendTrafficPolicy" {
					continue
				}
				meta, _ := obj["metadata"].(map[string]any)
				spec, _ := obj["spec"].(map[string]any)
				refs, _ := spec["targetRefs"].([]any)
				for _, r := range refs {
					ref, _ := r.(map[string]any)
					if ref["kind"] != "Gateway" {
						continue
					}
					found++
					if meta["namespace"] != serving.GatewayNamespace {
						t.Errorf("BackendTrafficPolicy %v targets Gateway %v but is in namespace %v, want %s",
							meta["name"], ref["name"], meta["namespace"], serving.GatewayNamespace)
					}
				}
			}
			if found == 0 {
				t.Fatal("no BackendTrafficPolicy targets the Gateway")
			}
		})
	}
}

func TestEveryServingObjectHasANamespace(t *testing.T) {
	for _, p := range profiles {
		t.Run(p, func(t *testing.T) {
			for _, obj := range build(t, filepath.Join("platform", "serving", "overlays", p)) {
				meta, _ := obj["metadata"].(map[string]any)
				if obj["kind"] == "Namespace" {
					continue
				}
				if ns, _ := meta["namespace"].(string); ns == "" {
					t.Errorf("%v %v has no namespace", obj["kind"], meta["name"])
				}
			}
		})
	}
}
