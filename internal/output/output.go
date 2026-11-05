// Package output renders command results as human-readable text or JSON.
package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// Format is an output format.
type Format string

// Supported formats.
const (
	Text Format = "text"
	JSON Format = "json"
)

// Parse validates a format flag value.
func Parse(s string) (Format, error) {
	switch Format(s) {
	case Text, JSON:
		return Format(s), nil
	default:
		return "", fmt.Errorf("invalid output format %q (want text or json)", s)
	}
}

// TextRenderer is implemented by values that know how to print themselves as text.
type TextRenderer interface {
	RenderText(w io.Writer) error
}

// Print writes v to w in the given format. In text mode v must implement TextRenderer
// or be a string.
func Print(w io.Writer, f Format, v any) error {
	if f == JSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	switch t := v.(type) {
	case TextRenderer:
		return t.RenderText(w)
	case string:
		_, err := fmt.Fprintln(w, t)
		return err
	default:
		return fmt.Errorf("output: %T cannot be rendered as text", v)
	}
}
