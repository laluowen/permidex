package main

import "testing"

const flagRefresh = "--refresh"

func TestWantsCacheDir(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{[]string{"--cache-dir"}, true},
		{[]string{"-c"}, true},
		{[]string{prov, "-c"}, true},
		{[]string{"-c", prov, flagRefresh}, true},
		{[]string{prov, flagRefresh}, false},
		{[]string{prov}, false},
		{nil, false},
	}
	for _, tc := range tests {
		if got := wantsCacheDir(tc.args); got != tc.want {
			t.Errorf("wantsCacheDir(%v) = %v, want %v", tc.args, got, tc.want)
		}
	}
}
