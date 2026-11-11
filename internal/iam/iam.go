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

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// DefaultURL is the roles.list endpoint.
const DefaultURL = "https://iam.googleapis.com/v1/roles"

const (
	scope      = "https://www.googleapis.com/auth/cloud-platform"
	pageSize   = 1000
	maxErrBody = 1 << 10
)

// Role is a predefined role. IncludedPermissions is only set with view=FULL.
type Role struct {
	Name                string   `json:"name"`
	Title               string   `json:"title"`
	Description         string   `json:"description"`
	Stage               string   `json:"stage"`
	IncludedPermissions []string `json:"includedPermissions"`
}

// Doer is the part of *http.Client that Client uses, so tests can fake it.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Client lists roles. HTTP must send authorized requests (see NewADCClient).
// URL is the roles.list endpoint; empty means DefaultURL.
type Client struct {
	HTTP Doer
	URL  string
}

// NewADCClient returns an *http.Client that authorizes with Application Default Credentials.
func NewADCClient(ctx context.Context) (*http.Client, error) {
	creds, err := google.FindDefaultCredentials(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("application default credentials: %w (run `gcloud auth application-default login`)", err)
	}
	return oauth2.NewClient(ctx, creds.TokenSource), nil
}

// Roles returns every predefined role with its permissions, following all pages.
func (c Client) Roles(ctx context.Context) ([]Role, error) {
	base := c.URL
	if base == "" {
		base = DefaultURL
	}
	var roles []Role
	var token string
	for {
		page, err := c.page(ctx, base, token)
		if err != nil {
			return nil, err
		}
		roles = append(roles, page.Roles...)
		if page.NextPageToken == "" {
			return roles, nil
		}
		if page.NextPageToken == token {
			return nil, fmt.Errorf("roles.list: page token %q repeated", token)
		}
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
