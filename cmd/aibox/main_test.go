package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunPrintsPlaceholder(t *testing.T) {
	var buf bytes.Buffer
	if code := run(&buf); code != 0 {
		t.Fatalf("run() = %d, want 0", code)
	}
	if !strings.Contains(buf.String(), "under construction") {
		t.Fatalf("unexpected output %q", buf.String())
	}
}
