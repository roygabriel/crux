package plugin_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/roygabriel/crux/internal/plugin"
	"github.com/roygabriel/crux/pkg/types"
)

// stubPlugin implements AgentPlugin with no-op methods for testing.
type stubPlugin struct {
	name string
}

func (s *stubPlugin) Name() string { return s.name }

func (s *stubPlugin) LaunchCmd(_ plugin.AgentConfig) (string, []string, error) {
	return "", nil, nil
}

func (s *stubPlugin) DetectReady(_ string) bool          { return false }
func (s *stubPlugin) DetectBusy(_ string) bool           { return false }
func (s *stubPlugin) DetectError(_ string) (string, bool) { return "", false }

func (s *stubPlugin) DetectRateLimit(_ string) (time.Duration, bool) {
	return 0, false
}

func (s *stubPlugin) DetectPrompt(_ string) (plugin.PromptResponse, bool) {
	return plugin.PromptResponse{}, false
}

func (s *stubPlugin) FormatMessage(_ types.Message) string { return "" }

func (s *stubPlugin) ParseOutput(_ string) (plugin.AgentOutput, error) {
	return plugin.AgentOutput{}, nil
}

func (s *stubPlugin) Capabilities() []plugin.Capability { return nil }

func newStubFactory(name string) plugin.PluginFactory {
	return func() plugin.AgentPlugin {
		return &stubPlugin{name: name}
	}
}

func TestRegistryRegisterAndGet(t *testing.T) {
	t.Parallel()

	r := plugin.NewRegistry()
	err := r.Register("claude", newStubFactory("claude"))
	if err != nil {
		t.Fatalf("Register: unexpected error: %v", err)
	}

	p, err := r.Get("claude")
	if err != nil {
		t.Fatalf("Get: unexpected error: %v", err)
	}
	if p.Name() != "claude" {
		t.Errorf("Name() = %q, want %q", p.Name(), "claude")
	}
}

func TestRegistryGetUnknown(t *testing.T) {
	t.Parallel()

	r := plugin.NewRegistry()
	_, err := r.Get("nonexistent")
	if err == nil {
		t.Fatal("Get: expected error, got nil")
	}
	if !errors.Is(err, plugin.ErrPluginNotFound) {
		t.Errorf("Get error = %v, want wrapping %v", err, plugin.ErrPluginNotFound)
	}
}

func TestRegistryRegisterDuplicate(t *testing.T) {
	t.Parallel()

	r := plugin.NewRegistry()
	if err := r.Register("claude", newStubFactory("claude")); err != nil {
		t.Fatalf("first Register: unexpected error: %v", err)
	}

	err := r.Register("claude", newStubFactory("claude"))
	if err == nil {
		t.Fatal("second Register: expected error, got nil")
	}
	if !errors.Is(err, plugin.ErrPluginAlreadyRegistered) {
		t.Errorf("Register error = %v, want wrapping %v", err, plugin.ErrPluginAlreadyRegistered)
	}
}

func TestRegistryRegisterValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pName   string
		factory plugin.PluginFactory
	}{
		{"empty-name", "", newStubFactory("x")},
		{"nil-factory", "valid", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := plugin.NewRegistry()
			err := r.Register(tt.pName, tt.factory)
			if err == nil {
				t.Error("Register: expected error, got nil")
			}
		})
	}
}

func TestRegistryList(t *testing.T) {
	t.Parallel()

	r := plugin.NewRegistry()
	// Register out of order to verify sorting.
	for _, name := range []string{"codex", "claude", "gemini"} {
		if err := r.Register(name, newStubFactory(name)); err != nil {
			t.Fatalf("Register %q: %v", name, err)
		}
	}

	got := r.List()
	want := []string{"claude", "codex", "gemini"}
	if len(got) != len(want) {
		t.Fatalf("List() returned %d items, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("List()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRegistryListEmpty(t *testing.T) {
	t.Parallel()

	r := plugin.NewRegistry()
	got := r.List()
	if got == nil {
		t.Fatal("List() returned nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("List() returned %d items, want 0", len(got))
	}
}

func TestRegistryGetReturnsFreshInstance(t *testing.T) {
	t.Parallel()

	r := plugin.NewRegistry()
	if err := r.Register("claude", newStubFactory("claude")); err != nil {
		t.Fatalf("Register: %v", err)
	}

	p1, err := r.Get("claude")
	if err != nil {
		t.Fatalf("first Get: %v", err)
	}
	p2, err := r.Get("claude")
	if err != nil {
		t.Fatalf("second Get: %v", err)
	}

	if p1 == p2 {
		t.Error("Get returned same pointer twice, want distinct instances")
	}
}

func TestRegistryConcurrentAccess(t *testing.T) {
	t.Parallel()

	r := plugin.NewRegistry()
	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(n int) {
			defer wg.Done()

			name := "plugin-" + itoa(n)
			// Register may fail for duplicates when multiple goroutines
			// race on the same index — that's expected.
			_ = r.Register(name, newStubFactory(name))
			_, _ = r.Get(name)
			_ = r.List()
		}(i)
	}

	wg.Wait()

	// After all goroutines finish, verify the registry is consistent.
	names := r.List()
	if len(names) == 0 {
		t.Fatal("List() returned empty after concurrent registrations")
	}
	// Verify sorted order.
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			t.Errorf("List() not sorted: %q >= %q at index %d", names[i-1], names[i], i)
		}
	}
}

// itoa converts a non-negative int to its string representation without
// importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
