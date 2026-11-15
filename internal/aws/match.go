package aws

import "strings"

// Catalogue is every known action, grouped by service prefix, so a pattern only scans the actions of the
// service it names.
type Catalogue struct {
	byService map[string][]action
	services  []string
}

// action is a known action: its full ID, such as s3:GetObject, and its lowercased name for matching.
type action struct {
	id   string
	name string
}

// NewCatalogue builds the catalogue from service reference documents.
func NewCatalogue(docs []Document) Catalogue {
	c := Catalogue{byService: make(map[string][]action)}
	for _, d := range docs {
		svc := strings.ToLower(d.Name)
		if _, seen := c.byService[svc]; !seen {
			c.services = append(c.services, svc)
		}
		for _, a := range d.Actions {
			c.byService[svc] = append(c.byService[svc], action{
				id:   d.Name + ":" + a.Name,
				name: strings.ToLower(a.Name),
			})
		}
	}
	return c
}

// Expand returns the known actions a policy pattern covers. A pattern with no wildcard is returned as it
// is, whether or not the catalogue knows it, so nothing a policy names is dropped. Matching is
// case-insensitive, as IAM is.
func (c Catalogue) Expand(pattern string) []string {
	if !strings.ContainsAny(pattern, "*?") {
		if id, ok := c.lookup(pattern); ok {
			return []string{id}
		}
		return []string{pattern}
	}

	svc, name, found := strings.Cut(pattern, ":")
	if !found {
		name, svc = svc, "*"
	}
	lowName := strings.ToLower(name)

	var out []string
	for _, s := range c.serviceCandidates(svc) {
		for _, a := range c.byService[s] {
			if glob(lowName, a.name) {
				out = append(out, a.id)
			}
		}
	}
	if len(out) == 0 {
		return []string{pattern}
	}
	return out
}

// lookup finds an exact, case-insensitive match for an action ID.
func (c Catalogue) lookup(id string) (string, bool) {
	svc, name, ok := strings.Cut(id, ":")
	if !ok {
		return "", false
	}
	for _, a := range c.byService[strings.ToLower(svc)] {
		if a.name == strings.ToLower(name) {
			return a.id, true
		}
	}
	return "", false
}

// serviceCandidates returns the services to scan: the named one, or all of them when the service part
// is a wildcard.
func (c Catalogue) serviceCandidates(svc string) []string {
	if !strings.ContainsAny(svc, "*?") {
		return []string{strings.ToLower(svc)}
	}
	return c.services
}

// glob reports whether s matches pattern, where * matches any run of characters and ? matches one.
// Both arguments must already be lowercase.
func glob(pattern, s string) bool {
	p, n := 0, 0
	starP, starN := -1, 0
	for n < len(s) {
		switch {
		case p < len(pattern) && (pattern[p] == '?' || pattern[p] == s[n]):
			p++
			n++
		case p < len(pattern) && pattern[p] == '*':
			starP, starN = p, n
			p++
		case starP >= 0:
			p = starP + 1
			starN++
			n = starN
		default:
			return false
		}
	}
	for p < len(pattern) && pattern[p] == '*' {
		p++
	}
	return p == len(pattern)
}
