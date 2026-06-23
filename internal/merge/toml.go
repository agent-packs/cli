package merge

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/agent-packs/cli/internal/util"
)

// ApplyTOMLMerge merges a small TOML/JSON object fragment into a TOML config
// file. It is intentionally conservative and add-only: existing keys are user
// owned and are not overwritten. The parser preserves unrelated lines/comments
// by appending missing keys rather than reformatting the entire file.
func ApplyTOMLMerge(path, mergeKey string, fragment []byte) (Result, error) {
	existing, err := readFileAllowMissing(path)
	if err != nil {
		return Result{}, err
	}
	current := parseTOMLKeys(existing)
	values, err := fragmentToTOMLValues(mergeKey, fragment)
	if err != nil {
		return Result{}, err
	}
	owned := []string{}
	for key := range values {
		if !current[key] {
			owned = append(owned, key)
		}
	}
	sort.Strings(owned)
	if len(owned) == 0 {
		return Result{Changed: false, OwnedKeys: owned, ContentHash: ""}, nil
	}
	updated := appendTOMLValues(existing, values, owned)
	if err := util.AtomicWriteFile(path, []byte(updated), 0o644); err != nil {
		return Result{}, err
	}
	return Result{Changed: true, OwnedKeys: owned, ContentHash: tomlOwnedHash(values, owned)}, nil
}

func RetractTOMLMerge(path, mergeKey string, ownedKeys []string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	existing, err := readFileAllowMissing(path)
	if err != nil {
		return err
	}
	updated := removeTOMLKeys(existing, ownedKeys)
	return util.AtomicWriteFile(path, []byte(updated), 0o644)
}

func OwnedTOMLKeysState(path string, ownedKeys []string) (allPresent bool, hash string, err error) {
	existing, err := readFileAllowMissing(path)
	if err != nil {
		return false, "", err
	}
	values := parseTOMLValues(existing)
	subset := map[string]string{}
	for _, key := range ownedKeys {
		value, ok := values[key]
		if !ok {
			return false, "", nil
		}
		subset[key] = value
	}
	return true, tomlOwnedHash(subset, ownedKeys), nil
}

func fragmentToTOMLValues(mergeKey string, fragment []byte) (map[string]string, error) {
	trimmed := strings.TrimSpace(string(fragment))
	values := map[string]string{}
	if strings.HasPrefix(trimmed, "{") {
		var root any
		if err := json.Unmarshal(fragment, &root); err != nil {
			return nil, fmt.Errorf("invalid JSON fragment for TOML settings: %w", err)
		}
		if mergeKey != "" {
			root = nestAny(mergeKey, root)
		}
		flattenJSONToTOML("", root, values)
		if len(values) == 0 {
			return nil, fmt.Errorf("settings fragment must contain at least one scalar or array value")
		}
		return values, nil
	}
	section := ""
	for _, line := range strings.Split(trimmed, "\n") {
		clean := stripTOMLComment(line)
		if clean == "" {
			continue
		}
		if strings.HasPrefix(clean, "[") && strings.HasSuffix(clean, "]") {
			section = strings.TrimSpace(strings.Trim(clean, "[]"))
			continue
		}
		parts := strings.SplitN(clean, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("unsupported TOML fragment line: %s", line)
		}
		key := strings.TrimSpace(parts[0])
		if section != "" {
			key = section + "." + key
		}
		if mergeKey != "" {
			key = mergeKey + "." + key
		}
		values[key] = strings.TrimSpace(parts[1])
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("settings fragment must contain at least one key")
	}
	return values, nil
}

func nestAny(mergeKey string, value any) any {
	for i := len(strings.Split(mergeKey, ".")) - 1; i >= 0; i-- {
		parts := strings.Split(mergeKey, ".")
		value = map[string]any{parts[i]: value}
	}
	return value
}

func flattenJSONToTOML(prefix string, value any, out map[string]string) {
	switch v := value.(type) {
	case map[string]any:
		keys := []string{}
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			next := key
			if prefix != "" {
				next = prefix + "." + key
			}
			flattenJSONToTOML(next, v[key], out)
		}
	case []any:
		parts := []string{}
		for _, item := range v {
			parts = append(parts, tomlLiteral(item))
		}
		out[prefix] = "[" + strings.Join(parts, ", ") + "]"
	default:
		out[prefix] = tomlLiteral(v)
	}
}

func tomlLiteral(value any) string {
	switch v := value.(type) {
	case string:
		return strconv.Quote(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%v", v)
	default:
		data, _ := json.Marshal(v)
		return strconv.Quote(string(data))
	}
}

func parseTOMLKeys(content string) map[string]bool {
	values := parseTOMLValues(content)
	keys := map[string]bool{}
	for key := range values {
		keys[key] = true
	}
	return keys
}

func parseTOMLValues(content string) map[string]string {
	values := map[string]string{}
	section := ""
	for _, line := range strings.Split(content, "\n") {
		clean := stripTOMLComment(line)
		if clean == "" {
			continue
		}
		if strings.HasPrefix(clean, "[") && strings.HasSuffix(clean, "]") {
			section = strings.TrimSpace(strings.Trim(clean, "[]"))
			continue
		}
		parts := strings.SplitN(clean, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		if section != "" {
			key = section + "." + key
		}
		values[key] = strings.TrimSpace(parts[1])
	}
	return values
}

func stripTOMLComment(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return ""
	}
	inString := false
	escaped := false
	for i, r := range line {
		if r == '\\' && inString {
			escaped = !escaped
			continue
		}
		if r == '"' && !escaped {
			inString = !inString
		}
		escaped = false
		if r == '#' && !inString {
			return strings.TrimSpace(line[:i])
		}
	}
	return line
}

func appendTOMLValues(existing string, values map[string]string, owned []string) string {
	grouped := map[string][]string{}
	for _, key := range owned {
		section, leaf := splitTOMLKey(key)
		grouped[section] = append(grouped[section], leaf)
	}
	rendered := map[string][]string{}
	for section, leaves := range grouped {
		sort.Strings(leaves)
		for _, leaf := range leaves {
			full := leaf
			if section != "" {
				full = section + "." + leaf
			}
			rendered[section] = append(rendered[section], leaf+" = "+values[full])
		}
	}

	content, remaining := insertIntoExistingTOMLSections(existing, rendered)
	if len(remaining) == 0 {
		return ensureTrailingNewline(content)
	}

	var b strings.Builder
	b.WriteString(strings.TrimRight(content, "\n"))
	if strings.TrimSpace(content) != "" {
		b.WriteString("\n")
	}
	sections := []string{}
	for section := range remaining {
		sections = append(sections, section)
	}
	sort.Strings(sections)
	for _, section := range sections {
		b.WriteString("\n")
		if section != "" {
			b.WriteString("[")
			b.WriteString(section)
			b.WriteString("]\n")
		}
		for _, line := range remaining[section] {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return strings.TrimLeft(b.String(), "\n")
}

func insertIntoExistingTOMLSections(existing string, additions map[string][]string) (string, map[string][]string) {
	remaining := map[string][]string{}
	for section, lines := range additions {
		remaining[section] = append([]string(nil), lines...)
	}
	if strings.TrimSpace(existing) == "" {
		return existing, remaining
	}
	lines := strings.Split(existing, "\n")
	out := []string{}
	current := ""
	inserted := map[string]bool{}
	for _, line := range lines {
		clean := stripTOMLComment(line)
		if strings.HasPrefix(clean, "[") && strings.HasSuffix(clean, "]") {
			if !inserted[current] && len(remaining[current]) > 0 {
				out = append(out, remaining[current]...)
				delete(remaining, current)
				inserted[current] = true
			}
			current = strings.TrimSpace(strings.Trim(clean, "[]"))
		}
		out = append(out, line)
	}
	if !inserted[current] && len(remaining[current]) > 0 {
		out = append(out, remaining[current]...)
		delete(remaining, current)
	}
	return strings.Join(out, "\n"), remaining
}

func ensureTrailingNewline(content string) string {
	if content == "" || strings.HasSuffix(content, "\n") {
		return content
	}
	return content + "\n"
}

func removeTOMLKeys(content string, ownedKeys []string) string {
	owned := map[string]bool{}
	for _, key := range ownedKeys {
		owned[key] = true
	}
	section := ""
	lines := strings.Split(content, "\n")
	out := []string{}
	for _, line := range lines {
		clean := stripTOMLComment(line)
		if strings.HasPrefix(clean, "[") && strings.HasSuffix(clean, "]") {
			section = strings.TrimSpace(strings.Trim(clean, "[]"))
			out = append(out, line)
			continue
		}
		parts := strings.SplitN(clean, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			if section != "" {
				key = section + "." + key
			}
			if owned[key] {
				continue
			}
		}
		out = append(out, line)
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
}

func splitTOMLKey(key string) (section, leaf string) {
	parts := strings.Split(key, ".")
	if len(parts) == 1 {
		return "", key
	}
	return strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1]
}

func tomlOwnedHash(values map[string]string, ownedKeys []string) string {
	if len(ownedKeys) == 0 {
		return ""
	}
	keys := append([]string(nil), ownedKeys...)
	sort.Strings(keys)
	parts := []string{}
	for _, key := range keys {
		parts = append(parts, key+"="+values[key])
	}
	return HashString(strings.Join(parts, "\n"))
}
