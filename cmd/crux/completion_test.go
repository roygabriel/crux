package main

import (
	"bytes"
	"strings"
	"testing"
)

func executeCommand(args ...string) (string, error) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	}()
	err := rootCmd.Execute()
	return buf.String(), err
}

func TestCompletion_AllShells(t *testing.T) {
	tests := []struct {
		shell string
	}{
		{"bash"},
		{"zsh"},
		{"fish"},
		{"powershell"},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			out, err := executeCommand("completion", tt.shell)
			if err != nil {
				t.Fatalf("completion %s returned error: %v", tt.shell, err)
			}
			if out == "" {
				t.Errorf("completion %s produced empty output", tt.shell)
			}
			if !strings.Contains(strings.ToLower(out), "crux") {
				t.Errorf("completion %s output does not contain 'crux'", tt.shell)
			}
		})
	}
}

func TestCompletion_InvalidShell(t *testing.T) {
	_, err := executeCommand("completion", "invalid")
	if err == nil {
		t.Error("expected error for invalid shell, got nil")
	}
}

func TestCompletion_NoArgs(t *testing.T) {
	_, err := executeCommand("completion")
	if err == nil {
		t.Error("expected error for no args, got nil")
	}
}
