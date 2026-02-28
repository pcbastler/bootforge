// Package toml scans the config directory and reads all .toml files.
// It detects content type by structure ([server], [[menu]], [[client]])
// and merges everything into the domain config types.
package toml
