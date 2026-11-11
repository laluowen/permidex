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

func testModel(t *testing.T) *Model {
	t.Helper()
	idx := index.Build([]iam.Role{
		{Name: "roles/compute.viewer", Title: "Compute Viewer", IncludedPermissions: []string{permGet, "compute.instances.list"}},
		{Name: "roles/compute.admin", Title: "Compute Admin", IncludedPermissions: []string{permGet, "compute.instances.delete"}},
		{Name: "roles/storage.viewer", Title: "Storage Viewer", IncludedPermissions: []string{"storage.objects.get"}},
	})
	m := New(idx, time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC))
	m.height = 40
	return m
}

const (
	permGet = "compute.instances.get"
	allView = "all"
)

var keys = map[string]tea.KeyPressMsg{
	"esc":       {Code: tea.KeyEscape},
	"enter":     {Code: tea.KeyEnter},
	"backspace": {Code: tea.KeyBackspace},
	"up":        {Code: tea.KeyUp},
	"down":      {Code: tea.KeyDown},
	"ctrl+r":    {Code: 'r', Mod: tea.ModCtrl},
	"ctrl+p":    {Code: 'p', Mod: tea.ModCtrl},
	"ctrl+s":    {Code: 's', Mod: tea.ModCtrl},
	"ctrl+o":    {Code: 'o', Mod: tea.ModCtrl},
}

func press(m *Model, s string) {
	k, ok := keys[s]
	if !ok {
		k = tea.KeyPressMsg{Code: rune(s[0]), Text: s}
	}
	_, _ = m.Update(k)
}

func selectedName(t *testing.T, m *Model) string {
	t.Helper()
	it, ok := m.selected()
	if !ok {
		t.Fatal("nothing selected")
	}
	return it.Name
}

func selectNamed(t *testing.T, m *Model, name string) {
	t.Helper()
	for i, it := range m.shown {
		if it.Name == name {
			m.cursor = i
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

func TestRolesPivotAndBack(t *testing.T) {
	m := testModel(t)
	selectNamed(t, m, "compute.instances.list")
	press(m, "ctrl+r")
	if m.cur.title == allView {
		t.Fatal("pivot did not change the list")
	}
	if got, want := shownNames(m), []string{"roles/compute.viewer"}; len(got) != 1 || got[0] != want[0] {
		t.Errorf("roles pivot = %v, want %v", got, want)
	}
	press(m, "esc")
	if m.cur.title != allView || len(m.stack) != 0 {
		t.Errorf("after esc: title %q, stack %d", m.cur.title, len(m.stack))
	}
	if got := selectedName(t, m); got != "compute.instances.list" {
		t.Errorf("cursor after pop = %q, want the permission we pivoted from", got)
	}
}

func TestPermsPivotFromRole(t *testing.T) {
	m := testModel(t)
	selectNamed(t, m, "roles/compute.admin")
	press(m, "ctrl+p")
	want := []string{"compute.instances.delete", permGet}
	if got := shownNames(m); !reflect.DeepEqual(got, want) {
		t.Errorf("perms = %v, want %v", got, want)
	}
}

func TestServicePivotFromPermission(t *testing.T) {
	m := testModel(t)
	selectNamed(t, m, "storage.objects.get")
	press(m, "ctrl+s")
	if got := shownNames(m); !reflect.DeepEqual(got, []string{"storage.objects.get"}) {
		t.Errorf("service perms = %v", got)
	}
}

func TestPivotNeedsRightKind(t *testing.T) {
	m := testModel(t)
	selectNamed(t, m, "roles/compute.admin")
	press(m, "ctrl+r")
	if m.cur.title != allView || len(m.stack) != 0 {
		t.Error("ctrl+r on a role should not push")
	}
	if m.status == "" {
		t.Error("want a status message")
	}
}

func TestTypingFiltersAndBackspaceEditsBeforePopping(t *testing.T) {
	m := testModel(t)
	press(m, "ctrl+s")
	selectNamed(t, m, "compute")
	press(m, "ctrl+s")
	press(m, "c")
	press(m, "o")
	press(m, "backspace")
	if len(m.stack) != 1 {
		t.Fatalf("backspace with text should edit, not pop (stack %d)", len(m.stack))
	}
	if got := m.input.Value(); got != "c" {
		t.Errorf("filter = %q, want c", got)
	}
	press(m, "backspace")
	press(m, "backspace")
	if len(m.stack) != 0 {
		t.Errorf("backspace on empty filter should pop (stack %d)", len(m.stack))
	}
}

func TestResetToFullIndex(t *testing.T) {
	m := testModel(t)
	selectNamed(t, m, "roles/compute.admin")
	press(m, "ctrl+p")
	press(m, "ctrl+r")
	press(m, "ctrl+o")
	if len(m.stack) != 0 || m.cur.title != allView {
		t.Errorf("ctrl+o left stack %d, title %q", len(m.stack), m.cur.title)
	}
}

func TestQuitOnlyWithEmptyFilter(t *testing.T) {
	m := testModel(t)
	press(m, "c")
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd != nil {
		t.Error("q with a filter should type, not quit")
	}
	if m.input.Value() != "cq" {
		t.Errorf("filter = %q, want cq", m.input.Value())
	}
	m.setFilter("")
	_, cmd = m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Error("q with empty filter should quit")
	}
}

func TestDetailOpenClose(t *testing.T) {
	m := testModel(t)
	selectNamed(t, m, "roles/compute.viewer")
	press(m, "enter")
	if !m.detail {
		t.Fatal("enter did not open detail")
	}
	press(m, "esc")
	if m.detail {
		t.Error("esc did not close detail")
	}
}

func TestViewRendersListAndDetail(t *testing.T) {
	m := testModel(t)
	list := m.View().Content
	if !strings.Contains(list, "data from 2026-11-11") || !strings.Contains(list, "roles/storage.viewer") {
		t.Errorf("list view missing header or row:\n%s", list)
	}
	selectNamed(t, m, "roles/compute.admin")
	press(m, "enter")
	detail := m.View().Content
	if !strings.Contains(detail, "Compute Admin") || !strings.Contains(detail, "2 permissions") {
		t.Errorf("detail view wrong:\n%s", detail)
	}
}
