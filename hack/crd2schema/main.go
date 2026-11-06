// Command crd2schema converts CustomResourceDefinitions into strict JSON Schemas for
// kubeconform, written as <out>/<group>/<kind>_<version>.json (kind lowercased).
//
// With -openapi and -kind it instead extracts a built-in kind from a Kubernetes OpenAPI v3
// document (used for CustomResourceDefinition itself, which the default kubeconform
// schema registry does not publish).
//
// Usage:
//
//	go run ./hack/crd2schema -out hack/schemas hack/crds/*.yaml
//	go run ./hack/crd2schema -out hack/schemas -openapi spec.json -kind CustomResourceDefinition
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
			Name    string `json:"name"`
			Storage bool   `json:"storage"`
			Schema  struct {
				OpenAPIV3Schema map[string]any `json:"openAPIV3Schema"`
			} `json:"schema"`
		} `json:"versions"`
	} `json:"spec"`
}

func main() {
	out := flag.String("out", "hack/schemas", "output directory")
	openapi := flag.String("openapi", "", "Kubernetes OpenAPI v3 document to extract -kind from")
	kind := flag.String("kind", "", "built-in kind to extract from -openapi")
	flag.Parse()
	if *openapi != "" {
		if err := convertOpenAPI(*openapi, *kind, *out); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
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
			if !v.Storage { // AIBox manifests use storage versions; this keeps the files small
				continue
			}
			schema := v.Schema.OpenAPIV3Schema
			strict(schema)
			if err := write(out, c.Spec.Group, c.Spec.Names.Kind, v.Name, schema); err != nil {
				return err
			}
		}
	}
	return nil
}

// convertOpenAPI writes the schema of a built-in kind with every definition it references
// (OpenAPI component references become draft-07 definitions, so recursive types work).
func convertOpenAPI(path, kind, out string) error {
	data, err := os.ReadFile(path) //nolint:gosec // developer-provided path
	if err != nil {
		return err
	}
	var doc struct {
		Components struct {
			Schemas map[string]map[string]any `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	const prefix = "#/components/schemas/"
	defs := map[string]any{}
	var visit func(node any)
	visit = func(node any) {
		switch n := node.(type) {
		case map[string]any:
			for key, val := range n {
				if ref, ok := val.(string); ok && key == "$ref" && strings.HasPrefix(ref, prefix) {
					name := strings.TrimPrefix(ref, prefix)
					n[key] = "#/definitions/" + name
					if _, seen := defs[name]; !seen {
						def := doc.Components.Schemas[name]
						defs[name] = def
						strict(def)
						visit(def)
					}
					continue
				}
				visit(val)
			}
		case []any:
			for _, val := range n {
				visit(val)
			}
		}
	}
	for _, schema := range doc.Components.Schemas {
		gvks, _ := schema["x-kubernetes-group-version-kind"].([]any)
		for _, g := range gvks {
			gvk, _ := g.(map[string]any)
			if gvk["kind"] != kind {
				continue
			}
			strict(schema)
			visit(schema)
			// The API server ignores status on create; controller-gen emits it with null fields.
			if props, ok := schema["properties"].(map[string]any); ok {
				if _, ok := props["status"]; ok {
					props["status"] = map[string]any{}
				}
			}
			schema["definitions"] = defs
			group, _ := gvk["group"].(string)
			version, _ := gvk["version"].(string)
			return write(out, group, kind, version, schema)
		}
	}
	return fmt.Errorf("%s: kind %q not found", path, kind)
}

func write(out, group, kind, version string, schema map[string]any) error {
	stripDescriptions(schema)
	schema["$schema"] = "http://json-schema.org/draft-07/schema#"
	buf, err := json.Marshal(schema)
	if err != nil {
		return err
	}
	dir := filepath.Join(out, group)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	file := filepath.Join(dir, strings.ToLower(kind)+"_"+version+".json")
	if err := os.WriteFile(file, append(buf, '\n'), 0o600); err != nil {
		return err
	}
	fmt.Println("wrote", file)
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

// stripDescriptions removes documentation from a schema (it is not needed for validation and
// makes up most of the size of large CRDs). A property named "description" is kept.
func stripDescriptions(node any) {
	switch n := node.(type) {
	case map[string]any:
		if d, ok := n["description"]; ok {
			if _, isString := d.(string); isString {
				delete(n, "description")
			}
		}
		for _, v := range n {
			stripDescriptions(v)
		}
	case []any:
		for _, v := range n {
			stripDescriptions(v)
		}
	}
}
