package aws

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/laluowen/permidex/v2/internal/browse"
)

const (
	kindPolicy = "policy"
	kindAction = "action"
	kindSvc    = "service"
)

// Source is the AWS catalogue, for browse. The three sections are policies, actions and services. Each view
// lists the policies first, then the actions, then the services, as the GCP views do.
type Source struct {
	fetched  time.Time
	policies map[string]Policy
	// policyNames is every policy name, in the order the cache lists them.
	policyNames []string
	// actionPolicies maps a full action ID to the names of the policies that grant it.
	actionPolicies map[string][]string
	// svcActions maps a service prefix to its actions, as full IDs.
	svcActions map[string][]string
	svcTitle   map[string]string
	// svcPolicies maps a service prefix to the policies that grant any of its actions.
	svcPolicies map[string][]string
	services    []string
}

// NewSource indexes a loaded snapshot.
func NewSource(s Snapshot) *Source {
	src := &Source{
		fetched:        s.FetchedAt,
		policies:       make(map[string]Policy, len(s.Policies)),
		actionPolicies: map[string][]string{},
		svcActions:     map[string][]string{},
		svcTitle:       map[string]string{},
		svcPolicies:    map[string][]string{},
	}
	for _, p := range s.Policies {
		src.policies[p.Name] = p
		src.policyNames = append(src.policyNames, p.Name)
		for _, a := range p.Grants {
			src.actionPolicies[a] = append(src.actionPolicies[a], p.Name)
			svc := serviceOf(a)
			if !slices.Contains(src.svcPolicies[svc], p.Name) {
				src.svcPolicies[svc] = append(src.svcPolicies[svc], p.Name)
			}
		}
	}
	for _, sv := range s.Services {
		src.services = append(src.services, sv.Name)
		src.svcTitle[sv.Name] = sv.ServiceName
		src.svcActions[sv.Name] = sv.Actions
	}
	return src
}

// serviceOf is the service prefix of an action ID: s3 in s3:GetObject.
func serviceOf(action string) string {
	svc, _, _ := strings.Cut(action, ":")
	return svc
}

// Name implements browse.Source.
func (s *Source) Name() string { return "aws" }

// FetchedAt implements browse.Source.
func (s *Source) FetchedAt() time.Time { return s.fetched }

// Sections implements browse.Source.
func (s *Source) Sections() []browse.Section {
	return []browse.Section{
		{Kind: kindPolicy, Title: "policies", Key: "policies"},
		{Kind: kindAction, Title: "actions", Key: "actions"},
		{Kind: kindSvc, Title: "services", Key: "services"},
	}
}

// Root implements browse.Source.
func (s *Source) Root() browse.View {
	var items []browse.Item
	for _, n := range s.policyNames {
		items = append(items, browse.Item{Kind: kindPolicy, Name: n})
	}
	for _, svc := range s.services {
		for _, a := range s.svcActions[svc] {
			items = append(items, browse.Item{Kind: kindAction, Name: a})
		}
	}
	for _, svc := range s.services {
		items = append(items, browse.Item{Kind: kindSvc, Name: svc})
	}
	return browse.View{
		Crumb:       "search",
		Placeholder: "search policies, actions and services",
		Items:       items,
		Home:        true,
	}
}

// Open implements browse.Source.
func (s *Source) Open(it browse.Item) browse.View {
	v := browse.View{Crumb: s.Label(it)}
	switch it.Kind {
	case kindPolicy:
		p := s.policies[it.Name]
		v.Info = []string{
			"description: " + p.Description,
			fmt.Sprintf("version: %s, %d actions, %d expanded", p.Version, len(p.Actions), len(p.Grants)),
		}
		if p.HasNotAction {
			v.Info = append(v.Info, "uses NotAction: not expanded")
		}
		for _, a := range p.Grants {
			v.Items = append(v.Items, browse.Item{Kind: kindAction, Name: a})
		}
		v.Placeholder = "search actions"
	case kindAction:
		svc := serviceOf(it.Name)
		v.Info = []string{"service: " + svc}
		for _, n := range s.actionPolicies[it.Name] {
			v.Items = append(v.Items, browse.Item{Kind: kindPolicy, Name: n})
		}
		v.Items = append(v.Items, browse.Item{Kind: kindSvc, Name: svc})
		v.Placeholder = "search policies and service"
	default:
		v.Info = []string{
			fmt.Sprintf("%d actions", len(s.svcActions[it.Name])),
			fmt.Sprintf("%d policies grant something here", len(s.svcPolicies[it.Name])),
		}
		for _, n := range s.svcPolicies[it.Name] {
			v.Items = append(v.Items, browse.Item{Kind: kindPolicy, Name: n})
		}
		for _, a := range s.svcActions[it.Name] {
			v.Items = append(v.Items, browse.Item{Kind: kindAction, Name: a})
		}
		v.Placeholder = "search policies and actions"
	}
	return v
}

// Title implements browse.Source: a policy's description, or a service's human-readable name.
func (s *Source) Title(it browse.Item) string {
	switch it.Kind {
	case kindPolicy:
		return s.policies[it.Name].Description
	case kindSvc:
		return s.svcTitle[it.Name]
	}
	return ""
}

// Label implements browse.Source.
func (s *Source) Label(it browse.Item) string {
	if it.Kind == kindSvc && s.svcTitle[it.Name] != "" {
		return s.svcTitle[it.Name] + " [" + it.Name + "]"
	}
	return it.Name
}
