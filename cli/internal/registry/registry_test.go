package registry

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
