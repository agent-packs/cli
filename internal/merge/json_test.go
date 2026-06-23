package merge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return m
}

func TestJSONMergeCreatesFileWhenAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deep", "settings.json")
	res, err := ApplyJSONMerge(path, "", []byte(`{"model":"opus"}`))
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := readJSON(t, path)["model"]; got != "opus" {
		t.Fatalf("want model=opus, got %v", got)
	}
	if len(res.OwnedKeys) != 1 || res.OwnedKeys[0] != "model" {
		t.Fatalf("expected owned [model], got %v", res.OwnedKeys)
	}
}

func TestJSONMergePreservesSiblingKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"theme":"dark","permissions":{"allow":["a"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyJSONMerge(path, "", []byte(`{"model":"opus","permissions":{"deny":["b"]}}`)); err != nil {
		t.Fatalf("apply: %v", err)
	}
	got := readJSON(t, path)
	if got["theme"] != "dark" {
		t.Fatalf("sibling theme lost: %v", got)
	}
	if got["model"] != "opus" {
		t.Fatalf("model not added: %v", got)
	}
	perms := got["permissions"].(map[string]any)
	if _, ok := perms["allow"]; !ok {
		t.Fatalf("nested user key permissions.allow lost: %v", perms)
	}
	if _, ok := perms["deny"]; !ok {
		t.Fatalf("nested pack key permissions.deny not added: %v", perms)
	}
}

func TestJSONMergeUserWinsOnConflict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"model":"mine"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := ApplyJSONMerge(path, "", []byte(`{"model":"theirs"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := readJSON(t, path)["model"]; got != "mine" {
		t.Fatalf("user value should win, got %v", got)
	}
	if len(res.OwnedKeys) != 0 {
		t.Fatalf("pre-existing key must not be owned, got %v", res.OwnedKeys)
	}
}

func TestJSONMergeWithMergeKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	res, err := ApplyJSONMerge(path, "mcpServers", []byte(`{"fs":{"command":"x"}}`))
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	got := readJSON(t, path)
	servers := got["mcpServers"].(map[string]any)
	if _, ok := servers["fs"]; !ok {
		t.Fatalf("expected mcpServers.fs, got %v", got)
	}
	// Ownership is tracked at leaf granularity, so retract removes exactly the
	// scalar the pack set and can prune the empty parents it created.
	if len(res.OwnedKeys) != 1 || res.OwnedKeys[0] != "mcpServers.fs.command" {
		t.Fatalf("expected owned [mcpServers.fs.command], got %v", res.OwnedKeys)
	}
}

func TestJSONRetractRemovesOnlyPackKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	original := `{"theme":"dark","permissions":{"allow":["a"]}}`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	var want map[string]any
	_ = json.Unmarshal([]byte(original), &want)

	res, err := ApplyJSONMerge(path, "", []byte(`{"model":"opus","permissions":{"deny":["b"]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := RetractJSONMerge(path, "", res.OwnedKeys); err != nil {
		t.Fatalf("retract: %v", err)
	}
	got := readJSON(t, path)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("retract did not round-trip:\nwant %v\ngot  %v", want, got)
	}
}

func TestJSONRetractPrunesEmptyParents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	res, err := ApplyJSONMerge(path, "mcpServers", []byte(`{"fs":{"command":"x"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := RetractJSONMerge(path, "mcpServers", res.OwnedKeys); err != nil {
		t.Fatal(err)
	}
	got := readJSON(t, path)
	if _, ok := got["mcpServers"]; ok {
		t.Fatalf("empty pack-created parent should be pruned, got %v", got)
	}
}

func TestJSONMergeMalformedFailsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	bad := "{not valid json"
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyJSONMerge(path, "", []byte(`{"model":"opus"}`)); err == nil {
		t.Fatal("expected error on malformed existing JSON")
	}
	if got := read(t, path); got != bad {
		t.Fatalf("malformed file must be left untouched, got %q", got)
	}
}

func TestJSONOwnedKeysStateDetectsChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	res, err := ApplyJSONMerge(path, "", []byte(`{"model":"opus"}`))
	if err != nil {
		t.Fatal(err)
	}
	present, hash, err := OwnedKeysState(path, res.OwnedKeys)
	if err != nil || !present {
		t.Fatalf("expected present, got present=%v err=%v", present, err)
	}
	if hash != res.ContentHash {
		t.Fatalf("hash should match install-time hash")
	}
	// User edits the owned value -> hash diverges.
	if err := os.WriteFile(path, []byte(`{"model":"sonnet"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, hash2, _ := OwnedKeysState(path, res.OwnedKeys)
	if hash2 == res.ContentHash {
		t.Fatal("expected hash to change after user edit")
	}
}
