// Package gateway checks the rendered gateway and serving overlays against Gateway API rules
// that kustomize cannot enforce: a policy's targetRefs resolve only within the policy's own
// namespace, so a namespace override silently detaches it from the Gateway.
package gateway

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hermeto/aibox/internal/serving"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"
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

// kind maps host ports on the control-plane node only, so the local Envoy Service must forward
// NodePort traffic to Envoy pods on other nodes. The gateway base inflates Helm charts, so the
// overlay's EnvoyProxy patch is checked directly instead of rendering the overlay.
func TestLocalEnvoyServiceForwardsAcrossNodes(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(root, "platform", "gateway", "overlays", "local", "kustomization.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var k struct {
		Patches []struct {
			Target struct{ Kind string } `json:"target"`
			Patch  string                `json:"patch"`
		} `json:"patches"`
	}
	if err := yaml.Unmarshal(data, &k); err != nil {
		t.Fatal(err)
	}
	for _, p := range k.Patches {
		if p.Target.Kind != "EnvoyProxy" {
			continue
		}
		var ops []struct {
			Path  string         `json:"path"`
			Value map[string]any `json:"value"`
		}
		if err := yaml.Unmarshal([]byte(p.Patch), &ops); err != nil {
			t.Fatal(err)
		}
		for _, op := range ops {
			if op.Path != "/spec/provider/kubernetes/envoyService" {
				continue
			}
			if op.Value["type"] != "NodePort" || op.Value["externalTrafficPolicy"] != "Cluster" {
				t.Fatalf("local Envoy Service: type=%v externalTrafficPolicy=%v, want NodePort and Cluster",
					op.Value["type"], op.Value["externalTrafficPolicy"])
			}
			return
		}
	}
	t.Fatal("no EnvoyProxy envoyService patch in the local gateway overlay")
}
