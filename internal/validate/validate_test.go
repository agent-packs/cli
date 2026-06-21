package validate

import (
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

func containsSubstr(errs []string, sub string) bool {
	for _, e := range errs {
		if strings.Contains(e, sub) {
			return true
		}
	}
	return false
}
