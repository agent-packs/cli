package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

// canonicalTrustLevels is the fallback allowlist used when the registry's JSON
// schema cannot be located (for example when validating a single pack file that
// lives outside a registry checkout). The authoritative source of truth is the
// `trust.enum` on the skillRef/pluginRef object definitions in the registry's
// schemas/agent-pack.schema.json; this list must stay in sync with it. CLI
// validation prefers reading the schema so the two do not drift, and only falls
// back to this constant when the schema is unreachable.
//
// Trust levels record the provenance of an object ref's upstream source:
//   - official:  maintained by the source tool's own vendor org, or first-party
//     content shipped from the agent-packs/registry repo itself.
//   - verified:  third-party integrations packaged/curated by a trusted vendor.
//   - community: independent third-party community sources, referenced but not
//     vendor-maintained.
var canonicalTrustLevels = []string{
	"official",
	"community",
	"verified",
}

// schemaTrustCache memoizes the trust enum read from a given schema directory so
// repeated validation (e.g. LintAll over every pack) does not re-read the schema
// file each time.
var (
	schemaTrustCache   = map[string][]string{}
	schemaTrustCacheMu sync.Mutex
)

// AllowedTrustLevels returns the set of valid object-ref trust values. When
// startDir is non-empty it walks up from startDir looking for
// schemas/agent-pack.schema.json and returns the enum declared there; this keeps
// the CLI in lockstep with the registry's own schema. If no schema is found it
// returns the canonical fallback list.
func AllowedTrustLevels(startDir string) []string {
	if startDir != "" {
		if levels, ok := trustLevelsFromSchemaDir(startDir); ok {
			return levels
		}
	}
	out := make([]string, len(canonicalTrustLevels))
	copy(out, canonicalTrustLevels)
	return out
}

func trustLevelsFromSchemaDir(startDir string) ([]string, bool) {
	schemaPath := findSchemaPath(startDir)
	if schemaPath == "" {
		return nil, false
	}
	schemaTrustCacheMu.Lock()
	defer schemaTrustCacheMu.Unlock()
	if cached, ok := schemaTrustCache[schemaPath]; ok {
		return cached, true
	}
	levels, ok := readTrustEnum(schemaPath)
	if !ok {
		return nil, false
	}
	schemaTrustCache[schemaPath] = levels
	return levels, true
}

// readTrustEnum extracts the trust enum from the skillRef object definition in
// the schema. The skillRef and pluginRef definitions share the same trust enum;
// reading either is sufficient.
func readTrustEnum(schemaPath string) ([]string, bool) {
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, false
	}
	var schema struct {
		Defs map[string]struct {
			OneOf []struct {
				Type       string `json:"type"`
				Properties struct {
					Trust struct {
						Enum []string `json:"enum"`
					} `json:"trust"`
				} `json:"properties"`
			} `json:"oneOf"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, false
	}
	for _, name := range []string{"skillRef", "pluginRef"} {
		def, ok := schema.Defs[name]
		if !ok {
			continue
		}
		for _, branch := range def.OneOf {
			if branch.Type != "object" {
				continue
			}
			if enum := branch.Properties.Trust.Enum; len(enum) > 0 {
				return enum, true
			}
		}
	}
	return nil, false
}

// validateTrust returns errors when an object ref's trust is missing or outside
// the allowed set. Bare-string refs (trust == "" and not an object ref) are
// handled by the caller and never reach this function.
func validateTrust(trust string, allowed []string, prefix string) []string {
	if trust == "" {
		return []string{fmt.Sprintf("%s.trust is required; valid values: %s", prefix, strings.Join(sortedCopy(allowed), ", "))}
	}
	for _, level := range allowed {
		if trust == level {
			return nil
		}
	}
	return []string{fmt.Sprintf("%s.trust %q is not allowed; valid values: %s", prefix, trust, strings.Join(sortedCopy(allowed), ", "))}
}
