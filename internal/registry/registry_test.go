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
