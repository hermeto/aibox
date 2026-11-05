package output

import (
	"bytes"
	"io"
	"testing"
)

type thing struct{ N int }

func (t thing) RenderText(w io.Writer) error {
	_, err := io.WriteString(w, "thing\n")
	return err
}

func TestParse(t *testing.T) {
	for _, ok := range []string{"text", "json"} {
		if _, err := Parse(ok); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Parse("yaml"); err == nil {
		t.Fatal("expected error")
	}
}

func TestPrint(t *testing.T) {
	var b bytes.Buffer
	if err := Print(&b, JSON, thing{N: 1}); err != nil || b.String() != "{\n  \"N\": 1\n}\n" {
		t.Fatalf("json: %q %v", b.String(), err)
	}
	b.Reset()
	if err := Print(&b, Text, thing{}); err != nil || b.String() != "thing\n" {
		t.Fatalf("text: %q %v", b.String(), err)
	}
	b.Reset()
	if err := Print(&b, Text, "hi"); err != nil || b.String() != "hi\n" {
		t.Fatalf("string: %q %v", b.String(), err)
	}
	if err := Print(&b, Text, 42); err == nil {
		t.Fatal("expected error for non-renderable")
	}
}
