package tmux

import (
	"errors"
	"strings"
	"testing"
)

func TestSanitizeInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantErr  bool
		sentinel error
	}{
		{name: "simple-text", input: "hello world"},
		{name: "go-command", input: "go test ./..."},
		{name: "path-with-slashes", input: "cat /etc/hosts"},
		{name: "flags", input: "ls -la --color=auto"},
		{name: "equals-sign", input: "KEY=value"},
		{name: "quoted-string", input: `echo "hello"`},
		{name: "single-quotes", input: "echo 'hello'"},
		{name: "empty-string", input: ""},
		{name: "single-char", input: "a"},
		{name: "exactly-max-length", input: strings.Repeat("a", MaxInputLength)},

		// Rejection cases.
		{
			name:     "semicolon",
			input:    "echo hello; rm -rf /",
			wantErr:  true,
			sentinel: ErrUnsafeInput,
		},
		{
			name:     "double-ampersand",
			input:    "true && echo pwned",
			wantErr:  true,
			sentinel: ErrUnsafeInput,
		},
		{
			name:     "double-pipe",
			input:    "false || echo pwned",
			wantErr:  true,
			sentinel: ErrUnsafeInput,
		},
		{
			name:     "dollar-paren",
			input:    "echo $(whoami)",
			wantErr:  true,
			sentinel: ErrUnsafeInput,
		},
		{
			name:     "backtick",
			input:    "echo `whoami`",
			wantErr:  true,
			sentinel: ErrUnsafeInput,
		},
		{
			name:     "newline",
			input:    "line1\nline2",
			wantErr:  true,
			sentinel: ErrUnsafeInput,
		},
		{
			name:     "exceeds-max-length",
			input:    strings.Repeat("a", MaxInputLength+1),
			wantErr:  true,
			sentinel: ErrInputTooLong,
		},
		{
			name:     "semicolon-at-start",
			input:    ";echo pwned",
			wantErr:  true,
			sentinel: ErrUnsafeInput,
		},
		{
			name:     "semicolon-at-end",
			input:    "echo hello;",
			wantErr:  true,
			sentinel: ErrUnsafeInput,
		},
		{
			name:     "dollar-paren-nested",
			input:    "echo $(echo $(whoami))",
			wantErr:  true,
			sentinel: ErrUnsafeInput,
		},
		{
			name:     "backtick-in-middle",
			input:    "file-`date`-backup.tar",
			wantErr:  true,
			sentinel: ErrUnsafeInput,
		},
		{
			name:     "newline-only",
			input:    "\n",
			wantErr:  true,
			sentinel: ErrUnsafeInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := SanitizeInput(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("SanitizeInput(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if tt.sentinel != nil && !errors.Is(err, tt.sentinel) {
				t.Errorf("SanitizeInput(%q) error should wrap %v, got %v", tt.input, tt.sentinel, err)
			}
			if !tt.wantErr && got != tt.input {
				t.Errorf("SanitizeInput(%q) = %q, want original input", tt.input, got)
			}
		})
	}
}

func TestValidateLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty", input: ""},
		{name: "normal-text", input: "hello world"},
		{name: "exactly-max", input: strings.Repeat("x", MaxInputLength)},
		{name: "exceeds-max", input: strings.Repeat("x", MaxInputLength+1), wantErr: true},
		{name: "backticks-allowed", input: "```go\nfmt.Println()\n```"},
		{name: "semicolons-allowed", input: "go build ./...; go test ./..."},
		{name: "newlines-allowed", input: "line1\nline2\nline3"},
		{name: "double-ampersand-allowed", input: "go build && go test"},
		{name: "dollar-paren-allowed", input: "echo $(whoami)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateLength(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateLength() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrInputTooLong) {
				t.Errorf("ValidateLength() error should wrap ErrInputTooLong, got %v", err)
			}
		})
	}
}

func TestSanitizeInputAllowsSingleAmpersand(t *testing.T) {
	t.Parallel()
	// A single & (background operator) is not in the reject list.
	got, err := SanitizeInput("echo hello &")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if got != "echo hello &" {
		t.Errorf("got %q, want %q", got, "echo hello &")
	}
}

func TestSanitizeInputAllowsSinglePipe(t *testing.T) {
	t.Parallel()
	// A single | is not in the reject list.
	got, err := SanitizeInput("echo hello | grep hi")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if got != "echo hello | grep hi" {
		t.Errorf("got %q, want %q", got, "echo hello | grep hi")
	}
}
