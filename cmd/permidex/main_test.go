package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// run executes the command tree with args and returns its output.
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestRootCacheDirPrintsBaseDirectory(t *testing.T) {
	base := t.TempDir()
	t.Setenv("PERMIDEX_CACHE_DIR", base)
	t.Setenv("XDG_CACHE_HOME", "")
	out, err := run(t, "-c")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != base {
		t.Errorf("permidex -c = %q, want %q", out, base)
	}
}

func TestProviderCacheDirPrintsProviderDirectory(t *testing.T) {
	base := t.TempDir()
	t.Setenv("PERMIDEX_CACHE_DIR", base)
	for _, p := range []string{"gcp", "aws"} {
		out, err := run(t, p, "-c")
		if err != nil {
			t.Fatal(err)
		}
		if want := filepath.Join(base, p); strings.TrimSpace(out) != want {
			t.Errorf("permidex %s -c = %q, want %q", p, out, want)
		}
	}
}

func TestInvalidPermidexCacheDirIsAnErrorEverywhere(t *testing.T) {
	t.Setenv("PERMIDEX_CACHE_DIR", "relative/cache")
	if _, err := run(t, "-c"); err == nil || !strings.Contains(err.Error(), "PERMIDEX_CACHE_DIR") {
		t.Errorf("permidex -c: err = %v, want a PERMIDEX_CACHE_DIR error", err)
	}
	if _, err := run(t, "gcp", "-c"); err == nil || !strings.Contains(err.Error(), "PERMIDEX_CACHE_DIR") {
		t.Errorf("permidex gcp -c: err = %v, want a PERMIDEX_CACHE_DIR error", err)
	}
}

func TestUnknownProviderIsAnError(t *testing.T) {
	if _, err := run(t, "azure"); err == nil {
		t.Error("an unknown provider should be an error")
	}
}

func TestProviderHelpDescribesFlags(t *testing.T) {
	out, err := run(t, "gcp", "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--refresh", "--cache-dir", "refresh the cache from GCP", "Browse GCP predefined IAM roles"} {
		if !strings.Contains(out, want) {
			t.Errorf("gcp --help missing %q:\n%s", want, out)
		}
	}
}

func TestRootHelpListsProviders(t *testing.T) {
	out, err := run(t, "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"gcp", "aws", "--cache-dir"} {
		if !strings.Contains(out, want) {
			t.Errorf("root --help missing %q:\n%s", want, out)
		}
	}
}
