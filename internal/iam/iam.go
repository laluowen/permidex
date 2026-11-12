// Package iam fetches the predefined GCP IAM roles and their permissions.
package iam

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// DefaultURL is the roles.list endpoint.
const DefaultURL = "https://iam.googleapis.com/v1/roles"

const (
	scope      = "https://www.googleapis.com/auth/cloud-platform"
	pageSize   = 1000
	maxErrBody = 1 << 10
	// requestTimeout bounds each roles.list page request, so a stalled connection cannot hang refresh.
	requestTimeout = 30 * time.Second
)

// Role is a predefined role. IncludedPermissions is only set with view=FULL.
type Role struct {
	Name                string   `json:"name"`
	Title               string   `json:"title,omitempty"`
	Description         string   `json:"description,omitempty"`
	Stage               string   `json:"stage,omitempty"`
	IncludedPermissions []string `json:"includedPermissions,omitempty"`
}

// Doer is the part of *http.Client that Client uses, so tests can fake it.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Client lists roles. HTTP must send authorized requests (see NewADCClient).
// URL is the roles.list endpoint; empty means DefaultURL. OnPage, if set, is called after each page
// with the page count and the roles fetched so far.
type Client struct {
	HTTP   Doer
	URL    string
	OnPage func(pages, roles int)
}

// NewADCClient returns an *http.Client that authorizes with Application Default Credentials.
func NewADCClient(ctx context.Context) (*http.Client, error) {
	creds, err := google.FindDefaultCredentials(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("application default credentials: %w (run `gcloud auth application-default login`)", err)
	}
	hc := oauth2.NewClient(ctx, creds.TokenSource)
	hc.Timeout = requestTimeout
	return hc, nil
}

// Roles returns every predefined role with its permissions, following all pages.
func (c Client) Roles(ctx context.Context) ([]Role, error) {
	base := c.URL
	if base == "" {
		base = DefaultURL
	}
	var roles []Role
	var token string
	seen := map[string]bool{}
	for pages := 1; ; pages++ {
		page, err := c.page(ctx, base, token)
		if err != nil {
			return nil, err
		}
		roles = append(roles, page.Roles...)
		if c.OnPage != nil {
			c.OnPage(pages, len(roles))
		}
		if page.NextPageToken == "" {
			return roles, nil
		}
		if seen[page.NextPageToken] {
			return nil, fmt.Errorf("roles.list: page token %q repeated", page.NextPageToken)
		}
		seen[page.NextPageToken] = true
		token = page.NextPageToken
	}
}

type listResponse struct {
	Roles         []Role `json:"roles"`
	NextPageToken string `json:"nextPageToken"`
}

func (c Client) page(ctx context.Context, base, token string) (listResponse, error) {
	u, err := url.Parse(base)
	if err != nil {
		return listResponse{}, fmt.Errorf("roles URL: %w", err)
	}
	q := u.Query()
	q.Set("view", "FULL")
	q.Set("pageSize", strconv.Itoa(pageSize))
	if token != "" {
		q.Set("pageToken", token)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return listResponse{}, fmt.Errorf("roles.list request: %w", err)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return listResponse{}, fmt.Errorf("roles.list: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrBody))
		return listResponse{}, fmt.Errorf("roles.list: %s: %s", resp.Status, string(bytes.TrimSpace(body)))
	}
	var out listResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return listResponse{}, fmt.Errorf("roles.list: decode: %w", err)
	}
	return out, nil
}
