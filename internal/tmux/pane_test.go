package tmux

import (
	"context"
	"errors"
	"testing"
)

func TestPaneManagerCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		session  string
		dir      string
		runFunc  func(ctx context.Context, args ...string) (string, error)
		wantID   string
		wantErr  bool
		sentinel error
	}{
		{
			name:    "success",
			session: "test-session",
			dir:     "/tmp/work",
			runFunc: func(_ context.Context, args ...string) (string, error) {
				if args[0] != "split-window" {
					t.Errorf("expected split-window, got %s", args[0])
				}
				// Verify -c flag is present when dir is specified.
				found := false
				for i, a := range args {
					if a == "-c" && i+1 < len(args) && args[i+1] == "/tmp/work" {
						found = true
					}
				}
				if !found {
					t.Errorf("expected -c /tmp/work in args %v", args)
				}
				return "%1", nil
			},
			wantID: "%1",
		},
		{
			name:     "invalid-session",
			session:  "bad session",
			dir:      "/tmp",
			runFunc:  func(_ context.Context, _ ...string) (string, error) { return "", nil },
			wantErr:  true,
			sentinel: ErrInvalidSessionName,
		},
		{
			name:    "empty-dir",
			session: "test-session",
			dir:     "",
			runFunc: func(_ context.Context, args ...string) (string, error) {
				// Verify -c flag is NOT present when dir is empty.
				for _, a := range args {
					if a == "-c" {
						t.Errorf("unexpected -c flag in args %v", args)
					}
				}
				return "%2", nil
			},
			wantID: "%2",
		},
		{
			name:    "commander-error",
			session: "test-session",
			dir:     "",
			runFunc: func(_ context.Context, _ ...string) (string, error) {
				return "", errors.New("no space for pane")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pm := NewPaneManager(&mockCommander{runFunc: tt.runFunc}, newTestLogger())
			got, err := pm.Create(context.Background(), tt.session, tt.dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create(%q, %q) error = %v, wantErr %v", tt.session, tt.dir, err, tt.wantErr)
			}
			if tt.sentinel != nil && !errors.Is(err, tt.sentinel) {
				t.Errorf("Create(%q, %q) error should wrap %v, got %v", tt.session, tt.dir, tt.sentinel, err)
			}
			if !tt.wantErr && got != tt.wantID {
				t.Errorf("Create(%q, %q) = %q, want %q", tt.session, tt.dir, got, tt.wantID)
			}
		})
	}
}

func TestPaneManagerList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		session  string
		runFunc  func(ctx context.Context, args ...string) (string, error)
		want     int
		wantErr  bool
		sentinel error
	}{
		{
			name:    "multiple-panes",
			session: "test-session",
			runFunc: func(_ context.Context, _ ...string) (string, error) {
				return "%0:1234:bash\n%1:5678:vim", nil
			},
			want: 2,
		},
		{
			name:    "single-pane",
			session: "test-session",
			runFunc: func(_ context.Context, _ ...string) (string, error) {
				return "%0:1234:bash", nil
			},
			want: 1,
		},
		{
			name:    "empty",
			session: "test-session",
			runFunc: func(_ context.Context, _ ...string) (string, error) {
				return "", nil
			},
			want: 0,
		},
		{
			name:    "commander-error",
			session: "test-session",
			runFunc: func(_ context.Context, _ ...string) (string, error) {
				return "", errors.New("session not found")
			},
			wantErr: true,
		},
		{
			name:     "invalid-session",
			session:  "bad session",
			runFunc:  func(_ context.Context, _ ...string) (string, error) { return "", nil },
			wantErr:  true,
			sentinel: ErrInvalidSessionName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pm := NewPaneManager(&mockCommander{runFunc: tt.runFunc}, newTestLogger())
			got, err := pm.List(context.Background(), tt.session)
			if (err != nil) != tt.wantErr {
				t.Errorf("List(%q) error = %v, wantErr %v", tt.session, err, tt.wantErr)
			}
			if tt.sentinel != nil && !errors.Is(err, tt.sentinel) {
				t.Errorf("List(%q) error should wrap %v, got %v", tt.session, tt.sentinel, err)
			}
			if !tt.wantErr && len(got) != tt.want {
				t.Errorf("List(%q) returned %d panes, want %d", tt.session, len(got), tt.want)
			}
		})
	}
}

func TestParsePaneList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{
			name:  "standard",
			input: "%0:1234:bash\n%1:5678:vim",
			want:  2,
		},
		{
			name:  "empty",
			input: "",
			want:  0,
		},
		{
			name:  "trailing-newline",
			input: "%0:1234:bash\n",
			want:  1,
		},
		{
			name:    "malformed-too-few-fields",
			input:   "%0:1234",
			wantErr: true,
		},
		{
			name:    "bad-pid",
			input:   "%0:notanumber:bash",
			wantErr: true,
		},
		{
			name:  "command-with-colons",
			input: "%0:1234:python3:main.py:8080",
			want:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parsePaneList(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePaneList(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && len(got) != tt.want {
				t.Errorf("parsePaneList(%q) returned %d panes, want %d", tt.input, len(got), tt.want)
			}
		})
	}

	// Verify field values for standard case.
	t.Run("field-values", func(t *testing.T) {
		t.Parallel()
		panes, err := parsePaneList("%5:9999:zsh")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(panes) != 1 {
			t.Fatalf("expected 1 pane, got %d", len(panes))
		}
		p := panes[0]
		if p.ID != "%5" {
			t.Errorf("ID = %q, want %%5", p.ID)
		}
		if p.PID != 9999 {
			t.Errorf("PID = %d, want 9999", p.PID)
		}
		if p.Command != "zsh" {
			t.Errorf("Command = %q, want zsh", p.Command)
		}
	})

	// Verify command with colons preserves them.
	t.Run("colon-command-value", func(t *testing.T) {
		t.Parallel()
		panes, err := parsePaneList("%0:1234:python3:main.py:8080")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if panes[0].Command != "python3:main.py:8080" {
			t.Errorf("Command = %q, want python3:main.py:8080", panes[0].Command)
		}
	})
}

func TestPaneManagerCapture(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		paneID  string
		lines   int
		runFunc func(ctx context.Context, args ...string) (string, error)
		want    string
		wantErr bool
	}{
		{
			name:   "success",
			paneID: "%0",
			lines:  100,
			runFunc: func(_ context.Context, args ...string) (string, error) {
				if args[0] != "capture-pane" {
					t.Errorf("expected capture-pane, got %s", args[0])
				}
				// Verify -S flag is present with correct value.
				found := false
				for i, a := range args {
					if a == "-S" && i+1 < len(args) && args[i+1] == "-100" {
						found = true
					}
				}
				if !found {
					t.Errorf("expected -S -100 in args %v", args)
				}
				return "$ echo hello\nhello\n$", nil
			},
			want: "$ echo hello\nhello\n$",
		},
		{
			name:   "zero-lines-no-S-flag",
			paneID: "%0",
			lines:  0,
			runFunc: func(_ context.Context, args ...string) (string, error) {
				for _, a := range args {
					if a == "-S" {
						t.Errorf("unexpected -S flag in args %v", args)
					}
				}
				return "visible content", nil
			},
			want: "visible content",
		},
		{
			name:    "empty-pane-id",
			paneID:  "",
			lines:   50,
			runFunc: func(_ context.Context, _ ...string) (string, error) { return "", nil },
			wantErr: true,
		},
		{
			name:   "commander-error",
			paneID: "%0",
			lines:  50,
			runFunc: func(_ context.Context, _ ...string) (string, error) {
				return "", errors.New("pane not found")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pm := NewPaneManager(&mockCommander{runFunc: tt.runFunc}, newTestLogger())
			got, err := pm.Capture(context.Background(), tt.paneID, tt.lines)
			if (err != nil) != tt.wantErr {
				t.Errorf("Capture(%q, %d) error = %v, wantErr %v", tt.paneID, tt.lines, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Capture(%q, %d) = %q, want %q", tt.paneID, tt.lines, got, tt.want)
			}
		})
	}
}

func TestPaneManagerSendKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		paneID  string
		text    string
		runFunc func(ctx context.Context, args ...string) (string, error)
		wantErr bool
	}{
		{
			name:   "success",
			paneID: "%0",
			text:   "echo hello",
			runFunc: func(_ context.Context, args ...string) (string, error) {
				if args[0] != "send-keys" {
					t.Errorf("expected send-keys, got %s", args[0])
				}
				// Verify Enter is the last arg.
				if args[len(args)-1] != "Enter" {
					t.Errorf("expected last arg to be Enter, got %s", args[len(args)-1])
				}
				// Verify the text arg is present.
				if args[3] != "echo hello" {
					t.Errorf("expected text arg 'echo hello', got %s", args[3])
				}
				return "", nil
			},
		},
		{
			name:    "empty-pane-id",
			paneID:  "",
			text:    "echo hello",
			runFunc: func(_ context.Context, _ ...string) (string, error) { return "", nil },
			wantErr: true,
		},
		{
			name:    "unsafe-input-semicolon",
			paneID:  "%0",
			text:    "echo hello; rm -rf /",
			runFunc: func(_ context.Context, _ ...string) (string, error) { return "", nil },
			wantErr: true,
		},
		{
			name:    "unsafe-input-backtick",
			paneID:  "%0",
			text:    "echo `whoami`",
			runFunc: func(_ context.Context, _ ...string) (string, error) { return "", nil },
			wantErr: true,
		},
		{
			name:    "unsafe-input-newline",
			paneID:  "%0",
			text:    "line1\nline2",
			runFunc: func(_ context.Context, _ ...string) (string, error) { return "", nil },
			wantErr: true,
		},
		{
			name:   "commander-error",
			paneID: "%0",
			text:   "echo hello",
			runFunc: func(_ context.Context, _ ...string) (string, error) {
				return "", errors.New("pane not found")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pm := NewPaneManager(&mockCommander{runFunc: tt.runFunc}, newTestLogger())
			err := pm.SendKeys(context.Background(), tt.paneID, tt.text)
			if (err != nil) != tt.wantErr {
				t.Errorf("SendKeys(%q, %q) error = %v, wantErr %v", tt.paneID, tt.text, err, tt.wantErr)
			}
		})
	}
}

func TestPaneManagerSendKeysRaw(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		paneID  string
		keys    []string
		runFunc func(ctx context.Context, args ...string) (string, error)
		wantErr bool
	}{
		{
			name:   "success-single-key",
			paneID: "%0",
			keys:   []string{"C-c"},
			runFunc: func(_ context.Context, args ...string) (string, error) {
				if args[0] != "send-keys" {
					t.Errorf("expected send-keys, got %s", args[0])
				}
				if args[3] != "C-c" {
					t.Errorf("expected C-c, got %s", args[3])
				}
				return "", nil
			},
		},
		{
			name:   "success-multiple-keys",
			paneID: "%0",
			keys:   []string{"Escape", ":", "q", "Enter"},
			runFunc: func(_ context.Context, args ...string) (string, error) {
				// args: send-keys -t %0 Escape : q Enter
				if len(args) != 7 {
					t.Errorf("expected 7 args, got %d: %v", len(args), args)
				}
				return "", nil
			},
		},
		{
			name:    "empty-pane-id",
			paneID:  "",
			keys:    []string{"C-c"},
			runFunc: func(_ context.Context, _ ...string) (string, error) { return "", nil },
			wantErr: true,
		},
		{
			name:    "no-keys",
			paneID:  "%0",
			keys:    []string{},
			runFunc: func(_ context.Context, _ ...string) (string, error) { return "", nil },
			wantErr: true,
		},
		{
			name:   "commander-error",
			paneID: "%0",
			keys:   []string{"C-c"},
			runFunc: func(_ context.Context, _ ...string) (string, error) {
				return "", errors.New("pane not found")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pm := NewPaneManager(&mockCommander{runFunc: tt.runFunc}, newTestLogger())
			err := pm.SendKeysRaw(context.Background(), tt.paneID, tt.keys...)
			if (err != nil) != tt.wantErr {
				t.Errorf("SendKeysRaw(%q, %v) error = %v, wantErr %v", tt.paneID, tt.keys, err, tt.wantErr)
			}
		})
	}
}

func TestPaneManagerKill(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		paneID  string
		runFunc func(ctx context.Context, args ...string) (string, error)
		wantErr bool
	}{
		{
			name:   "success",
			paneID: "%0",
			runFunc: func(_ context.Context, args ...string) (string, error) {
				if args[0] != "kill-pane" {
					t.Errorf("expected kill-pane, got %s", args[0])
				}
				return "", nil
			},
		},
		{
			name:    "empty-pane-id",
			paneID:  "",
			runFunc: func(_ context.Context, _ ...string) (string, error) { return "", nil },
			wantErr: true,
		},
		{
			name:   "commander-error",
			paneID: "%0",
			runFunc: func(_ context.Context, _ ...string) (string, error) {
				return "", errors.New("pane not found")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pm := NewPaneManager(&mockCommander{runFunc: tt.runFunc}, newTestLogger())
			err := pm.Kill(context.Background(), tt.paneID)
			if (err != nil) != tt.wantErr {
				t.Errorf("Kill(%q) error = %v, wantErr %v", tt.paneID, err, tt.wantErr)
			}
		})
	}
}
