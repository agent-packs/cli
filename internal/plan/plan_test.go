package plan

import (
	"path/filepath"
	"testing"

	"github.com/agent-packs/cli/internal/model"
)

func skillCapability(name, source string) model.Capability {
	return model.Capability{Type: "skill", Name: name, Source: source, Format: "agent-skill", Entry: "SKILL.md"}
}

func pluginCapability(name string) model.Capability {
	return model.Capability{
		Type:   "plugin",
		Name:   name,
		Source: "https://example.com/plugin",
		Format: "anthropic-plugin",
		Install: map[string]string{
			"method":  "shell",
			"command": "install.sh",
		},
	}
}

func TestBuildInstallPlanSkillModes(t *testing.T) {
	cases := []struct {
		name       string
		mode       string
		source     string
		wantAction string
		wantDest   bool
	}{
		{"reference local", "reference", "/tmp/local-skill", "reference", false},
		{"symlink local", "symlink", "/tmp/local-skill", "symlink", true},
		{"copy local", "copy", "/tmp/local-skill", "copy", true},
		{"copy remote fetches", "copy", "https://example.com/skill", "fetch-copy", true},
		{"native acts as reference for skills", "native", "/tmp/local-skill", "reference", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pack := model.Pack{ID: "p", Version: "1.0.0", Capabilities: []model.Capability{skillCapability("My Skill", c.source)}}
			p := BuildInstallPlanWithOptions(pack, "/target", "claude", "all", model.InstallOptions{Mode: c.mode, OnConflict: "overwrite", Scope: "target"})
			if len(p.Capabilities) != 1 {
				t.Fatalf("expected 1 capability, got %d", len(p.Capabilities))
			}
			item := p.Capabilities[0]
			if item.Action != c.wantAction {
				t.Fatalf("mode %q: want action %q, got %q", c.mode, c.wantAction, item.Action)
			}
			if c.wantDest {
				want := filepath.Join("/target", ".claude/skills", "my-skill")
				if item.Destination != want {
					t.Fatalf("mode %q: want destination %q, got %q", c.mode, want, item.Destination)
				}
			} else if item.Destination != "" {
				t.Fatalf("mode %q: expected empty destination, got %q", c.mode, item.Destination)
			}
		})
	}
}

func TestBuildInstallPlanProjectScopeUsesProjectRoot(t *testing.T) {
	pack := model.Pack{ID: "p", Version: "1.0.0", Capabilities: []model.Capability{skillCapability("My Skill", "/tmp/local-skill")}}
	p := BuildInstallPlanWithOptions(pack, "/project", "codex", "all", model.InstallOptions{Mode: "copy", OnConflict: "overwrite", Scope: "project"})
	want := filepath.Join("/project", ".agents/skills", "my-skill")
	if p.Capabilities[0].Destination != want {
		t.Fatalf("want project destination %q, got %q", want, p.Capabilities[0].Destination)
	}
}

func TestBuildInstallPlanPluginActions(t *testing.T) {
	pack := model.Pack{ID: "p", Version: "1.0.0", Capabilities: []model.Capability{pluginCapability("ci-plugin")}}

	ref := BuildInstallPlanWithOptions(pack, "/target", "claude", "all", model.InstallOptions{Mode: "reference", OnConflict: "skip", Scope: "target"})
	if ref.Capabilities[0].Action != "reference" {
		t.Fatalf("reference mode: want plugin action reference, got %q", ref.Capabilities[0].Action)
	}

	native := BuildInstallPlanWithOptions(pack, "/target", "claude", "all", model.InstallOptions{Mode: "native", OnConflict: "skip", Scope: "target"})
	if native.Capabilities[0].Action != "native-install" {
		t.Fatalf("native mode: want plugin action native-install, got %q", native.Capabilities[0].Action)
	}
	if native.Capabilities[0].Command != "install.sh" || native.Capabilities[0].Method != "shell" {
		t.Fatalf("plugin install metadata not mapped: %+v", native.Capabilities[0])
	}
}

func TestBuildInstallPlanPluginReferenceFlagStaysReference(t *testing.T) {
	cap := pluginCapability("ci-plugin")
	cap.Reference = true
	pack := model.Pack{ID: "p", Version: "1.0.0", Capabilities: []model.Capability{cap}}
	p := BuildInstallPlanWithOptions(pack, "/target", "claude", "all", model.InstallOptions{Mode: "copy", OnConflict: "skip", Scope: "target"})
	if p.Capabilities[0].Action != "reference" {
		t.Fatalf("plugin with Reference=true should stay reference, got %q", p.Capabilities[0].Action)
	}
}

func TestBuildInstallPlanOnlyFilter(t *testing.T) {
	pack := model.Pack{
		ID:      "p",
		Version: "1.0.0",
		Capabilities: []model.Capability{
			skillCapability("skill-a", "/tmp/a"),
			pluginCapability("plugin-b"),
			{Type: "command", Name: "cmd-c", Source: "/tmp/c"},
		},
	}
	cases := []struct {
		only      string
		wantTypes []string
	}{
		{"all", []string{"skill", "plugin", "command"}},
		{"skills", []string{"skill"}},
		{"plugins", []string{"plugin"}},
	}
	for _, c := range cases {
		t.Run(c.only, func(t *testing.T) {
			p := BuildInstallPlanWithOptions(pack, "/target", "claude", c.only, model.InstallOptions{Mode: "reference", OnConflict: "skip", Scope: "target"})
			if len(p.Capabilities) != len(c.wantTypes) {
				t.Fatalf("only=%q: want %d capabilities, got %d", c.only, len(c.wantTypes), len(p.Capabilities))
			}
			for i, want := range c.wantTypes {
				if p.Capabilities[i].Type != want {
					t.Fatalf("only=%q: capability %d want type %q, got %q", c.only, i, want, p.Capabilities[i].Type)
				}
			}
		})
	}
}

func TestBuildInstallPlanDefaultsMode(t *testing.T) {
	// BuildInstallPlanWithOptions normalizes empty options to reference/skip/target.
	pack := model.Pack{ID: "p", Version: "1.0.0", Capabilities: []model.Capability{skillCapability("s", "/tmp/s")}}
	p := BuildInstallPlanWithOptions(pack, "/target", "claude", "all", model.InstallOptions{})
	if p.Mode != "reference" || p.OnConflict != "skip" || p.Scope != "target" {
		t.Fatalf("defaults not applied: mode=%q conflict=%q scope=%q", p.Mode, p.OnConflict, p.Scope)
	}
	if p.Capabilities[0].Action != "reference" {
		t.Fatalf("default mode should reference, got %q", p.Capabilities[0].Action)
	}
}

func memoryCapability(name, content string) model.Capability {
	return model.Capability{Type: "memory", Name: name, Content: content}
}

func TestBuildInstallPlanMemoryMapsToMemoryFile(t *testing.T) {
	pack := model.Pack{ID: "mypack", Version: "1.0.0", Capabilities: []model.Capability{memoryCapability("House Rules", "Use tabs.")}}
	p := BuildInstallPlanWithOptions(pack, "/target", "claude", "all", model.InstallOptions{Mode: "copy", OnConflict: "overwrite", Scope: "project"})
	item := p.Capabilities[0]
	if item.Action != "merge" {
		t.Fatalf("want action merge, got %q", item.Action)
	}
	if want := filepath.Join("/target", "CLAUDE.md"); item.Destination != want {
		t.Fatalf("want destination %q, got %q", want, item.Destination)
	}
	if item.FileKind != "markdown" {
		t.Fatalf("want fileKind markdown, got %q", item.FileKind)
	}
	if item.BlockID != "mypack/house-rules" {
		t.Fatalf("want blockID mypack/house-rules, got %q", item.BlockID)
	}
}

func TestBuildInstallPlanSettingsMapsToJSON(t *testing.T) {
	cap := model.Capability{Type: "settings", Name: "model", Content: `{"model":"opus"}`}
	pack := model.Pack{ID: "p", Version: "1.0.0", Capabilities: []model.Capability{cap}}
	p := BuildInstallPlanWithOptions(pack, "/target", "claude", "all", model.InstallOptions{Mode: "copy", OnConflict: "overwrite", Scope: "target"})
	item := p.Capabilities[0]
	if item.FileKind != "json" {
		t.Fatalf("want fileKind json, got %q", item.FileKind)
	}
	if want := filepath.Join("/target", ".claude/settings.json"); item.Destination != want {
		t.Fatalf("want destination %q, got %q", want, item.Destination)
	}
}

func TestBuildInstallPlanMergeReferenceModeRecordsOnly(t *testing.T) {
	pack := model.Pack{ID: "p", Version: "1.0.0", Capabilities: []model.Capability{memoryCapability("m", "body")}}
	p := BuildInstallPlanWithOptions(pack, "/target", "claude", "all", model.InstallOptions{Mode: "reference", OnConflict: "skip", Scope: "target"})
	item := p.Capabilities[0]
	if item.Action != "record" {
		t.Fatalf("reference mode should record, got %q", item.Action)
	}
	if item.Destination != "" {
		t.Fatalf("record mode should not set a destination, got %q", item.Destination)
	}
}

func TestBuildInstallPlanUnsupportedPairSkips(t *testing.T) {
	// cursor has no settings destination wired -> skip+unsupported.
	cap := model.Capability{Type: "settings", Name: "s", Content: `{"x":1}`}
	pack := model.Pack{ID: "p", Version: "1.0.0", Capabilities: []model.Capability{cap}}
	p := BuildInstallPlanWithOptions(pack, "/target", "cursor", "all", model.InstallOptions{Mode: "copy", OnConflict: "skip", Scope: "target"})
	item := p.Capabilities[0]
	if item.Action != "skip" || item.Status != "unsupported" {
		t.Fatalf("want skip/unsupported, got action=%q status=%q", item.Action, item.Status)
	}
	if item.Destination != "" {
		t.Fatalf("unsupported item must not have a destination, got %q", item.Destination)
	}
}

func TestBuildInstallPlanOnlyFilterNewTypes(t *testing.T) {
	pack := model.Pack{
		ID: "p", Version: "1.0.0",
		Capabilities: []model.Capability{
			skillCapability("skill-a", "/tmp/a"),
			memoryCapability("mem-b", "body"),
			{Type: "settings", Name: "set-c", Content: `{"x":1}`},
		},
	}
	for _, c := range []struct {
		only string
		want string
	}{
		{"memory", "memory"},
		{"settings", "settings"},
	} {
		p := BuildInstallPlanWithOptions(pack, "/target", "claude", c.only, model.InstallOptions{Mode: "reference", OnConflict: "skip", Scope: "target"})
		if len(p.Capabilities) != 1 || p.Capabilities[0].Type != c.want {
			t.Fatalf("only=%q: want single %q capability, got %+v", c.only, c.want, p.Capabilities)
		}
	}
}

func TestBuildInstallPlanCommandCapabilityRecords(t *testing.T) {
	pack := model.Pack{ID: "p", Version: "1.0.0", Capabilities: []model.Capability{{Type: "prompt", Name: "pr", Source: "/tmp/pr"}}}
	p := BuildInstallPlanWithOptions(pack, "/target", "claude", "all", model.InstallOptions{Mode: "copy", OnConflict: "skip", Scope: "target"})
	if p.Capabilities[0].Action != "record" {
		t.Fatalf("non-skill/non-plugin capability should record, got %q", p.Capabilities[0].Action)
	}
}
