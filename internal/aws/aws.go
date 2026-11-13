// Package aws browses AWS IAM actions, services and managed policies. The service reference is public;
// it lists every service and its actions. Managed policies come from IAM and need credentials.
package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DefaultIndexURL is the service reference index: one entry per service, with the URL of its document.
const DefaultIndexURL = "https://servicereference.us-east-1.amazonaws.com/"

const maxDoc = 16 << 20

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

// Client fetches the service reference. HTTP needs no credentials for it.
type Client struct {
	HTTP     Doer
	IndexURL string
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

func (c Client) getJSON(ctx context.Context, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
		return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxDoc)).Decode(v); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}
