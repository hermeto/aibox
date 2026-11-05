// Command crd2schema converts CustomResourceDefinitions into strict JSON Schemas for
// kubeconform, written as <out>/<group>/<kind>_<version>.json (kind lowercased).
//
// Usage: go run ./hack/crd2schema -out hack/schemas hack/crds/*.yaml
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

type crd struct {
	Kind string `json:"kind"`
	Spec struct {
		Group string `json:"group"`
		Names struct {
			Kind string `json:"kind"`
		} `json:"names"`
		Versions []struct {
			Name   string `json:"name"`
			Schema struct {
				OpenAPIV3Schema map[string]any `json:"openAPIV3Schema"`
			} `json:"schema"`
		} `json:"versions"`
	} `json:"spec"`
}

func main() {
	out := flag.String("out", "hack/schemas", "output directory")
	flag.Parse()
	for _, path := range flag.Args() {
		if err := convertFile(path, *out); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func convertFile(path, out string) error {
	data, err := os.ReadFile(path) //nolint:gosec // developer-provided path
	if err != nil {
		return err
	}
	for _, doc := range bytes.Split(data, []byte("\n---")) {
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}
		var c crd
		if err := yaml.Unmarshal(doc, &c); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if c.Kind != "CustomResourceDefinition" {
			continue
		}
		for _, v := range c.Spec.Versions {
			schema := v.Schema.OpenAPIV3Schema
			strict(schema)
			schema["$schema"] = "http://json-schema.org/draft-07/schema#"
			buf, err := json.MarshalIndent(schema, "", "  ")
			if err != nil {
				return err
			}
			dir := filepath.Join(out, c.Spec.Group)
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return err
			}
			file := filepath.Join(dir, strings.ToLower(c.Spec.Names.Kind)+"_"+v.Name+".json")
			if err := os.WriteFile(file, append(buf, '\n'), 0o600); err != nil {
				return err
			}
			fmt.Println("wrote", file)
		}
	}
	return nil
}

// strict mirrors kubeconform's openapi2jsonschema strict mode: objects with explicit
// properties reject unknown fields, and int-or-string becomes a JSON Schema union.
func strict(node map[string]any) {
	if node["x-kubernetes-int-or-string"] == true {
		delete(node, "type")
		node["oneOf"] = []any{map[string]any{"type": "string"}, map[string]any{"type": "integer"}}
	}
	if props, ok := node["properties"].(map[string]any); ok {
		if _, set := node["additionalProperties"]; !set && node["x-kubernetes-preserve-unknown-fields"] != true {
			node["additionalProperties"] = false
		}
		for _, p := range props {
			if m, ok := p.(map[string]any); ok {
				strict(m)
			}
		}
	}
	for _, key := range []string{"items", "additionalProperties"} {
		if m, ok := node[key].(map[string]any); ok {
			strict(m)
		}
	}
	for _, key := range []string{"allOf", "anyOf", "oneOf"} {
		if list, ok := node[key].([]any); ok {
			for _, it := range list {
				if m, ok := it.(map[string]any); ok {
					strict(m)
				}
			}
		}
	}
}
