// Package scheduling checks that serving pods can only land on GPU nodes: their tolerations
// and node selectors must match the taints and labels of the profile's GPU nodes (Karpenter
// pools on AWS, the fake GPU worker on kind).
package scheduling

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"
)

const root = "../.."

type node struct {
	labels map[string]string
	taints []string // key=value:effect
}

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

// kindGPUNodes reads the fake GPU workers from the kind config.
func kindGPUNodes(t *testing.T) []node {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "infra", "local", "kind-config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Nodes []struct {
			Labels               map[string]string `json:"labels"`
			KubeadmConfigPatches []string          `json:"kubeadmConfigPatches"`
		} `json:"nodes"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	var nodes []node
	for _, n := range cfg.Nodes {
		if _, ok := n.Labels["aibox.dev/gpu-class"]; !ok {
			continue
		}
		gn := node{labels: n.Labels}
		for _, p := range n.KubeadmConfigPatches {
			var jc struct {
				NodeRegistration struct {
					Taints []struct{ Key, Value, Effect string } `json:"taints"`
				} `json:"nodeRegistration"`
			}
			_ = yaml.Unmarshal([]byte(p), &jc)
			for _, tt := range jc.NodeRegistration.Taints {
				gn.taints = append(gn.taints, tt.Key+"="+tt.Value+":"+tt.Effect)
			}
		}
		nodes = append(nodes, gn)
	}
	return nodes
}

func podSpecs(objs []map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, o := range objs {
		if o["kind"] != "Deployment" && o["kind"] != "StatefulSet" {
			continue
		}
		spec, _ := o["spec"].(map[string]any)
		tmpl, _ := spec["template"].(map[string]any)
		ps, _ := tmpl["spec"].(map[string]any)
		meta, _ := o["metadata"].(map[string]any)
		out[meta["name"].(string)] = ps
	}
	return out
}

func schedulable(ps map[string]any, n node) bool {
	sel, _ := ps["nodeSelector"].(map[string]any)
	for k, v := range sel {
		if n.labels[k] != v {
			return false
		}
	}
	tols, _ := ps["tolerations"].([]any)
	for _, taint := range n.taints {
		key, rest, _ := strings.Cut(taint, "=")
		_, effect, _ := strings.Cut(rest, ":")
		ok := false
		for _, tl := range tols {
			m := tl.(map[string]any)
			if m["key"] == key && (m["effect"] == effect || m["effect"] == nil) && (m["operator"] == "Exists" || m["value"] == strings.Split(rest, ":")[0]) {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	return len(sel) > 0 // must select a GPU class explicitly
}

func TestLocalServingRunsOnFakeGPUNode(t *testing.T) {
	nodes := kindGPUNodes(t)
	if len(nodes) != 1 || nodes[0].labels["aibox.dev/gpu-class"] != "gpu-sim" || len(nodes[0].taints) != 1 {
		t.Fatalf("expected one fake GPU worker, got %+v", nodes)
	}
	pods := podSpecs(build(t, "platform/serving/overlays/local"))
	if len(pods) == 0 {
		t.Fatal("no serving workloads")
	}
	for name, ps := range pods {
		if !schedulable(ps, nodes[0]) {
			t.Fatalf("%s cannot schedule on the fake GPU node (selector/tolerations mismatch)", name)
		}
	}
}

func TestSchedulableRules(t *testing.T) {
	n := node{labels: map[string]string{"aibox.dev/gpu-class": "gpu-l4"}, taints: []string{"nvidia.com/gpu=present:NoSchedule"}}
	ok := map[string]any{
		"nodeSelector": map[string]any{"aibox.dev/gpu-class": "gpu-l4"},
		"tolerations":  []any{map[string]any{"key": "nvidia.com/gpu", "operator": "Exists", "effect": "NoSchedule"}},
	}
	if !schedulable(ok, n) {
		t.Fatal("matching pod must schedule")
	}
	noTol := map[string]any{"nodeSelector": map[string]any{"aibox.dev/gpu-class": "gpu-l4"}}
	if schedulable(noTol, n) {
		t.Fatal("pod without toleration must not schedule")
	}
	wrong := map[string]any{"nodeSelector": map[string]any{"aibox.dev/gpu-class": "gpu-a10g"}, "tolerations": ok["tolerations"]}
	if schedulable(wrong, n) {
		t.Fatal("pod selecting another class must not schedule")
	}
}
