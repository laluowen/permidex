package aws

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/laluowen/permidex/v2/internal/cache"
)

// CacheVersion is the format of the AWS cache file. Bump it, and update the AWS schema, on any change.
const CacheVersion = 1

// CacheService is one service in the cache: its prefix, its human-readable name, and its actions as full IDs.
type CacheService struct {
	Name        string   `json:"name"`
	ServiceName string   `json:"serviceName"`
	Actions     []string `json:"actions"`
}

// Snapshot is the AWS cache file, written by `permidex aws -r`.
type Snapshot struct {
	Version   int            `json:"version"`
	FetchedAt time.Time      `json:"fetchedAt"`
	Services  []CacheService `json:"services"`
	Policies  []Policy       `json:"policies"`
}

// CachePath is where the AWS cache lives.
func CachePath() (string, error) {
	return cache.Path("aws", "catalogue.json")
}

// Load reads the AWS cache at path. It returns cache.ErrMissing if there is none, and a clear error if the
// format version does not match.
func Load(path string) (Snapshot, error) {
	var s Snapshot
	if err := cache.ReadJSON(path, &s); err != nil {
		return Snapshot{}, err
	}
	if s.Version != CacheVersion {
		return Snapshot{}, fmt.Errorf("cache %s is format version %d, want %d (run `permidex aws -r`)", path, s.Version, CacheVersion)
	}
	return s, nil
}

// Refresh fetches the service reference and every AWS-managed policy, and saves the cache at path. Progress
// lines go to progress. It returns the number of services and policies written.
func Refresh(ctx context.Context, path string, progress io.Writer) (int, int, error) {
	client := Client{}
	entries, err := client.Index(ctx)
	if err != nil {
		return 0, 0, err
	}
	var docs []Document
	for i, e := range entries {
		doc, err := client.Service(ctx, e.URL)
		if err != nil {
			return 0, 0, err
		}
		docs = append(docs, doc)
		if (i+1)%50 == 0 || i+1 == len(entries) {
			_, _ = fmt.Fprintf(progress, "services: %d/%d\n", i+1, len(entries))
		}
	}

	api, err := NewPolicyAPI(ctx)
	if err != nil {
		return 0, 0, err
	}
	policies, err := Policies(ctx, api, NewCatalogue(docs), func(done int) {
		if done%100 == 0 {
			_, _ = fmt.Fprintf(progress, "policies: %d\n", done)
		}
	})
	if err != nil {
		return 0, 0, err
	}

	snap := Snapshot{Version: CacheVersion, FetchedAt: time.Now().UTC(), Policies: policies}
	for _, d := range docs {
		svc := CacheService{Name: d.Name, ServiceName: d.ServiceName}
		for _, a := range d.Actions {
			svc.Actions = append(svc.Actions, d.Name+":"+a.Name)
		}
		snap.Services = append(snap.Services, svc)
	}
	if err := cache.WriteJSON(path, snap); err != nil {
		return 0, 0, err
	}
	return len(snap.Services), len(snap.Policies), nil
}
