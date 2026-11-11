// Package cache stores the fetched roles as one JSON file.
package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/lalu-uk/permidex/internal/iam"
)

// ErrMissing means no cache file exists yet.
var ErrMissing = errors.New("no cache")

// Snapshot is the cache file content.
type Snapshot struct {
	FetchedAt time.Time  `json:"fetchedAt"`
	Roles     []iam.Role `json:"roles"`
}

// Path returns $XDG_CACHE_HOME/permidex/roles.json, or ~/.cache/permidex/roles.json.
func Path() (string, error) {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("home directory: %w", err)
		}
		dir = filepath.Join(home, ".cache")
	}
	return filepath.Join(dir, "permidex", "roles.json"), nil
}

// Write replaces path atomically: it writes a temp file in the same directory, syncs it, then renames it.
func Write(path string, s Snapshot) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".roles-*.json")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("encode cache: %w", err)
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

// Read loads the cache at path. It returns ErrMissing if the file does not exist.
func Read(path string) (Snapshot, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is the cache file from Path or a caller-chosen test path
	if errors.Is(err, fs.ErrNotExist) {
		return Snapshot{}, ErrMissing
	}
	if err != nil {
		return Snapshot{}, fmt.Errorf("read cache: %w", err)
	}
	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return Snapshot{}, fmt.Errorf("decode %s: %w (run `permidex refresh`)", path, err)
	}
	return s, nil
}
