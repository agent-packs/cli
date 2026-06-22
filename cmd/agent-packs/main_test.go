package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/agent-packs/cli/internal/agentpacks"
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

func TestPrintSearchResultsDetails(t *testing.T) {
	packs := []agentpacks.Pack{{
		ID:           "frontend-engineer",
		Name:         "Frontend Engineer",
		Stability:    "experimental",
		ReviewStatus: "reviewed",
		LastVerified: "2026-06-16",
		Tools:        []string{"codex", "claude-code"},
		Scope:        []string{"global", "project"},
		Tags:         []string{"frontend"},
	}}
	var out strings.Builder
	printSearchResults(&out, packs, true)
	got := out.String()
	for _, want := range []string{
		"frontend-engineer",
		"experimental",
		"reviewed",
		"2026-06-16",
		"codex,claude-code",
		"global,project",
		"agent-packs install frontend-engineer",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("detailed search output missing %q: %s", want, got)
		}
	}
}
