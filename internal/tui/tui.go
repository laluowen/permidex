// Package tui is the Bubble Tea browser: a filtered list, pivots with a back stack, and a detail view.
package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/lalu-uk/permidex/internal/index"
)

const (
	defaultHeight = 24
	chromeLines   = 4
	pageStep      = 10
)

// pivot is one list on the stack: what it shows, the filter typed on it, and where the cursor was.
type pivot struct {
	title  string
	items  []index.Item
	filter string
	cursor int
}

// Model is the Bubble Tea model. Build it with New.
type Model struct {
	idx     *index.Index
	fetched time.Time
	all     []index.Item
	cur     pivot
	stack   []pivot
	input   textinput.Model
	shown   []index.Item
	cursor  int
	height  int
	detail  bool
	scroll  int
	status  string
}

// New returns a model that shows every item in idx. fetched is the cache's fetch time, shown in the header.
func New(idx *index.Index, fetched time.Time) *Model {
	all := idx.All()
	in := textinput.New()
	in.Prompt = "/ "
	in.Placeholder = "type to filter"
	in.Focus()
	m := &Model{
		idx:     idx,
		fetched: fetched,
		all:     all,
		cur:     pivot{title: "all", items: all},
		input:   in,
		height:  defaultHeight,
	}
	m.refresh()
	return m
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		return m, nil
	case tea.KeyPressMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m *Model) key(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := k.String()
	if s == "ctrl+c" {
		return m, tea.Quit
	}
	if m.detail {
		m.detailKey(s)
		return m, nil
	}
	m.status = ""

	switch s {
	case "q":
		if m.input.Value() == "" {
			return m, tea.Quit
		}
		m.typeKey(k)
	case "esc":
		if len(m.stack) > 0 {
			m.pop()
		} else {
			m.setFilter("")
		}
	case "backspace":
		if m.input.Value() == "" && len(m.stack) > 0 {
			m.pop()
		} else {
			m.typeKey(k)
		}
	case "ctrl+o", "home":
		m.stack = nil
		m.cur = pivot{title: "all", items: m.all}
		m.setFilter("")
	case "enter":
		if _, ok := m.selected(); ok {
			m.detail = true
			m.scroll = 0
		}
	case "ctrl+r":
		m.pivotTo(index.KindPermission, "roles granting ", m.idx.RolesGranting)
	case "ctrl+p":
		m.pivotTo(index.KindRole, "permissions in ", m.idx.PermissionsIn)
	case "ctrl+s":
		m.pivotService()
	case "up":
		m.moveCursor(-1)
	case "down":
		m.moveCursor(1)
	case "pgup":
		m.moveCursor(-pageStep)
	case "pgdown":
		m.moveCursor(pageStep)
	default:
		m.typeKey(k)
	}
	m.refresh()
	return m, nil
}

func (m *Model) typeKey(k tea.KeyPressMsg) {
	before := m.input.Value()
	m.input, _ = m.input.Update(k)
	if m.input.Value() != before {
		m.cursor = 0
	}
}

func (m *Model) detailKey(s string) {
	switch s {
	case "esc", "backspace", "enter", "q":
		m.detail = false
	case "up":
		m.scroll--
	case "down":
		m.scroll++
	case "pgup":
		m.scroll -= pageStep
	case "pgdown":
		m.scroll += pageStep
	}
}

// pivotTo pushes the list of items of kind want that the selected item leads to, using look.
func (m *Model) pivotTo(want index.Kind, label string, look func(string) []index.Item) {
	it, ok := m.selected()
	if !ok || it.Kind != want {
		m.status = fmt.Sprintf("select a %s first", kindLabel(want))
		return
	}
	m.push(label+it.Name, look(it.Name))
}

func (m *Model) pivotService() {
	it, ok := m.selected()
	if !ok || (it.Kind != index.KindService && it.Kind != index.KindPermission) {
		m.status = "select a service or permission first"
		return
	}
	svc := it.Name
	if it.Kind == index.KindPermission {
		svc = index.Service(it.Name)
	}
	m.push("permissions in service "+svc, m.idx.PermissionsOf(svc))
}

func (m *Model) push(title string, items []index.Item) {
	m.cur.filter = m.input.Value()
	m.cur.cursor = m.cursor
	m.stack = append(m.stack, m.cur)
	m.cur = pivot{title: title, items: items}
	m.setFilter("")
}

func (m *Model) pop() {
	top := m.stack[len(m.stack)-1]
	m.stack = m.stack[:len(m.stack)-1]
	m.cur = top
	m.setFilter(top.filter)
	m.cursor = top.cursor
}

func (m *Model) setFilter(s string) {
	m.input.SetValue(s)
	m.cursor = 0
}

func (m *Model) moveCursor(d int) {
	m.cursor += d
}

// refresh recomputes the visible rows from the current pivot and filter, and clamps the cursor.
func (m *Model) refresh() {
	q := strings.ToLower(m.input.Value())
	m.shown = m.shown[:0]
	for _, it := range m.cur.items {
		if q == "" || strings.Contains(strings.ToLower(it.Name), q) {
			m.shown = append(m.shown, it)
		}
	}
	m.cursor = min(max(m.cursor, 0), max(len(m.shown)-1, 0))
}

func (m *Model) selected() (index.Item, bool) {
	if m.cursor < 0 || m.cursor >= len(m.shown) {
		return index.Item{}, false
	}
	return m.shown[m.cursor], true
}

// View implements tea.Model.
func (m *Model) View() tea.View {
	var b strings.Builder
	if m.detail {
		b.WriteString(m.detailView())
	} else {
		b.WriteString(m.listView())
	}
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

var (
	bold    = lipgloss.NewStyle().Bold(true)
	reverse = lipgloss.NewStyle().Reverse(true)
	dim     = lipgloss.NewStyle().Faint(true)
)

func (m *Model) listView() string {
	var b strings.Builder
	header := fmt.Sprintf("permidex  %s  (%d)  data from %s", m.cur.title, len(m.shown), m.fetched.Format("2006-01-02"))
	b.WriteString(bold.Render(header) + "\n")
	b.WriteString(m.input.View() + "\n\n")

	rows := max(m.height-chromeLines, 1)
	start := min(max(m.cursor-rows/2, 0), max(len(m.shown)-rows, 0))
	end := min(start+rows, len(m.shown))
	if len(m.shown) == 0 {
		b.WriteString(dim.Render("no matches") + "\n")
	}
	for i := start; i < end; i++ {
		it := m.shown[i]
		line := fmt.Sprintf("%-8s %s", kindLabel(it.Kind), it.Name)
		if i == m.cursor {
			line = reverse.Render("> " + line)
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
	}

	b.WriteString("\n")
	if m.status != "" {
		b.WriteString(m.status + "\n")
	}
	b.WriteString(dim.Render("enter detail  ctrl+r roles  ctrl+p perms  ctrl+s service  esc back  ctrl+o home  q quit"))
	return b.String()
}

func (m *Model) detailView() string {
	it, ok := m.selected()
	if !ok {
		m.detail = false
		return ""
	}
	lines := m.detailLines(it)
	rows := max(m.height-2, 1)
	m.scroll = min(max(m.scroll, 0), max(len(lines)-rows, 0))
	end := min(m.scroll+rows, len(lines))

	var b strings.Builder
	b.WriteString(bold.Render(kindLabel(it.Kind)+" "+it.Name) + "\n\n")
	for _, l := range lines[m.scroll:end] {
		b.WriteString(l + "\n")
	}
	b.WriteString("\n" + dim.Render("esc back  up/down scroll"))
	return b.String()
}

func (m *Model) detailLines(it index.Item) []string {
	switch it.Kind {
	case index.KindRole:
		r, ok := m.idx.Role(it.Name)
		if !ok {
			return []string{"unknown role"}
		}
		perms := m.idx.PermissionsIn(it.Name)
		return append(detailHead(
			"title:       "+r.Title,
			"stage:       "+r.Stage,
			"description: "+r.Description,
			"",
			fmt.Sprintf("%d permissions:", len(r.IncludedPermissions)),
		), names(perms)...)
	case index.KindPermission:
		svc := index.Service(it.Name)
		roles := m.idx.RolesGranting(it.Name)
		return append(detailHead(
			"service: "+svc,
			"",
			fmt.Sprintf("granted by %d roles:", len(roles)),
		), names(roles)...)
	default:
		return []string{
			fmt.Sprintf("%d permissions", len(m.idx.PermissionsOf(it.Name))),
			fmt.Sprintf("%d roles", len(m.idx.RolesOf(it.Name))),
		}
	}
}

func detailHead(lines ...string) []string {
	return lines
}

func names(items []index.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = "  " + it.Name
	}
	return out
}

func kindLabel(k index.Kind) string {
	switch k {
	case index.KindRole:
		return "role"
	case index.KindPermission:
		return "perm"
	default:
		return "service"
	}
}
