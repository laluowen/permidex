package browse

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	defaultHeight = 24
	pageStep      = 10
	keyEsc        = "esc"
	keyEnter      = "enter"
	keyTab        = "tab"
	keyCtrlC      = "ctrl+c"
	keyCtrlQ      = "ctrl+q"
)

// frame is one screen on the stack: its View plus the search and cursor it was left with.
type frame struct {
	View
	filter string
	cursor int
}

// Model is the Bubble Tea model. Build it with New.
type Model struct {
	src      Source
	sections []Section
	titles   map[string]string
	cur      frame
	stack    []frame
	input    textinput.Model
	shown    []Item
	counts   map[string]int
	height   int
	width    int
	help     bool
	picked   map[Item]bool
	emit     bool
}

// New returns a model on src's start screen.
func New(src Source) *Model {
	in := textinput.New()
	in.Prompt = "/ "
	in.Focus()
	m := &Model{
		src:      src,
		sections: src.Sections(),
		titles:   map[string]string{},
		input:    in,
		height:   defaultHeight,
		picked:   map[Item]bool{},
	}
	for _, s := range m.sections {
		m.titles[s.Kind] = s.Title
	}
	m.cur = frame{View: src.Root()}
	m.refresh()
	return m
}

// Selection returns the picked items as YAML, or "" unless the session ended with ctrl+q.
func (m *Model) Selection() string {
	if !m.emit {
		return ""
	}
	var b strings.Builder
	for _, sec := range m.sections {
		var lines []string
		for it := range m.picked {
			if it.Kind != sec.Kind {
				continue
			}
			line := "  - " + it.Name
			if t := m.src.Title(it); t != "" {
				line += " # " + strings.Join(strings.Fields(t), " ")
			}
			lines = append(lines, line)
		}
		if len(lines) == 0 {
			continue
		}
		slices.Sort(lines)
		fmt.Fprintf(&b, "%s:\n%s\n", sec.Key, strings.Join(lines, "\n"))
	}
	return b.String()
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

// jumpKeys jump to the first, second and third section. The same keys apply to every provider.
var jumpKeys = []string{"ctrl+j", "ctrl+k", "ctrl+l"}

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

	for i, sec := range m.sections {
		if i < len(jumpKeys) && s == jumpKeys[i] {
			m.jump(sec.Kind)
			m.refresh()
			return m, nil
		}
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
	case keyEnter:
		m.open()
	case "ctrl+o", "home":
		m.stack = nil
		m.cur = frame{View: m.src.Root()}
		m.setFilter("")
	case "up":
		m.cur.cursor--
	case "down":
		m.cur.cursor++
	case "pgup":
		m.cur.cursor -= pageStep
	case "pgdown":
		m.cur.cursor += pageStep
	default:
		m.typeKey(k)
	}
	m.refresh()
	return m, nil
}

// jump moves the cursor to the first item of kind in the current list.
func (m *Model) jump(kind string) {
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
	m.push(frame{View: m.src.Open(it)})
}

func (m *Model) push(f frame) {
	m.cur.filter = m.input.Value()
	m.stack = append(m.stack, m.cur)
	m.cur = f
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

// refresh recomputes the visible items from the current view and search, and clamps the cursor.
// The start screen shows nothing until something is typed.
func (m *Model) refresh() {
	m.input.Placeholder = m.cur.Placeholder
	m.shown = nil
	m.counts = map[string]int{}
	q := strings.ToLower(m.input.Value())
	if m.cur.Home && q == "" {
		m.cur.cursor = 0
		return
	}
	for _, it := range m.cur.Items {
		if q != "" && !m.matches(it, q) {
			continue
		}
		m.shown = append(m.shown, it)
		m.counts[it.Kind]++
	}
	m.cur.cursor = min(max(m.cur.cursor, 0), max(len(m.shown)-1, 0))
}

// matches reports whether q (already lowercased) appears in the item's name or title.
func (m *Model) matches(it Item, q string) bool {
	if strings.Contains(strings.ToLower(it.Name), q) {
		return true
	}
	return strings.Contains(strings.ToLower(m.src.Title(it)), q)
}

// label is what a row shows for it.
func (m *Model) label(it Item) string {
	return m.src.Label(it)
}

func (m *Model) selected() (Item, bool) {
	if m.cur.cursor < 0 || m.cur.cursor >= len(m.shown) {
		return Item{}, false
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

var listHints = []keyHint{
	{"↑↓", "move"},
	{keyEnter, "open"},
	{"esc", "back"},
	{keyTab, "select"},
	{"?", "help"},
	{keyCtrlQ, "done"},
	{keyCtrlC, "abort"},
}

func (m *Model) helpHints() []keyHint {
	return []keyHint{
		{"type", "search (matches name or title)"},
		{"↑↓ pgup pgdn", "move"},
		{keyEnter, "open the selected item"},
		{"esc", "back (esc clears the search first)"},
		{"backspace", "delete"},
		{keyTab, "select or deselect the item"},
		{strings.Join(jumpKeys[:min(len(m.sections), len(jumpKeys))], " "), "jump to a section, in the order listed"},
		{keyCtrlQ, "quit and print the selection as YAML"},
		{"ctrl+o home", "reset to the start screen"},
		{"?", "this help"},
		{keyCtrlC, "abort (print nothing)"},
	}
}

type row struct {
	text string
	idx  int // index into shown, or -1 for a section header
}

// rows lays out the visible items with a header before each kind.
func (m *Model) rows() []row {
	var out []row
	for i, it := range m.shown {
		if i == 0 || it.Kind != m.shown[i-1].Kind {
			out = append(out, row{text: sectionSty.Render(fmt.Sprintf("%s (%d)", m.titles[it.Kind], m.counts[it.Kind])), idx: -1})
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
	for _, f := range m.stack {
		parts = append(parts, f.Crumb)
	}
	parts = append(parts, m.cur.Crumb)
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
	top = append(top, titleStyle.Render("permidex "+m.src.Name())+dim.Render(fmt.Sprintf("  data from %s  selected %d", m.src.FetchedAt().Format(time.DateOnly), len(m.picked))))
	top = append(top, bold.Render(m.breadcrumb()))
	top = append(top, m.cur.Info...)
	top = append(top, m.input.View(), "")

	var body []string
	switch {
	case len(m.shown) == 0 && m.cur.Home && m.input.Value() == "":
		body = append(body, dim.Render("type to search"))
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
	hints := m.helpHints()
	width := 0
	for _, h := range hints {
		width = max(width, lipgloss.Width(h.key))
	}
	for _, h := range hints {
		pad := strings.Repeat(" ", width-lipgloss.Width(h.key))
		b.WriteString("  " + keyStyle.Render(h.key) + pad + "  " + h.desc + "\n")
	}
	b.WriteString("\n" + dim.Render("? or esc close"))
	return b.String()
}
