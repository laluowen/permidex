// Package index builds the in-memory lookups behind the TUI's pivots.
package index

import (
	"slices"
	"strings"

	"github.com/lalu-uk/permidex/internal/iam"
)

// Kind is what an Item names.
type Kind int

// Item kinds.
const (
	KindRole Kind = iota
	KindPermission
	KindService
)

// Item is a role, permission or service, identified by its name.
type Item struct {
	Kind Kind
	Name string
}

// Index holds the reverse lookups. Build it once; it is read-only after that.
type Index struct {
	roles          []iam.Role
	byName         map[string]iam.Role
	grantedBy      map[string][]string
	permsByService map[string][]string
	rolesByService map[string][]string
	perms          []string
	services       []string
}

// Service returns the service of a permission: the part before the first dot.
func Service(perm string) string {
	svc, _, _ := strings.Cut(perm, ".")
	return svc
}

// Build indexes roles. Input order does not matter; all results are sorted by name.
func Build(roles []iam.Role) *Index {
	x := &Index{
		roles:          slices.Clone(roles),
		byName:         make(map[string]iam.Role, len(roles)),
		grantedBy:      make(map[string][]string),
		permsByService: make(map[string][]string),
		rolesByService: make(map[string][]string),
	}
	slices.SortFunc(x.roles, func(a, b iam.Role) int { return strings.Compare(a.Name, b.Name) })

	permSeen := make(map[string]bool)
	svcSeen := make(map[string]bool)
	for _, r := range x.roles {
		x.byName[r.Name] = r
		clear(svcSeen)
		clear(permSeen)
		for _, p := range r.IncludedPermissions {
			if permSeen[p] {
				continue
			}
			permSeen[p] = true
			x.grantedBy[p] = append(x.grantedBy[p], r.Name)
			svc := Service(p)
			if !svcSeen[svc] {
				svcSeen[svc] = true
				x.rolesByService[svc] = append(x.rolesByService[svc], r.Name)
			}
		}
	}

	for p := range x.grantedBy {
		x.perms = append(x.perms, p)
		svc := Service(p)
		x.permsByService[svc] = append(x.permsByService[svc], p)
	}
	for s := range x.rolesByService {
		x.services = append(x.services, s)
	}
	slices.Sort(x.perms)
	slices.Sort(x.services)
	for s := range x.permsByService {
		slices.Sort(x.permsByService[s])
	}
	return x
}

// All returns every role, permission and service, in that order. This is the search list.
func (x *Index) All() []Item {
	out := make([]Item, 0, len(x.roles)+len(x.perms)+len(x.services))
	for _, r := range x.roles {
		out = append(out, Item{Kind: KindRole, Name: r.Name})
	}
	for _, p := range x.perms {
		out = append(out, Item{Kind: KindPermission, Name: p})
	}
	for _, s := range x.services {
		out = append(out, Item{Kind: KindService, Name: s})
	}
	return out
}

// Role returns the role with the given full name, such as roles/compute.viewer.
func (x *Index) Role(name string) (iam.Role, bool) {
	r, ok := x.byName[name]
	return r, ok
}

// RolesGranting returns the roles that include perm.
func (x *Index) RolesGranting(perm string) []Item {
	return items(KindRole, x.grantedBy[perm])
}

// PermissionsIn returns the permissions included in role, sorted.
func (x *Index) PermissionsIn(role string) []Item {
	r, ok := x.byName[role]
	if !ok {
		return nil
	}
	perms := slices.Clone(r.IncludedPermissions)
	slices.Sort(perms)
	return items(KindPermission, slices.Compact(perms))
}

// PermissionsOf returns the permissions that belong to service.
func (x *Index) PermissionsOf(service string) []Item {
	return items(KindPermission, x.permsByService[service])
}

// RolesOf returns the roles that include at least one permission of service.
func (x *Index) RolesOf(service string) []Item {
	return items(KindRole, x.rolesByService[service])
}

func items(kind Kind, names []string) []Item {
	out := make([]Item, len(names))
	for i, n := range names {
		out[i] = Item{Kind: kind, Name: n}
	}
	return out
}
