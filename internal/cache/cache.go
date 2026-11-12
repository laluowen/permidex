// Package cache stores provider catalogues as JSON files under the user's cache directory.
package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ErrMissing means no cache file exists yet.
var ErrMissing = errors.New("no cache")

// Path returns the cache file at parts under $XDG_CACHE_HOME/permidex, or ~/.cache/permidex.
func Path(parts ...string) (string, error) {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("home directory: %w", err)
		}
		dir = filepath.Join(home, ".cache")
	}
	return filepath.Join(append([]string{dir, "permidex"}, parts...)...), nil
}

// WriteJSON replaces path atomically with v encoded as indented JSON.
func WriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode cache: %w", err)
	}
	return WriteFile(path, append(data, '\n'))
}

// WriteFile replaces path atomically: it writes a temp file in the same directory, syncs it, then renames it.
func WriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".cache-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write cache: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync cache: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close cache: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replace cache: %w", err)
	}
	return nil
}

// ReadJSON decodes the cache at path into v. It returns ErrMissing if the file does not exist.
func ReadJSON(path string, v any) error {
	data, err := os.ReadFile(path) //nolint:gosec // path comes from Path or a caller-chosen test path
	if errors.Is(err, fs.ErrNotExist) {
		return ErrMissing
	}
	if err != nil {
		return fmt.Errorf("read cache: %w", err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("decode %s: %w (refresh the cache)", path, err)
	}
	return nil
}
