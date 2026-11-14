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

// Path returns the cache file at parts, in the directory Dir returns.
func Path(parts ...string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{dir}, parts...)...), nil
}

// Dir is the permidex cache directory: $PERMIDEX_CACHE_DIR if set, else $XDG_CACHE_HOME/permidex if
// XDG_CACHE_HOME is an absolute path, else ~/.cache/permidex. PERMIDEX_CACHE_DIR is ours, so a relative
// value is an error. XDG_CACHE_HOME is the XDG spec's variable, so a relative value is ignored, as the spec
// says. An empty variable counts as unset.
func Dir() (string, error) {
	if d := os.Getenv("PERMIDEX_CACHE_DIR"); d != "" {
		if !filepath.IsAbs(d) {
			return "", fmt.Errorf("PERMIDEX_CACHE_DIR must be an absolute path, got %q", d)
		}
		return d, nil
	}
	if d := os.Getenv("XDG_CACHE_HOME"); d != "" && filepath.IsAbs(d) {
		return filepath.Join(d, "permidex"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home directory: %w", err)
	}
	return filepath.Join(home, ".cache", "permidex"), nil
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
