// Package gcp browses GCP predefined IAM roles and their permissions. It fetches and caches them, and
// implements browse.Source over the index.
package gcp

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/laluowen/permidex/v2/internal/browse"
	"github.com/laluowen/permidex/v2/internal/cache"
	"github.com/laluowen/permidex/v2/internal/iam"
	"github.com/laluowen/permidex/v2/internal/index"
	"github.com/laluowen/permidex/v2/schema"
)

// CacheVersion is the format of the GCP cache file. Bump it, and update schema/gcp-roles.schema.json, on any change.
const CacheVersion = 1

const (
	kindRole       = "role"
	kindPermission = "permission"
	kindService    = "service"
)

// Snapshot is the GCP cache file: every predefined role, as written by `permidex gcp --refresh|-r`.
type Snapshot struct {
	Version   int        `json:"version"`
	FetchedAt time.Time  `json:"fetchedAt"`
	Roles     []iam.Role `json:"roles"`
}

// CachePath is where the GCP cache lives.
func CachePath() (string, error) {
	return cache.Path("gcp", "roles.json")
}

// Load reads the GCP cache at path. It returns cache.ErrMissing if there is none, and a clear error if the
// format version does not match.
func Load(path string) (Snapshot, error) {
	var s Snapshot
	if err := cache.ReadJSON(path, &s); err != nil {
		return Snapshot{}, err
	}
	if s.Version != CacheVersion {
		return Snapshot{}, fmt.Errorf("cache %s is format version %d, want %d (run `permidex gcp --refresh|-r`)", path, s.Version, CacheVersion)
	}
	return s, nil
}

// Refresh fetches every predefined role from IAM and saves the cache at path. Progress lines go to progress.
// It returns the number of roles written.
func Refresh(ctx context.Context, path string, progress io.Writer) (int, error) {
	hc, err := iam.NewADCClient(ctx)
	if err != nil {
		return 0, err
	}
	client := iam.Client{
		HTTP: hc,
		OnPage: func(pages, roles int) {
			_, _ = fmt.Fprintf(progress, "fetched page %d: %d roles so far\n", pages, roles)
		},
	}
	roles, err := client.Roles(ctx)
	if err != nil {
		return 0, err
	}
	if err := Save(path, Snapshot{Version: CacheVersion, FetchedAt: time.Now().UTC(), Roles: roles}); err != nil {
		return 0, err
	}
	return len(roles), nil
}

// SchemaFile is the name of the schema written beside the GCP cache.
const SchemaFile = "gcp-roles.schema.json"

// Save writes the snapshot to path, and the matching schema beside it, so the cache directory is
// self-describing.
func Save(path string, snap Snapshot) error {
	snap.Version = CacheVersion
	if err := cache.WriteJSON(path, snap); err != nil {
		return err
	}
	schemaPath := filepath.Join(filepath.Dir(path), SchemaFile)
	if err := cache.WriteFile(schemaPath, schema.GCPRoles); err != nil {
		return fmt.Errorf("cache saved to %s, but writing %s failed: %w", path, schemaPath, err)
	}
	return nil
}

// Source is the GCP catalogue, for browse.
type Source struct {
	idx     *index.Index
	fetched time.Time
}

// NewSource indexes a loaded snapshot.
func NewSource(s Snapshot) *Source {
	return &Source{idx: index.Build(s.Roles), fetched: s.FetchedAt}
}

// Name implements browse.Source.
func (s *Source) Name() string { return "gcp" }

// FetchedAt implements browse.Source.
func (s *Source) FetchedAt() time.Time { return s.fetched }

// Sections implements browse.Source.
func (s *Source) Sections() []browse.Section {
	return []browse.Section{
		{Kind: kindRole, Title: "roles", Key: "roles"},
		{Kind: kindPermission, Title: "permissions", Key: "permissions"},
		{Kind: kindService, Title: "services", Key: "services"},
	}
}

// Root implements browse.Source.
func (s *Source) Root() browse.View {
	return browse.View{
		Crumb:       "search",
		Placeholder: "search roles, permissions and services",
		Items:       fromIndex(s.idx.All()),
		Home:        true,
	}
}

// Open implements browse.Source. Each view lists its roles first, then permissions, then services.
func (s *Source) Open(it browse.Item) browse.View {
	v := browse.View{Crumb: s.Label(it)}
	switch it.Kind {
	case kindRole:
		r, _ := s.idx.Role(it.Name)
		v.Info = []string{
			"title:       " + r.Title,
			"stage:       " + r.Stage,
			"description: " + r.Description,
			fmt.Sprintf("%d permissions", len(r.IncludedPermissions)),
		}
		v.Items = fromIndex(s.idx.PermissionsIn(it.Name))
		v.Placeholder = "search permissions"
	case kindPermission:
		svc := index.Service(it.Name)
		v.Info = []string{"service: " + svc}
		v.Items = append(fromIndex(s.idx.RolesGranting(it.Name)), browse.Item{Kind: kindService, Name: svc})
		v.Placeholder = "search service and roles"
	default:
		perms := s.idx.PermissionsOf(it.Name)
		roles := s.idx.RolesOf(it.Name)
		v.Info = []string{
			fmt.Sprintf("%d permissions", len(perms)),
			fmt.Sprintf("%d roles", len(roles)),
		}
		v.Items = append(fromIndex(roles), fromIndex(perms)...)
		v.Placeholder = "search permissions and roles"
	}
	return v
}

// Title implements browse.Source: a role's human-readable title.
func (s *Source) Title(it browse.Item) string {
	if it.Kind != kindRole {
		return ""
	}
	r, _ := s.idx.Role(it.Name)
	return r.Title
}

// Label implements browse.Source: "Compute Admin [roles/compute.admin]" for roles, the ID otherwise.
func (s *Source) Label(it browse.Item) string {
	if t := s.Title(it); t != "" {
		return t + " [" + it.Name + "]"
	}
	return it.Name
}

func fromIndex(items []index.Item) []browse.Item {
	out := make([]browse.Item, len(items))
	for i, it := range items {
		out[i] = browse.Item{Kind: kindName(it.Kind), Name: it.Name}
	}
	return out
}

func kindName(k index.Kind) string {
	switch k {
	case index.KindRole:
		return kindRole
	case index.KindPermission:
		return kindPermission
	default:
		return kindService
	}
}
