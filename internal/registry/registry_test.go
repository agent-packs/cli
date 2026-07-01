package registry

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeMinimalPack(t *testing.T, dir, id string) {
	t.Helper()
	pack := `{
  "id": "` + id + `",
  "name": "` + id + ` Pack",
  "version": "0.1.0",
  "description": "Test pack ` + id + `.",
  "capabilities": []
}`
	if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(pack), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writePack(t *testing.T, dir, id, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadPacksMissingRegistryGivesActionableError(t *testing.T) {
	_, err := LoadPacks(filepath.Join(t.TempDir(), "does-not-exist", "packs"))
	if err == nil {
		t.Fatal("expected an error for a missing registry directory")
	}
	for _, want := range []string{"registry not found", "AGENT_PACKS_REGISTRY"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error should mention %q, got: %v", want, err)
		}
	}
}

func TestFilteredMatchPacksSupportsDiscoveryFacets(t *testing.T) {
	dir := t.TempDir()
	writePack(t, dir, "frontend", `{
  "id": "frontend",
  "name": "Frontend",
  "version": "0.1.0",
  "description": "Frontend pack.",
  "stability": "experimental",
  "reviewStatus": "reviewed",
  "trust": "community",
  "lastVerified": "2026-06-29",
  "compatibility": {
    "codex": {
      "status": "verified",
      "lastVerified": "2026-06-29"
    }
  },
  "useCases": ["Review frontend launch readiness"],
  "examplePrompts": ["Create a visual regression plan."],
  "tools": ["codex", "claude-code"],
  "scope": ["global", "project"],
  "tags": ["ui"],
  "categories": ["frontend"],
  "capabilities": [
    {"type":"skill","name":"Frontend","source":"/tmp/frontend","format":"agent-skill","entry":"SKILL.md"},
    {"type":"prompt","name":"Accessibility readiness","content":"Check focus contrast responsive behavior.","format":"markdown"}
  ]
}`)
	writePack(t, dir, "backend", `{
  "id": "backend",
  "name": "Backend",
  "version": "0.1.0",
  "description": "Backend pack.",
  "stability": "stable",
  "reviewStatus": "draft",
  "tools": ["gemini"],
  "scope": ["global"],
  "tags": ["api"],
  "categories": ["backend"],
  "capabilities": [{"type":"skill","name":"Backend","source":"/tmp/backend","format":"agent-skill","entry":"SKILL.md"}]
}`)

	matches, err := FilteredMatchPacks(dir, "", SearchFilter{Tool: "claude", ReviewStatus: "REVIEWED", Scope: "PROJECT"})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].ID != "frontend" {
		t.Fatalf("expected frontend match for tool/review/scope facets, got %#v", matches)
	}

	matches, err = FilteredMatchPacks(dir, "", SearchFilter{Tool: "codex", Scope: "global"})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].ID != "frontend" {
		t.Fatalf("expected codex/global to match frontend, got %#v", matches)
	}

	matches, err = FilteredMatchPacks(dir, "visual regression", SearchFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].ID != "frontend" {
		t.Fatalf("expected example prompt search to match frontend, got %#v", matches)
	}

	matches, err = FilteredMatchPacks(dir, "visual plan frontend", SearchFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].ID != "frontend" {
		t.Fatalf("expected tokenized metadata search to match frontend, got %#v", matches)
	}

	matches, err = FilteredMatchPacks(dir, "responsive focus contrast", SearchFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].ID != "frontend" {
		t.Fatalf("expected capability content search to match frontend, got %#v", matches)
	}

	matches, err = FilteredMatchPacks(dir, "visual backend", SearchFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected tokenized search to require every term, got %#v", matches)
	}

	matches, err = FilteredMatchPacks(dir, "", SearchFilter{Trust: "community", CompatibleWith: "codex", CompatStatus: "verified"})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].ID != "frontend" {
		t.Fatalf("expected trusted compatible frontend match, got %#v", matches)
	}

	matches, err = FilteredMatchPacks(dir, "", SearchFilter{CompatibleWith: "claude-code", CompatStatus: "verified"})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no claude-code verified compatibility evidence, got %#v", matches)
	}

	matches, err = FilteredMatchPacks(dir, "", SearchFilter{Freshness: "fresh"})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].ID != "frontend" {
		t.Fatalf("expected fresh frontend match, got %#v", matches)
	}
}

func TestFilteredMatchPacksRecommendedStarterPath(t *testing.T) {
	dir := t.TempDir()
	writePack(t, dir, "backend-engineer", `{
  "id": "backend-engineer",
  "name": "Backend Engineer",
  "version": "0.1.0",
  "description": "Backend pack.",
  "recommendation": {"path":"starter","order":20,"reason":"Common backend service workflow."},
  "capabilities": []
}`)
	writeMinimalPack(t, dir, "random-helper")

	matches, err := FilteredMatchPacks(dir, "", SearchFilter{Recommended: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].ID != "backend-engineer" {
		t.Fatalf("expected only recommended starter packs, got %#v", matches)
	}
}

func TestShowIncludesUseCasesAndExamplePrompts(t *testing.T) {
	dir := t.TempDir()
	writePack(t, dir, "leader", `{
  "id": "leader",
  "name": "Leader",
  "version": "0.1.0",
  "description": "Leadership pack.",
  "trust": "community",
  "reviewStatus": "reviewed",
  "lastVerified": "2026-06-29",
  "tools": ["codex"],
  "scope": ["project"],
  "compatibility": {
    "codex": {"status":"verified","lastVerified":"2026-06-29"}
  },
  "tags": ["leadership"],
  "useCases": ["Review quarterly engineering strategy."],
  "examplePrompts": ["Review this strategy memo for weak tradeoffs."],
  "capabilities": []
}`)
	var out strings.Builder
	if err := Show(dir, "leader", &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"Use cases:",
		"- Review quarterly engineering strategy.",
		"Example prompts:",
		"- Review this strategy memo for weak tradeoffs.",
		"Trust: community",
		"Review status: reviewed",
		"Last verified: 2026-06-29 (fresh)",
		"Works with: codex",
		"Scope: project",
		"Compatibility: codex=verified",
		"Provenance: skills 0 (0 object, 0 bare), plugins 0 (0 object, 0 bare), inline 0",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("show output missing %q:\n%s", want, got)
		}
	}
}

func TestInfoIncludesTrustSnapshot(t *testing.T) {
	dir := t.TempDir()
	writePack(t, dir, "leader", `{
  "id": "leader",
  "name": "Leader",
  "version": "0.1.0",
  "description": "Leadership pack.",
  "trust": "community",
  "reviewStatus": "reviewed",
  "lastVerified": "2026-06-29",
  "tools": ["codex"],
  "scope": ["project"],
  "skills": [{"id":"strategy","trust":"community"}],
  "plugins": ["review-helper"],
  "capabilities": [{"type":"prompt","name":"SPACE","content":"Use SPACE."}]
}`)
	var out strings.Builder
	if err := Info(dir, t.TempDir(), "leader", &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"Trust:         community",
		"Review status: reviewed",
		"Last verified: 2026-06-29 (fresh)",
		"Scope:         project",
		"Provenance:    skills 1 (1 object, 0 bare), plugins 1 (0 object, 1 bare), inline 1",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("info output missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateIndexIsStableWhenContentUnchanged(t *testing.T) {
	dir := t.TempDir()
	writeMinimalPack(t, dir, "alpha")
	writeMinimalPack(t, dir, "beta")
	out := filepath.Join(t.TempDir(), "index.json")

	if err := GenerateIndex(dir, out, io.Discard); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}

	// Regenerating from an unchanged registry must produce byte-identical
	// output (same generatedAt), so the index doesn't churn in git.
	if err := GenerateIndex(dir, out, io.Discard); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("index changed on regenerate with no content change:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

func TestExpandPackResolvesReusableJSONCapabilities(t *testing.T) {
	root := t.TempDir()
	packs := filepath.Join(root, "packs")
	for _, dir := range []string{"packs", "commands", "tools", "mcp"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "commands", "review-pr.json"), []byte(`{"type":"command","name":"Review PR","content":"Review this PR."}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tools", "browser-verify.json"), []byte(`{"type":"tool","name":"Browser Verify","content":"{\"name\":\"browser-verify\"}","format":"json"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "mcp", "filesystem.json"), []byte(`{"type":"mcp","name":"Filesystem","serverName":"filesystem","command":"npx","args":["server"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writePack(t, packs, "ops", `{
  "id": "ops",
  "name": "Ops",
  "version": "0.1.0",
  "description": "Ops pack.",
  "commands": ["review-pr"],
  "toolRefs": [{"id":"browser-verify","trust":"community"}],
  "mcp": ["filesystem"]
}`)

	pack, err := FindPack(packs, "ops")
	if err != nil {
		t.Fatal(err)
	}
	expanded, err := ExpandPack(packs, pack, map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, capability := range expanded.Capabilities {
		got[capability.Type+":"+capability.Name] = true
	}
	for _, want := range []string{"command:Review PR", "tool:Browser Verify", "mcp:Filesystem"} {
		if !got[want] {
			t.Fatalf("expanded capabilities missing %s: %+v", want, expanded.Capabilities)
		}
	}
}

func TestGenerateIndexIncludesCapabilityTypeCountsAndCompatibility(t *testing.T) {
	dir := t.TempDir()
	writePack(t, dir, "alpha", `{
  "id": "alpha",
  "name": "Alpha",
  "version": "0.1.0",
  "description": "Alpha pack.",
  "trust": "community",
  "lastVerified": "2026-06-01",
  "useCases": ["Review an AI feature before launch"],
  "examplePrompts": ["Review this launch plan for production AI readiness."],
  "compatibility": {
    "codex": {"status":"verified","lastVerified":"2026-06-01"}
  },
  "capabilities": [
    {"type":"command","name":"Review","content":"review"},
    {"type":"tool","name":"Browser Verify","content":"{\"name\":\"browser\"}","format":"json"}
  ]
}`)
	out := filepath.Join(t.TempDir(), "index.json")
	if err := GenerateIndex(dir, out, io.Discard); err != nil {
		t.Fatal(err)
	}
	index, err := loadIndex(out)
	if err != nil {
		t.Fatal(err)
	}
	entry := index.Packs[0]
	if entry.CapabilityTypes["command"] != 1 || entry.CapabilityTypes["tool"] != 1 {
		t.Fatalf("expected command/tool capability counts, got %#v", entry.CapabilityTypes)
	}
	if entry.Trust != "community" {
		t.Fatalf("expected trust to be indexed, got %q", entry.Trust)
	}
	if len(entry.UseCases) != 1 || entry.UseCases[0] != "Review an AI feature before launch" {
		t.Fatalf("expected use cases to be indexed, got %#v", entry.UseCases)
	}
	if len(entry.ExamplePrompts) != 1 || entry.ExamplePrompts[0] != "Review this launch plan for production AI readiness." {
		t.Fatalf("expected example prompts to be indexed, got %#v", entry.ExamplePrompts)
	}
	if entry.Compatibility["codex"].Status != "verified" {
		t.Fatalf("expected codex compatibility evidence, got %#v", entry.Compatibility)
	}
	if entry.Freshness != "fresh" {
		t.Fatalf("expected fresh status, got %q", entry.Freshness)
	}
	if entry.Recommended {
		t.Fatalf("non-starter pack should not be marked recommended")
	}
}

func TestGenerateIndexMarksRecommendedStarterPacks(t *testing.T) {
	dir := t.TempDir()
	writePack(t, dir, "backend-engineer", `{
  "id": "backend-engineer",
  "name": "Backend Engineer",
  "version": "0.1.0",
  "description": "Backend pack.",
  "recommendation": {"path":"starter","order":20,"reason":"Common backend service workflow."},
  "capabilities": []
}`)
	writeMinimalPack(t, dir, "random-helper")
	out := filepath.Join(t.TempDir(), "index.json")
	if err := GenerateIndex(dir, out, io.Discard); err != nil {
		t.Fatal(err)
	}
	index, err := loadIndex(out)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, entry := range index.Packs {
		got[entry.ID] = entry.Recommended
	}
	if !got["backend-engineer"] {
		t.Fatalf("backend-engineer should be marked recommended: %#v", got)
	}
	if got["random-helper"] {
		t.Fatalf("random-helper should not be marked recommended: %#v", got)
	}
}

func TestGenerateIndexRestampsWhenContentChanges(t *testing.T) {
	dir := t.TempDir()
	writeMinimalPack(t, dir, "alpha")
	out := filepath.Join(t.TempDir(), "index.json")

	if err := GenerateIndex(dir, out, io.Discard); err != nil {
		t.Fatal(err)
	}
	before, err := loadIndex(out)
	if err != nil {
		t.Fatal(err)
	}

	// Adding a pack changes content, so generatedAt must advance.
	writeMinimalPack(t, dir, "gamma")
	if err := GenerateIndex(dir, out, io.Discard); err != nil {
		t.Fatal(err)
	}
	after, err := loadIndex(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Packs) != 2 {
		t.Fatalf("expected 2 packs after adding one, got %d", len(after.Packs))
	}
	if after.GeneratedAt == before.GeneratedAt {
		t.Fatalf("generatedAt should change when content changes, stayed %q", after.GeneratedAt)
	}
}

func TestCheckIndexCleanPassesIgnoringGeneratedAt(t *testing.T) {
	dir := t.TempDir()
	writeMinimalPack(t, dir, "alpha")
	writeMinimalPack(t, dir, "beta")
	out := filepath.Join(t.TempDir(), "index.json")
	if err := GenerateIndex(dir, out, io.Discard); err != nil {
		t.Fatal(err)
	}

	// Mutate only generatedAt on disk; --check must still pass.
	idx, err := loadIndex(out)
	if err != nil {
		t.Fatal(err)
	}
	idx.GeneratedAt = "1999-01-01T00:00:00Z"
	if err := os.WriteFile(out, mustJSON(t, idx), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	if err := CheckIndex(dir, out, &buf); err != nil {
		t.Fatalf("expected clean check despite generatedAt drift, got %v (out: %s)", err, buf.String())
	}
	if !strings.Contains(buf.String(), "up to date") {
		t.Fatalf("expected up-to-date message, got: %s", buf.String())
	}
}

func TestCheckIndexDetectsDrift(t *testing.T) {
	dir := t.TempDir()
	writeMinimalPack(t, dir, "alpha")
	out := filepath.Join(t.TempDir(), "index.json")
	if err := GenerateIndex(dir, out, io.Discard); err != nil {
		t.Fatal(err)
	}

	// Add a pack to the registry but not to the index file => drift.
	writeMinimalPack(t, dir, "gamma")

	var buf strings.Builder
	err := CheckIndex(dir, out, &buf)
	if err == nil {
		t.Fatal("expected CheckIndex to report drift")
	}
	got := buf.String()
	if !strings.Contains(got, "out of date") {
		t.Fatalf("expected out-of-date message, got: %s", got)
	}
	if !strings.Contains(got, "gamma") {
		t.Fatalf("expected drift summary to name the new pack, got: %s", got)
	}
}

func TestCheckIndexFieldDrift(t *testing.T) {
	dir := t.TempDir()
	writeMinimalPack(t, dir, "alpha")
	out := filepath.Join(t.TempDir(), "index.json")
	if err := GenerateIndex(dir, out, io.Discard); err != nil {
		t.Fatal(err)
	}

	// Change a pack's version on disk in the index only.
	idx, err := loadIndex(out)
	if err != nil {
		t.Fatal(err)
	}
	idx.Packs[0].Version = "9.9.9"
	if err := os.WriteFile(out, mustJSON(t, idx), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	if err := CheckIndex(dir, out, &buf); err == nil {
		t.Fatal("expected field drift to be reported")
	}
	if !strings.Contains(buf.String(), `field "version" differs`) {
		t.Fatalf("expected version field drift, got: %s", buf.String())
	}
}

func TestCheckIndexMissingFile(t *testing.T) {
	dir := t.TempDir()
	writeMinimalPack(t, dir, "alpha")
	out := filepath.Join(t.TempDir(), "missing-index.json")
	var buf strings.Builder
	if err := CheckIndex(dir, out, &buf); err == nil {
		t.Fatal("expected error when index file does not exist")
	}
	if !strings.Contains(buf.String(), "does not exist") {
		t.Fatalf("expected does-not-exist message, got: %s", buf.String())
	}
}
