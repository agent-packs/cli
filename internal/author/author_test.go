package author

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agent-packs/cli/internal/model"
	"github.com/agent-packs/cli/internal/validate"
)

func TestNewScaffoldsValidCapabilities(t *testing.T) {
	for _, kind := range []string{"command", "hook", "memory", "settings", "subagent", "prompt", "template", "tool"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			path, err := New(NewOptions{Kind: kind, ID: "demo-" + kind, Dir: dir})
			if err != nil {
				t.Fatalf("New(%s): %v", kind, err)
			}
			if want := filepath.Join(dir, "demo-"+kind+".json"); path != want {
				t.Fatalf("path = %q, want %q", path, want)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var capability model.Capability
			if err := json.Unmarshal(data, &capability); err != nil {
				t.Fatalf("scaffolded %s is not valid JSON: %v", kind, err)
			}
			if capability.Type != kind {
				t.Fatalf("type = %q, want %q", capability.Type, kind)
			}
			if errs := validate.ValidateCapability(capability, "capability"); len(errs) != 0 {
				t.Fatalf("scaffolded %s failed validation: %v", kind, errs)
			}
		})
	}
}

func TestNewPackScaffoldIncludesContributorQualityMetadata(t *testing.T) {
	dir := t.TempDir()
	path, err := New(NewOptions{Kind: "pack", ID: "demo-pack", Name: "Demo Pack", Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var pack model.Pack
	if err := json.Unmarshal(data, &pack); err != nil {
		t.Fatalf("scaffolded pack is not valid JSON: %v", err)
	}
	if len(pack.Categories) == 0 {
		t.Fatalf("expected scaffolded pack categories, got %#v", pack)
	}
	if len(pack.Requirements.Tools) == 0 {
		t.Fatalf("expected scaffolded pack tool requirements, got %#v", pack.Requirements)
	}
	if len(pack.UseCases) == 0 || len(pack.ExamplePrompts) == 0 {
		t.Fatalf("expected scaffolded use cases and example prompts, got %#v / %#v", pack.UseCases, pack.ExamplePrompts)
	}
	if len(pack.Skills) != 1 || !pack.Skills[0].IsObjectRef() || pack.Skills[0].Trust != "community" {
		t.Fatalf("expected trust-bearing object skill ref, got %#v", pack.Skills)
	}
	if errs := validate.ValidatePack(pack); len(errs) != 0 {
		t.Fatalf("scaffolded pack failed validation: %v", errs)
	}
}

func TestNewUnknownKind(t *testing.T) {
	if _, err := New(NewOptions{Kind: "bogus", ID: "x", Dir: t.TempDir()}); err == nil {
		t.Fatal("expected error for unknown kind")
	}
}
