package aws

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	"github.com/laluowen/permidex/v2/internal/cache"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

// CacheVersion is the format of the AWS cache file. Bump it, and update the AWS schema, on any change.
const CacheVersion = 1

// CacheService is one service in the cache: its prefix, its human-readable name, its actions as full IDs,
// and the document's modified timestamp, which a delta refresh compares against the index.
type CacheService struct {
	Name        string   `json:"name"`
	ServiceName string   `json:"serviceName"`
	Actions     []string `json:"actions"`
	Modified    int64    `json:"modified"`
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

// fetchDocuments returns every service document, in the index's order. A service whose modified timestamp
// matches the previous cache is rebuilt from that cache with no request. The rest are fetched, several at a
// time.
func fetchDocuments(ctx context.Context, client Client, entries []IndexEntry, prev map[string]CacheService, progress io.Writer) ([]Document, error) {
	docs := make([]Document, len(entries))
	var fetch []int
	for i, e := range entries {
		if p, ok := prev[e.Service]; ok && p.Modified == e.Modified && e.Modified != 0 {
			docs[i] = documentFromCache(p)
			continue
		}
		fetch = append(fetch, i)
	}
	if progress != nil && len(fetch) < len(entries) {
		_, _ = fmt.Fprintf(progress, "services: %d unchanged, %d to fetch\n", len(entries)-len(fetch), len(fetch))
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(serviceWorkers)
	var done atomic.Int64
	for _, i := range fetch {
		g.Go(func() error {
			doc, err := client.Service(gctx, entries[i].URL)
			if err != nil {
				return err
			}
			docs[i] = doc
			if n := done.Add(1); n%50 == 0 || int(n) == len(fetch) {
				_, _ = fmt.Fprintf(progress, "services fetched: %d/%d\n", n, len(fetch))
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return docs, nil
}

// documentFromCache rebuilds the part of a service document that the catalogue needs, from the cache.
func documentFromCache(c CacheService) Document {
	d := Document{Name: c.Name, ServiceName: c.ServiceName}
	prefix := c.Name + ":"
	for _, a := range c.Actions {
		d.Actions = append(d.Actions, Action{Name: strings.TrimPrefix(a, prefix)})
	}
	return d
}

// Refresh fetches the service reference and every AWS-managed policy, and saves the cache at path. Progress
// lines go to progress. It returns the number of services and policies written.
func Refresh(ctx context.Context, path string, progress io.Writer) (int, int, error) {
	// A previous cache, if there is a usable one, is the base for the delta. Anything unreadable or in
	// another format version means a full refresh.
	prev, err := Load(path)
	if err != nil {
		prev = Snapshot{}
	}
	prevServices := make(map[string]CacheService, len(prev.Services))
	for _, s := range prev.Services {
		prevServices[s.Name] = s
	}
	prevPolicies := make(map[string]Policy, len(prev.Policies))
	for _, p := range prev.Policies {
		prevPolicies[p.Name] = p
	}

	client := Client{Limiter: rate.NewLimiter(referenceRate, referenceRate)}
	entries, err := client.Index(ctx)
	if err != nil {
		return 0, 0, err
	}
	docs, err := fetchDocuments(ctx, client, entries, prevServices, progress)
	if err != nil {
		return 0, 0, err
	}

	api, err := NewPolicyAPI(ctx)
	if err != nil {
		return 0, 0, err
	}
	var reused atomic.Int64
	policies, err := Policies(ctx, api, NewCatalogue(docs), prevPolicies, func(done int, wasReused bool) {
		if wasReused {
			reused.Add(1)
		}
		if done%100 == 0 {
			_, _ = fmt.Fprintf(progress, "policies: %d (%d reused)\n", done, reused.Load())
		}
	})
	if err != nil {
		return 0, 0, err
	}
	_, _ = fmt.Fprintf(progress, "policies: %d (%d reused)\n", len(policies), reused.Load())

	snap := Snapshot{Version: CacheVersion, FetchedAt: time.Now().UTC(), Policies: policies}
	for i, d := range docs {
		svc := CacheService{Name: d.Name, ServiceName: d.ServiceName, Modified: entries[i].Modified}
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
