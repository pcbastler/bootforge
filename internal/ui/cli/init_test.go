package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitCreatesConfig(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "bootforge-test")

	// Set the flag and run.
	initDir = target
	err := runInit(nil, nil)
	if err != nil {
		t.Fatalf("runInit() error = %v", err)
	}

	// Check config file exists.
	cfgPath := filepath.Join(target, "bootforge.toml")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Error("bootforge.toml should be created")
	}

	// Check directories exist.
	dirs := []string{
		filepath.Join(target, "data", "bootloader"),
		filepath.Join(target, "data", "tools"),
		filepath.Join(target, "data", "installers"),
	}
	for _, d := range dirs {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			t.Errorf("directory %s should be created", d)
		}
	}

	// Check config file has content.
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 100 {
		t.Errorf("config file too short: %d bytes", len(data))
	}
}

func TestInitFailsOnExistingDir(t *testing.T) {
	dir := t.TempDir()

	// Create a file in the directory to make it non-empty.
	os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("hello"), 0644)

	initDir = dir
	err := runInit(nil, nil)
	if err == nil {
		t.Error("runInit() should fail on non-empty directory")
	}
}
