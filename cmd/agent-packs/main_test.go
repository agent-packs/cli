package main

import (
	"flag"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/agent-packs/cli/internal/agentpacks"
)

// parseFlags must accept flags after positionals for any registered flag —
// including value flags like --min-trust, which the old hand-maintained
// allowlists once missed (treating the value as a pack ID).
func TestParseFlagsAcceptsFlagsAfterPositionals(t *testing.T) {
	newSet := func() (*flag.FlagSet, *string, *bool) {
		fs := flag.NewFlagSet("install", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		minTrust := fs.String("min-trust", "", "")
		fs.String("agent", "", "")
		dryRun := fs.Bool("dry-run", false, "")
		return fs, minTrust, dryRun
	}

	fs, minTrust, dryRun := newSet()
	if err := parseFlags(fs, []string{"backend-engineer", "--min-trust", "community", "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if *minTrust != "community" || !*dryRun {
		t.Fatalf("flags not parsed: min-trust=%q dry-run=%v", *minTrust, *dryRun)
	}
	if got := fs.Args(); !reflect.DeepEqual(got, []string{"backend-engineer"}) {
		t.Fatalf("positionals = %v; want [backend-engineer]", got)
	}

	fs, minTrust, _ = newSet()
	if err := parseFlags(fs, []string{"backend-engineer", "--min-trust=community"}); err != nil {
		t.Fatal(err)
	}
	if *minTrust != "community" {
		t.Fatalf("equal-form flag not parsed: %q", *minTrust)
	}
}

// A bool flag must not swallow the following positional, and "--" must end
// flag parsing.
func TestParseFlagsBoolAndTerminator(t *testing.T) {
	fs := flag.NewFlagSet("x", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	verbose := fs.Bool("verbose", false, "")
	if err := parseFlags(fs, []string{"--verbose", "pack-a", "--", "--not-a-flag"}); err != nil {
		t.Fatal(err)
	}
	if !*verbose {
		t.Fatal("bool flag not set")
	}
	if got := fs.Args(); !reflect.DeepEqual(got, []string{"pack-a", "--not-a-flag"}) {
		t.Fatalf("positionals = %v", got)
	}
}

func TestPrintSearchResultsDetails(t *testing.T) {
	packs := []agentpacks.Pack{{
		ID:           "frontend-engineer",
		Name:         "Frontend Engineer",
		Stability:    "experimental",
		ReviewStatus: "reviewed",
		Trust:        "community",
		LastVerified: "2026-06-16",
		Tools:        []string{"codex", "claude-code"},
		Scope:        []string{"global", "project"},
		Tags:         []string{"frontend"},
		Compatibility: map[string]agentpacks.CompatibilityEvidence{
			"codex": {Status: "verified", LastVerified: "2026-06-16"},
		},
	}}
	var out strings.Builder
	printSearchResults(&out, packs, true, "codex", "visual regression", true, true)
	got := out.String()
	for _, want := range []string{
		"frontend-engineer",
		"experimental",
		"reviewed",
		"2026-06-16",
		"community",
		"fresh",
		"codex:verified",
		"codex,claude-code",
		"global,project",
		"agent-packs install frontend-engineer",
		"guidance: verified for codex; install with: agent-packs install frontend-engineer --agent codex",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("detailed search output missing %q: %s", want, got)
		}
	}
}

func TestSearchResultsIncludeMatchSnippet(t *testing.T) {
	packs := []agentpacks.Pack{{
		ID:             "frontend-engineer",
		Name:           "Frontend Engineer",
		Description:    "Frontend pack.",
		ExamplePrompts: []string{"Create a visual regression plan."},
	}}
	results := searchResults(packs, "", "visual regression", false)
	if len(results) != 1 {
		t.Fatalf("expected one result, got %#v", results)
	}
	if results[0].Match != "Create a visual regression plan." {
		t.Fatalf("expected example prompt match, got %q", results[0].Match)
	}
}

func TestSearchResultsIncludeGuidanceWhenRequested(t *testing.T) {
	packs := []agentpacks.Pack{{
		ID:    "backend-engineer",
		Name:  "Backend Engineer",
		Tools: []string{"codex"},
		Compatibility: map[string]agentpacks.CompatibilityEvidence{
			"codex": {Status: "compatible", LastVerified: "2026-06-16"},
		},
	}}
	results := searchResults(packs, "codex", "", true)
	if len(results) != 1 {
		t.Fatalf("expected one result, got %#v", results)
	}
	want := "compatible for codex; install with: agent-packs install backend-engineer --agent codex"
	if results[0].Guidance != want {
		t.Fatalf("guidance = %q, want %q", results[0].Guidance, want)
	}
}

func TestParseFlagsAllowsSearchFlagsAfterQuery(t *testing.T) {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	freshness := fs.String("freshness", "", "")
	why := fs.Bool("why", false, "")
	guidance := fs.Bool("guidance", false, "")
	limit := fs.Int("limit", 0, "")
	if err := parseFlags(fs, []string{"backend", "--freshness", "fresh", "--why", "--guidance", "--limit=3"}); err != nil {
		t.Fatal(err)
	}
	if *freshness != "fresh" || !*why || !*guidance || *limit != 3 {
		t.Fatalf("flags not parsed: freshness=%q why=%v guidance=%v limit=%d", *freshness, *why, *guidance, *limit)
	}
	if got := fs.Args(); !reflect.DeepEqual(got, []string{"backend"}) {
		t.Fatalf("positionals = %v; want [backend]", got)
	}
}

func TestRunTestRunValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing pack-id",
			args:    []string{},
			wantErr: "usage: agent-packs test-run <pack-id>",
		},
		{
			name:    "invalid mode",
			args:    []string{"my-pack", "--mode", "bogus"},
			wantErr: "invalid --mode \"bogus\"",
		},
		{
			name:    "invalid agent",
			args:    []string{"my-pack", "--agent", "non-existent-tool"},
			wantErr: "invalid agent \"non-existent-tool\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runTestRun("mock-registry", "mock-target", tt.args)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}
