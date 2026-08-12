package project

import "testing"

func TestPreferredToolchainSecurityFloor(t *testing.T) {
	tests := []struct {
		input, wantToolchain, wantBuild string
	}{
		{"1.25", "go1.25.12", "1.25.12"},
		{"1.25.0", "go1.25.12", "1.25.12"},
		{"1.25.4", "go1.25.12", "1.25.12"},
		{"1.25.11", "go1.25.12", "1.25.12"},
		{"1.25.12", "go1.25.12", "1.25.12"},
		{"1.25.13", "go1.25.13", "1.25.13"},
		{"1.26", "", "1.26"},
		{"1.26.5", "go1.26.5", "1.26.5"},
		{"auto", "", "auto"},
	}
	for _, test := range tests {
		if got := PreferredToolchain(test.input); got != test.wantToolchain {
			t.Fatalf("PreferredToolchain(%q)=%q want %q", test.input, got, test.wantToolchain)
		}
		if got := BuildGoVersion(test.input); got != test.wantBuild {
			t.Fatalf("BuildGoVersion(%q)=%q want %q", test.input, got, test.wantBuild)
		}
	}
}
