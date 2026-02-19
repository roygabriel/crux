package main

import (
	"strings"
	"testing"
)

func TestFormatVersionOutput(t *testing.T) {
	got := formatVersionOutput("v1.2.3", "abc123", "2026-02-19T00:00:00Z", "false")
	want := "crux v1.2.3 (commit=abc123 date=2026-02-19T00:00:00Z dirty=false)"
	if got != want {
		t.Fatalf("formatVersionOutput() = %q, want %q", got, want)
	}
}

func TestFormatVersionOutputDefaults(t *testing.T) {
	got := formatVersionOutput("", "", "", "")
	if !strings.Contains(got, "crux dev") {
		t.Fatalf("expected dev default, got %q", got)
	}
	if !strings.Contains(got, "commit=unknown") {
		t.Fatalf("expected unknown commit default, got %q", got)
	}
}

func TestRootVersionFlagHasShorthand(t *testing.T) {
	f := rootCmd.Flags().Lookup("version")
	if f == nil {
		t.Fatal("expected --version flag to exist")
	}
	if f.Shorthand != "v" {
		t.Fatalf("version shorthand = %q, want %q", f.Shorthand, "v")
	}
}
