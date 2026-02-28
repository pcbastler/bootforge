package health

import (
	"os"
	"path/filepath"
	"testing"

	"bootforge/internal/domain"
)

func TestFileProbeAllPresent(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		filepath.Join(dir, "file1.txt"),
		filepath.Join(dir, "file2.txt"),
	}
	for _, f := range files {
		os.WriteFile(f, []byte("data"), 0644)
	}

	probe := NewFileProbe("test-files", files)
	result := probe.Check()

	if result.Status != domain.StatusOK {
		t.Errorf("status = %v, want OK", result.Status)
	}
	if result.Name != "test-files" {
		t.Errorf("name = %q, want %q", result.Name, "test-files")
	}
}

func TestFileProbeOneMissing(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "exists.txt")
	os.WriteFile(existing, []byte("data"), 0644)
	missing := filepath.Join(dir, "missing.txt")

	probe := NewFileProbe("test-files", []string{existing, missing})
	result := probe.Check()

	if result.Status != domain.StatusFail {
		t.Errorf("status = %v, want Fail", result.Status)
	}
}

func TestFileProbeAllMissing(t *testing.T) {
	probe := NewFileProbe("test-files", []string{"/nonexistent/a", "/nonexistent/b"})
	result := probe.Check()

	if result.Status != domain.StatusFail {
		t.Errorf("status = %v, want Fail", result.Status)
	}
}

func TestFileProbeEmptyList(t *testing.T) {
	probe := NewFileProbe("test-files", nil)
	result := probe.Check()

	if result.Status != domain.StatusOK {
		t.Errorf("status = %v, want OK for empty file list", result.Status)
	}
}
