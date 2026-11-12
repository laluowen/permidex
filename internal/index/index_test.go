package index

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/laluowen/permidex/internal/iam"
)

const computeGet = "compute.instances.get"

func fixture(t *testing.T) *Index {
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
	return Build(f.Roles)
}

func names(items []Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Name
	}
	return out
}

func TestService(t *testing.T) {
	for in, want := range map[string]string{
		computeGet:          "compute",
		"resourcemanager.x": "resourcemanager",
		"standalone":        "standalone",
	} {
		if got := Service(in); got != want {
			t.Errorf("Service(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRolesGranting(t *testing.T) {
	x := fixture(t)
	tests := []struct {
		perm string
		want []string
	}{
		{"resourcemanager.projects.get", []string{
			"roles/accessapproval.admin", "roles/accessapproval.approver", "roles/accessapproval.configEditor",
		}},
		{computeGet, []string{"roles/compute.viewer"}},
		{"nope.nothing.here", nil},
	}
	for _, tc := range tests {
		if got := names(x.RolesGranting(tc.perm)); len(got) != len(tc.want) || (len(got) > 0 && !reflect.DeepEqual(got, tc.want)) {
			t.Errorf("RolesGranting(%q) = %v, want %v", tc.perm, got, tc.want)
		}
	}
}

func TestPermissionsInIsSortedAndUnique(t *testing.T) {
	x := fixture(t)
	got := names(x.PermissionsIn("roles/compute.viewer"))
	want := []string{"compute.instances.get", "compute.instances.list", "compute.projects.get"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PermissionsIn = %v, want %v", got, want)
	}
	if x.PermissionsIn("roles/missing") != nil {
		t.Error("PermissionsIn(missing) should be nil")
	}
}

func TestServiceGrouping(t *testing.T) {
	x := fixture(t)
	if got := names(x.PermissionsOf("compute")); len(got) != 3 {
		t.Errorf("PermissionsOf(compute) = %v, want 3", got)
	}
	if got := names(x.RolesOf("compute")); !reflect.DeepEqual(got, []string{"roles/compute.viewer"}) {
		t.Errorf("RolesOf(compute) = %v", got)
	}
	if got := names(x.RolesOf("accessapproval")); len(got) != 3 {
		t.Errorf("RolesOf(accessapproval) = %v, want 3 roles", got)
	}
	if got := x.PermissionsOf("nope"); len(got) != 0 {
		t.Errorf("PermissionsOf(nope) = %v", got)
	}
}

func TestAllOrderAndKinds(t *testing.T) {
	x := fixture(t)
	all := x.All()
	var kinds []Kind
	for _, it := range all {
		if len(kinds) == 0 || kinds[len(kinds)-1] != it.Kind {
			kinds = append(kinds, it.Kind)
		}
	}
	want := []Kind{KindRole, KindPermission, KindService}
	if !reflect.DeepEqual(kinds, want) {
		t.Errorf("kind runs = %v, want %v", kinds, want)
	}
	if all[0].Name != "roles/accessapproval.admin" {
		t.Errorf("first item = %q, want roles sorted by name", all[0].Name)
	}
}

func TestRoleLookup(t *testing.T) {
	x := fixture(t)
	r, ok := x.Role("roles/compute.viewer")
	if !ok || r.Title != "Compute Viewer" {
		t.Errorf("Role = %+v, %v", r, ok)
	}
	if _, ok := x.Role("roles/nope"); ok {
		t.Error("Role(nope) found")
	}
}
