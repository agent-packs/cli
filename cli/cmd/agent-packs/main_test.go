package main

import (
	"reflect"
	"testing"
)

func TestNormalizeInstallArgsMinTrust(t *testing.T) {
	// Regression: --min-trust and its value were missing from the allowlist,
	// causing them to be treated as pack IDs instead of a flag+value pair.
	input := []string{"backend-engineer", "--min-trust", "community", "--dry-run"}
	got := normalizeInstallArgs(input)
	want := []string{"--min-trust", "community", "--dry-run", "backend-engineer"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeInstallArgs(%v) = %v; want %v", input, got, want)
	}
}

func TestNormalizeInstallArgsMinTrustEqualForm(t *testing.T) {
	input := []string{"backend-engineer", "--min-trust=community"}
	got := normalizeInstallArgs(input)
	want := []string{"--min-trust=community", "backend-engineer"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeInstallArgs(%v) = %v; want %v", input, got, want)
	}
}

func TestNormalizeInstallArgsFlagsBeforePositionals(t *testing.T) {
	// Standard flags should always be moved before positional pack IDs.
	input := []string{"my-pack", "--agent", "claude", "--mode", "copy"}
	got := normalizeInstallArgs(input)
	// Flags should come first; positionals last.
	if len(got) == 0 || got[len(got)-1] != "my-pack" {
		t.Fatalf("pack ID should be last positional, got %v", got)
	}
	if got[0] == "my-pack" {
		t.Fatalf("pack ID should not be first when flags are present, got %v", got)
	}
}
