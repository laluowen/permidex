package gcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/laluowen/permidex/v2/internal/browse"
	"github.com/laluowen/permidex/v2/internal/cache"
	"github.com/laluowen/permidex/v2/internal/iam"
	"github.com/laluowen/permidex/v2/schema"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	roleAB     = "roles/a.b"
	roleViewer = "roles/compute.viewer"
	permGet    = "compute.instances.get"
)

func fixture(t *testing.T) *Source {
	t.Helper()
	data, err := os.ReadFile("../../testdata/roles.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Roles []iam.Role `json:"roles"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	return NewSource(Snapshot{Version: CacheVersion, FetchedAt: time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC), Roles: f.Roles})
}

func names(items []browse.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Name
	}
	return out
}

func TestSectionsInOrder(t *testing.T) {
	secs := fixture(t).Sections()
	keys := make([]string, 0, len(secs))
	for _, s := range secs {
		keys = append(keys, s.Key)
	}
	if want := []string{"roles", "permissions", "services"}; !slices.Equal(keys, want) {
		t.Errorf("section keys = %v, want %v", keys, want)
	}
}

func TestRootIsHomeWithEverything(t *testing.T) {
	root := fixture(t).Root()
	if !root.Home || len(root.Items) == 0 {
		t.Fatalf("root = home %v, %d items", root.Home, len(root.Items))
	}
	if root.Items[0].Kind != kindRole {
		t.Errorf("first root item kind = %q, want roles first", root.Items[0].Kind)
	}
}

func TestOpenRoleListsItsPermissions(t *testing.T) {
	v := fixture(t).Open(browse.Item{Kind: kindRole, Name: roleViewer})
	if want := []string{permGet, "compute.instances.list", "compute.projects.get"}; !slices.Equal(names(v.Items), want) {
		t.Errorf("role items = %v, want %v", names(v.Items), want)
	}
	if !strings.Contains(strings.Join(v.Info, "\n"), "3 permissions") {
		t.Errorf("role info = %v", v.Info)
	}
}

func TestOpenPermissionListsRolesThenService(t *testing.T) {
	v := fixture(t).Open(browse.Item{Kind: kindPermission, Name: permGet})
	want := []string{roleViewer, "compute"}
	if !slices.Equal(names(v.Items), want) {
		t.Errorf("permission items = %v, want %v", names(v.Items), want)
	}
	if v.Items[len(v.Items)-1].Kind != kindService {
		t.Errorf("service should come last, got kind %q", v.Items[len(v.Items)-1].Kind)
	}
}

func TestOpenServiceListsRolesThenPermissions(t *testing.T) {
	v := fixture(t).Open(browse.Item{Kind: kindService, Name: "compute"})
	if len(v.Items) == 0 || v.Items[0].Kind != kindRole {
		t.Fatalf("service items should start with roles: %v", v.Items)
	}
	last := v.Items[len(v.Items)-1]
	if last.Kind != kindPermission {
		t.Errorf("service items should end with permissions, got %v", last)
	}
}

func TestLabelAndTitle(t *testing.T) {
	s := fixture(t)
	role := browse.Item{Kind: kindRole, Name: roleViewer}
	if got := s.Label(role); got != "Compute Viewer [roles/compute.viewer]" {
		t.Errorf("role label = %q", got)
	}
	if got := s.Title(role); got != "Compute Viewer" {
		t.Errorf("role title = %q", got)
	}
	perm := browse.Item{Kind: kindPermission, Name: permGet}
	if s.Label(perm) != perm.Name || s.Title(perm) != "" {
		t.Errorf("permission label/title = %q/%q", s.Label(perm), s.Title(perm))
	}
}

func TestLoadMissingAndWrongVersion(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(filepath.Join(dir, "roles.json")); !errors.Is(err, cache.ErrMissing) {
		t.Errorf("missing: err = %v, want ErrMissing", err)
	}
	path := filepath.Join(dir, "roles.json")
	if err := cache.WriteJSON(path, Snapshot{Version: CacheVersion + 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "permidex gcp --refresh|-r") {
		t.Errorf("wrong version: err = %v, want a refresh hint", err)
	}
}

func TestLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "roles.json")
	want := Snapshot{
		Version:   CacheVersion,
		FetchedAt: time.Date(2026, 11, 11, 9, 30, 0, 0, time.UTC),
		Roles:     []iam.Role{{Name: roleAB, IncludedPermissions: []string{"a.b.c"}}},
	}
	if err := cache.WriteJSON(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !got.FetchedAt.Equal(want.FetchedAt) || !reflect.DeepEqual(got.Roles, want.Roles) {
		t.Errorf("Load = %+v, want %+v", got, want)
	}
}

func TestSchemaMatchesStructs(t *testing.T) {
	data, err := os.ReadFile("../../schema/gcp-roles.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]struct {
			Const *int `json:"const"`
		} `json:"properties"`
		Defs struct {
			Role struct {
				Properties map[string]json.RawMessage `json:"properties"`
			} `json:"role"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}

	full := Snapshot{
		Version:   CacheVersion,
		FetchedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Roles: []iam.Role{{
			Name: roleAB, Title: "T", Description: "D", Stage: "GA", IncludedPermissions: []string{"a.b.c"},
		}},
	}
	raw, err := json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatal(err)
	}
	var roles []map[string]json.RawMessage
	if err := json.Unmarshal(top["roles"], &roles); err != nil {
		t.Fatal(err)
	}

	assertKeys(t, "top level", keysOf(top), keysOf(schema.Properties))
	assertKeys(t, "role", keysOf(roles[0]), keysOf(schema.Defs.Role.Properties))
	if schema.Properties["version"].Const == nil || *schema.Properties["version"].Const != CacheVersion {
		t.Errorf("schema version const does not match CacheVersion (%d)", CacheVersion)
	}
}

func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func assertKeys(t *testing.T, what string, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("%s keys = %v, schema has %v", what, got, want)
	}
}

func TestSaveWritesSchemaBesideCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gcp", "roles.json")
	if err := Save(path, Snapshot{FetchedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Roles: []iam.Role{{Name: roleAB}}}); err != nil {
		t.Fatal(err)
	}
	snap, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Version != CacheVersion {
		t.Errorf("saved version = %d, want %d", snap.Version, CacheVersion)
	}
	got, err := os.ReadFile(filepath.Join(filepath.Dir(path), SchemaFile))
	if err != nil {
		t.Fatalf("schema not written beside the cache: %v", err)
	}
	if !bytes.Equal(got, schema.GCPRoles) {
		t.Error("schema beside the cache differs from the embedded schema")
	}
}

// validate reports whether doc, marshalled as JSON, passes the embedded GCP schema.
func validate(t *testing.T, doc any) error {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema.GCPRoles))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("gcp-roles.schema.json", schemaDoc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile("gcp-roles.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return s.Validate(inst)
}

func TestSchemaValidatesRealOutput(t *testing.T) {
	data, err := os.ReadFile("../../testdata/roles.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Roles []iam.Role `json:"roles"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	snap := Snapshot{
		Version:   CacheVersion,
		FetchedAt: time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC),
		Roles:     append(f.Roles, iam.Role{Name: "roles/bare.role"}),
	}
	if err := validate(t, snap); err != nil {
		t.Errorf("real output fails the schema: %v", err)
	}
}

func TestSchemaRejectsBadOutput(t *testing.T) {
	base := func() Snapshot {
		return Snapshot{Version: CacheVersion, FetchedAt: time.Now().UTC(), Roles: []iam.Role{{Name: roleAB}}}
	}
	bad := base()
	bad.Roles[0].Name = "not-a-role"
	if err := validate(t, bad); err == nil {
		t.Error("role name without roles/ prefix should fail")
	}
	bad = base()
	bad.Roles[0].IncludedPermissions = []string{"Not A Permission"}
	if err := validate(t, bad); err == nil {
		t.Error("malformed permission should fail")
	}
	bad = base()
	bad.Version = CacheVersion + 1
	if err := validate(t, bad); err == nil {
		t.Error("wrong version should fail")
	}
}
