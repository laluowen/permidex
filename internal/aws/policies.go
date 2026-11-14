package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
)

// PolicyRef is a managed policy as ListPolicies returns it.
type PolicyRef struct {
	Name             string
	ARN              string
	DefaultVersionID string
}

// PolicyAPI is the part of IAM that the policy fetch uses. The SDK adapter implements it in production;
// tests use a fake.
type PolicyAPI interface {
	// ListPolicies returns one page of AWS-managed policies, and the marker for the next page ("" when done).
	ListPolicies(ctx context.Context, marker string) ([]PolicyRef, string, error)
	// Description returns the policy's description.
	Description(ctx context.Context, arn string) (string, error)
	// Document returns the URL-encoded JSON document of one policy version.
	Document(ctx context.Context, arn, versionID string) (string, error)
}

// Policy is an AWS-managed policy and the actions its document allows, as written (patterns such as
// s3:Get* are kept as they are).
type Policy struct {
	Name        string   `json:"name"`
	ARN         string   `json:"arn"`
	Description string   `json:"description,omitempty"`
	Version     string   `json:"version"`
	Actions     []string `json:"actions,omitempty"`
	// Grants is every known action the Actions patterns cover, expanded against the catalogue.
	Grants []string `json:"grants,omitempty"`
	// HasNotAction is set when the document uses NotAction, which v1 does not expand.
	HasNotAction bool `json:"hasNotAction,omitempty"`
}

// policyWorkers is how many policies are read at once. The API limiter (see sdkIAM) sets the real rate.
const policyWorkers = 8

// Policies lists every AWS-managed policy and reads its default version, several at a time. progress, if
// set, is called after each policy with the count so far, from any goroutine.
func Policies(ctx context.Context, api PolicyAPI, cat Catalogue, progress func(done int)) ([]Policy, error) {
	var refs []PolicyRef
	marker := ""
	for {
		page, next, err := api.ListPolicies(ctx, marker)
		if err != nil {
			return nil, fmt.Errorf("list policies: %w", err)
		}
		refs = append(refs, page...)
		if next == "" {
			break
		}
		marker = next
	}

	out := make([]Policy, len(refs))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(policyWorkers)
	var done atomic.Int64
	for i, ref := range refs {
		g.Go(func() error {
			p, err := readPolicy(gctx, api, cat, ref)
			if err != nil {
				return err
			}
			out[i] = p
			if progress != nil {
				progress(int(done.Add(1)))
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return out, nil
}

func readPolicy(ctx context.Context, api PolicyAPI, cat Catalogue, ref PolicyRef) (Policy, error) {
	desc, err := api.Description(ctx, ref.ARN)
	if err != nil {
		return Policy{}, fmt.Errorf("get policy %s: %w", ref.ARN, err)
	}
	raw, err := api.Document(ctx, ref.ARN, ref.DefaultVersionID)
	if err != nil {
		return Policy{}, fmt.Errorf("get policy version %s %s: %w", ref.ARN, ref.DefaultVersionID, err)
	}
	actions, notAction, err := allowedActions(raw)
	if err != nil {
		return Policy{}, fmt.Errorf("parse %s: %w", ref.ARN, err)
	}
	var grants []string
	for _, pat := range actions {
		grants = append(grants, cat.Expand(pat)...)
	}
	return Policy{
		Name:         ref.Name,
		ARN:          ref.ARN,
		Description:  desc,
		Version:      ref.DefaultVersionID,
		Actions:      actions,
		Grants:       slices.Compact(slices.Sorted(slices.Values(grants))),
		HasNotAction: notAction,
	}, nil
}

// allowedActions returns the actions of the Allow statements in a URL-encoded policy document.
// Statement and Action may each be a single value or a list, as IAM allows.
func allowedActions(encoded string) ([]string, bool, error) {
	doc, err := url.QueryUnescape(encoded)
	if err != nil {
		return nil, false, err
	}
	var d struct {
		Statement json.RawMessage `json:"Statement"`
	}
	if err := json.Unmarshal([]byte(doc), &d); err != nil {
		return nil, false, err
	}
	statements, err := asList[statement](d.Statement)
	if err != nil {
		return nil, false, fmt.Errorf("statement: %w", err)
	}

	var actions []string
	notAction := false
	for _, s := range statements {
		if s.NotAction != nil {
			notAction = true
		}
		if s.Effect != "Allow" {
			continue
		}
		list, err := asList[string](s.Action)
		if err != nil {
			return nil, false, fmt.Errorf("action: %w", err)
		}
		actions = append(actions, list...)
	}
	return actions, notAction, nil
}

type statement struct {
	Effect    string          `json:"Effect"`
	Action    json.RawMessage `json:"Action"`
	NotAction json.RawMessage `json:"NotAction"`
}

// asList decodes a JSON value that is either one T or an array of them. An absent value is an empty list.
func asList[T any](raw json.RawMessage) ([]T, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var many []T
	if err := json.Unmarshal(raw, &many); err == nil {
		return many, nil
	}
	var one T
	if err := json.Unmarshal(raw, &one); err != nil {
		return nil, err
	}
	return []T{one}, nil
}
