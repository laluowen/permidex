// Package tui is the Bubble Tea browser: a search box, grouped results, pivots with a back stack, and a detail view.
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
	// listChrome is the fixed lines around the results: header, input, blank, status, hint bar.
	listChrome = 5
	// detailChrome is the fixed lines around the detail body: header, blank, status, hint bar.
	detailChrome = 4
	pageStep     = 10
	searchTitle  = "search"
	keyEsc       = "esc"
)

// pivot is one list on the stack: what it shows, the filter typed on it, and where the cursor was.
type pivot struct {
	title  string
	items  []index.Item
	filter string
	cursor int
	home   bool
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
	counts  [3]int
	cursor  int
	height  int
	detail  bool
	help    bool
	scroll  int
	status  string
}

// New returns a model that searches every item in idx. fetched is the cache's fetch time, shown in the header.
func New(idx *index.Index, fetched time.Time) *Model {
	all := idx.All()
	in := textinput.New()
	in.Prompt = "/ "
	in.Placeholder = "search roles, permissions and services"
	in.Focus()
	m := &Model{
		idx:     idx,
		fetched: fetched,
		all:     all,
		cur:     pivot{title: searchTitle, items: all, home: true},
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
	if m.help {
		if s == "?" || s == keyEsc {
			m.help = false
		}
		return m, nil
	}
	if m.detail {
		m.detailKey(s)
		return m, nil
	}
	m.status = ""

	switch s {
	case "?":
		m.help = true
	case "q":
		if m.input.Value() == "" {
			return m, tea.Quit
		}
		m.typeKey(k)
	case keyEsc:
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
		m.cur = pivot{title: searchTitle, items: m.all, home: true}
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
	case keyEsc, "backspace", "enter", "q":
		m.detail = false
	case "?":
		m.help = true
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
		m.status = fmt.Sprintf("select a %s first", kindName(want))
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

// refresh recomputes the visible items from the current list and filter, and clamps the cursor.
// The start screen shows nothing until something is typed.
func (m *Model) refresh() {
	m.shown = nil
	m.counts = [3]int{}
	q := strings.ToLower(m.input.Value())
	if m.cur.home && q == "" {
		m.cursor = 0
		return
	}
	for _, it := range m.cur.items {
		if q != "" && !m.matches(it, q) {
			continue
		}
		m.shown = append(m.shown, it)
		m.counts[it.Kind]++
	}
	m.cursor = min(max(m.cursor, 0), max(len(m.shown)-1, 0))
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
	if m.cursor < 0 || m.cursor >= len(m.shown) {
		return index.Item{}, false
	}
	return m.shown[m.cursor], true
}

// View implements tea.Model.
func (m *Model) View() tea.View {
	var body string
	switch {
	case m.help:
		body = m.helpView()
	case m.detail:
		body = m.detailView()
	default:
		body = m.listView()
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
		if i == m.cursor {
			text = reverse.Render("> " + m.label(it))
		}
		out = append(out, row{text: text, idx: i})
	}
	return out
}

func (m *Model) listView() string {
	var b strings.Builder
	header := fmt.Sprintf("permidex  %s  (%d)  data from %s", m.cur.title, len(m.shown), m.fetched.Format("2006-01-02"))
	b.WriteString(bold.Render(header) + "\n")
	b.WriteString(m.input.View() + "\n\n")

	switch {
	case len(m.shown) == 0 && m.cur.home && m.input.Value() == "":
		b.WriteString(dim.Render("type to search roles, permissions and services") + "\n")
	case len(m.shown) == 0:
		b.WriteString(dim.Render("no matches") + "\n")
	default:
		rows := m.rows()
		visible := max(m.height-listChrome, 1)
		cursorRow := 0
		for i, r := range rows {
			if r.idx == m.cursor {
				cursorRow = i
			}
		}
		start := min(max(cursorRow-visible/2, 0), max(len(rows)-visible, 0))
		end := min(start+visible, len(rows))
		for _, r := range rows[start:end] {
			b.WriteString(r.text + "\n")
		}
	}
	b.WriteString(m.footer())
	return b.String()
}

func (m *Model) detailView() string {
	it, ok := m.selected()
	if !ok {
		return dim.Render("nothing selected") + "\n" + m.footer()
	}
	lines := m.detailLines(it)
	rows := max(m.height-detailChrome, 1)
	m.scroll = min(max(m.scroll, 0), max(len(lines)-rows, 0))
	end := min(m.scroll+rows, len(lines))

	var b strings.Builder
	b.WriteString(bold.Render(kindName(it.Kind)+" "+m.label(it)) + "\n\n")
	for _, l := range lines[m.scroll:end] {
		b.WriteString(l + "\n")
	}
	b.WriteString(m.footer())
	return b.String()
}

func (m *Model) helpView() string {
	var b strings.Builder
	b.WriteString(bold.Render("keys") + "\n\n")
	for _, l := range []string{
		"type             search (roles match name or title)",
		"up down pgup pgdn  move",
		"enter            details",
		"ctrl+r           roles granting the selected permission",
		"ctrl+p           permissions in the selected role",
		"ctrl+s           permissions in the selected service",
		"esc              back (or clear the search)",
		"backspace        delete, or back when the search is empty",
		"ctrl+o, home     reset to the start",
		"?                this help",
		"q, ctrl+c        quit (q only when the search is empty)",
	} {
		b.WriteString("  " + l + "\n")
	}
	b.WriteString("\n" + dim.Render("? or esc close"))
	return b.String()
}

// footer is the status line and the hint bar, always the last two lines on screen.
func (m *Model) footer() string {
	hint := "↑↓ move  enter details  esc back  ? help  q quit"
	if m.detail {
		hint = "↑↓ scroll  esc back  ? help"
	}
	return m.status + "\n" + dim.Render(hint)
}

func (m *Model) detailLines(it index.Item) []string {
	switch it.Kind {
	case index.KindRole:
		r, ok := m.idx.Role(it.Name)
		if !ok {
			return []string{"unknown role"}
		}
		perms := m.idx.PermissionsIn(it.Name)
		return append([]string{
			"title:       " + r.Title,
			"stage:       " + r.Stage,
			"description: " + r.Description,
			"",
			fmt.Sprintf("%d permissions:", len(r.IncludedPermissions)),
		}, names(perms)...)
	case index.KindPermission:
		roles := m.idx.RolesGranting(it.Name)
		return append([]string{
			"service: " + index.Service(it.Name),
			"",
			fmt.Sprintf("granted by %d roles:", len(roles)),
		}, names(roles)...)
	default:
		return []string{
			fmt.Sprintf("%d permissions", len(m.idx.PermissionsOf(it.Name))),
			fmt.Sprintf("%d roles", len(m.idx.RolesOf(it.Name))),
		}
	}
}

func names(items []index.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = "  " + it.Name
	}
	return out
}

func kindName(k index.Kind) string {
	switch k {
	case index.KindRole:
		return "role"
	case index.KindPermission:
		return "permission"
	default:
		return "service"
	}
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
