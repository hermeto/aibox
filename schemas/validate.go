package schemas

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// Issue is one schema violation at a JSON pointer.
type Issue struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

var (
	mu       sync.Mutex
	compiled = map[string]*jsonschema.Schema{}
)

func compile(name string) (*jsonschema.Schema, error) {
	mu.Lock()
	defer mu.Unlock()
	if s, ok := compiled[name]; ok {
		return s, nil
	}
	raw, err := FS.ReadFile(name)
	if err != nil {
		return nil, err
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource(name, doc); err != nil {
		return nil, err
	}
	s, err := c.Compile(name)
	if err != nil {
		return nil, err
	}
	compiled[name] = s
	return s, nil
}

// Validate checks a JSON document against the named schema and returns every violation,
// sorted by field. An error means the schema itself could not be compiled.
func Validate(name string, jsonData []byte) ([]Issue, error) {
	sch, err := compile(name)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", name, err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(jsonData))
	if err != nil {
		return []Issue{{Field: "/", Reason: err.Error()}}, nil
	}
	if err := sch.Validate(inst); err != nil {
		var ve *jsonschema.ValidationError
		if errors.As(err, &ve) {
			return flatten(ve), nil
		}
		return nil, err
	}
	return nil, nil
}

// flatten reports the leaf causes (the useful messages) with their instance paths.
func flatten(ve *jsonschema.ValidationError) []Issue {
	printer := message.NewPrinter(language.English)
	seen := map[string]bool{}
	var issues []Issue
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) > 0 {
			for _, c := range e.Causes {
				walk(c)
			}
			return
		}
		field := "/" + strings.Join(e.InstanceLocation, "/")
		reason := e.ErrorKind.LocalizedString(printer)
		key := field + "\x00" + reason
		if seen[key] {
			return
		}
		seen[key] = true
		issues = append(issues, Issue{Field: field, Reason: reason})
	}
	walk(ve)
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Field != issues[j].Field {
			return issues[i].Field < issues[j].Field
		}
		return issues[i].Reason < issues[j].Reason
	})
	return issues
}
