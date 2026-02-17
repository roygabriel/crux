package claude_test

import (
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/plugin"
	"github.com/roygabriel/crux/pkg/types"
	"github.com/roygabriel/crux/plugins/claude"
)

func newPlugin() *claude.Plugin {
	return claude.New()
}

func TestPluginName(t *testing.T) {
	t.Parallel()

	if got := newPlugin().Name(); got != "claude" {
		t.Errorf("Name() = %q, want %q", got, "claude")
	}
}

func TestLaunchCmd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cfg       plugin.AgentConfig
		wantBin   string
		wantErr   bool
		checkFlag string // flag that must appear in args
	}{
		{
			name: "standard-permission",
			cfg: plugin.AgentConfig{
				ID:         "agent-1",
				WorkDir:    "/tmp/project",
				Permission: types.PermStandard,
			},
			wantBin: "claude",
		},
		{
			name: "autonomous-adds-skip-permissions",
			cfg: plugin.AgentConfig{
				ID:         "agent-2",
				WorkDir:    "/tmp/project",
				Permission: types.PermAutonomous,
			},
			wantBin:   "claude",
			checkFlag: "--dangerously-skip-permissions",
		},
		{
			name: "empty-workdir",
			cfg: plugin.AgentConfig{
				ID:         "agent-3",
				WorkDir:    "",
				Permission: types.PermStandard,
			},
			wantErr: true,
		},
		{
			name: "extra-args-appended",
			cfg: plugin.AgentConfig{
				ID:         "agent-4",
				WorkDir:    "/tmp/project",
				Permission: types.PermStandard,
				ExtraArgs:  []string{"--verbose"},
			},
			wantBin:   "claude",
			checkFlag: "--verbose",
		},
		{
			name: "output-format-json-present",
			cfg: plugin.AgentConfig{
				ID:         "agent-5",
				WorkDir:    "/tmp/project",
				Permission: types.PermStandard,
			},
			wantBin:   "claude",
			checkFlag: "--output-format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bin, args, err := newPlugin().LaunchCmd(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatal("LaunchCmd: expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("LaunchCmd: unexpected error: %v", err)
			}
			if bin != tt.wantBin {
				t.Errorf("bin = %q, want %q", bin, tt.wantBin)
			}
			if tt.checkFlag != "" && !containsStr(args, tt.checkFlag) {
				t.Errorf("args %v missing expected flag %q", args, tt.checkFlag)
			}
		})
	}
}

func TestDetectReady(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		paneContent string
		want        bool
	}{
		{"prompt-arrow", "some output\n>\n", true},
		{"prompt-claude", "some output\nclaude>\n", true},
		{"prompt-arrow-trailing-space", "some output\n>  \n", true},
		{"prompt-with-ansi", "some output\n\x1b[32m>\x1b[0m\n", true},
		{"busy-spinner", "some output\n⠋ Thinking...\n", false},
		{"no-prompt", "just some text\n", false},
		{"empty", "", false},
		{"arrow-mid-output", "> some text\nmore output\nstill busy\n⠙\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := newPlugin().DetectReady(tt.paneContent); got != tt.want {
				t.Errorf("DetectReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectBusy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		paneContent string
		want        bool
	}{
		{"spinner-braille", "output\n⠋ Working...\n", true},
		{"thinking-text", "line 1\nline 2\nThinking...\n", true},
		{"working-text", "output\nWorking...\n", true},
		{"generating-text", "output\nGenerating...\n", true},
		{"reading-text", "output\nReading...\n", true},
		{"case-insensitive", "output\nTHINKING...\n", true},
		{"ready-prompt", "output\n>\n", false},
		{"empty", "", false},
		{"plain-text", "just regular output\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := newPlugin().DetectBusy(tt.paneContent); got != tt.want {
				t.Errorf("DetectBusy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		paneContent string
		wantMsg     string
		wantErr     bool
	}{
		{
			name:        "error-prefix",
			paneContent: "output\nError: file not found\n>",
			wantMsg:     "file not found",
			wantErr:     true,
		},
		{
			name:        "error-lowercase",
			paneContent: "output\nerror: something broke\n>",
			wantMsg:     "something broke",
			wantErr:     true,
		},
		{
			name:        "panic-line",
			paneContent: "output\npanic: runtime error: index out of range\n",
			wantMsg:     "panic: runtime error: index out of range",
			wantErr:     true,
		},
		{
			name:        "fatal-error",
			paneContent: "output\nfatal error: all goroutines are asleep\n",
			wantMsg:     "fatal error: all goroutines are asleep",
			wantErr:     true,
		},
		{
			name:        "clean-output",
			paneContent: "all good\n>\n",
			wantMsg:     "",
			wantErr:     false,
		},
		{
			name:        "empty",
			paneContent: "",
			wantMsg:     "",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			msg, isErr := newPlugin().DetectError(tt.paneContent)
			if isErr != tt.wantErr {
				t.Errorf("DetectError() isError = %v, want %v", isErr, tt.wantErr)
			}
			if isErr && msg != tt.wantMsg {
				t.Errorf("DetectError() msg = %q, want %q", msg, tt.wantMsg)
			}
		})
	}
}

func TestDetectRateLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		paneContent string
		wantLimited bool
		wantMin     time.Duration
	}{
		{
			name:        "rate-limit-with-retry-seconds",
			paneContent: "Rate limit exceeded. Retry after 30 seconds\n",
			wantLimited: true,
			wantMin:     30 * time.Second,
		},
		{
			name:        "429-error",
			paneContent: "HTTP 429 Too Many Requests\n",
			wantLimited: true,
			wantMin:     60 * time.Second, // default when no duration parsed
		},
		{
			name:        "rate-limit-minutes",
			paneContent: "Rate limited, wait 2 minutes\n",
			wantLimited: true,
			wantMin:     2 * time.Minute,
		},
		{
			name:        "clean-output",
			paneContent: "normal output\n>\n",
			wantLimited: false,
			wantMin:     0,
		},
		{
			name:        "empty",
			paneContent: "",
			wantLimited: false,
			wantMin:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dur, limited := newPlugin().DetectRateLimit(tt.paneContent)
			if limited != tt.wantLimited {
				t.Errorf("DetectRateLimit() limited = %v, want %v", limited, tt.wantLimited)
			}
			if limited && dur < tt.wantMin {
				t.Errorf("DetectRateLimit() duration = %v, want >= %v", dur, tt.wantMin)
			}
		})
	}
}

func TestFormatMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  types.Message
		want string
	}{
		{
			name: "task-string-payload",
			msg: types.Message{
				Type:    types.MessageTask,
				Payload: "implement the auth module",
			},
			want: "implement the auth module",
		},
		{
			name: "task-map-payload",
			msg: types.Message{
				Type: types.MessageTask,
				Payload: map[string]any{
					"task": "implement the auth module",
				},
			},
			want: "implement the auth module",
		},
		{
			name: "task-map-with-context-files",
			msg: types.Message{
				Type: types.MessageTask,
				Payload: map[string]any{
					"task":          "implement auth",
					"context_files": []any{"internal/auth/auth.go", "pkg/types/user.go"},
				},
			},
			want: "implement auth\n\nContext files:\n- internal/auth/auth.go\n- pkg/types/user.go",
		},
		{
			name: "status-message",
			msg: types.Message{
				Type:    types.MessageStatus,
				Payload: "checking",
			},
			want: "checking",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := newPlugin().FormatMessage(tt.msg)
			if got != tt.want {
				t.Errorf("FormatMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		paneContent  string
		wantFiles    []string
		wantErrors   int
		wantComplete bool
	}{
		{
			name:         "complete-with-files",
			paneContent:  "Created `internal/foo/bar.go`\nUpdated `internal/foo/baz.go`\n>\n",
			wantFiles:    []string{"internal/foo/bar.go", "internal/foo/baz.go"},
			wantComplete: true,
		},
		{
			name:        "with-errors",
			paneContent: "Error: compilation failed\nerror: test failed\n>\n",
			wantErrors:  2,
			wantComplete: true,
		},
		{
			name:         "incomplete",
			paneContent:  "Working on it...\n⠋ Thinking...\n",
			wantComplete: false,
		},
		{
			name:         "deduplicates-files",
			paneContent:  "Updated `main.go`\nModified `main.go`\n>\n",
			wantFiles:    []string{"main.go"},
			wantComplete: true,
		},
		{
			name:         "empty",
			paneContent:  "",
			wantComplete: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out, err := newPlugin().ParseOutput(tt.paneContent)
			if err != nil {
				t.Fatalf("ParseOutput: unexpected error: %v", err)
			}
			if out.Raw != tt.paneContent {
				t.Errorf("Raw = %q, want %q", out.Raw, tt.paneContent)
			}
			if out.IsComplete != tt.wantComplete {
				t.Errorf("IsComplete = %v, want %v", out.IsComplete, tt.wantComplete)
			}
			if tt.wantFiles != nil {
				if len(out.FilesChanged) != len(tt.wantFiles) {
					t.Fatalf("FilesChanged count = %d, want %d: %v", len(out.FilesChanged), len(tt.wantFiles), out.FilesChanged)
				}
				for i, f := range tt.wantFiles {
					if out.FilesChanged[i] != f {
						t.Errorf("FilesChanged[%d] = %q, want %q", i, out.FilesChanged[i], f)
					}
				}
			}
			if tt.wantErrors > 0 && len(out.Errors) != tt.wantErrors {
				t.Errorf("Errors count = %d, want %d: %v", len(out.Errors), tt.wantErrors, out.Errors)
			}
		})
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()

	caps := newPlugin().Capabilities()
	want := map[plugin.Capability]bool{
		plugin.CapCodeGen:  true,
		plugin.CapFileEdit: true,
		plugin.CapShellExec: true,
	}

	if len(caps) != len(want) {
		t.Fatalf("Capabilities() returned %d items, want %d", len(caps), len(want))
	}
	for _, c := range caps {
		if !want[c] {
			t.Errorf("unexpected capability %q", c)
		}
	}
}

func containsStr(ss []string, s string) bool {
	for _, item := range ss {
		if item == s {
			return true
		}
	}
	return false
}
