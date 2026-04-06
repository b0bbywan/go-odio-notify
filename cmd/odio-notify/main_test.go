package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestUsage(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	usage()

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close pipe writer: %v", err)
	}
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("failed to read pipe: %v", err)
	}
	output := buf.String()

	for _, expected := range []string{"play", "list", "version", "Usage"} {
		if !strings.Contains(output, expected) {
			t.Errorf("usage output missing %q", expected)
		}
	}
}

func TestVersionString(t *testing.T) {
	expected := fmt.Sprintf("odio-notify %s\n", version)
	if !strings.Contains(expected, "dev") {
		t.Error("default version should be 'dev'")
	}
}
