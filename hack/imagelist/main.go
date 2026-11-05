// Command imagelist reads a Kubernetes YAML stream on stdin and prints every container
// image (containers, initContainers, ephemeralContainers of any pod template), one per line,
// sorted and de-duplicated.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"

	"sigs.k8s.io/yaml"
)

func main() {
	data, err := io.ReadAll(bufio.NewReader(os.Stdin))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	images := map[string]bool{}
	for _, doc := range bytes.Split(data, []byte("\n---")) {
		var obj map[string]any
		if err := yaml.Unmarshal(doc, &obj); err != nil || obj == nil {
			continue
		}
		walk(obj, images)
	}
	list := make([]string, 0, len(images))
	for img := range images {
		list = append(list, img)
	}
	sort.Strings(list)
	for _, img := range list {
		fmt.Println(img)
	}
}

// walk finds container lists anywhere in the object (Deployments, DaemonSets, Jobs, CronJobs…).
func walk(v any, images map[string]bool) {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			if k == "containers" || k == "initContainers" || k == "ephemeralContainers" {
				if list, ok := child.([]any); ok {
					for _, c := range list {
						if m, ok := c.(map[string]any); ok {
							if img, ok := m["image"].(string); ok && img != "" {
								images[img] = true
							}
						}
					}
				}
			}
			walk(child, images)
		}
	case []any:
		for _, child := range t {
			walk(child, images)
		}
	}
}
