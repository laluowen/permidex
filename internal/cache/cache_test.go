package cache

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/laluowen/permidex/internal/iam"
)

func TestWriteReadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "roles.json")
	want := Snapshot{
		FetchedAt: time.Date(2026, 11, 11, 9, 30, 0, 0, time.UTC),
		Roles: []iam.Role{{
			Name:                "roles/compute.viewer",
			Title:               "Compute Viewer",
			Stage:               "GA",
			IncludedPermissions: []string{"compute.instances.get"},
		}},
	}
	if err := Write(path, want); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !got.FetchedAt.Equal(want.FetchedAt) || !reflect.DeepEqual(got.Roles, want.Roles) {
		t.Errorf("Read = %+v, want %+v", got, want)
	}
}

func TestWriteReplacesAndLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "roles.json")
	if err := Write(path, Snapshot{Roles: []iam.Role{{Name: "roles/old"}}}); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, Snapshot{Roles: []iam.Role{{Name: "roles/new"}}}); err != nil {
		t.Fatal(err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Roles) != 1 || got.Roles[0].Name != "roles/new" {
		t.Errorf("after replace: %+v", got.Roles)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("dir has %d entries, want only roles.json", len(entries))
	}
}

func TestReadMissing(t *testing.T) {
	_, err := Read(filepath.Join(t.TempDir(), "roles.json"))
	if !errors.Is(err, ErrMissing) {
		t.Fatalf("err = %v, want ErrMissing", err)
	}
}

func TestReadCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "roles.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Read(path)
	if err == nil || errors.Is(err, ErrMissing) {
		t.Fatalf("err = %v, want decode error", err)
	}
}

func TestPathUsesXDGCacheHome(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/xdg")
	got, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if want := "/xdg/permidex/roles.json"; got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}
