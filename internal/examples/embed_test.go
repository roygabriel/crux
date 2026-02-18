package examples_test

import (
	"io/fs"
	"testing"

	"github.com/roygabriel/crux/internal/examples"
)

func TestHTTPAPIFS_NotEmpty(t *testing.T) {
	fsys, err := examples.HTTPAPIFS()
	if err != nil {
		t.Fatalf("HTTPAPIFS: %v", err)
	}

	count := 0
	err = fs.WalkDir(fsys, ".", func(_ string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir: %v", err)
	}
	if count < 5 {
		t.Errorf("expected at least 5 files, got %d", count)
	}
}

func TestHTTPAPIFS_ContainsConfig(t *testing.T) {
	fsys, err := examples.HTTPAPIFS()
	if err != nil {
		t.Fatalf("HTTPAPIFS: %v", err)
	}

	data, err := fs.ReadFile(fsys, "config.yaml")
	if err != nil {
		t.Fatalf("ReadFile config.yaml: %v", err)
	}
	if len(data) == 0 {
		t.Error("config.yaml is empty")
	}
}

func TestHTTPAPIFS_ContainsPhases(t *testing.T) {
	fsys, err := examples.HTTPAPIFS()
	if err != nil {
		t.Fatalf("HTTPAPIFS: %v", err)
	}

	data, err := fs.ReadFile(fsys, "docs/phases/PHASE1.md")
	if err != nil {
		t.Fatalf("ReadFile PHASE1.md: %v", err)
	}
	if len(data) == 0 {
		t.Error("PHASE1.md is empty")
	}
}

func TestHTTPAPIFS_ContainsREADME(t *testing.T) {
	fsys, err := examples.HTTPAPIFS()
	if err != nil {
		t.Fatalf("HTTPAPIFS: %v", err)
	}

	data, err := fs.ReadFile(fsys, "README.md")
	if err != nil {
		t.Fatalf("ReadFile README.md: %v", err)
	}
	if len(data) == 0 {
		t.Error("README.md is empty")
	}
}
