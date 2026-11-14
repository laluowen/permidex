package main

import (
	"slices"
	"testing"
)

const prov = "gcp"

func TestProviderFirst(t *testing.T) {
	tests := []struct {
		in, want []string
	}{
		{[]string{prov, "-r"}, []string{prov, "-r"}},
		{[]string{"-r", prov}, []string{prov, "-r"}},
		{[]string{flagRefresh, prov}, []string{prov, flagRefresh}},
		{[]string{prov}, []string{prov}},
		{[]string{"-r"}, []string{"-r"}},
		{nil, nil},
	}
	for _, tc := range tests {
		if got := providerFirst(tc.in); !slices.Equal(got, tc.want) {
			t.Errorf("providerFirst(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
