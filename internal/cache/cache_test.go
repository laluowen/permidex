package cache

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type doc struct {
	Name  string   `json:"name"`
	Items []string `json:"items"`
}

func TestWriteReadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "doc.json")
	want := doc{Name: "x", Items: []string{"a", "b"}}
	if err := WriteJSON(path, want); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	var got doc
	if err := ReadJSON(path, &got); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReadJSON = %+v, want %+v", got, want)
	}
}

func TestWriteReplacesAndLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.json")
	if err := WriteJSON(path, doc{Name: "old"}); err != nil {
		t.Fatal(err)
	}
	if err := WriteJSON(path, doc{Name: "new"}); err != nil {
		t.Fatal(err)
	}
	var got doc
	if err := ReadJSON(path, &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "new" {
		t.Errorf("after replace: %+v", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("dir has %d entries, want only doc.json", len(entries))
	}
}

func TestReadMissing(t *testing.T) {
	var got doc
	err := ReadJSON(filepath.Join(t.TempDir(), "doc.json"), &got)
	if !errors.Is(err, ErrMissing) {
		t.Fatalf("err = %v, want ErrMissing", err)
	}
}

func TestReadCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	var got doc
	err := ReadJSON(path, &got)
	if err == nil || errors.Is(err, ErrMissing) {
		t.Fatalf("err = %v, want decode error", err)
	}
}

func TestPathUsesXDGCacheHome(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/xdg")
	got, err := Path("gcp", "roles.json")
	if err != nil {
		t.Fatal(err)
	}
	if want := "/xdg/permidex/gcp/roles.json"; got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}
