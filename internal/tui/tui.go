// Package tui is the Bubble Tea browser: a search box, grouped selectable lists, and drill-down views with a back stack.
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
	defaultHeight   = 24
	pageStep        = 10
	homeCrumb       = "search"
	homePlaceholder = "search roles, permissions and services"
	keyEsc          = "esc"
	hint            = "↑↓ move  → enter open  ← esc back  ? help  q quit"
)

// view is one screen: details about what it was opened from, a selectable list, and its own search.
type view struct {
	crumb       string
	placeholder string
	info        []string
	items       []index.Item
	filter      string
	cursor      int
	home        bool
}

// Model is the Bubble Tea model. Build it with New.
type Model struct {
	idx     *index.Index
	fetched time.Time
	all     []index.Item
	cur     view
	stack   []view
	input   textinput.Model
	shown   []index.Item
	counts  [3]int
	height  int
	help    bool
}

// New returns a model on the blank start screen. fetched is the cache's fetch time, shown in the header.
func New(idx *index.Index, fetched time.Time) *Model {
	in := textinput.New()
	in.Prompt = "/ "
	in.Focus()
	m := &Model{
		idx:     idx,
		fetched: fetched,
		all:     idx.All(),
		input:   in,
		height:  defaultHeight,
	}
	m.cur = m.homeView()
	m.refresh()
	return m
}

func (m *Model) homeView() view {
	return view{crumb: homeCrumb, placeholder: homePlaceholder, items: m.all, home: true}
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
	if m.help {
		if s == "?" || s == keyEsc {
			m.help = false
		}
		return m, nil
	}

	switch s {
	case "?":
		m.help = true
	case "q":
		if m.input.Value() == "" {
			return m, tea.Quit
		}
		m.typeKey(k)
	case keyEsc:
		if m.input.Value() != "" {
			m.setFilter("")
		} else {
			m.pop()
		}
	case "left":
		m.pop()
	case "backspace":
		if m.input.Value() == "" {
			m.pop()
		} else {
			m.typeKey(k)
		}
	case "right", "enter":
		m.open()
	case "ctrl+o", "home":
		m.stack = nil
		m.cur = m.homeView()
		m.setFilter("")
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
		m.cur.cursor = 0
	}
}

// open drills into the selected item: a new view with its details and related items.
func (m *Model) open() {
	it, ok := m.selected()
	if !ok {
		return
	}
	m.push(m.viewFor(it))
}

// viewFor builds the view for a role, permission or service.
func (m *Model) viewFor(it index.Item) view {
	v := view{crumb: m.label(it)}
	switch it.Kind {
	case index.KindRole:
		r, _ := m.idx.Role(it.Name)
		v.info = []string{
			"title:       " + r.Title,
			"stage:       " + r.Stage,
			"description: " + r.Description,
			fmt.Sprintf("%d permissions", len(r.IncludedPermissions)),
		}
		v.items = m.idx.PermissionsIn(it.Name)
		v.placeholder = "search permissions"
	case index.KindPermission:
		svc := index.Service(it.Name)
		v.info = []string{"service: " + svc}
		v.items = append([]index.Item{{Kind: index.KindService, Name: svc}}, m.idx.RolesGranting(it.Name)...)
		v.placeholder = "search service and roles"
	default:
		perms := m.idx.PermissionsOf(it.Name)
		roles := m.idx.RolesOf(it.Name)
		v.info = []string{
			fmt.Sprintf("%d permissions", len(perms)),
			fmt.Sprintf("%d roles", len(roles)),
		}
		v.items = append(perms, roles...)
		v.placeholder = "search permissions and roles"
	}
	return v
}

func (m *Model) push(v view) {
	m.cur.filter = m.input.Value()
	m.stack = append(m.stack, m.cur)
	m.cur = v
	m.setFilter("")
}

// pop goes back one view, restoring its search and cursor. It does nothing at the top.
func (m *Model) pop() {
	if len(m.stack) == 0 {
		return
	}
	top := m.stack[len(m.stack)-1]
	m.stack = m.stack[:len(m.stack)-1]
	m.cur = top
	m.input.SetValue(top.filter)
}

func (m *Model) setFilter(s string) {
	m.input.SetValue(s)
	m.cur.cursor = 0
}

func (m *Model) moveCursor(d int) {
	m.cur.cursor += d
}

// refresh recomputes the visible items from the current view and search, and clamps the cursor.
// The start screen shows nothing until something is typed.
func (m *Model) refresh() {
	m.input.Placeholder = m.cur.placeholder
	m.shown = nil
	m.counts = [3]int{}
	q := strings.ToLower(m.input.Value())
	if m.cur.home && q == "" {
		m.cur.cursor = 0
		return
	}
	for _, it := range m.cur.items {
		if q != "" && !m.matches(it, q) {
			continue
		}
		m.shown = append(m.shown, it)
		m.counts[it.Kind]++
	}
	m.cur.cursor = min(max(m.cur.cursor, 0), max(len(m.shown)-1, 0))
}

// matches reports whether q (already lowercased) appears in the item's name or, for roles, its title.
func (m *Model) matches(it index.Item, q string) bool {
	if strings.Contains(strings.ToLower(it.Name), q) {
		return true
	}
	return strings.Contains(strings.ToLower(m.title(it)), q)
}

// title returns the human-readable name of a role, or "" for other kinds.
func (m *Model) title(it index.Item) string {
	if it.Kind != index.KindRole {
		return ""
	}
	r, _ := m.idx.Role(it.Name)
	return r.Title
}

// label is what a row shows: "Compute Admin [roles/compute.admin]" for roles, the ID otherwise.
func (m *Model) label(it index.Item) string {
	if t := m.title(it); t != "" {
		return t + " [" + it.Name + "]"
	}
	return it.Name
}

func (m *Model) selected() (index.Item, bool) {
	if m.cur.cursor < 0 || m.cur.cursor >= len(m.shown) {
		return index.Item{}, false
	}
	return m.shown[m.cur.cursor], true
}

// View implements tea.Model.
func (m *Model) View() tea.View {
	body := m.listView()
	if m.help {
		body = m.helpView()
	}
	v := tea.NewView(body)
	v.AltScreen = true
	return v
}

var (
	bold    = lipgloss.NewStyle().Bold(true)
	reverse = lipgloss.NewStyle().Reverse(true)
	dim     = lipgloss.NewStyle().Faint(true)
)

type row struct {
	text string
	idx  int // index into shown, or -1 for a section header
}

// rows lays out the visible items with a header before each kind.
func (m *Model) rows() []row {
	var out []row
	for i, it := range m.shown {
		if i == 0 || it.Kind != m.shown[i-1].Kind {
			out = append(out, row{text: bold.Render(fmt.Sprintf("%s (%d)", kindTitle(it.Kind), m.counts[it.Kind])), idx: -1})
		}
		text := "  " + m.label(it)
		if i == m.cur.cursor {
			text = reverse.Render("> " + m.label(it))
		}
		out = append(out, row{text: text, idx: i})
	}
	return out
}

// crumbs is the path from the start screen to the current view.
func (m *Model) crumbs() string {
	parts := make([]string, 0, len(m.stack)+1)
	for _, v := range m.stack {
		parts = append(parts, v.crumb)
	}
	parts = append(parts, m.cur.crumb)
	return strings.Join(parts, " › ")
}

func (m *Model) listView() string {
	var top []string
	top = append(top, bold.Render("permidex  "+m.crumbs()+"  data from "+m.fetched.Format("2006-01-02")))
	top = append(top, m.cur.info...)
	top = append(top, m.input.View(), "")

	var body []string
	switch {
	case len(m.shown) == 0 && m.cur.home && m.input.Value() == "":
		body = append(body, dim.Render("type to search roles, permissions and services"))
	case len(m.shown) == 0 && m.input.Value() != "":
		body = append(body, dim.Render("no matches"))
	case len(m.shown) == 0:
		body = append(body, dim.Render("nothing here"))
	default:
		rows := m.rows()
		visible := max(m.height-len(top)-1, 1)
		cursorRow := 0
		for i, r := range rows {
			if r.idx == m.cur.cursor {
				cursorRow = i
			}
		}
		start := min(max(cursorRow-visible/2, 0), max(len(rows)-visible, 0))
		end := min(start+visible, len(rows))
		for _, r := range rows[start:end] {
			body = append(body, r.text)
		}
	}
	return strings.Join(append(append(top, body...), dim.Render(hint)), "\n")
}

func (m *Model) helpView() string {
	var b strings.Builder
	b.WriteString(bold.Render("keys") + "\n\n")
	for _, l := range []string{
		"type                 search (roles match name or title)",
		"up down pgup pgdn    move",
		"enter, right         open the selected item",
		"left, esc            back (esc clears the search first)",
		"backspace            delete, or back when the search is empty",
		"ctrl+o, home         reset to the start screen",
		"?                    this help",
		"q, ctrl+c            quit (q only when the search is empty)",
	} {
		b.WriteString("  " + l + "\n")
	}
	b.WriteString("\n" + dim.Render("? or esc close"))
	return b.String()
}

func kindTitle(k index.Kind) string {
	switch k {
	case index.KindRole:
		return "Roles"
	case index.KindPermission:
		return "Permissions"
	default:
		return "Services"
	}
}
