package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanner_AWSKey(t *testing.T) {
	t.Parallel()
	s := NewSecretsScanner(nil)
	findings := s.ScanContent("aws_key = AKIAIOSFODNN7EXAMPLE")
	if len(findings) == 0 {
		t.Fatal("expected AWS key finding")
	}
	if findings[0].Pattern != "AWS Access Key" {
		t.Errorf("pattern = %q, want %q", findings[0].Pattern, "AWS Access Key")
	}
}

func TestScanner_OpenAIKey(t *testing.T) {
	t.Parallel()
	s := NewSecretsScanner(nil)
	findings := s.ScanContent("key = sk-abc123def456ghi789jkl012mno345pqr678")
	if len(findings) == 0 {
		t.Fatal("expected OpenAI key finding")
	}
	found := false
	for _, f := range findings {
		if f.Pattern == "OpenAI API Key" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected OpenAI API Key pattern in findings")
	}
}

func TestScanner_GitHubToken(t *testing.T) {
	t.Parallel()
	s := NewSecretsScanner(nil)
	findings := s.ScanContent("token = ghp_abcdefghijklmnopqrstuvwxyz0123456789")
	if len(findings) == 0 {
		t.Fatal("expected GitHub PAT finding")
	}
	found := false
	for _, f := range findings {
		if f.Pattern == "GitHub PAT" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected GitHub PAT pattern in findings")
	}
}

func TestScanner_GenericSecret(t *testing.T) {
	t.Parallel()
	s := NewSecretsScanner(nil)
	findings := s.ScanContent(`password = "mysecretpassword123"`)
	if len(findings) == 0 {
		t.Fatal("expected generic secret finding")
	}
	found := false
	for _, f := range findings {
		if f.Pattern == "Generic Secret" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Generic Secret pattern in findings")
	}
}

func TestScanner_PEMKey(t *testing.T) {
	t.Parallel()
	s := NewSecretsScanner(nil)
	findings := s.ScanContent("-----BEGIN RSA PRIVATE KEY-----")
	if len(findings) == 0 {
		t.Fatal("expected PEM finding")
	}
	if findings[0].Pattern != "PEM Private Key" {
		t.Errorf("pattern = %q, want %q", findings[0].Pattern, "PEM Private Key")
	}
}

func TestScanner_NoSecrets(t *testing.T) {
	t.Parallel()
	s := NewSecretsScanner(nil)
	findings := s.ScanContent("func main() {\n\tfmt.Println(\"hello\")\n}")
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestScanner_MatchTruncated(t *testing.T) {
	t.Parallel()
	s := NewSecretsScanner(nil)
	// A very long AWS key-style match.
	findings := s.ScanContent("AKIAIOSFODNN7EXAMPLE")
	if len(findings) == 0 {
		t.Fatal("expected finding")
	}
	if !strings.HasSuffix(findings[0].Match, "...") {
		t.Errorf("expected truncated match, got %q", findings[0].Match)
	}
	if len(findings[0].Match) > 19 { // 16 + "..."
		t.Errorf("match too long: %d chars", len(findings[0].Match))
	}
}

func TestScanner_LineNumber(t *testing.T) {
	t.Parallel()
	s := NewSecretsScanner(nil)
	content := "line one\nline two\npassword = \"secretvalue12345\"\nline four"
	findings := s.ScanContent(content)
	if len(findings) == 0 {
		t.Fatal("expected finding")
	}
	found := false
	for _, f := range findings {
		if f.Pattern == "Generic Secret" && f.Line == 3 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Generic Secret on line 3, findings: %+v", findings)
	}
}

func TestScanner_ScanFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.env")
	os.WriteFile(path, []byte("API_KEY=sk-verylongopenaikey1234567890abc"), 0o644)

	s := NewSecretsScanner(nil)
	findings, err := s.ScanFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) == 0 {
		t.Fatal("expected findings")
	}
	if findings[0].File != path {
		t.Errorf("file = %q, want %q", findings[0].File, path)
	}
}

func TestScanner_ScanDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	os.MkdirAll(filepath.Join(dir, "src"), 0o755)
	os.MkdirAll(filepath.Join(dir, "vendor"), 0o755)

	os.WriteFile(filepath.Join(dir, "src", "config.go"), []byte("password = \"longpassword1234\""), 0o644)
	os.WriteFile(filepath.Join(dir, "src", "clean.go"), []byte("package main"), 0o644)
	os.WriteFile(filepath.Join(dir, "vendor", "secrets.go"), []byte("password = \"vendorsecret1234\""), 0o644)

	s := NewSecretsScanner(nil)
	findings, err := s.ScanDirectory(dir, []string{"vendor"})
	if err != nil {
		t.Fatal(err)
	}

	// Should find the secret in src/ but not in vendor/ (excluded).
	for _, f := range findings {
		if strings.Contains(f.File, "vendor") {
			t.Error("should not scan excluded vendor directory")
		}
	}
	if len(findings) == 0 {
		t.Error("expected at least one finding from src/")
	}
}

func TestScanner_AddPattern(t *testing.T) {
	t.Parallel()
	s := NewSecretsScanner(nil)
	if err := s.AddPattern("Custom", `CUSTOM_[0-9]{10}`); err != nil {
		t.Fatal(err)
	}

	findings := s.ScanContent("token=CUSTOM_1234567890")
	found := false
	for _, f := range findings {
		if f.Pattern == "Custom" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Custom pattern finding")
	}
}

func TestScanner_AddPatternBadRegex(t *testing.T) {
	t.Parallel()
	s := NewSecretsScanner(nil)
	err := s.AddPattern("Bad", `[invalid`)
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}
