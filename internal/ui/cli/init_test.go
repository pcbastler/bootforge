package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitFailsOnExistingConfig(t *testing.T) {
	dir := t.TempDir()

	// Create an existing bootforge.toml.
	os.WriteFile(filepath.Join(dir, "bootforge.toml"), []byte("[server]"), 0644)

	oldCfgDir := cfgDir
	cfgDir = dir
	defer func() { cfgDir = oldCfgDir }()

	err := runInit(nil, nil)
	if err == nil {
		t.Error("runInit() should fail when bootforge.toml already exists")
	}
}

func TestInitAllowsNonEmptyDirWithoutConfig(t *testing.T) {
	dir := t.TempDir()

	// Create some other file — should not block init.
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello"), 0644)

	oldCfgDir := cfgDir
	cfgDir = dir
	defer func() { cfgDir = oldCfgDir }()

	// runInit will call wizard.Run which needs a terminal,
	// so we only test that it doesn't fail on the directory check.
	// The wizard itself will fail because there's no terminal,
	// but we verify the error is NOT about existing config.
	err := runInit(nil, nil)
	if err != nil && err.Error() == "configuration already exists in "+dir+" (use 'bootforge edit' to modify)" {
		t.Error("should not complain about existing config when only other files exist")
	}
}
