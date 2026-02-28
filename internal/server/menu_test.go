package server

import (
	"testing"

	"bootforge/internal/domain"
)

func testMenuEntries() []*domain.MenuEntry {
	return []*domain.MenuEntry{
		{Name: "rescue", Label: "Rescue System", Type: domain.MenuLive,
			Boot: domain.BootParams{Kernel: "vmlinuz"}},
		{Name: "ubuntu-install", Label: "Ubuntu 24.04", Type: domain.MenuInstall,
			Boot: domain.BootParams{Kernel: "vmlinuz", Initrd: "initrd"}},
		{Name: "local-disk", Label: "Boot from local disk", Type: domain.MenuExit},
	}
}

func TestMenuResolverResolveValid(t *testing.T) {
	r := NewMenuResolver(testMenuEntries())

	entries, err := r.Resolve([]string{"rescue", "ubuntu-install"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("Resolve() count = %d, want 2", len(entries))
	}
	if entries[0].Name != "rescue" {
		t.Errorf("entries[0].Name = %q, want %q", entries[0].Name, "rescue")
	}
	if entries[1].Name != "ubuntu-install" {
		t.Errorf("entries[1].Name = %q, want %q", entries[1].Name, "ubuntu-install")
	}
}

func TestMenuResolverResolveMissing(t *testing.T) {
	r := NewMenuResolver(testMenuEntries())

	_, err := r.Resolve([]string{"rescue", "nonexistent"})
	if err == nil {
		t.Error("Resolve() should fail with missing entry")
	}
}

func TestMenuResolverResolveEmpty(t *testing.T) {
	r := NewMenuResolver(testMenuEntries())

	_, err := r.Resolve([]string{})
	if err == nil {
		t.Error("Resolve() should fail with empty entries")
	}
}

func TestMenuResolverFindByName(t *testing.T) {
	r := NewMenuResolver(testMenuEntries())

	entry, err := r.FindByName("rescue")
	if err != nil {
		t.Fatalf("FindByName() error = %v", err)
	}
	if entry.Name != "rescue" {
		t.Errorf("FindByName() = %q, want %q", entry.Name, "rescue")
	}
}

func TestMenuResolverFindByNameMissing(t *testing.T) {
	r := NewMenuResolver(testMenuEntries())

	_, err := r.FindByName("nonexistent")
	if err == nil {
		t.Error("FindByName() should fail with missing entry")
	}
}

func TestMenuResolverListAll(t *testing.T) {
	r := NewMenuResolver(testMenuEntries())

	all := r.ListAll()
	if len(all) != 3 {
		t.Errorf("ListAll() count = %d, want 3", len(all))
	}
}

func TestMenuResolverReload(t *testing.T) {
	r := NewMenuResolver(testMenuEntries())

	newMenus := []*domain.MenuEntry{
		{Name: "new-entry", Label: "New", Type: domain.MenuLive,
			Boot: domain.BootParams{Kernel: "vmlinuz"}},
	}
	r.Reload(newMenus)

	_, err := r.FindByName("rescue")
	if err == nil {
		t.Error("FindByName(rescue) should fail after reload with different menus")
	}

	entry, err := r.FindByName("new-entry")
	if err != nil {
		t.Fatalf("FindByName(new-entry) error = %v", err)
	}
	if entry.Label != "New" {
		t.Errorf("FindByName(new-entry).Label = %q, want %q", entry.Label, "New")
	}
}
