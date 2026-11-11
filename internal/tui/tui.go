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
	keyEnter        = "enter"
	keyTab          = "tab"
	keyCtrlC        = "ctrl+c"
	keyCtrlQ        = "ctrl+q"
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
	width   int
	help    bool
	picked  map[index.Item]bool
	emit    bool
}

// New returns a model on the blank start screen. fetched is the cache's fetch time, shown in the header.
func New(idx *index.Index, fetched time.Time) *Model {
	in := textinput.New()
	in.Prompt = "/ "
	in.Focus()
	m := &Model{
		idx:     idx,
		fetched: fetched,
		picked:  map[index.Item]bool{},
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
		m.width = msg.Width
		return m, nil
	case tea.KeyPressMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m *Model) key(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := k.String()
	if s == keyCtrlC {
		return m, tea.Quit
	}
	if s == keyCtrlQ && !m.help {
		m.emit = true
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
	case keyEsc:
		if m.input.Value() != "" {
			m.setFilter("")
		} else {
			m.pop()
		}
	case keyTab:
		if it, ok := m.selected(); ok {
			if m.picked[it] {
				delete(m.picked, it)
			} else {
				m.picked[it] = true
			}
		}
	case "ctrl+r":
		m.jump(index.KindRole)
	case "ctrl+s":
		m.jump(index.KindService)
	case "ctrl+p":
		m.jump(index.KindPermission)
	case keyEnter:
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

// jump moves the cursor to the first item of kind in the current list.
func (m *Model) jump(kind index.Kind) {
	for i, it := range m.shown {
		if it.Kind == kind {
			m.cur.cursor = i
			return
		}
	}
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
		v.items = append(m.idx.RolesGranting(it.Name), index.Item{Kind: index.KindService, Name: svc})
		v.placeholder = "search service and roles"
	default:
		perms := m.idx.PermissionsOf(it.Name)
		roles := m.idx.RolesOf(it.Name)
		v.info = []string{
			fmt.Sprintf("%d permissions", len(perms)),
			fmt.Sprintf("%d roles", len(roles)),
		}
		v.items = append(roles, perms...)
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

// Colours are ANSI palette indices, so they follow the terminal's theme.
var (
	bold        = lipgloss.NewStyle().Bold(true)
	dim         = lipgloss.NewStyle().Faint(true)
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	sectionSty  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	keyStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
	pickedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	plain       = lipgloss.NewStyle()
)

// keyHint is one key and what it does, for the hint bar and the help screen.
type keyHint struct {
	key, desc string
}

// hintBar renders key hints as "key desc", with the keys in their own colour.
func hintBar(hints []keyHint) string {
	parts := make([]string, len(hints))
	for i, h := range hints {
		parts[i] = keyStyle.Render(h.key) + " " + dim.Render(h.desc)
	}
	return strings.Join(parts, "  ")
}

var (
	listHints = []keyHint{
		{"↑↓", "move"},
		{keyEnter, "open"},
		{"esc", "back"},
		{keyTab, "select"},
		{"?", "help"},
		{keyCtrlQ, "done"},
		{keyCtrlC, "abort"},
	}
	helpHints = []keyHint{
		{"type", "search (roles match name or title)"},
		{"↑↓ pgup pgdn", "move"},
		{keyEnter, "open the selected item"},
		{"esc", "back (esc clears the search first)"},
		{"backspace", "delete"},
		{keyTab, "select or deselect the item"},
		{"ctrl+r ctrl+s ctrl+p", "jump to roles, services, permissions"},
		{keyCtrlQ, "quit and print the selection as YAML"},
		{"ctrl+o home", "reset to the start screen"},
		{"?", "this help"},
		{keyCtrlC, "abort (print nothing)"},
	}
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
			out = append(out, row{text: sectionSty.Render(fmt.Sprintf("%s (%d)", kindTitle(it.Kind), m.counts[it.Kind])), idx: -1})
		}
		mark, rowStyle := "[ ] ", plain
		if m.picked[it] {
			mark, rowStyle = "[x] ", pickedStyle
		}
		text := rowStyle.Render("  " + mark + m.label(it))
		if i == m.cur.cursor {
			text = rowStyle.Reverse(true).Render("> " + mark + m.label(it))
		}
		out = append(out, row{text: text, idx: i})
	}
	return out
}

// breadcrumb is the path from the start screen to the current view. When it is wider than the
// terminal it drops the oldest crumbs, so the current view always shows.
func (m *Model) breadcrumb() string {
	parts := make([]string, 0, len(m.stack)+1)
	for _, v := range m.stack {
		parts = append(parts, v.crumb)
	}
	parts = append(parts, m.cur.crumb)
	width := m.width
	if width <= 0 {
		width = 80
	}
	prefix := ""
	for len(parts) > 1 && lipgloss.Width(prefix+strings.Join(parts, " › ")) > width {
		parts = parts[1:]
		prefix = "… › "
	}
	return prefix + strings.Join(parts, " › ")
}

func (m *Model) listView() string {
	var top []string
	top = append(top, titleStyle.Render("permidex")+dim.Render(fmt.Sprintf("  data from %s  selected %d", m.fetched.Format("2006-01-02"), len(m.picked))))
	top = append(top, bold.Render(m.breadcrumb()))
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
	return strings.Join(append(append(top, body...), hintBar(listHints)), "\n")
}

func (m *Model) helpView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("keys") + "\n\n")
	width := 0
	for _, h := range helpHints {
		width = max(width, lipgloss.Width(h.key))
	}
	for _, h := range helpHints {
		pad := strings.Repeat(" ", width-lipgloss.Width(h.key))
		b.WriteString("  " + keyStyle.Render(h.key) + pad + "  " + h.desc + "\n")
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
