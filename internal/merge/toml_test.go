package merge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTOMLMergeCreatesFileFromJSONFragment(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".codex", "config.toml")
	res, err := ApplyTOMLMerge(path, "", []byte(`{"features":{"memories":true},"model":"gpt-5.5"}`))
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	got := read(t, path)
	if !strings.Contains(got, "[features]\nmemories = true") {
		t.Fatalf("features.memories not written:\n%s", got)
	}
	if !strings.Contains(got, `model = "gpt-5.5"`) {
		t.Fatalf("model not written:\n%s", got)
	}
	if len(res.OwnedKeys) != 2 {
		t.Fatalf("expected two owned keys, got %v", res.OwnedKeys)
	}
}

func TestTOMLMergeUserWinsAndRetractPreservesUserKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := "model = \"mine\"\n\n[features]\nexisting = true\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := ApplyTOMLMerge(path, "", []byte(`{"model":"pack","features":{"memories":true}}`))
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	got := read(t, path)
	if strings.Contains(got, `model = "pack"`) {
		t.Fatalf("user-owned model should not be overwritten:\n%s", got)
	}
	if strings.Count(got, "[features]") != 1 {
		t.Fatalf("existing TOML table should not be redeclared:\n%s", got)
	}
	if !strings.Contains(got, "memories = true") {
		t.Fatalf("pack key missing:\n%s", got)
	}
	if len(res.OwnedKeys) != 1 || res.OwnedKeys[0] != "features.memories" {
		t.Fatalf("expected only features.memories ownership, got %v", res.OwnedKeys)
	}
	if err := RetractTOMLMerge(path, "", res.OwnedKeys); err != nil {
		t.Fatalf("retract: %v", err)
	}
	after := read(t, path)
	if strings.Contains(after, "memories") {
		t.Fatalf("pack key should be removed:\n%s", after)
	}
	if !strings.Contains(after, `model = "mine"`) || !strings.Contains(after, "existing = true") {
		t.Fatalf("user keys should remain:\n%s", after)
	}
}

func TestTOMLOwnedKeysStateDetectsChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	res, err := ApplyTOMLMerge(path, "", []byte(`{"features":{"memories":true}}`))
	if err != nil {
		t.Fatal(err)
	}
	present, hash, err := OwnedTOMLKeysState(path, res.OwnedKeys)
	if err != nil || !present {
		t.Fatalf("expected present, got present=%v err=%v", present, err)
	}
	if hash != res.ContentHash {
		t.Fatalf("hash should match install hash")
	}
	if err := os.WriteFile(path, []byte("[features]\nmemories = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, changedHash, _ := OwnedTOMLKeysState(path, res.OwnedKeys)
	if changedHash == res.ContentHash {
		t.Fatal("expected hash to change after edit")
	}
}
