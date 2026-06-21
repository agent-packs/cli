package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// canonicalCategories is the fallback allowlist used when the registry's JSON
// schema cannot be located (for example when validating a single pack file that
// lives outside a registry checkout). The authoritative source of truth is the
// `categories.items.enum` in the registry's schemas/agent-pack.schema.json; this
// list must stay in sync with it. CLI validation prefers reading the schema so
// the two do not drift, and only falls back to this constant when the schema is
// unreachable.
var canonicalCategories = []string{
	"engineering",
	"frontend",
	"backend",
	"infrastructure",
	"platform",
	"data",
	"ml",
	"security",
	"quality",
	"testing",
	"reliability",
	"documentation",
	"product",
	"devex",
}

// schemaCategoryCache memoizes the category enum read from a given schema
// directory so repeated validation (e.g. LintAll over every pack) does not
// re-read the schema file each time.
var (
	schemaCategoryCache   = map[string][]string{}
	schemaCategoryCacheMu sync.Mutex
)

// AllowedCategories returns the set of valid pack categories. When startDir is
// non-empty it walks up from startDir looking for schemas/agent-pack.schema.json
// and returns the enum declared there; this keeps the CLI in lockstep with the
// registry's own schema. If no schema is found it returns the canonical
// fallback list.
func AllowedCategories(startDir string) []string {
	if startDir != "" {
		if cats, ok := categoriesFromSchemaDir(startDir); ok {
			return cats
		}
	}
	out := make([]string, len(canonicalCategories))
	copy(out, canonicalCategories)
	return out
}

func categoriesFromSchemaDir(startDir string) ([]string, bool) {
	schemaPath := findSchemaPath(startDir)
	if schemaPath == "" {
		return nil, false
	}
	schemaCategoryCacheMu.Lock()
	defer schemaCategoryCacheMu.Unlock()
	if cached, ok := schemaCategoryCache[schemaPath]; ok {
		return cached, true
	}
	cats, ok := readCategoryEnum(schemaPath)
	if !ok {
		return nil, false
	}
	schemaCategoryCache[schemaPath] = cats
	return cats, true
}

// findSchemaPath walks up from startDir looking for schemas/agent-pack.schema.json.
func findSchemaPath(startDir string) string {
	dir := startDir
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		candidate := filepath.Join(dir, "schemas", "agent-pack.schema.json")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func readCategoryEnum(schemaPath string) ([]string, bool) {
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, false
	}
	var schema struct {
		Properties struct {
			Categories struct {
				Items struct {
					Enum []string `json:"enum"`
				} `json:"items"`
			} `json:"categories"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, false
	}
	enum := schema.Properties.Categories.Items.Enum
	if len(enum) == 0 {
		return nil, false
	}
	return enum, true
}

// validateCategories returns errors for any category outside the allowed set.
func validateCategories(categories, allowed []string) []string {
	if len(categories) == 0 {
		return nil
	}
	allowedSet := map[string]bool{}
	for _, c := range allowed {
		allowedSet[c] = true
	}
	var errs []string
	for _, c := range categories {
		if !allowedSet[c] {
			errs = append(errs, fmt.Sprintf("category %q is not allowed; valid categories: %s", c, strings.Join(sortedCopy(allowed), ", ")))
		}
	}
	return errs
}

func sortedCopy(values []string) []string {
	out := make([]string, len(values))
	copy(out, values)
	sort.Strings(out)
	return out
}
