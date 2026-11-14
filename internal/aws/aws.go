// Package aws browses AWS IAM actions, services and managed policies. The service reference is public;
// it lists every service and its actions. Managed policies come from IAM and need credentials.
package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// DefaultIndexURL is the service reference index: one entry per service, with the URL of its document.
const DefaultIndexURL = "https://servicereference.us-east-1.amazonaws.com/"

const maxDoc = 16 << 20

// Retry settings for the public reference. retryBase is a variable so tests can shorten it.
var retryBase = 500 * time.Millisecond

const (
	maxAttempts = 5
	maxBackoff  = 20 * time.Second
)

// statusError is a non-200 response from the reference.
type statusError struct {
	code   int
	status string
	body   string
}

func (e *statusError) Error() string { return e.status + ": " + e.body }

// IndexEntry is one service in the index.
type IndexEntry struct {
	Service     string `json:"service"`
	ServiceName string `json:"serviceName"`
	URL         string `json:"url"`
}

// Action is one IAM action of a service. Name is unprefixed: the full action is Service + ":" + Name.
type Action struct {
	Name        string `json:"Name"`
	Annotations struct {
		Properties struct {
			IsDataPlane            bool `json:"IsDataPlane"`
			IsList                 bool `json:"IsList"`
			IsPermissionManagement bool `json:"IsPermissionManagement"`
			IsTaggingOnly          bool `json:"IsTaggingOnly"`
			IsWrite                bool `json:"IsWrite"`
		} `json:"Properties"`
	} `json:"Annotations"`
}

// Document is one service's reference document.
type Document struct {
	Name        string   `json:"Name"`
	ServiceName string   `json:"ServiceName"`
	Version     string   `json:"Version"`
	Actions     []Action `json:"Actions"`
}

// Doer is the part of *http.Client that Client uses, so tests can fake it.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Client fetches the service reference. HTTP needs no credentials for it. Limiter, if set, caps the request
// rate; nil means no cap.
type Client struct {
	HTTP     Doer
	IndexURL string
	Limiter  *rate.Limiter
}

// Index returns every service in the reference, in the order the index lists them.
func (c Client) Index(ctx context.Context) ([]IndexEntry, error) {
	var out []IndexEntry
	if err := c.getJSON(ctx, c.indexURL(), &out); err != nil {
		return nil, fmt.Errorf("service reference index: %w", err)
	}
	return out, nil
}

// Service fetches one service's document from its index URL.
func (c Client) Service(ctx context.Context, url string) (Document, error) {
	var doc Document
	if err := c.getJSON(ctx, url, &doc); err != nil {
		return Document{}, fmt.Errorf("service reference %s: %w", url, err)
	}
	return doc, nil
}

func (c Client) indexURL() string {
	if c.IndexURL == "" {
		return DefaultIndexURL
	}
	return c.IndexURL
}

// getJSON fetches url and decodes it into v, retrying throttled and transient failures with backoff.
func (c Client) getJSON(ctx context.Context, url string, v any) error {
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err = c.getOnce(ctx, url, v)
		if err == nil || !retryable(err) || attempt == maxAttempts {
			return err
		}
		if serr := sleep(ctx, backoff(attempt)); serr != nil {
			return serr
		}
	}
	return err
}

// retryable reports whether a failure is worth another attempt: throttling, server errors, or a network
// error. A bad document or a client error is not retried.
func retryable(err error) bool {
	var se *statusError
	if errors.As(err, &se) {
		return se.code == http.StatusTooManyRequests || se.code >= 500
	}
	return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

// backoff is the wait before attempt+1: exponential from retryBase, capped, with jitter.
func backoff(attempt int) time.Duration {
	d := retryBase << (attempt - 1)
	if d > maxBackoff {
		d = maxBackoff
	}
	return d/2 + rand.N(d/2+1) //nolint:gosec // retry jitter needs no cryptographic randomness
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (c Client) getOnce(ctx context.Context, url string, v any) error {
	if c.Limiter != nil {
		if err := c.Limiter.Wait(ctx); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
		return &statusError{code: resp.StatusCode, status: resp.Status, body: strings.TrimSpace(string(body))}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDoc+1))
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	if len(data) > maxDoc {
		return fmt.Errorf("document exceeds %d MiB", maxDoc>>20)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}
