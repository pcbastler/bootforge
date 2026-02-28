package cli

import (
	"strings"
	"testing"
)

func TestMenuList(t *testing.T) {
	cfgDir = setupTestConfig(t)

	out, err := captureOutput(t, func() error {
		return runMenuList(nil, nil)
	})
	if err != nil {
		t.Fatalf("runMenuList() error = %v", err)
	}

	if !strings.Contains(out, "NAME") {
		t.Error("output should contain header")
	}
	if !strings.Contains(out, "rescue") {
		t.Error("output should contain rescue menu")
	}
	if !strings.Contains(out, "local-disk") {
		t.Error("output should contain local-disk menu")
	}
}

func TestMenuShowKnown(t *testing.T) {
	cfgDir = setupTestConfig(t)

	out, err := captureOutput(t, func() error {
		return runMenuShow(nil, []string{"rescue"})
	})
	if err != nil {
		t.Fatalf("runMenuShow() error = %v", err)
	}

	if !strings.Contains(out, "Rescue System") {
		t.Error("output should contain menu label")
	}
	if !strings.Contains(out, "vmlinuz") {
		t.Error("output should contain kernel")
	}
}

func TestMenuShowUnknown(t *testing.T) {
	cfgDir = setupTestConfig(t)

	_, err := captureOutput(t, func() error {
		return runMenuShow(nil, []string{"nonexistent"})
	})
	if err == nil {
		t.Error("runMenuShow() should fail for unknown menu")
	}
}
