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

	"github.com/laluowen/permidex/internal/iam"
)

// ErrMissing means no cache file exists yet.
var ErrMissing = errors.New("no cache")

// Snapshot is the cache file content.
type Snapshot struct {
	Version   int        `json:"version"`
	FetchedAt time.Time  `json:"fetchedAt"`
	Roles     []iam.Role `json:"roles"`
}

// Version is the cache file format. Bump it, and update schema/roles.schema.json, on any change to the file.
const Version = 1

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
	s.Version = Version
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
	if s.Version != Version {
		return Snapshot{}, fmt.Errorf("cache %s is format version %d, want %d (run `permidex refresh`)", path, s.Version, Version)
	}
	return s, nil
}
