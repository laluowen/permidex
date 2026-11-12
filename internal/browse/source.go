// Package browse is the Bubble Tea browser for any catalogue: a search box, grouped selectable lists,
// and drill-down views with a back stack. Providers implement Source; browse knows nothing else about them.
package browse

import "time"

// Item is one row: a kind, as named by its Source, and an ID.
type Item struct {
	Kind string
	Name string
}

// Section is one kind, shown as its own group. Sections are listed in display order.
type Section struct {
	Kind  string // the Item.Kind this section groups
	Title string // header text, such as "roles"
	Key   string // YAML key in the selection output, such as "roles"
}

// View is one screen: what it lists and the details shown above the list.
type View struct {
	Crumb       string // breadcrumb text for this view
	Placeholder string // search placeholder for this view
	Info        []string
	Items       []Item
	Home        bool // the start screen: nothing is listed until a search is typed
}

// Source is one catalogue to browse.
type Source interface {
	// Name is shown in the header, such as "gcp".
	Name() string
	// FetchedAt is when the catalogue was built.
	FetchedAt() time.Time
	// Sections lists every kind, in display order.
	Sections() []Section
	// Root is the start screen.
	Root() View
	// Open is the view for an item the user opened.
	Open(it Item) View
	// Title is searchable human text for an item, or "" if it has none.
	Title(it Item) string
	// Label is how an item is shown in a row.
	Label(it Item) string
}
