package iam

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

const pageTwo = "page2"

func TestRolesPaginatesWithFullView(t *testing.T) {
	var tokens []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("view") != "FULL" {
			t.Errorf("view = %q, want FULL", q.Get("view"))
		}
		if q.Get("pageSize") != "1000" {
			t.Errorf("pageSize = %q, want 1000", q.Get("pageSize"))
		}
		tok := q.Get("pageToken")
		tokens = append(tokens, tok)
		switch tok {
		case "":
			writeJSON(t, w, listResponse{
				Roles:         []Role{{Name: "roles/a.one", IncludedPermissions: []string{"a.x.get"}}},
				NextPageToken: pageTwo,
			})
		case pageTwo:
			writeJSON(t, w, listResponse{
				Roles: []Role{{Name: "roles/b.two", IncludedPermissions: []string{"b.y.list"}}},
			})
		default:
			t.Errorf("unexpected pageToken %q", tok)
		}
	}))
	defer srv.Close()

	roles, err := Client{HTTP: srv.Client(), URL: srv.URL + "/v1/roles"}.Roles(t.Context())
	if err != nil {
		t.Fatalf("Roles: %v", err)
	}
	if got, want := names(roles), []string{"roles/a.one", "roles/b.two"}; !reflect.DeepEqual(got, want) {
		t.Errorf("roles = %v, want %v", got, want)
	}
	if want := []string{"", pageTwo}; !reflect.DeepEqual(tokens, want) {
		t.Errorf("page tokens requested = %q, want %q", tokens, want)
	}
}

func TestRolesFixture(t *testing.T) {
	fixture, err := os.ReadFile("../../testdata/roles.json")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	roles, err := Client{HTTP: srv.Client(), URL: srv.URL}.Roles(t.Context())
	if err != nil {
		t.Fatalf("Roles: %v", err)
	}
	if len(roles) != 4 {
		t.Fatalf("got %d roles, want 4", len(roles))
	}
	if n := len(roles[0].IncludedPermissions); n == 0 {
		t.Errorf("first role has no includedPermissions")
	}
}

func TestRolesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "PERMISSION_DENIED", http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := Client{HTTP: srv.Client(), URL: srv.URL}.Roles(t.Context())
	if err == nil || !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "PERMISSION_DENIED") {
		t.Fatalf("err = %v, want 403 with body", err)
	}
}

func TestRolesRepeatedTokenFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, listResponse{NextPageToken: "same"})
	}))
	defer srv.Close()

	_, err := Client{HTTP: srv.Client(), URL: srv.URL}.Roles(t.Context())
	if err == nil {
		t.Fatal("want error on repeated page token")
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Errorf("encode: %v", err)
	}
}

func names(roles []Role) []string {
	out := make([]string, len(roles))
	for i, r := range roles {
		out[i] = r.Name
	}
	return out
}
