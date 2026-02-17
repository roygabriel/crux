package security

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestSecretsManager_Load(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.env")
	os.WriteFile(path, []byte("KEY1=value1\nKEY2=value2\nKEY3=value3\n"), 0o644)

	mgr := NewSecretsManager(path, nil)
	if err := mgr.Load(); err != nil {
		t.Fatal(err)
	}

	if got := mgr.Get("KEY1"); got != "value1" {
		t.Errorf("KEY1 = %q, want %q", got, "value1")
	}
	if got := mgr.Get("KEY2"); got != "value2" {
		t.Errorf("KEY2 = %q, want %q", got, "value2")
	}
	if got := mgr.Get("KEY3"); got != "value3" {
		t.Errorf("KEY3 = %q, want %q", got, "value3")
	}
}

func TestSecretsManager_LoadComments(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.env")
	os.WriteFile(path, []byte("# This is a comment\nKEY=value\n# Another comment\n\n"), 0o644)

	mgr := NewSecretsManager(path, nil)
	if err := mgr.Load(); err != nil {
		t.Fatal(err)
	}

	names := mgr.Names()
	if len(names) != 1 {
		t.Errorf("expected 1 key, got %d: %v", len(names), names)
	}
	if names[0] != "KEY" {
		t.Errorf("key = %q, want %q", names[0], "KEY")
	}
}

func TestSecretsManager_LoadMissingFile(t *testing.T) {
	t.Parallel()
	mgr := NewSecretsManager("/nonexistent/path/secrets.env", nil)
	if err := mgr.Load(); err != nil {
		t.Errorf("expected nil for missing file, got %v", err)
	}
}

func TestSecretsManager_LoadValueWithEquals(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.env")
	os.WriteFile(path, []byte("KEY=value=with=equals\n"), 0o644)

	mgr := NewSecretsManager(path, nil)
	if err := mgr.Load(); err != nil {
		t.Fatal(err)
	}

	if got := mgr.Get("KEY"); got != "value=with=equals" {
		t.Errorf("KEY = %q, want %q", got, "value=with=equals")
	}
}

func TestSecretsManager_Redact(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.env")
	os.WriteFile(path, []byte("DB_PASS=hunter2\nAPI_KEY=sk-12345678\n"), 0o644)

	mgr := NewSecretsManager(path, nil)
	if err := mgr.Load(); err != nil {
		t.Fatal(err)
	}

	text := "connecting with password hunter2 and key sk-12345678"
	redacted := mgr.Redact(text)

	if got, want := redacted, "connecting with password [REDACTED] and key [REDACTED]"; got != want {
		t.Errorf("redacted = %q, want %q", got, want)
	}
}

func TestSecretsManager_RedactLongestFirst(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.env")
	// SHORT is a substring of LONG's value.
	os.WriteFile(path, []byte("SHORT=abc\nLONG=abcdef\n"), 0o644)

	mgr := NewSecretsManager(path, nil)
	if err := mgr.Load(); err != nil {
		t.Fatal(err)
	}

	text := "value is abcdef"
	redacted := mgr.Redact(text)

	// "abcdef" should be replaced as one unit, not partially.
	if got, want := redacted, "value is [REDACTED]"; got != want {
		t.Errorf("redacted = %q, want %q", got, want)
	}
}

func TestSecretsManager_RedactNoSecrets(t *testing.T) {
	t.Parallel()
	mgr := NewSecretsManager("/nonexistent", nil)
	_ = mgr.Load()

	text := "no secrets here"
	if got := mgr.Redact(text); got != text {
		t.Errorf("expected unchanged text, got %q", got)
	}
}

func TestSecretsManager_Get(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.env")
	os.WriteFile(path, []byte("MY_SECRET=supersecret\n"), 0o644)

	mgr := NewSecretsManager(path, nil)
	_ = mgr.Load()

	if got := mgr.Get("MY_SECRET"); got != "supersecret" {
		t.Errorf("got %q, want %q", got, "supersecret")
	}
}

func TestSecretsManager_GetMissing(t *testing.T) {
	t.Parallel()
	mgr := NewSecretsManager("/nonexistent", nil)
	_ = mgr.Load()

	if got := mgr.Get("MISSING"); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestSecretsManager_Names(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.env")
	os.WriteFile(path, []byte("BETA=b\nALPHA=a\nGAMMA=g\n"), 0o644)

	mgr := NewSecretsManager(path, nil)
	_ = mgr.Load()

	names := mgr.Names()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	expected := []string{"ALPHA", "BETA", "GAMMA"}
	for i, want := range expected {
		if names[i] != want {
			t.Errorf("names[%d] = %q, want %q", i, names[i], want)
		}
	}
}

func TestSecretsManager_ConcurrentRedact(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.env")
	os.WriteFile(path, []byte("SECRET=mysecretvalue\n"), 0o644)

	mgr := NewSecretsManager(path, nil)
	_ = mgr.Load()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = mgr.Redact("text with mysecretvalue embedded")
			}
		}()
	}
	wg.Wait()
}
