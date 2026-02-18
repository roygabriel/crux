package prefs

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestStore_RoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := NewStore(dir, nil)

	want := PresetDefaults(PresetPragmatic)
	if err := store.Save(want); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("round-trip mismatch:\ngot:  %+v\nwant: %+v", got, want)
	}
}

func TestStore_RoundTripAllPresets(t *testing.T) {
	t.Parallel()

	presets := []struct {
		name   string
		preset PresetName
	}{
		{"strict", PresetStrict},
		{"pragmatic", PresetPragmatic},
		{"startup", PresetStartup},
	}

	for _, tt := range presets {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			store := NewStore(dir, nil)

			want := PresetDefaults(tt.preset)
			if err := store.Save(want); err != nil {
				t.Fatalf("Save() error: %v", err)
			}

			got, err := store.Load()
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}

			if !reflect.DeepEqual(got, want) {
				t.Errorf("round-trip mismatch for preset %q", tt.preset)
			}
		})
	}
}

func TestStore_LoadMissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := NewStore(dir, nil)

	_, err := store.Load()
	if err == nil {
		t.Fatal("Load() expected error for missing file, got nil")
	}
	if err.Error() != ErrFileNotFound.Error() && !contains(err.Error(), "preferences file not found") {
		t.Errorf("Load() error = %q, want ErrFileNotFound", err)
	}
}

func TestStore_LoadInvalidYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "preferences.yaml")
	if err := os.WriteFile(path, []byte(":\n  :\n    - [invalid{yaml"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	store := NewStore(dir, nil)
	_, err := store.Load()
	if err == nil {
		t.Fatal("Load() expected error for invalid YAML, got nil")
	}
	if !contains(err.Error(), "parsing preferences") {
		t.Errorf("Load() error = %q, want containing 'parsing preferences'", err)
	}
}

func TestStore_LoadWrongVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := NewStore(dir, nil)

	// Save valid preferences first.
	p := PresetDefaults(PresetPragmatic)
	if err := store.Save(p); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Overwrite with wrong version.
	path := filepath.Join(dir, "preferences.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}

	modified := []byte("version: \"99.0\"\n" + string(data[len("version: \"1.0\"\n"):]))
	// Simpler: just write a complete wrong-version file.
	wrong := PresetDefaults(PresetPragmatic)
	wrong.Version = "99.0"
	content := "version: \"99.0\"\npreset: pragmatic\ntesting:\n  style: test-after\n  coverage_target: 70\n  mock_approach: interfaces\n  table_driven: true\n"
	_ = modified
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err = store.Load()
	if err == nil {
		t.Fatal("Load() expected error for wrong version, got nil")
	}
	if !contains(err.Error(), "unsupported preferences version") {
		t.Errorf("Load() error = %q, want containing 'unsupported preferences version'", err)
	}
}

func TestStore_Exists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := NewStore(dir, nil)

	if store.Exists() {
		t.Error("Exists() = true before save, want false")
	}

	p := PresetDefaults(PresetPragmatic)
	if err := store.Save(p); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	if !store.Exists() {
		t.Error("Exists() = false after save, want true")
	}
}

func TestStore_SaveSetsVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := NewStore(dir, nil)

	p := PresetDefaults(PresetPragmatic)
	p.Version = "" // Clear version.
	if err := store.Save(p); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got.Version != CurrentVersion {
		t.Errorf("Version = %q, want %q", got.Version, CurrentVersion)
	}
}

func TestStore_SaveCreatesDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	nested := filepath.Join(dir, "deep", "nested", "crux")
	store := NewStore(nested, nil)

	p := PresetDefaults(PresetPragmatic)
	if err := store.Save(p); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	path := filepath.Join(nested, "preferences.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("preferences.yaml not created at %s: %v", path, err)
	}
}

func TestStore_NewStoreNilLogger(t *testing.T) {
	t.Parallel()

	// Should not panic.
	store := NewStore(t.TempDir(), nil)
	if store == nil {
		t.Fatal("NewStore() returned nil")
	}
	if store.logger == nil {
		t.Error("store.logger is nil, want slog.Default() fallback")
	}
}
