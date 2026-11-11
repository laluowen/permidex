package tui

import (
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lalu-uk/permidex/internal/iam"
	"github.com/lalu-uk/permidex/internal/index"
)

const (
	permGet    = "compute.instances.get"
	permDelete = "compute.instances.delete"
	roleAdmin  = "roles/compute.admin"
)

func testModel(t *testing.T) *Model {
	t.Helper()
	idx := index.Build([]iam.Role{
		{Name: "roles/compute.viewer", Title: "Compute Viewer", IncludedPermissions: []string{permGet, "compute.instances.list"}},
		{Name: roleAdmin, Title: "Compute Admin", IncludedPermissions: []string{permGet, permDelete}},
		{Name: "roles/storage.viewer", Title: "Storage Viewer", IncludedPermissions: []string{"storage.objects.get"}},
	})
	m := New(idx, time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC))
	m.height = 40
	showAll(m)
	return m
}

// showAll leaves the blank start screen so every item is listed.
func showAll(m *Model) {
	m.cur.home = false
	m.refresh()
}

var keys = map[string]tea.KeyPressMsg{
	"esc":       {Code: tea.KeyEscape},
	"enter":     {Code: tea.KeyEnter},
	"backspace": {Code: tea.KeyBackspace},
	"left":      {Code: tea.KeyLeft},
	"right":     {Code: tea.KeyRight},
	"up":        {Code: tea.KeyUp},
	"down":      {Code: tea.KeyDown},
	"ctrl+o":    {Code: 'o', Mod: tea.ModCtrl},
}

func press(m *Model, s string) {
	k, ok := keys[s]
	if !ok {
		k = tea.KeyPressMsg{Code: rune(s[0]), Text: s}
	}
	_, _ = m.Update(k)
}

func selectNamed(t *testing.T, m *Model, name string) {
	t.Helper()
	for i, it := range m.shown {
		if it.Name == name {
			m.cur.cursor = i
			return
		}
	}
	t.Fatalf("%q not in list", name)
}

func shownNames(m *Model) []string {
	out := make([]string, len(m.shown))
	for i, it := range m.shown {
		out[i] = it.Name
	}
	return out
}

func TestOpenRoleListsPermissions(t *testing.T) {
	m := testModel(t)
	selectNamed(t, m, roleAdmin)
	press(m, "right")
	if want := []string{permDelete, permGet}; !reflect.DeepEqual(shownNames(m), want) {
		t.Errorf("role permissions = %v, want %v", shownNames(m), want)
	}
	if !strings.Contains(m.View().Content, "2 permissions") {
		t.Error("role view should show its details")
	}
}

func TestPermissionShowsServiceAndRoles(t *testing.T) {
	m := testModel(t)
	selectNamed(t, m, roleAdmin)
	press(m, "enter")
	selectNamed(t, m, permDelete)
	press(m, "enter")
	if want := []string{"compute", roleAdmin}; !reflect.DeepEqual(shownNames(m), want) {
		t.Errorf("permission view = %v, want service then roles %v", shownNames(m), want)
	}
	if !strings.Contains(m.View().Content, "service: compute") {
		t.Error("permission view should name the service")
	}
}

func TestServiceOpensPermissionsAndRoles(t *testing.T) {
	m := testModel(t)
	selectNamed(t, m, "compute")
	press(m, "enter")
	if len(m.shown) == 0 {
		t.Fatal("service view should list items")
	}
	if !strings.Contains(m.View().Content, "Roles (2)") {
		t.Errorf("service view should group roles:\n%s", m.View().Content)
	}
}

func TestDrillDownSelectionAndBack(t *testing.T) {
	m := testModel(t)
	selectNamed(t, m, roleAdmin)
	press(m, "right")
	selectNamed(t, m, permDelete)
	press(m, "right")
	if len(m.stack) != 2 {
		t.Fatalf("stack depth = %d, want 2", len(m.stack))
	}
	press(m, "left")
	if got := shownNames(m); !reflect.DeepEqual(got, []string{permDelete, permGet}) {
		t.Errorf("left should return to the role's permissions, got %v", got)
	}
	if got := m.cur.cursor; m.shown[got].Name != permDelete {
		t.Errorf("cursor not restored on the permission we came from")
	}
	press(m, "esc")
	if len(m.stack) != 0 {
		t.Errorf("esc should pop when the search is empty (stack %d)", len(m.stack))
	}
}

func TestBackspaceEditsSearchBeforeGoingBack(t *testing.T) {
	m := testModel(t)
	selectNamed(t, m, roleAdmin)
	press(m, "right")
	press(m, "g")
	press(m, "backspace")
	if len(m.stack) != 1 || m.input.Value() != "" {
		t.Fatalf("backspace with text should delete, not go back (stack %d, search %q)", len(m.stack), m.input.Value())
	}
	press(m, "backspace")
	if len(m.stack) != 0 {
		t.Errorf("backspace on empty search should go back (stack %d)", len(m.stack))
	}
}

func TestEscClearsSearchFirst(t *testing.T) {
	m := testModel(t)
	selectNamed(t, m, roleAdmin)
	press(m, "right")
	press(m, "g")
	press(m, "esc")
	if len(m.stack) != 1 || m.input.Value() != "" {
		t.Errorf("esc with text should clear it only (stack %d, search %q)", len(m.stack), m.input.Value())
	}
}

func TestLeftAtTopDoesNothing(t *testing.T) {
	m := testModel(t)
	before := len(m.shown)
	press(m, "left")
	if len(m.stack) != 0 || len(m.shown) != before {
		t.Errorf("left at top should be a no-op (stack %d, shown %d)", len(m.stack), len(m.shown))
	}
}

func TestResetToStartScreen(t *testing.T) {
	m := testModel(t)
	selectNamed(t, m, roleAdmin)
	press(m, "right")
	press(m, "ctrl+o")
	if len(m.stack) != 0 || !m.cur.home || len(m.shown) != 0 {
		t.Errorf("ctrl+o left stack %d, home %v, shown %d", len(m.stack), m.cur.home, len(m.shown))
	}
}

func TestQuitOnlyWithEmptySearch(t *testing.T) {
	m := testModel(t)
	press(m, "c")
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd != nil {
		t.Error("q with a search should type, not quit")
	}
	if m.input.Value() != "cq" {
		t.Errorf("search = %q, want cq", m.input.Value())
	}
	m.setFilter("")
	_, cmd = m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Error("q with empty search should quit")
	}
}

func TestStartScreenIsBlankUntilTyped(t *testing.T) {
	m := New(testModel(t).idx, time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC))
	if len(m.shown) != 0 {
		t.Fatalf("start screen shows %d items, want none", len(m.shown))
	}
	if !strings.Contains(m.View().Content, "type to search") {
		t.Error("start screen should prompt for a search")
	}
	press(m, "s")
	if len(m.shown) == 0 {
		t.Error("typing should show matches")
	}
}

func TestSearchMatchesRoleTitle(t *testing.T) {
	m := testModel(t)
	m.setFilter("storage viewer")
	m.refresh()
	if got := shownNames(m); len(got) != 1 || got[0] != "roles/storage.viewer" {
		t.Errorf("title search = %v", got)
	}
}

func TestRoleRowShowsTitleAndID(t *testing.T) {
	m := testModel(t)
	if got := m.label(index.Item{Kind: index.KindRole, Name: "roles/compute.admin"}); got != "Compute Admin [roles/compute.admin]" {
		t.Errorf("label = %q", got)
	}
	if got := m.label(index.Item{Kind: index.KindPermission, Name: permGet}); got != permGet {
		t.Errorf("permission label = %q", got)
	}
}

func TestHelpToggle(t *testing.T) {
	m := testModel(t)
	press(m, "?")
	if !m.help || !strings.Contains(m.View().Content, "left, esc") {
		t.Fatal("? should open the help overlay")
	}
	press(m, "down")
	if !m.help {
		t.Error("other keys should not close help")
	}
	press(m, "esc")
	if m.help {
		t.Error("esc should close help")
	}
}

func TestListViewHasSectionHeadersBreadcrumbAndHintBar(t *testing.T) {
	m := testModel(t)
	m.setFilter("compute")
	m.refresh()
	out := m.View().Content
	for _, want := range []string{"Roles (2)", "Permissions (3)", "→ enter open", "search"} {
		if !strings.Contains(out, want) {
			t.Errorf("list view missing %q:\n%s", want, out)
		}
	}
}

func TestViewRendersStartAndRows(t *testing.T) {
	m := testModel(t)
	out := m.View().Content
	if !strings.Contains(out, "data from 2026-11-11") || !strings.Contains(out, "Storage Viewer [roles/storage.viewer]") {
		t.Errorf("list view missing header or row:\n%s", out)
	}
}
