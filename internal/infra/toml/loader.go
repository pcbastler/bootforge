package toml

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"bootforge/internal/domain"
)

// LoadDir reads all .toml files from the given directory and merges them
// into a single FullConfig. It performs cross-file validation including
// duplicate MAC/menu name detection and reference integrity.
func LoadDir(dir string) (*domain.FullConfig, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading config directory %s: %w", dir, err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".toml") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no .toml files found in %s", dir)
	}

	// Sort for deterministic processing order.
	sort.Strings(files)

	var results []*fileResult
	for _, f := range files {
		r, err := parseFile(f)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}

	return merge(results)
}

// merge combines multiple file results into a single FullConfig.
// It detects conflicts (duplicate [server], duplicate MACs, duplicate menu names)
// and validates cross-references (client entries referencing existing menus).
func merge(results []*fileResult) (*domain.FullConfig, error) {
	cfg := &domain.FullConfig{}

	var serverSource string

	// Track seen MACs and menu names for duplicate detection.
	seenMACs := make(map[string]string)  // MAC string → source file
	seenMenus := make(map[string]string) // menu name → source file
	wildcardCount := 0
	var wildcardSource string

	for _, r := range results {
		// Merge server config (only one allowed).
		if r.server != nil {
			if serverSource != "" {
				return nil, fmt.Errorf("[server] defined in both %s and %s", serverSource, r.filename)
			}
			cfg.Server = *r.server
			serverSource = r.filename
		}

		// Merge service configs — last one wins.
		if r.dhcpProxy != nil {
			cfg.DHCPProxy = *r.dhcpProxy
		}
		if r.tftp != nil {
			cfg.TFTP = *r.tftp
		}
		if r.http != nil {
			cfg.HTTP = *r.http
		}
		if r.health != nil {
			cfg.Health = *r.health
		}
		if r.bootloader != nil {
			cfg.Bootloader = *r.bootloader
		}

		// Merge menus with duplicate detection.
		for _, m := range r.menus {
			if existing, ok := seenMenus[m.Name]; ok {
				return nil, fmt.Errorf("duplicate menu name %q: defined in %s and %s", m.Name, existing, r.filename)
			}
			seenMenus[m.Name] = r.filename
			cfg.Menus = append(cfg.Menus, m)
		}

		// Merge clients with duplicate MAC detection.
		for _, c := range r.clients {
			macKey := c.MAC.String()
			if c.IsWildcard() {
				wildcardCount++
				if wildcardCount > 1 {
					return nil, fmt.Errorf("multiple wildcard clients (mac=\"*\"): defined in %s and %s", wildcardSource, r.filename)
				}
				wildcardSource = r.filename
			} else {
				if existing, ok := seenMACs[macKey]; ok {
					return nil, fmt.Errorf("duplicate client MAC %s: defined in %s and %s", macKey, existing, r.filename)
				}
				seenMACs[macKey] = r.filename
			}
			cfg.Clients = append(cfg.Clients, c)
		}
	}

	// Require server config.
	if serverSource == "" {
		return nil, fmt.Errorf("[server] section not found in any config file")
	}

	// Cross-validate: client menu entries must reference existing menus.
	for _, c := range cfg.Clients {
		for _, entryName := range c.Menu.Entries {
			if _, ok := seenMenus[entryName]; !ok {
				suggestion := findSimilar(entryName, seenMenus)
				msg := fmt.Sprintf("client %s (%s) in %s: menu entry %q not found",
					c.MAC, c.Name, c.SourceFile, entryName)
				if suggestion != "" {
					msg += fmt.Sprintf(" (did you mean %q?)", suggestion)
				}
				return nil, fmt.Errorf("%s", msg)
			}
		}
	}

	return cfg, nil
}

// findSimilar returns the most similar key in the map, or "" if none is close enough.
func findSimilar(target string, available map[string]string) string {
	target = strings.ToLower(target)
	var best string
	bestScore := 0

	for name := range available {
		lower := strings.ToLower(name)
		score := 0

		// Check prefix match.
		for i := 0; i < len(target) && i < len(lower); i++ {
			if target[i] == lower[i] {
				score++
			} else {
				break
			}
		}

		// Check if it's a substring.
		if strings.Contains(lower, target) || strings.Contains(target, lower) {
			score += 3
		}

		if score > bestScore && score >= 3 {
			bestScore = score
			best = name
		}
	}

	return best
}
