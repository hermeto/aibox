package platform

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"sigs.k8s.io/yaml"
)

// Every NodePort a local overlay declares must be mapped to the host by the kind config,
// or the endpoint exists in the cluster but 127.0.0.1:<port> is unreachable.
func TestLocalNodePortsAreMappedByKind(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "infra", "local", "kind-config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Nodes []struct {
			ExtraPortMappings []struct {
				ContainerPort int `json:"containerPort"`
			} `json:"extraPortMappings"`
		} `json:"nodes"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	mapped := map[int]bool{}
	for _, n := range cfg.Nodes {
		for _, m := range n.ExtraPortMappings {
			mapped[m.ContainerPort] = true
		}
	}
	// `nodePort: N` in manifests, or a JSON patch on .../nodePort followed by `value: N`.
	re := regexp.MustCompile(`nodePort(?::\s*(\d+)|\s*\n\s*value:\s*(\d+))`)
	files, err := filepath.Glob(filepath.Join(root, "platform", "*", "overlays", "local", "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			s := m[1] + m[2]
			p, err := strconv.Atoi(s)
			if err != nil {
				t.Fatalf("%s: nodePort %q", f, s)
			}
			found++
			if !mapped[p] {
				t.Errorf("%s: NodePort %d has no extraPortMappings entry in infra/local/kind-config.yaml", f, p)
			}
		}
	}
	if found < 5 {
		t.Fatalf("found only %d local NodePorts; the pattern no longer matches the overlays", found)
	}
}
