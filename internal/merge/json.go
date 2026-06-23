package merge

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/agent-packs/cli/internal/util"
)

// ApplyJSONMerge deep-merges the JSON fragment into the JSON object file at path
// (under the optional dotted mergeKey), creating the file if absent. Semantics
// are user-wins, add-only: a key the pack introduces is set and recorded in
// Result.OwnedKeys; a key that already exists is left untouched and not owned,
// so the user's value is never clobbered and uninstall can restore the original
// exactly. A file that exists but is not a JSON object is an error (fail closed
// — never truncate a user's file).
func ApplyJSONMerge(path, mergeKey string, fragment []byte) (Result, error) {
	root, err := readJSONObject(path)
	if err != nil {
		return Result{}, err
	}
	nested, err := nestFragment(mergeKey, fragment)
	if err != nil {
		return Result{}, err
	}
	owned := []string{}
	deepMergeAddOnly(root, nested, "", &owned)
	sort.Strings(owned)

	data, err := marshalJSON(root)
	if err != nil {
		return Result{}, err
	}
	if err := util.AtomicWriteFile(path, data, 0o644); err != nil {
		return Result{}, err
	}
	return Result{Changed: true, OwnedKeys: owned, ContentHash: ownedHash(root, owned)}, nil
}

// RetractJSONMerge removes exactly the ownedKeys the pack added from the JSON
// file at path and prunes any now-empty parent objects the pack created. User
// keys (never present in ownedKeys) are preserved, so the file returns to its
// pre-install state. A missing file is a no-op.
func RetractJSONMerge(path, mergeKey string, ownedKeys []string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	root, err := readJSONObject(path)
	if err != nil {
		return err
	}
	for _, key := range ownedKeys {
		deletePath(root, strings.Split(key, "."))
	}
	data, err := marshalJSON(root)
	if err != nil {
		return err
	}
	return util.AtomicWriteFile(path, data, 0o644)
}

// OwnedKeysState reports, for drift detection, whether every ownedKey is still
// present in the file and the hash of the current owned values.
func OwnedKeysState(path string, ownedKeys []string) (allPresent bool, hash string, err error) {
	root, err := readJSONObject(path)
	if err != nil {
		return false, "", err
	}
	for _, key := range ownedKeys {
		if _, ok := lookupPath(root, strings.Split(key, ".")); !ok {
			return false, "", nil
		}
	}
	return true, ownedHash(root, ownedKeys), nil
}

// deepMergeAddOnly merges src into dst, recording newly added leaf paths in
// owned. Existing keys in dst are left untouched (user wins).
func deepMergeAddOnly(dst, src map[string]any, prefix string, owned *[]string) {
	for key, srcVal := range src {
		full := key
		if prefix != "" {
			full = prefix + "." + key
		}
		srcMap, srcIsMap := srcVal.(map[string]any)
		existing, present := dst[key]
		if srcIsMap {
			dstMap, dstIsMap := existing.(map[string]any)
			if present && !dstIsMap {
				// Conflict: pack wants an object, user has a scalar. User wins.
				continue
			}
			if !present {
				dstMap = map[string]any{}
				dst[key] = dstMap
			}
			deepMergeAddOnly(dstMap, srcMap, full, owned)
			continue
		}
		if present {
			continue // user-owned leaf; do not overwrite
		}
		dst[key] = srcVal
		*owned = append(*owned, full)
	}
}

// nestFragment parses fragment and wraps it under the dotted mergeKey path.
func nestFragment(mergeKey string, fragment []byte) (map[string]any, error) {
	var value any
	if err := json.Unmarshal(fragment, &value); err != nil {
		return nil, fmt.Errorf("invalid JSON fragment: %w", err)
	}
	if mergeKey == "" {
		obj, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("settings fragment must be a JSON object when no mergeKey is set")
		}
		return obj, nil
	}
	parts := strings.Split(mergeKey, ".")
	for i := len(parts) - 1; i >= 0; i-- {
		value = map[string]any{parts[i]: value}
	}
	return value.(map[string]any), nil
}

func deletePath(root map[string]any, parts []string) {
	if len(parts) == 0 {
		return
	}
	if len(parts) == 1 {
		delete(root, parts[0])
		return
	}
	child, ok := root[parts[0]].(map[string]any)
	if !ok {
		return
	}
	deletePath(child, parts[1:])
	if len(child) == 0 {
		delete(root, parts[0]) // prune empty parent the pack created
	}
}

func lookupPath(root map[string]any, parts []string) (any, bool) {
	cur := any(root)
	for _, part := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, ok := m[part]
		if !ok {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

func ownedHash(root map[string]any, ownedKeys []string) string {
	if len(ownedKeys) == 0 {
		return ""
	}
	sorted := append([]string(nil), ownedKeys...)
	sort.Strings(sorted)
	subset := map[string]any{}
	for _, key := range sorted {
		if v, ok := lookupPath(root, strings.Split(key, ".")); ok {
			subset[key] = v
		}
	}
	data, _ := json.Marshal(subset)
	return HashString(string(data))
}

func readJSONObject(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]any{}, nil
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("%s is not a valid JSON object: %w", path, err)
	}
	return root, nil
}

func marshalJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
