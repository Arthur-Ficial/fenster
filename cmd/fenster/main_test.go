package main

import (
	"bytes"
	"strings"
	"testing"
)

// First red→green Go test: --version prints something resembling a version.
// At M0 this is the only Go-side test that should pass.
func TestVersionFlag(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--version"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--version returned error: %v", err)
	}
}

func TestServeFlagIsNotImplemented(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--serve"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for --serve at M0, got nil")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("expected 'not implemented' error, got: %v", err)
	}
}
