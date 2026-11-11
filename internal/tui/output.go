package tui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/lalu-uk/permidex/internal/index"
)

// Selection returns the picked items as YAML, or "" unless the session ended with ctrl+q.
func (m *Model) Selection() string {
	if !m.emit {
		return ""
	}
	var roles, services, perms []string
	for it := range m.picked {
		switch it.Kind {
		case index.KindRole:
			line := "  - " + it.Name
			if t := m.title(it); t != "" {
				line += " # " + t
			}
			roles = append(roles, line)
		case index.KindService:
			services = append(services, "  - "+it.Name)
		default:
			perms = append(perms, "  - "+it.Name)
		}
	}

	var b strings.Builder
	for _, sec := range []struct {
		key   string
		lines []string
	}{{"roles", roles}, {"services", services}, {"permissions", perms}} {
		if len(sec.lines) == 0 {
			continue
		}
		slices.Sort(sec.lines)
		fmt.Fprintf(&b, "%s:\n%s\n", sec.key, strings.Join(sec.lines, "\n"))
	}
	return b.String()
}
