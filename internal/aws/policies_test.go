package aws

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"slices"
	"testing"
)

const (
	arnRO   = "arn:ro"
	arnView = "arn:view"
	arnA    = "arn:a"
)

// fakeIAM serves pages of policies, descriptions and documents from maps. Every field is optional.
type fakeIAM struct {
	pages   [][]PolicyRef
	desc    map[string]string
	docs    map[string]string
	failDoc bool
	markers []string
}

func (f *fakeIAM) ListPolicies(_ context.Context, marker string) ([]PolicyRef, string, error) {
	f.markers = append(f.markers, marker)
	idx := len(f.markers) - 1
	next := ""
	if idx+1 < len(f.pages) {
		next = fmt.Sprintf("m%d", idx+1)
	}
	return f.pages[idx], next, nil
}

func (f *fakeIAM) Description(_ context.Context, arn string) (string, error) {
	return f.desc[arn], nil
}

func (f *fakeIAM) Document(_ context.Context, arn, _ string) (string, error) {
	if f.failDoc {
		return "", errors.New("access denied")
	}
	return f.docs[arn], nil
}

// encode URL-encodes a policy document the way IAM returns it.
func encode(doc string) string {
	return url.QueryEscape(doc)
}

func TestPoliciesPaginatesAndReadsEachPolicy(t *testing.T) {
	api := &fakeIAM{
		pages: [][]PolicyRef{
			{{Name: policyRO, ARN: arnRO, DefaultVersionID: "v2"}},
			{{Name: "ViewOnlyAccess", ARN: arnView, DefaultVersionID: "v1"}},
		},
		desc: map[string]string{arnRO: readOnlyDesc, arnView: "view only"},
		docs: map[string]string{
			arnRO:   encode(`{"Statement":[{"Effect":"Allow","Action":["s3:Get*","s3:List*"]}]}`),
			arnView: encode(`{"Statement":{"Effect":"Allow","Action":"ec2:Describe*"}}`),
		},
	}
	got, err := Policies(t.Context(), api, sampleCatalogue(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(api.markers, []string{"", "m1"}) {
		t.Errorf("list markers = %q, want the first page then the marker", api.markers)
	}
	if len(got) != 2 || got[0].Description != readOnlyDesc || got[0].Version != "v2" {
		t.Fatalf("policies = %+v", got)
	}
	if !slices.Equal(got[0].Actions, []string{s3GetWildcard, "s3:List*"}) {
		t.Errorf("list-form actions = %v", got[0].Actions)
	}
	if !slices.Equal(got[1].Actions, []string{"ec2:Describe*"}) {
		t.Errorf("single statement and string action = %v", got[1].Actions)
	}
}

func TestOnlyAllowStatementsCount(t *testing.T) {
	api := &fakeIAM{
		pages: [][]PolicyRef{{{Name: "Mixed", ARN: "arn:mixed", DefaultVersionID: "v1"}}},
		docs: map[string]string{"arn:mixed": encode(`{"Statement":[
			{"Effect":"Deny","Action":"iam:*"},
			{"Effect":"Allow","Action":"s3:GetObject"}
		]}`)},
	}
	got, err := Policies(t.Context(), api, sampleCatalogue(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got[0].Actions, []string{s3GetObject}) {
		t.Errorf("actions = %v, want only the Allow statement", got[0].Actions)
	}
}

func TestNotActionIsFlagged(t *testing.T) {
	api := &fakeIAM{
		pages: [][]PolicyRef{{{Name: "Not", ARN: "arn:not", DefaultVersionID: "v1"}}},
		docs:  map[string]string{"arn:not": encode(`{"Statement":[{"Effect":"Allow","NotAction":"iam:*"}]}`)},
	}
	got, err := Policies(t.Context(), api, sampleCatalogue(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got[0].HasNotAction || len(got[0].Actions) != 0 {
		t.Errorf("NotAction policy = %+v, want flagged with no actions", got[0])
	}
}

func TestProgressAndErrors(t *testing.T) {
	api := &fakeIAM{
		pages: [][]PolicyRef{{{Name: "A", ARN: arnA, DefaultVersionID: "v1"}}},
		docs:  map[string]string{arnA: encode(`{"Statement":[]}`)},
	}
	var seen []int
	if _, err := Policies(t.Context(), api, sampleCatalogue(), func(n int) { seen = append(seen, n) }); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(seen, []int{1}) {
		t.Errorf("progress = %v", seen)
	}

	failing := &fakeIAM{
		pages:   [][]PolicyRef{{{Name: "A", ARN: arnA, DefaultVersionID: "v1"}}},
		failDoc: true,
	}
	if _, err := Policies(t.Context(), failing, sampleCatalogue(), nil); err == nil {
		t.Error("a failed document read should be an error")
	}
}

func TestGrantsExpandPatterns(t *testing.T) {
	api := &fakeIAM{
		pages: [][]PolicyRef{{{Name: "Ro", ARN: arnRO, DefaultVersionID: "v1"}}},
		docs:  map[string]string{arnRO: encode(`{"Statement":[{"Effect":"Allow","Action":["s3:Get*","ec2:DescribeVolumes"]}]}`)},
	}
	got, err := Policies(t.Context(), api, sampleCatalogue(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ec2:DescribeVolumes", s3GetBucketPolicy, s3GetObject}
	if !slices.Equal(got[0].Grants, want) {
		t.Errorf("grants = %v, want %v", got[0].Grants, want)
	}
}
