package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rollingstart-dev/rollingstart/internal/version"
)

func TestWriteVersionIncludesBuildMetadata(t *testing.T) {
	var buf bytes.Buffer
	if err := writeVersion(&buf); err != nil {
		t.Fatalf("writeVersion: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"rolling", version.Version, "commit", "go", "platform"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestVersionCommandRejectsArgs(t *testing.T) {
	// `rolling version` takes no arguments; a typo like `rolling version foo`
	// should be an error rather than silently ignored.
	if err := versionCmd.Args(versionCmd, []string{"foo"}); err == nil {
		t.Error("expected an error for unexpected argument, got nil")
	}
}
