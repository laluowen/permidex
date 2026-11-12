package browse

import (
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

const (
	kindThing = "thing"
	kindPart  = "part"
	thingOne  = "t/one"
	thingTwo  = "t/two"
	partAlpha = "p.alpha"
	partBeta  = "p.beta"
)

var ansi = regexp.MustCompile("\x1b\\[[0-9;]*m")

// fake is a hand-written Source: things open to parts, and parts open back to things.
type fake struct{}

func (fake) Name() string         { return "fake" }
func (fake) FetchedAt() time.Time { return time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC) }
func (fake) Sections() []Section {
	return []Section{
		{Kind: kindThing, Title: "Things", Key: "things"},
		{Kind: kindPart, Title: "Parts", Key: "parts"},
	}
}

func (fake) Root() View {
	return View{
		Crumb:       "search",
		Placeholder: "search things and parts",
		Home:        true,
		Items: []Item{
			{Kind: kindThing, Name: thingOne},
			{Kind: kindThing, Name: thingTwo},
			{Kind: kindPart, Name: partAlpha},
			{Kind: kindPart, Name: partBeta},
		},
	}
}

func (fake) Open(it Item) View {
	if it.Kind == kindThing {
		return View{
			Crumb: it.Name, Placeholder: "search parts", Info: []string{"a thing"},
			Items: []Item{{Kind: kindPart, Name: partAlpha}},
		}
	}
	return View{Crumb: it.Name, Placeholder: "search things", Items: []Item{{Kind: kindThing, Name: thingOne}}}
}

func (fake) Title(it Item) string {
	if it.Name == thingOne {
		return "First Thing"
	}
	return ""
}

func (f fake) Label(it Item) string {
	if t := f.Title(it); t != "" {
		return t + " [" + it.Name + "]"
	}
	return it.Name
}

func newModel() *Model {
	m := New(fake{})
	m.height = 40
	m.cur.Home = false
	m.refresh()
	return m
}

var keys = map[string]tea.KeyPressMsg{
	"esc":       {Code: tea.KeyEscape},
	"enter":     {Code: tea.KeyEnter},
	"backspace": {Code: tea.KeyBackspace},
	"up":        {Code: tea.KeyUp},
	"down":      {Code: tea.KeyDown},
	"tab":       {Code: tea.KeyTab},
	"home":      {Code: tea.KeyHome},
	"end":       {Code: tea.KeyEnd},
	"ctrl+j":    {Code: 'j', Mod: tea.ModCtrl},
	"ctrl+k":    {Code: 'k', Mod: tea.ModCtrl},
	"ctrl+o":    {Code: 'o', Mod: tea.ModCtrl},
	"ctrl+q":    {Code: 'q', Mod: tea.ModCtrl},
	"ctrl+c":    {Code: 'c', Mod: tea.ModCtrl},
}

func press(m *Model, s string) {
	k, ok := keys[s]
	if !ok {
		k = tea.KeyPressMsg{Code: rune(s[0]), Text: s}
	}
	_, _ = m.Update(k)
}

func shownNames(m *Model) []string {
	out := make([]string, len(m.shown))
	for i, it := range m.shown {
		out[i] = it.Name
	}
	return out
}

func selectNamed(t *testing.T, m *Model, name string) {
	t.Helper()
	for i, it := range m.shown {
		if it.Name == name {
			m.cur.cursor = i
			return
		}
	}
	t.Fatalf("%q not in list %v", name, shownNames(m))
}

func plainView(m *Model) string {
	return ansi.ReplaceAllString(m.View().Content, "")
}

func TestStartScreenIsBlankUntilTyped(t *testing.T) {
	m := New(fake{})
	if len(m.shown) != 0 {
		t.Fatalf("start screen shows %d items, want none", len(m.shown))
	}
	if !strings.Contains(plainView(m), "type to search") {
		t.Error("start screen should prompt for a search")
	}
	press(m, "p")
	if len(m.shown) == 0 {
		t.Error("typing should show matches")
	}
}

func TestSearchMatchesTitleAndName(t *testing.T) {
	m := newModel()
	m.setFilter("first thing")
	m.refresh()
	if got := shownNames(m); !reflect.DeepEqual(got, []string{thingOne}) {
		t.Errorf("title search = %v", got)
	}
	m.setFilter("beta")
	m.refresh()
	if got := shownNames(m); !reflect.DeepEqual(got, []string{partBeta}) {
		t.Errorf("name search = %v", got)
	}
}

func TestEnterOpensAndEscGoesBack(t *testing.T) {
	m := newModel()
	selectNamed(t, m, thingTwo)
	press(m, "enter")
	if !slices.Equal(shownNames(m), []string{partAlpha}) || len(m.stack) != 1 {
		t.Fatalf("open: shown %v, stack %d", shownNames(m), len(m.stack))
	}
	press(m, "esc")
	if len(m.stack) != 0 {
		t.Errorf("esc should pop with an empty search (stack %d)", len(m.stack))
	}
	if got := m.shown[m.cur.cursor].Name; got != thingTwo {
		t.Errorf("cursor after back = %q, want the item opened from", got)
	}
}

func TestEscClearsSearchBeforePopping(t *testing.T) {
	m := newModel()
	selectNamed(t, m, thingOne)
	press(m, "enter")
	press(m, "x")
	press(m, "esc")
	if len(m.stack) != 1 || m.input.Value() != "" {
		t.Errorf("esc with text should clear it only (stack %d, search %q)", len(m.stack), m.input.Value())
	}
}

func TestBackspaceNeverGoesBack(t *testing.T) {
	m := newModel()
	selectNamed(t, m, thingOne)
	press(m, "enter")
	press(m, "x")
	press(m, "backspace")
	press(m, "backspace")
	if len(m.stack) != 1 {
		t.Errorf("backspace should only delete (stack %d)", len(m.stack))
	}
}

func TestEscAtTopDoesNothing(t *testing.T) {
	m := newModel()
	before := len(m.shown)
	press(m, "esc")
	if len(m.stack) != 0 || len(m.shown) != before {
		t.Errorf("esc at top should be a no-op (stack %d, shown %d)", len(m.stack), len(m.shown))
	}
}

func TestSearchIsScopedToTheView(t *testing.T) {
	m := newModel()
	selectNamed(t, m, thingOne)
	press(m, "enter")
	press(m, "a")
	press(m, "l")
	press(m, "p")
	if len(m.shown) != 1 {
		t.Errorf("search inside the view = %v", shownNames(m))
	}
	if m.input.Placeholder != "search parts" {
		t.Errorf("placeholder = %q", m.input.Placeholder)
	}
}

func TestJumpKeysMoveToSection(t *testing.T) {
	m := newModel()
	press(m, "ctrl+k")
	if m.shown[m.cur.cursor].Kind != kindPart {
		t.Errorf("ctrl+k landed on %v", m.shown[m.cur.cursor])
	}
	press(m, "ctrl+j")
	if m.shown[m.cur.cursor].Kind != kindThing {
		t.Errorf("ctrl+j landed on %v", m.shown[m.cur.cursor])
	}
}

func TestResetToStartScreen(t *testing.T) {
	m := newModel()
	selectNamed(t, m, thingOne)
	press(m, "enter")
	press(m, "ctrl+o")
	if len(m.stack) != 0 || !m.cur.Home || len(m.shown) != 0 {
		t.Errorf("ctrl+o left stack %d, home %v, shown %d", len(m.stack), m.cur.Home, len(m.shown))
	}
}

func TestTabSelectsAndOutputsYAMLInSectionOrder(t *testing.T) {
	m := newModel()
	selectNamed(t, m, partBeta)
	press(m, "tab")
	selectNamed(t, m, thingOne)
	press(m, "tab")
	if got := m.Selection(); got != "" {
		t.Errorf("Selection before ctrl+q = %q, want empty", got)
	}
	press(m, "ctrl+q")
	want := "things:\n  - t/one # First Thing\nparts:\n  - p.beta\n"
	if got := m.Selection(); got != want {
		t.Errorf("Selection = %q, want %q", got, want)
	}
}

func TestQuitKeys(t *testing.T) {
	m := newModel()
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd != nil || m.input.Value() != "q" {
		t.Error("q should type, not quit")
	}
	_, cmd = m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Error("ctrl+c should quit")
	}
}

func TestCtrlCAbortsWithNoOutput(t *testing.T) {
	m := newModel()
	selectNamed(t, m, thingOne)
	press(m, "tab")
	press(m, "ctrl+c")
	if m.Selection() != "" {
		t.Error("ctrl+c should print nothing")
	}
}

func TestQuestionMarkTypesIntoSearch(t *testing.T) {
	m := newModel()
	press(m, "?")
	if m.input.Value() != "?" {
		t.Errorf("? should type into the search, got %q", m.input.Value())
	}
}

func TestHomeAndEndJumpToTopAndBottom(t *testing.T) {
	m := newModel()
	press(m, "end")
	if m.cur.cursor != len(m.shown)-1 {
		t.Errorf("end cursor = %d, want %d", m.cur.cursor, len(m.shown)-1)
	}
	press(m, "home")
	if m.cur.cursor != 0 {
		t.Errorf("home cursor = %d, want 0", m.cur.cursor)
	}
}

func TestFooterListsEveryKeyAndNoHelpScreen(t *testing.T) {
	m := newModel()
	m.width = 200
	out := plainView(m)
	for _, want := range []string{"↑↓ pgup pgdn move", "home end top or bottom", "ctrl+o start screen", "ctrl+q done", "ctrl+j Things"} {
		if !strings.Contains(out, want) {
			t.Errorf("footer missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "? help") || strings.Contains(out, "ctrl+c") {
		t.Errorf("footer should not show ? or ctrl+c:\n%s", out)
	}
}

func TestListViewShowsHeadersBreadcrumbAndHints(t *testing.T) {
	m := newModel()
	m.setFilter("one")
	m.refresh()
	out := plainView(m)
	for _, want := range []string{"Things (1)", "enter open", "search", "permidex fake"} {
		if !strings.Contains(out, want) {
			t.Errorf("list view missing %q:\n%s", want, out)
		}
	}
}

func TestBreadcrumbDropsOldestCrumbs(t *testing.T) {
	m := newModel()
	m.width = 20
	selectNamed(t, m, thingOne)
	press(m, "enter")
	selectNamed(t, m, partAlpha)
	press(m, "enter")
	crumb := m.breadcrumb()
	if !strings.HasPrefix(crumb, "… › ") || !strings.HasSuffix(crumb, partAlpha) {
		t.Errorf("breadcrumb = %q, want truncated from the left", crumb)
	}
}

func TestBottomBarShowsJumpKeysAndSectionNames(t *testing.T) {
	m := newModel()
	m.width = 200
	out := plainView(m)
	if !strings.Contains(out, "ctrl+j Things") || !strings.Contains(out, "ctrl+k Parts") {
		t.Errorf("bar should name each jump key's section:\n%s", out)
	}
}

func TestBottomBarWrapsOnNarrowTerminal(t *testing.T) {
	m := newModel()
	m.height = 40
	m.width = 40
	wide := newModel()
	wide.height = 40
	wide.width = 200
	if strings.Count(plainView(m), "\n") <= strings.Count(plainView(wide), "\n") {
		t.Errorf("narrow bar should take more lines than the wide one")
	}
}
