package aws

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/laluowen/permidex/v2/internal/cache"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
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

// referenceRate caps requests per second to the public reference. It is a starting guess: tune it against
// real throttling. serviceWorkers is how many service documents are fetched at once.
const (
	referenceRate  = 20
	serviceWorkers = 8
)

// fetchDocuments fetches every service document, several at a time, keeping the index's order.
func fetchDocuments(ctx context.Context, client Client, entries []IndexEntry, progress io.Writer) ([]Document, error) {
	docs := make([]Document, len(entries))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(serviceWorkers)
	var done atomic.Int64
	for i, e := range entries {
		g.Go(func() error {
			doc, err := client.Service(gctx, e.URL)
			if err != nil {
				return err
			}
			docs[i] = doc
			if n := done.Add(1); n%50 == 0 || int(n) == len(entries) {
				_, _ = fmt.Fprintf(progress, "services: %d/%d\n", n, len(entries))
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return docs, nil
}

// Refresh fetches the service reference and every AWS-managed policy, and saves the cache at path. Progress
// lines go to progress. It returns the number of services and policies written.
func Refresh(ctx context.Context, path string, progress io.Writer) (int, int, error) {
	client := Client{Limiter: rate.NewLimiter(referenceRate, referenceRate)}
	entries, err := client.Index(ctx)
	if err != nil {
		return 0, 0, err
	}
	docs, err := fetchDocuments(ctx, client, entries, progress)
	if err != nil {
		return 0, 0, err
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
