package aws

import (
	"slices"
	"testing"

	"github.com/laluowen/permidex/v2/internal/browse"
)

const (
	policyRO      = "ReadOnlyAccess"
	policyS3Full  = "S3Full"
	readOnlyDesc  = "read only"
	s3GetWildcard = "s3:Get*"
)

const (
	testS3Get  = "s3:GetObject"
	testS3List = "s3:ListBucket"
)

func sampleSnapshot() Snapshot {
	return Snapshot{
		Version: CacheVersion,
		Services: []CacheService{
			{Name: "s3", ServiceName: "Amazon S3", Actions: []string{testS3Get, testS3List}},
		},
		Policies: []Policy{
			{Name: policyRO, Description: "read only", Version: "v1", Actions: []string{"s3:Get*"}, Grants: []string{testS3Get}},
			{Name: policyS3Full, Description: "full s3", Version: "v2", Actions: []string{"s3:*"}, Grants: []string{testS3Get, testS3List}},
		},
	}
}

func names(items []browse.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Name
	}
	return out
}

func TestSectionsAreThePlannedThree(t *testing.T) {
	secs := NewSource(sampleSnapshot()).Sections()
	keys := make([]string, 0, len(secs))
	for _, s := range secs {
		keys = append(keys, s.Key)
	}
	if !slices.Equal(keys, []string{"policies", "actions", "services"}) {
		t.Errorf("sections = %v", keys)
	}
}

func TestPolicyViewListsItsGrants(t *testing.T) {
	v := NewSource(sampleSnapshot()).Open(browse.Item{Kind: kindPolicy, Name: policyS3Full})
	if !slices.Equal(names(v.Items), []string{testS3Get, testS3List}) {
		t.Errorf("policy items = %v", names(v.Items))
	}
}

func TestActionViewListsPoliciesThenService(t *testing.T) {
	v := NewSource(sampleSnapshot()).Open(browse.Item{Kind: kindAction, Name: testS3Get})
	want := []string{policyRO, policyS3Full, "s3"}
	if !slices.Equal(names(v.Items), want) {
		t.Errorf("action items = %v, want %v", names(v.Items), want)
	}
}

func TestServiceViewListsPoliciesThenActions(t *testing.T) {
	v := NewSource(sampleSnapshot()).Open(browse.Item{Kind: kindSvc, Name: "s3"})
	want := []string{policyRO, policyS3Full, testS3Get, testS3List}
	if !slices.Equal(names(v.Items), want) {
		t.Errorf("service items = %v, want %v", names(v.Items), want)
	}
}

func TestLabelsAndTitles(t *testing.T) {
	src := NewSource(sampleSnapshot())
	if got := src.Label(browse.Item{Kind: kindSvc, Name: "s3"}); got != "Amazon S3 [s3]" {
		t.Errorf("service label = %q", got)
	}
	if got := src.Title(browse.Item{Kind: kindPolicy, Name: policyS3Full}); got != "full s3" {
		t.Errorf("policy title = %q", got)
	}
}
