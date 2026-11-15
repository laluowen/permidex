package aws

import (
	"slices"
	"testing"
)

const (
	s3GetObject          = "s3:GetObject"
	s3GetBucketPolicy    = "s3:GetBucketPolicy"
	ec2DescribeInstances = "ec2:DescribeInstances"
	ec2DescribeVolumes   = "ec2:DescribeVolumes"
	getObjectLower       = "getobject"
)

func sampleCatalogue() Catalogue {
	return NewCatalogue([]Document{
		{Name: "s3", Actions: []Action{{Name: "GetObject"}, {Name: "GetBucketPolicy"}, {Name: "ListBucket"}, {Name: "PutObject"}}},
		{Name: "ec2", Actions: []Action{{Name: "DescribeInstances"}, {Name: "DescribeVolumes"}, {Name: "RunInstances"}}},
	})
}

func TestGlob(t *testing.T) {
	tests := []struct {
		pattern, s string
		want       bool
	}{
		{getObjectLower, getObjectLower, true},
		{"get*", getObjectLower, true},
		{"get*", "putobject", false},
		{"*", "anything", true},
		{"*object", getObjectLower, true},
		{"g?tobject", getObjectLower, true},
		{"g?tobject", "gettobject", false},
		{"*a*b*", "xaxbx", true},
		{"a*b", "a", false},
		{"", "", true},
		{"", "x", false},
	}
	for _, tc := range tests {
		if got := glob(tc.pattern, tc.s); got != tc.want {
			t.Errorf("glob(%q, %q) = %v, want %v", tc.pattern, tc.s, got, tc.want)
		}
	}
}

func TestExpand(t *testing.T) {
	c := sampleCatalogue()
	tests := []struct {
		pattern string
		want    []string
	}{
		{s3GetObject, []string{s3GetObject}},
		{"s3:getobject", []string{s3GetObject}},
		{s3GetWildcard, []string{s3GetObject, s3GetBucketPolicy}},
		{"ec2:Describe*", []string{ec2DescribeInstances, ec2DescribeVolumes}},
		{"s3:*", []string{s3GetObject, s3GetBucketPolicy, testS3List, "s3:PutObject"}},
		{"*", []string{s3GetObject, s3GetBucketPolicy, testS3List, "s3:PutObject", ec2DescribeInstances, ec2DescribeVolumes, "ec2:RunInstances"}},
		{"*:Describe*", []string{ec2DescribeInstances, ec2DescribeVolumes}},
		{"lambda:InvokeFunction", []string{"lambda:InvokeFunction"}},
		{"lambda:Invoke*", []string{"lambda:Invoke*"}},
	}
	for _, tc := range tests {
		if got := c.Expand(tc.pattern); !slices.Equal(got, tc.want) {
			t.Errorf("Expand(%q) = %v, want %v", tc.pattern, got, tc.want)
		}
	}
}
