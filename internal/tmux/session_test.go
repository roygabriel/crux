package tmux

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
)

// mockCommander is a test double for Commander that delegates to a closure.
type mockCommander struct {
	runFunc func(ctx context.Context, args ...string) (string, error)
}

func (m *mockCommander) Run(ctx context.Context, args ...string) (string, error) {
	return m.runFunc(ctx, args...)
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestValidateSessionName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "simple", input: "foo", wantErr: false},
		{name: "with-hyphen", input: "my-session", wantErr: false},
		{name: "single-char", input: "a", wantErr: false},
		{name: "alphanumeric", input: "agent1", wantErr: false},
		{name: "numbers-only", input: "123", wantErr: false},
		{name: "long-name", input: "a-b-c-d-e", wantErr: false},
		{name: "empty", input: "", wantErr: true},
		{name: "leading-hyphen", input: "-foo", wantErr: true},
		{name: "trailing-hyphen", input: "foo-", wantErr: true},
		{name: "spaces", input: "foo bar", wantErr: true},
		{name: "underscore", input: "foo_bar", wantErr: true},
		{name: "dots", input: "foo.bar", wantErr: true},
		{name: "slash", input: "foo/bar", wantErr: true},
		{name: "special-chars", input: "foo@bar", wantErr: true},
		{name: "only-hyphen", input: "-", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateSessionName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSessionName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, ErrInvalidSessionName) {
				t.Errorf("validateSessionName(%q) error should wrap ErrInvalidSessionName, got %v", tt.input, err)
			}
		})
	}
}

func TestSessionManagerCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		session  string
		runFunc  func(ctx context.Context, args ...string) (string, error)
		wantErr  bool
		sentinel error
	}{
		{
			name:    "success",
			session: "test-session",
			runFunc: func(_ context.Context, args ...string) (string, error) {
				if args[0] != "new-session" {
					t.Errorf("expected new-session, got %s", args[0])
				}
				return "", nil
			},
			wantErr: false,
		},
		{
			name:     "invalid-name",
			session:  "",
			runFunc:  func(_ context.Context, _ ...string) (string, error) { return "", nil },
			wantErr:  true,
			sentinel: ErrInvalidSessionName,
		},
		{
			name:    "commander-error",
			session: "test-session",
			runFunc: func(_ context.Context, _ ...string) (string, error) {
				return "", errors.New("tmux server not running")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sm := NewSessionManager(&mockCommander{runFunc: tt.runFunc}, newTestLogger())
			err := sm.Create(context.Background(), tt.session)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create(%q) error = %v, wantErr %v", tt.session, err, tt.wantErr)
			}
			if tt.sentinel != nil && !errors.Is(err, tt.sentinel) {
				t.Errorf("Create(%q) error should wrap %v, got %v", tt.session, tt.sentinel, err)
			}
		})
	}
}

func TestSessionManagerExists(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		session  string
		runFunc  func(ctx context.Context, args ...string) (string, error)
		want     bool
		wantErr  bool
		sentinel error
	}{
		{
			name:    "exists",
			session: "test-session",
			runFunc: func(_ context.Context, _ ...string) (string, error) {
				return "", nil
			},
			want: true,
		},
		{
			name:    "does-not-exist",
			session: "test-session",
			runFunc: func(_ context.Context, _ ...string) (string, error) {
				return "", errors.New("session not found")
			},
			want: false,
		},
		{
			name:     "invalid-name",
			session:  "bad name",
			runFunc:  func(_ context.Context, _ ...string) (string, error) { return "", nil },
			want:     false,
			wantErr:  true,
			sentinel: ErrInvalidSessionName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sm := NewSessionManager(&mockCommander{runFunc: tt.runFunc}, newTestLogger())
			got, err := sm.Exists(context.Background(), tt.session)
			if (err != nil) != tt.wantErr {
				t.Errorf("Exists(%q) error = %v, wantErr %v", tt.session, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Exists(%q) = %v, want %v", tt.session, got, tt.want)
			}
			if tt.sentinel != nil && !errors.Is(err, tt.sentinel) {
				t.Errorf("Exists(%q) error should wrap %v, got %v", tt.session, tt.sentinel, err)
			}
		})
	}
}

func TestSessionManagerList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		runFunc func(ctx context.Context, args ...string) (string, error)
		want    int
		wantErr bool
	}{
		{
			name: "multiple-sessions",
			runFunc: func(_ context.Context, _ ...string) (string, error) {
				return "session1\nsession2\nsession3", nil
			},
			want: 3,
		},
		{
			name: "single-session",
			runFunc: func(_ context.Context, _ ...string) (string, error) {
				return "session1", nil
			},
			want: 1,
		},
		{
			name: "empty",
			runFunc: func(_ context.Context, _ ...string) (string, error) {
				return "", nil
			},
			want: 0,
		},
		{
			name: "trailing-newline",
			runFunc: func(_ context.Context, _ ...string) (string, error) {
				return "session1\nsession2\n", nil
			},
			want: 2,
		},
		{
			name: "commander-error",
			runFunc: func(_ context.Context, _ ...string) (string, error) {
				return "", errors.New("server not running")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sm := NewSessionManager(&mockCommander{runFunc: tt.runFunc}, newTestLogger())
			got, err := sm.List(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("List() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && len(got) != tt.want {
				t.Errorf("List() returned %d sessions, want %d", len(got), tt.want)
			}
		})
	}
}

func TestSessionManagerKill(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		session  string
		runFunc  func(ctx context.Context, args ...string) (string, error)
		wantErr  bool
		sentinel error
	}{
		{
			name:    "success",
			session: "test-session",
			runFunc: func(_ context.Context, args ...string) (string, error) {
				if args[0] != "kill-session" {
					t.Errorf("expected kill-session, got %s", args[0])
				}
				return "", nil
			},
		},
		{
			name:     "invalid-name",
			session:  "",
			runFunc:  func(_ context.Context, _ ...string) (string, error) { return "", nil },
			wantErr:  true,
			sentinel: ErrInvalidSessionName,
		},
		{
			name:    "commander-error",
			session: "test-session",
			runFunc: func(_ context.Context, _ ...string) (string, error) {
				return "", errors.New("session not found")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sm := NewSessionManager(&mockCommander{runFunc: tt.runFunc}, newTestLogger())
			err := sm.Kill(context.Background(), tt.session)
			if (err != nil) != tt.wantErr {
				t.Errorf("Kill(%q) error = %v, wantErr %v", tt.session, err, tt.wantErr)
			}
			if tt.sentinel != nil && !errors.Is(err, tt.sentinel) {
				t.Errorf("Kill(%q) error should wrap %v, got %v", tt.session, tt.sentinel, err)
			}
		})
	}
}
