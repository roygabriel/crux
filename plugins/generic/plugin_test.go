package generic_test

import (
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/plugin"
	"github.com/roygabriel/crux/pkg/types"
	"github.com/roygabriel/crux/plugins/generic"
)

func validConfig() generic.GenericPluginConfig {
	return generic.GenericPluginConfig{
		Name:             "test-agent",
		Binary:           "my-agent",
		Args:             []string{"--json"},
		ReadyPattern:     `^(>|test-agent>)$`,
		BusyPattern:      `(?i:working\.\.\.|processing\.\.\.)`,
		ErrorPattern:     `(?mi)^\s*(?:Error|error):\s*(.+)$`,
		RateLimitPattern: `(?i)rate.?limit|429`,
		Capabilities:     []string{"code-gen", "file-edit"},
	}
}

func newPlugin(t *testing.T) *generic.Plugin {
	t.Helper()
	p, err := generic.New(validConfig())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	return p
}

func TestNewValidConfig(t *testing.T) {
	t.Parallel()

	p, err := generic.New(validConfig())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if p.Name() != "test-agent" {
		t.Errorf("Name() = %q, want %q", p.Name(), "test-agent")
	}
}

func TestNewInvalidPatterns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*generic.GenericPluginConfig)
		errSub string
	}{
		{
			name:   "invalid-ready-pattern",
			mutate: func(c *generic.GenericPluginConfig) { c.ReadyPattern = "[invalid" },
			errSub: "ready_pattern",
		},
		{
			name:   "invalid-busy-pattern",
			mutate: func(c *generic.GenericPluginConfig) { c.BusyPattern = "[invalid" },
			errSub: "busy_pattern",
		},
		{
			name:   "invalid-error-pattern",
			mutate: func(c *generic.GenericPluginConfig) { c.ErrorPattern = "[invalid" },
			errSub: "error_pattern",
		},
		{
			name:   "invalid-rate-limit-pattern",
			mutate: func(c *generic.GenericPluginConfig) { c.RateLimitPattern = "[invalid" },
			errSub: "rate_limit_pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := validConfig()
			tt.mutate(&cfg)
			_, err := generic.New(cfg)
			if err == nil {
				t.Fatal("New() expected error, got nil")
			}
			if !containsSubstr(err.Error(), tt.errSub) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestNewEmptyPatterns(t *testing.T) {
	t.Parallel()

	cfg := generic.GenericPluginConfig{
		Name:   "minimal",
		Binary: "minimal-agent",
	}
	p, err := generic.New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Empty patterns return safe defaults.
	if p.DetectReady(">\n") {
		t.Error("DetectReady() = true with no pattern, want false")
	}
	if p.DetectBusy("working...\n") {
		t.Error("DetectBusy() = true with no pattern, want false")
	}
	if _, isErr := p.DetectError("Error: something\n"); isErr {
		t.Error("DetectError() = true with no pattern, want false")
	}
	if _, limited := p.DetectRateLimit("rate limit\n"); limited {
		t.Error("DetectRateLimit() = true with no pattern, want false")
	}
}

func TestPluginName(t *testing.T) {
	t.Parallel()

	if got := newPlugin(t).Name(); got != "test-agent" {
		t.Errorf("Name() = %q, want %q", got, "test-agent")
	}
}

func TestLaunchCmd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cfg       plugin.AgentConfig
		wantBin   string
		wantErr   bool
		checkFlag string
	}{
		{
			name: "basic-launch",
			cfg: plugin.AgentConfig{
				ID:         "agent-1",
				WorkDir:    "/tmp/project",
				Permission: types.PermStandard,
			},
			wantBin:   "my-agent",
			checkFlag: "--json",
		},
		{
			name: "empty-workdir",
			cfg: plugin.AgentConfig{
				ID:         "agent-2",
				WorkDir:    "",
				Permission: types.PermStandard,
			},
			wantErr: true,
		},
		{
			name: "extra-args-appended",
			cfg: plugin.AgentConfig{
				ID:         "agent-3",
				WorkDir:    "/tmp/project",
				Permission: types.PermStandard,
				ExtraArgs:  []string{"--verbose"},
			},
			wantBin:   "my-agent",
			checkFlag: "--verbose",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			bin, args, err := newPlugin(t).LaunchCmd(tt.cfg)
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
		{"prompt-named", "some output\ntest-agent>\n", true},
		{"prompt-with-ansi", "some output\n\x1b[32m>\x1b[0m\n", true},
		{"no-prompt", "just some text\n", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := newPlugin(t).DetectReady(tt.paneContent); got != tt.want {
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
		{"working-text", "output\nWorking...\n", true},
		{"processing-text", "output\nProcessing...\n", true},
		{"case-insensitive", "output\nWORKING...\n", true},
		{"ready-prompt", "output\n>\n", false},
		{"empty", "", false},
		{"plain-text", "just regular output\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := newPlugin(t).DetectBusy(tt.paneContent); got != tt.want {
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

			msg, isErr := newPlugin(t).DetectError(tt.paneContent)
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
			wantMin:     60 * time.Second,
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

			dur, limited := newPlugin(t).DetectRateLimit(tt.paneContent)
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

			got := newPlugin(t).FormatMessage(tt.msg)
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
		wantComplete bool
	}{
		{
			name:         "complete",
			paneContent:  "some output\n>\n",
			wantComplete: true,
		},
		{
			name:         "incomplete",
			paneContent:  "Working on it...\n",
			wantComplete: false,
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

			out, err := newPlugin(t).ParseOutput(tt.paneContent)
			if err != nil {
				t.Fatalf("ParseOutput: unexpected error: %v", err)
			}
			if out.Raw != tt.paneContent {
				t.Errorf("Raw = %q, want %q", out.Raw, tt.paneContent)
			}
			if out.IsComplete != tt.wantComplete {
				t.Errorf("IsComplete = %v, want %v", out.IsComplete, tt.wantComplete)
			}
		})
	}
}

func TestCapabilities(t *testing.T) {
	t.Parallel()

	caps := newPlugin(t).Capabilities()
	want := map[plugin.Capability]bool{
		plugin.CapCodeGen:  true,
		plugin.CapFileEdit: true,
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

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
