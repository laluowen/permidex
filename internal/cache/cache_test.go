package cache

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestPathDefaultsToDotCache(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PERMIDEX_CACHE_DIR", "")
	t.Setenv("XDG_CACHE_HOME", "")
	got, err := Path("gcp", "roles.json")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".cache", "permidex", "gcp", "roles.json"); got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}

func TestPathHonoursXDGCacheHome(t *testing.T) {
	t.Setenv("PERMIDEX_CACHE_DIR", "")
	xdg := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", xdg)
	got, err := Path("gcp", "roles.json")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(xdg, "permidex", "gcp", "roles.json"); got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}

func TestRelativeXDGCacheHomeIsIgnored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PERMIDEX_CACHE_DIR", "")
	t.Setenv("XDG_CACHE_HOME", "relative/cache")
	got, err := Path("gcp", "roles.json")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".cache", "permidex", "gcp", "roles.json"); got != want {
		t.Errorf("relative XDG_CACHE_HOME should be ignored: got %q, want %q", got, want)
	}
}

func TestPathHonoursPermidexCacheDir(t *testing.T) {
	t.Setenv("PERMIDEX_CACHE_DIR", "/custom/cache")
	got, err := Path("aws", "catalogue.json")
	if err != nil {
		t.Fatal(err)
	}
	if want := "/custom/cache/aws/catalogue.json"; got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}

func TestRelativePermidexCacheDirIsAnError(t *testing.T) {
	t.Setenv("PERMIDEX_CACHE_DIR", "rel/cache")
	t.Setenv("XDG_CACHE_HOME", "/absolute/xdg")
	if _, err := Path("gcp", "roles.json"); err == nil || !strings.Contains(err.Error(), "PERMIDEX_CACHE_DIR") {
		t.Errorf("err = %v, want a PERMIDEX_CACHE_DIR error even with an absolute XDG set", err)
	}
}
