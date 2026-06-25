package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agent-packs/cli/internal/model"
)

func validPack() model.Pack {
	return model.Pack{
		ID:           "my-pack",
		Name:         "My Pack",
		Version:      "1.0.0",
		Description:  "A pack",
		Capabilities: []model.Capability{{Type: "skill", Name: "s", Source: "/tmp/s", Format: "agent-skill", Entry: "SKILL.md"}},
	}
}

func TestValidatePackValid(t *testing.T) {
	if errs := ValidatePack(validPack()); len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidatePackRequiredFields(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*model.Pack)
		wantSub string
	}{
		{"bad id", func(p *model.Pack) { p.ID = "Bad_ID" }, "id must be kebab-case"},
		{"missing name", func(p *model.Pack) { p.Name = "" }, "name is required"},
		{"missing version", func(p *model.Pack) { p.Version = "" }, "version is required"},
		{"missing description", func(p *model.Pack) { p.Description = "" }, "description is required"},
		{"bad stability", func(p *model.Pack) { p.Stability = "alpha" }, "stability must be"},
		{"bad reviewStatus", func(p *model.Pack) { p.ReviewStatus = "wip" }, "reviewStatus must be"},
		{"deprecated needs replacement", func(p *model.Pack) { p.Deprecated = true }, "replacement is required"},
		{"empty content", func(p *model.Pack) { p.Capabilities = nil }, "capabilities, packs, skills, or plugins is required"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := validPack()
			c.mutate(&p)
			errs := ValidatePack(p)
			if !containsSubstr(errs, c.wantSub) {
				t.Fatalf("expected error containing %q, got %v", c.wantSub, errs)
			}
		})
	}
}

func TestValidatePackCategoryAllowlistFallback(t *testing.T) {
	// With no schema dir, the canonical fallback list is used.
	good := validPack()
	good.Categories = []string{"backend", "engineering", "devex"}
	if errs := ValidatePack(good); len(errs) != 0 {
		t.Fatalf("valid categories should pass, got %v", errs)
	}

	bad := validPack()
	bad.Categories = []string{"backend", "research"}
	errs := ValidatePack(bad)
	if !containsSubstr(errs, `category "research" is not allowed`) {
		t.Fatalf("expected off-allowlist rejection, got %v", errs)
	}
	if !containsSubstr(errs, "valid categories:") {
		t.Fatalf("expected message to list valid categories, got %v", errs)
	}
	// Message should enumerate the canonical set (sorted).
	joined := strings.Join(errs, " ")
	for _, c := range canonicalCategories {
		if !strings.Contains(joined, c) {
			t.Fatalf("expected valid-category list to include %q, got %v", c, errs)
		}
	}
}

func TestValidatePackCategoryAllowlistFromSchema(t *testing.T) {
	dir := t.TempDir()
	schemaDir := filepath.Join(dir, "schemas")
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A schema whose enum is intentionally narrower than the canonical fallback,
	// to prove the CLI reads from the schema rather than the constant.
	schema := `{
      "properties": {
        "categories": {
          "items": { "enum": ["backend", "data"] }
        }
      }
    }`
	if err := os.WriteFile(filepath.Join(schemaDir, "agent-pack.schema.json"), []byte(schema), 0o644); err != nil {
		t.Fatal(err)
	}

	p := validPack()
	p.Categories = []string{"data"}
	if errs := ValidatePackWithSchemaDir(p, dir); len(errs) != 0 {
		t.Fatalf("category allowed by schema should pass, got %v", errs)
	}

	// "engineering" is in the canonical fallback but NOT in this schema's enum.
	p.Categories = []string{"engineering"}
	errs := ValidatePackWithSchemaDir(p, dir)
	if !containsSubstr(errs, `category "engineering" is not allowed`) {
		t.Fatalf("expected schema enum to reject 'engineering', got %v", errs)
	}
	if !containsSubstr(errs, "backend, data") {
		t.Fatalf("expected message to list schema enum values, got %v", errs)
	}
}

func TestAllowedCategoriesFallbackMatchesCanonical(t *testing.T) {
	got := AllowedCategories("")
	if len(got) != len(canonicalCategories) {
		t.Fatalf("fallback length mismatch: got %d want %d", len(got), len(canonicalCategories))
	}
	if len(canonicalCategories) != 14 {
		t.Fatalf("expected 14 canonical categories, got %d", len(canonicalCategories))
	}
}

func TestValidateCapability(t *testing.T) {
	cases := []struct {
		name    string
		cap     model.Capability
		wantSub string
	}{
		{"skill bad format", model.Capability{Type: "skill", Name: "n", Source: "s", Format: "other", Entry: "x"}, "format must be agent-skill"},
		{"skill missing entry", model.Capability{Type: "skill", Name: "n", Source: "s", Format: "agent-skill"}, "entry is required"},
		{"plugin missing install", model.Capability{Type: "plugin", Name: "n", Source: "s", Format: "anthropic-plugin"}, "install.method is required"},
		{"plugin command needs execution", model.Capability{Type: "plugin", Name: "n", Source: "s", Format: "anthropic-plugin", Install: map[string]string{"method": "shell", "command": "x"}}, "requiresExecution must be true"},
		{"missing name", model.Capability{Type: "skill", Source: "s", Format: "agent-skill", Entry: "x"}, ".name is required"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			errs := ValidateCapability(c.cap, "cap")
			if !containsSubstr(errs, c.wantSub) {
				t.Fatalf("expected %q, got %v", c.wantSub, errs)
			}
		})
	}
}

func TestValidateSkillManifest(t *testing.T) {
	ok := model.SkillManifest{Name: "my-skill", Description: "ok"}
	if errs := ValidateSkillManifest("my-skill", ok); len(errs) != 0 {
		t.Fatalf("expected valid skill manifest, got %v", errs)
	}
	mismatch := model.SkillManifest{Name: "my-skill", Description: "ok"}
	if errs := ValidateSkillManifest("other-dir", mismatch); !containsSubstr(errs, "must match parent directory") {
		t.Fatalf("expected directory-name mismatch error, got %v", errs)
	}
	emptyDesc := model.SkillManifest{Name: "my-skill", Description: ""}
	if errs := ValidateSkillManifest("my-skill", emptyDesc); !containsSubstr(errs, "description must be") {
		t.Fatalf("expected description error, got %v", errs)
	}
}

func TestValidatePathRejectsBadCategory(t *testing.T) {
	dir := t.TempDir()
	schemaDir := filepath.Join(dir, "schemas")
	packDir := filepath.Join(dir, "packs")
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		t.Fatal(err)
	}
	schema := `{"properties":{"categories":{"items":{"enum":["backend","data","ml"]}}}}`
	if err := os.WriteFile(filepath.Join(schemaDir, "agent-pack.schema.json"), []byte(schema), 0o644); err != nil {
		t.Fatal(err)
	}
	packJSON := `{
      "id": "bad-cat",
      "name": "Bad Cat",
      "version": "1.0.0",
      "description": "d",
      "categories": ["backend", "research"],
      "capabilities": [{"type":"skill","name":"s","source":"/tmp/s","format":"agent-skill","entry":"SKILL.md"}]
    }`
	packPath := filepath.Join(packDir, "bad-cat.json")
	if err := os.WriteFile(packPath, []byte(packJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	err := ValidatePath(packPath, &out)
	if err == nil {
		t.Fatal("expected ValidatePath to fail for off-allowlist category")
	}
	if !strings.Contains(out.String(), `category "research" is not allowed`) {
		t.Fatalf("expected category error in output, got: %s", out.String())
	}
}

func packFromJSON(t *testing.T, raw string) model.Pack {
	t.Helper()
	var p model.Pack
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal pack: %v", err)
	}
	return p
}

func TestValidatePackTrustObjectRefValid(t *testing.T) {
	p := packFromJSON(t, `{
      "id": "trust-ok",
      "name": "Trust OK",
      "version": "1.0.0",
      "description": "d",
      "skills": [{"id": "s1", "source": "https://example.com/s", "format": "agent-skill", "trust": "community"}],
      "plugins": [{"id": "p1", "source": "https://example.com/p", "format": "anthropic-plugin", "trust": "official"}]
    }`)
	if errs := ValidatePack(p); len(errs) != 0 {
		t.Fatalf("expected no errors for valid trust, got %v", errs)
	}
}

func TestValidatePackTrustMissingOnObjectRef(t *testing.T) {
	p := packFromJSON(t, `{
      "id": "trust-missing",
      "name": "Trust Missing",
      "version": "1.0.0",
      "description": "d",
      "skills": [{"id": "s1", "source": "https://example.com/s", "format": "agent-skill"}]
    }`)
	errs := ValidatePack(p)
	if !containsSubstr(errs, "skills[0].trust is required") {
		t.Fatalf("expected missing-trust error, got %v", errs)
	}
	if !containsSubstr(errs, "valid values:") {
		t.Fatalf("expected message to list valid trust values, got %v", errs)
	}
	joined := strings.Join(errs, " ")
	for _, level := range canonicalTrustLevels {
		if !strings.Contains(joined, level) {
			t.Fatalf("expected valid-trust list to include %q, got %v", level, errs)
		}
	}
}

func TestValidatePackTrustOutsideEnum(t *testing.T) {
	p := packFromJSON(t, `{
      "id": "trust-bad",
      "name": "Trust Bad",
      "version": "1.0.0",
      "description": "d",
      "plugins": [{"id": "p1", "source": "https://example.com/p", "format": "anthropic-plugin", "trust": "trusted"}]
    }`)
	errs := ValidatePack(p)
	if !containsSubstr(errs, `plugins[0].trust "trusted" is not allowed`) {
		t.Fatalf("expected off-enum trust rejection, got %v", errs)
	}
}

func TestValidatePackBareStringRefExemptFromTrust(t *testing.T) {
	// Bare-string refs carry no provenance metadata and must remain valid
	// without a trust value (matching the schema's oneOf[string, object]).
	p := packFromJSON(t, `{
      "id": "bare-ok",
      "name": "Bare OK",
      "version": "1.0.0",
      "description": "d",
      "skills": ["frontend-implementation-guidance"]
    }`)
	if errs := ValidatePack(p); len(errs) != 0 {
		t.Fatalf("expected bare-string skill ref to pass without trust, got %v", errs)
	}
}

func TestValidatePackTrustFromSchema(t *testing.T) {
	dir := t.TempDir()
	schemaDir := filepath.Join(dir, "schemas")
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A schema whose trust enum is intentionally narrower than the canonical
	// fallback, to prove the CLI reads the enum from the schema.
	schema := `{
      "$defs": {
        "skillRef": {
          "oneOf": [
            {"type": "string"},
            {"type": "object", "required": ["id","trust"], "properties": {"trust": {"enum": ["official"]}}}
          ]
        },
        "pluginRef": {
          "oneOf": [
            {"type": "string"},
            {"type": "object", "required": ["id","trust"], "properties": {"trust": {"enum": ["official"]}}}
          ]
        }
      }
    }`
	if err := os.WriteFile(filepath.Join(schemaDir, "agent-pack.schema.json"), []byte(schema), 0o644); err != nil {
		t.Fatal(err)
	}

	p := packFromJSON(t, `{
      "id": "schema-trust",
      "name": "Schema Trust",
      "version": "1.0.0",
      "description": "d",
      "skills": [{"id": "s1", "source": "https://example.com/s", "format": "agent-skill", "trust": "community"}]
    }`)
	// "community" is in the canonical fallback but NOT in this schema's enum.
	errs := ValidatePackWithSchemaDir(p, dir)
	if !containsSubstr(errs, `skills[0].trust "community" is not allowed`) {
		t.Fatalf("expected schema enum to reject 'community', got %v", errs)
	}
	if !containsSubstr(errs, "official") {
		t.Fatalf("expected message to list schema enum value, got %v", errs)
	}
}

func TestAllowedTrustLevelsFallbackMatchesCanonical(t *testing.T) {
	got := AllowedTrustLevels("")
	if len(got) != len(canonicalTrustLevels) {
		t.Fatalf("fallback length mismatch: got %d want %d", len(got), len(canonicalTrustLevels))
	}
}

func TestValidatePathRejectsMissingTrust(t *testing.T) {
	dir := t.TempDir()
	packDir := filepath.Join(dir, "packs")
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		t.Fatal(err)
	}
	packJSON := `{
      "id": "no-trust",
      "name": "No Trust",
      "version": "1.0.0",
      "description": "d",
      "plugins": [{"id": "p1", "source": "https://example.com/p", "format": "anthropic-plugin"}]
    }`
	packPath := filepath.Join(packDir, "no-trust.json")
	if err := os.WriteFile(packPath, []byte(packJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := ValidatePath(packPath, &out); err == nil {
		t.Fatal("expected ValidatePath to fail for object ref missing trust")
	}
	if !strings.Contains(out.String(), "plugins[0].trust is required") {
		t.Fatalf("expected trust error in output, got: %s", out.String())
	}
}

func TestPublishReportIncludesNonBlockingMetadataCoverage(t *testing.T) {
	root := t.TempDir()
	packDir := filepath.Join(root, "packs")
	for _, dir := range []string{packDir, filepath.Join(root, "skills"), filepath.Join(root, "plugins"), filepath.Join(root, "schemas", "examples")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	packJSON := `{
      "id": "metadata-warn",
      "name": "Metadata Warn",
      "version": "1.0.0",
      "description": "d",
      "lastVerified": "2026-06-01",
      "capabilities": [{"type":"skill","name":"s","source":"/tmp/s","format":"agent-skill","entry":"SKILL.md"}]
    }`
	if err := os.WriteFile(filepath.Join(packDir, "metadata-warn.json"), []byte(packJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := PublishReport(packDir, "")
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("metadata warnings should not fail publish report: %#v", report.Checks)
	}
	if report.Metadata == nil {
		t.Fatal("expected metadata coverage report")
	}
	if report.Metadata.Fields[0].Present != 0 || report.Metadata.Fields[0].Total != 1 {
		t.Fatalf("unexpected metadata field coverage: %#v", report.Metadata.Fields[0])
	}
	if !hasCheck(report.Checks, "metadata", "requirements", "WARN") {
		t.Fatalf("expected non-blocking requirements warning in checks: %#v", report.Checks)
	}
}

func hasCheck(checks []model.CheckEntry, kind, target, status string) bool {
	for _, check := range checks {
		if check.Kind == kind && check.Target == target && check.Status == status {
			return true
		}
	}
	return false
}

func containsSubstr(errs []string, sub string) bool {
	for _, e := range errs {
		if strings.Contains(e, sub) {
			return true
		}
	}
	return false
}

func TestValidatePackBlockedKeys(t *testing.T) {
	// Case 1: Plain variable names in description and setup instructions should be allowed.
	p := validPack()
	p.Description = "This pack requires XQUIK_API_KEY or OPENAI_API_KEY to run. Please set it in your environment."
	errs := ValidatePack(p)
	if len(errs) != 0 {
		t.Fatalf("expected plain env var names in description to be allowed, got: %v", errs)
	}

	// Case 2: Env variable set to placeholder value should be allowed.
	p2 := validPack()
	p2.Capabilities[0].Env = map[string]string{
		"OPENAI_API_KEY": "<your-openai-api-key>",
		"PORT":           "8080",
	}
	errs2 := ValidatePack(p2)
	if len(errs2) != 0 {
		t.Fatalf("expected env variables with placeholder/standard values to be allowed, got: %v", errs2)
	}

	// Case 2b: Env variable set to an environment variable reference/interpolation should be allowed.
	p2b := validPack()
	p2b.Capabilities[0].Env = map[string]string{
		"OPENAI_API_KEY": "${OPENAI_API_KEY}",
		"ANOTHER_KEY":    "$ANOTHER_KEY",
		"WIN_KEY":        "%WIN_KEY%",
	}
	errs2b := ValidatePack(p2b)
	if len(errs2b) != 0 {
		t.Fatalf("expected env variables with references to be allowed, got: %v", errs2b)
	}

	// Case 3: Env variable set to actual credential value should be blocked, and the output must be redacted.
	p3 := validPack()
	p3.Capabilities[0].Env = map[string]string{
		"OPENAI_API_KEY": "sk-proj-somekeyvalue12345",
	}
	errs3 := ValidatePack(p3)
	if !containsSubstr(errs3, "contains a literal credentials value") {
		t.Fatalf("expected env variable with literal key to be blocked, got: %v", errs3)
	}
	// Assert that it's redacted
	if containsSubstr(errs3, "somekeyvalue12345") {
		t.Fatalf("expected env validation error to redact the secret, but found it in: %v", errs3)
	}
	if !containsSubstr(errs3, "sk-...2345") {
		t.Fatalf("expected env validation error to contain redacted fingerprint 'sk-...2345', got: %v", errs3)
	}

	// Case 4: Literal secrets in settings should be blocked and redacted.
	p4 := validPack()
	p4.Capabilities[0].Type = "settings"
	p4.Capabilities[0].Format = "other"
	p4.Capabilities[0].Content = `{"secretToken": "ghp_abcdefghijklmnopqrstuvwxyz1234567890"}`
	errs4 := ValidatePack(p4)
	if !containsSubstr(errs4, "contains a secret-looking value") && !containsSubstr(errs4, "contains a literal credentials value") {
		t.Fatalf("expected settings with literal secret token to be blocked, got: %v", errs4)
	}
	if containsSubstr(errs4, "abcdefghijklmnopqrstuvwxyz") {
		t.Fatalf("expected settings validation error to redact the secret, but found it in: %v", errs4)
	}
	if !containsSubstr(errs4, "ghp_...7890") {
		t.Fatalf("expected settings validation error to contain redacted fingerprint 'ghp_...7890', got: %v", errs4)
	}

	// Case 5: Inline content for non-settings (like commands/hooks) containing secrets should be blocked.
	p5 := validPack()
	p5.Capabilities[0].Type = "command"
	p5.Capabilities[0].Format = "shell-command"
	p5.Capabilities[0].Content = "curl -H 'Authorization: Bearer sk-proj-supersecretkeyinplain1234'"
	errs5 := ValidatePack(p5)
	if !containsSubstr(errs5, "contains a blocked secret") {
		t.Fatalf("expected command with inline secret content to be blocked, got: %v", errs5)
	}
	if containsSubstr(errs5, "supersecretkey") {
		t.Fatalf("expected command validation error to redact the secret, but found it in: %v", errs5)
	}
}


