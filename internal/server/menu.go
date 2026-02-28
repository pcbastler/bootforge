package server

import (
	"fmt"

	"bootforge/internal/domain"
)

// MenuResolver resolves client menu entry references to actual MenuEntry definitions.
type MenuResolver struct {
	menus map[string]*domain.MenuEntry // name → MenuEntry
}

// NewMenuResolver creates a resolver from a list of menu entries.
func NewMenuResolver(menus []*domain.MenuEntry) *MenuResolver {
	m := make(map[string]*domain.MenuEntry, len(menus))
	for _, entry := range menus {
		m[entry.Name] = entry
	}
	return &MenuResolver{menus: m}
}

// Resolve turns a client's menu entry names into actual MenuEntry objects.
func (r *MenuResolver) Resolve(entries []string) ([]*domain.MenuEntry, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("no menu entries to resolve")
	}

	result := make([]*domain.MenuEntry, 0, len(entries))
	for _, name := range entries {
		entry, ok := r.menus[name]
		if !ok {
			return nil, fmt.Errorf("menu entry %q not found", name)
		}
		result = append(result, entry)
	}
	return result, nil
}

// FindByName returns a single menu entry by name.
func (r *MenuResolver) FindByName(name string) (*domain.MenuEntry, error) {
	entry, ok := r.menus[name]
	if !ok {
		return nil, fmt.Errorf("menu entry %q not found", name)
	}
	return entry, nil
}

// ListAll returns all menu entries.
func (r *MenuResolver) ListAll() []*domain.MenuEntry {
	result := make([]*domain.MenuEntry, 0, len(r.menus))
	for _, entry := range r.menus {
		result = append(result, entry)
	}
	return result
}

// Reload replaces the menu entries.
func (r *MenuResolver) Reload(menus []*domain.MenuEntry) {
	r.menus = make(map[string]*domain.MenuEntry, len(menus))
	for _, entry := range menus {
		r.menus[entry.Name] = entry
	}
}
