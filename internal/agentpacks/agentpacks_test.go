package agentpacks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildInstallPlanTargetsCodexSkills(t *testing.T) {
	pack := testPack("/tmp/example-skill")
	plan := BuildInstallPlanWithOptions(pack, "/tmp/target", "codex", "skills", InstallOptions{Mode: "copy", OnConflict: "overwrite"})

	if len(plan.Capabilities) != 1 {
		t.Fatalf("expected 1 capability, got %d", len(plan.Capabilities))
	}
	item := plan.Capabilities[0]
	if item.Type != "skill" {
		t.Fatalf("expected skill, got %s", item.Type)
	}
	if item.Action != "copy" {
		t.Fatalf("expected copy action, got %s", item.Action)
	}
	if !strings.HasSuffix(item.Destination, filepath.Join(".codex", "skills", "example-skill")) {
		t.Fatalf("unexpected destination: %s", item.Destination)
	}
}

func TestProjectScopeUsesTargetMatrix(t *testing.T) {
	pack := testPack("/tmp/example-skill")
	plan := BuildInstallPlanWithOptions(pack, "/tmp/project", "codex", "skills", InstallOptions{Mode: "copy", OnConflict: "overwrite", Scope: "project"})
	if len(plan.Capabilities) != 1 {
		t.Fatalf("expected 1 capability, got %d", len(plan.Capabilities))
	}
	if !strings.HasSuffix(plan.Capabilities[0].Destination, filepath.Join(".agents", "skills", "example-skill")) {
		t.Fatalf("unexpected project destination: %s", plan.Capabilities[0].Destination)
	}
}

func TestPrintTargetMatrixIncludesCodex(t *testing.T) {
	var output strings.Builder
	if err := PrintTargetMatrix(&output); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.Contains(text, "codex") || !strings.Contains(text, ".agents/skills") {
		t.Fatalf("target matrix missing codex project mapping: %s", text)
	}
}

func TestExecutePlanInstallsLocalSkill(t *testing.T) {
	temp := t.TempDir()
	skill := filepath.Join(temp, "skill")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("# Example Skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pack := testPack(skill)
	plan := BuildInstallPlanWithOptions(pack, filepath.Join(temp, "target"), "codex", "skills", InstallOptions{Mode: "copy", OnConflict: "overwrite"})
	result := ExecutePlan(plan, false)
	item := result.Capabilities[0]

	if item.Status != "installed" {
		t.Fatalf("expected installed, got %s: %s", item.Status, item.Reason)
	}
	installed := filepath.Join(temp, "target", ".codex", "skills", "example-skill", "SKILL.md")
	data, err := os.ReadFile(installed)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# Example Skill\n" {
		t.Fatalf("unexpected installed skill content: %q", string(data))
	}
}

func TestWriteReceipt(t *testing.T) {
	temp := t.TempDir()
	pack := testPack("/tmp/example-skill")
	plan := BuildInstallPlanWithOptions(pack, temp, "generic", "plugins", InstallOptions{Mode: "copy", OnConflict: "overwrite"})
	result := ExecutePlan(plan, false)

	receiptPath, err := WriteReceipt(temp, pack, result)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipt Receipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Pack.ID != "example" {
		t.Fatalf("unexpected receipt pack id: %s", receipt.Pack.ID)
	}
	if receipt.Plan.Capabilities[0].Status != "pending" {
		t.Fatalf("expected pending plugin, got %s", receipt.Plan.Capabilities[0].Status)
	}
}

func TestExpandPackComposesCapabilities(t *testing.T) {
	temp := t.TempDir()
	registry := filepath.Join(temp, "packs")
	if err := os.MkdirAll(registry, 0o755); err != nil {
		t.Fatal(err)
	}
	child := testPack("/tmp/example-skill")
	child.ID = "child"
	parent := Pack{ID: "parent", Name: "Parent", Version: "0.1.0", Description: "Parent pack", Packs: []string{"child"}}
	writeTestPack(t, registry, child)
	writeTestPack(t, registry, parent)

	loaded, err := FindPack(registry, "parent")
	if err != nil {
		t.Fatal(err)
	}
	expanded, err := ExpandPack(registry, loaded, map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if len(expanded.Capabilities) != len(child.Capabilities) {
		t.Fatalf("expected child capabilities, got %d", len(expanded.Capabilities))
	}
}

func TestExpandPackIncludesRegistrySkillsAndPlugins(t *testing.T) {
	temp := t.TempDir()
	registry := filepath.Join(temp, "registry")
	if err := os.MkdirAll(filepath.Join(registry, "packs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(registry, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(registry, "plugins"), 0o755); err != nil {
		t.Fatal(err)
	}
	pack := Pack{ID: "referenced", Name: "Referenced", Version: "0.1.0", Description: "Referenced pack", Skills: CapabilityRefs{{ID: "review"}}, Plugins: CapabilityRefs{{ID: "browser"}}}
	writeTestPack(t, filepath.Join(registry, "packs"), pack)
	writeTestSkill(t, filepath.Join(registry, "skills"), "review")
	writeTestPlugin(t, filepath.Join(registry, "plugins"), "browser")

	loaded, err := FindPack(filepath.Join(registry, "packs"), "referenced")
	if err != nil {
		t.Fatal(err)
	}
	expanded, err := ExpandPack(filepath.Join(registry, "packs"), loaded, map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if len(expanded.Capabilities) != 2 {
		t.Fatalf("expected 2 referenced capabilities, got %d", len(expanded.Capabilities))
	}
	if expanded.Capabilities[0].Name != "review" || expanded.Capabilities[1].Name != "Browser Tool" {
		t.Fatalf("unexpected capabilities: %#v", expanded.Capabilities)
	}

	plan := BuildInstallPlanWithOptions(expanded, filepath.Join(temp, "target"), "codex", "all", InstallOptions{Mode: "reference", OnConflict: "skip"})
	if len(plan.Capabilities) != 2 {
		t.Fatalf("expected 2 planned capabilities, got %d", len(plan.Capabilities))
	}
	for _, item := range plan.Capabilities {
		if item.Action != "reference" {
			t.Fatalf("expected registry capability to be referenced, got %#v", item)
		}
		if item.Destination != "" {
			t.Fatalf("referenced capability should not have destination: %#v", item)
		}
	}

	result := ExecutePlan(plan, false)
	for _, item := range result.Capabilities {
		if item.Status != "referenced" {
			t.Fatalf("expected referenced status, got %#v", item)
		}
	}
}

func TestExpandPackIncludesRemoteSkillAndPluginRefs(t *testing.T) {
	temp := t.TempDir()
	registry := filepath.Join(temp, "registry", "packs")
	if err := os.MkdirAll(registry, 0o755); err != nil {
		t.Fatal(err)
	}
	pack := Pack{
		ID:          "remote-refs",
		Name:        "Remote Refs",
		Version:     "0.1.0",
		Description: "Remote references pack",
		Skills: CapabilityRefs{{
			ID:     "strategy",
			Name:   "Strategy Skill",
			Source: "https://github.com/example/skills/tree/main/strategy",
		}},
		Plugins: CapabilityRefs{{
			ID:     "review-plugin",
			Source: "https://github.com/example/plugins/tree/main/review",
			Format: "anthropic-plugin",
		}},
	}
	writeTestPack(t, registry, pack)

	loaded, err := FindPack(registry, "remote-refs")
	if err != nil {
		t.Fatal(err)
	}
	expanded, err := ExpandPack(registry, loaded, map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if len(expanded.Capabilities) != 2 {
		t.Fatalf("expected 2 remote ref capabilities, got %d", len(expanded.Capabilities))
	}
	skill := expanded.Capabilities[0]
	if skill.Type != "skill" || !skill.Reference || skill.Source != "https://github.com/example/skills/tree/main/strategy" {
		t.Fatalf("unexpected remote skill capability: %#v", skill)
	}
	plugin := expanded.Capabilities[1]
	if plugin.Type != "plugin" || !plugin.Reference || plugin.Entry != ".claude-plugin/plugin.json" || plugin.Install["method"] != "manual" {
		t.Fatalf("unexpected remote plugin capability: %#v", plugin)
	}

	plan := BuildInstallPlanWithOptions(expanded, filepath.Join(temp, "target"), "codex", "all", InstallOptions{Mode: "reference", OnConflict: "skip"})
	for _, item := range plan.Capabilities {
		if item.Action != "reference" || item.Destination != "" {
			t.Fatalf("remote refs should be reference-only: %#v", item)
		}
	}
}

func TestResolveSourceClassifiesPinnedAndMovingGitHubRefs(t *testing.T) {
	pinned := ResolveSource("https://github.com/example/repo/tree/0123456789abcdef0123456789abcdef01234567/skills/foo")
	if !pinned.Pinned || pinned.Warning != "" || pinned.Revision != "0123456789abcdef0123456789abcdef01234567" {
		t.Fatalf("unexpected pinned resolution: %#v", pinned)
	}
	moving := ResolveSource("https://github.com/example/repo/tree/main/skills/foo")
	if moving.Pinned || !strings.Contains(moving.Warning, "moving ref") {
		t.Fatalf("unexpected moving resolution: %#v", moving)
	}
}

func TestVerifyWarnsForMovingRefs(t *testing.T) {
	temp := t.TempDir()
	registry := filepath.Join(temp, "packs")
	if err := os.MkdirAll(registry, 0o755); err != nil {
		t.Fatal(err)
	}
	pack := Pack{
		ID:          "moving",
		Name:        "Moving",
		Version:     "0.1.0",
		Description: "Moving refs",
		Skills:      CapabilityRefs{{ID: "moving-skill", Source: "https://github.com/example/repo/tree/main/skills/foo"}},
	}
	writeTestPack(t, registry, pack)
	var output strings.Builder
	if err := Verify(registry, "moving", &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "moving ref") {
		t.Fatalf("expected moving ref warning, got %s", output.String())
	}
}

func TestRegistryConfigRoundTrip(t *testing.T) {
	home := t.TempDir()
	if err := RegistryAdd(home, "local", "/tmp/registry"); err != nil {
		t.Fatal(err)
	}
	config, err := LoadRegistryConfig(home)
	if err != nil {
		t.Fatal(err)
	}
	if config.Registries["local"] != "/tmp/registry" {
		t.Fatalf("registry not saved: %#v", config.Registries)
	}
	if err := RegistryRemove(home, "local"); err != nil {
		t.Fatal(err)
	}
	config, err = LoadRegistryConfig(home)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := config.Registries["local"]; ok {
		t.Fatal("registry was not removed")
	}
}

func TestValidatePackRequiresExecutionForPluginCommands(t *testing.T) {
	pack := testPack("/tmp/example-skill")
	pack.Capabilities[1].RequiresExecution = false
	errors := ValidatePack(pack)
	joined := strings.Join(errors, "\n")
	if !strings.Contains(joined, "requiresExecution") {
		t.Fatalf("expected requiresExecution validation error, got %v", errors)
	}
}

func TestValidatePackRequiresExecutionForPluginUninstallCommands(t *testing.T) {
	pack := testPack("/tmp/example-skill")
	pack.Capabilities[1].RequiresExecution = false
	pack.Capabilities[1].Install["command"] = ""
	pack.Capabilities[1].Install["uninstall"] = "echo uninstall-plugin"
	errors := ValidatePack(pack)
	joined := strings.Join(errors, "\n")
	if !strings.Contains(joined, "install.uninstall") {
		t.Fatalf("expected install.uninstall validation error, got %v", errors)
	}
}

func TestWriteLockfile(t *testing.T) {
	temp := t.TempDir()
	pack := testPack("/tmp/example-skill")
	if err := WriteLockfile(temp, pack); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(temp, "agent-pack.lock"))
	if err != nil {
		t.Fatal(err)
	}
	var lock Lockfile
	if err := json.Unmarshal(data, &lock); err != nil {
		t.Fatal(err)
	}
	if lock.Pack != "example" || len(lock.Capabilities) != 2 {
		t.Fatalf("unexpected lockfile: %#v", lock)
	}
	if !strings.HasPrefix(lock.Capabilities[0].Digest, "sha256:") {
		t.Fatalf("missing digest: %#v", lock.Capabilities[0])
	}
}

func TestUninstallRemovesInstalledSkillAndReceipt(t *testing.T) {
	temp := t.TempDir()
	skill := filepath.Join(temp, "skill")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("# Example Skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pack := testPack(skill)
	plan := BuildInstallPlanWithOptions(pack, temp, "codex", "skills", InstallOptions{Mode: "copy", OnConflict: "overwrite"})
	result := ExecutePlan(plan, false)
	if _, err := WriteReceipt(temp, pack, result); err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(temp, ".codex", "skills", "example-skill")
	if _, err := os.Stat(installed); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	if err := Uninstall(temp, "example", &output); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(installed); !os.IsNotExist(err) {
		t.Fatalf("skill still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(temp, "receipts", "example.json")); !os.IsNotExist(err) {
		t.Fatalf("receipt still exists: %v", err)
	}
}

func TestUninstallExecutesPluginCleanupWhenOptedIn(t *testing.T) {
	temp := t.TempDir()
	pack := Pack{
		ID:          "plugin-pack",
		Name:        "Plugin Pack",
		Version:     "0.1.0",
		Description: "A plugin lifecycle pack.",
		Capabilities: []Capability{{
			Type:    "plugin",
			Name:    "Example Plugin",
			Source:  "https://example.com/plugin",
			Format:  "anthropic-plugin",
			Install: map[string]string{"method": "manual", "command": "echo install-plugin", "uninstall": "printf cleaned > plugin-cleanup.txt"},
		}},
	}
	plan := BuildInstallPlanWithOptions(pack, temp, "codex", "plugins", InstallOptions{Mode: "copy", OnConflict: "overwrite"})
	result := ExecutePlan(plan, false)
	if _, err := WriteReceipt(temp, pack, result); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_PACKS_PLUGIN_CWD", temp)
	var output strings.Builder
	if err := UninstallWithOptions(temp, "plugin-pack", true, &output); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(temp, "plugin-cleanup.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "cleaned" {
		t.Fatalf("unexpected plugin cleanup marker: %q", string(data))
	}
	if _, err := os.Stat(filepath.Join(temp, "receipts", "plugin-pack.json")); !os.IsNotExist(err) {
		t.Fatalf("receipt still exists: %v", err)
	}
}

func TestStandaloneSkillLifecycleUsesSkillReceipts(t *testing.T) {
	temp := t.TempDir()
	skill := filepath.Join(temp, "skill")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	skillBody := "---\nname: Standalone Skill\ndescription: A standalone skill.\n---\n# Standalone Skill\n"
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(skillBody), 0o644); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	err := InstallStandalone(filepath.Join(temp, "registry", "packs"), skill, "skills", temp, "codex", false, false, InstallOptions{Mode: "copy", OnConflict: "overwrite"}, &output)
	if err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(temp, ".codex", "skills", "standalone-skill", "SKILL.md")
	if _, err := os.Stat(installed); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(temp, "receipts", "standalone-skill.json")); !os.IsNotExist(err) {
		t.Fatalf("standalone skill wrote pack receipt: %v", err)
	}
	if _, err := os.Stat(filepath.Join(temp, "receipts", "skills", "standalone-skill.json")); err != nil {
		t.Fatalf("standalone skill receipt missing: %v", err)
	}

	output.Reset()
	if err := ListStandalone(temp, "skills", &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "standalone-skill") {
		t.Fatalf("standalone skill missing from list: %s", output.String())
	}

	output.Reset()
	if err := UninstallStandalone(temp, "standalone-skill", "skills", false, &output); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(installed); !os.IsNotExist(err) {
		t.Fatalf("standalone skill still installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(temp, "receipts", "skills", "standalone-skill.json")); !os.IsNotExist(err) {
		t.Fatalf("standalone skill receipt still exists: %v", err)
	}
}

func TestStandalonePluginLifecycleExecutesCleanupWhenOptedIn(t *testing.T) {
	temp := t.TempDir()
	pack := Pack{
		ID:          "unused",
		Name:        "Unused",
		Version:     "0.1.0",
		Description: "Unused pack.",
		Capabilities: []Capability{{
			Type:    "plugin",
			Name:    "Standalone Plugin",
			Source:  "https://example.com/plugin",
			Format:  "anthropic-plugin",
			Install: map[string]string{"method": "manual", "command": "printf installed > plugin-install.txt", "uninstall": "printf cleaned > plugin-cleanup.txt"},
		}},
	}
	capability := pack.Capabilities[0]
	standalone := Pack{ID: "standalone-plugin", Name: capability.Name, Version: "0.1.0", Description: "Standalone plugin.", Capabilities: []Capability{capability}}
	plan := BuildInstallPlanWithOptions(standalone, temp, "codex", "plugins", InstallOptions{Mode: "native", OnConflict: "overwrite"})
	t.Setenv("AGENT_PACKS_PLUGIN_CWD", temp)
	result := ExecutePlan(plan, true)
	if _, err := WriteStandaloneReceipt(temp, "plugins", "standalone-plugin", standalone, result); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(temp, "plugin-install.txt")); err != nil {
		t.Fatal(err)
	}

	var output strings.Builder
	if err := UninstallStandalone(temp, "standalone-plugin", "plugins", true, &output); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(temp, "plugin-cleanup.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "cleaned" {
		t.Fatalf("unexpected plugin cleanup marker: %q", string(data))
	}
	if _, err := os.Stat(filepath.Join(temp, "receipts", "plugins", "standalone-plugin.json")); !os.IsNotExist(err) {
		t.Fatalf("standalone plugin receipt still exists: %v", err)
	}
}

func writeTestPack(t *testing.T, registry string, pack Pack) {
	t.Helper()
	data, err := json.MarshalIndent(pack, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(registry, pack.ID+".json"), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeTestSkill(t *testing.T, dir, id string) {
	t.Helper()
	skillDir := filepath.Join(dir, id)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + id + "\ndescription: Review code changes and identify bugs. Use when reviewing pull requests or diffs.\n---\n\n# Review\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeTestPlugin(t *testing.T, dir, id string) {
	t.Helper()
	pluginDir := filepath.Join(dir, id, ".claude-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := PluginManifest{Name: id, DisplayName: "Browser Tool", Version: "0.1.0", Description: "Test plugin", Skills: "./skills"}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeTestCapability(t *testing.T, dir, id string, capability Capability) {
	t.Helper()
	data, err := json.MarshalIndent(capability, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCompatibilityAcceptsClaudeCodeAlias(t *testing.T) {
	temp := t.TempDir()
	registry := filepath.Join(temp, "packs")
	if err := os.MkdirAll(registry, 0o755); err != nil {
		t.Fatal(err)
	}
	pack := Pack{ID: "alias-pack", Name: "Alias Pack", Version: "0.1.0", Description: "Alias test", Tools: []string{"claude-code"}, Capabilities: []Capability{{Type: "skill", Name: "example", Source: "/tmp/s", Format: "agent-skill", Entry: "SKILL.md"}}}
	writeTestPack(t, registry, pack)
	var output strings.Builder
	if err := Compatibility(registry, "alias-pack", "claude-code", &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "compatible with claude") {
		t.Fatalf("expected claude compatibility, got %s", output.String())
	}
}

func TestAuditReportsCapabilities(t *testing.T) {
	temp := t.TempDir()
	registry := filepath.Join(temp, "packs")
	if err := os.MkdirAll(registry, 0o755); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(temp, "example-skill")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: example-skill\ndescription: Local skill for audit test.\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pack := Pack{ID: "audit-pack", Name: "Audit Pack", Version: "0.1.0", Description: "Audit test", Capabilities: []Capability{{Type: "skill", Name: "Example Skill", Source: skill, Format: "agent-skill", Entry: "SKILL.md"}}}
	writeTestPack(t, registry, pack)
	var output strings.Builder
	if err := Audit(registry, "audit-pack", &output); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.Contains(text, "SBOM:") || !strings.Contains(text, "skill:Example Skill") {
		t.Fatalf("audit missing capability report: %s", text)
	}
}

func TestOutdatedReportsPackVersion(t *testing.T) {
	temp := t.TempDir()
	registry := filepath.Join(temp, "registry", "packs")
	target := filepath.Join(temp, "home")
	if err := os.MkdirAll(registry, 0o755); err != nil {
		t.Fatal(err)
	}
	pack := testPack("/tmp/example-skill")
	pack.Version = "0.2.0"
	writeTestPack(t, registry, pack)
	packDir := filepath.Join(target, "packs", "example")
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		t.Fatal(err)
	}
	lock := Lockfile{Pack: "example", Version: "0.1.0", Capabilities: []LockEntry{{Type: "skill", Name: "Example Skill", Source: "/tmp/example-skill", Revision: "abc"}}}
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "agent-pack.lock"), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	if err := Outdated(registry, target, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "pack-version") || !strings.Contains(output.String(), "locked=0.1.0") {
		t.Fatalf("expected pack version drift, got %s", output.String())
	}
}

func TestDependencyTreeIncludesReferencedCapabilities(t *testing.T) {
	temp := t.TempDir()
	registryRoot := filepath.Join(temp, "registry")
	packs := filepath.Join(registryRoot, "packs")
	if err := os.MkdirAll(packs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(registryRoot, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestSkill(t, filepath.Join(registryRoot, "skills"), "review")
	pack := Pack{ID: "tree-pack", Name: "Tree Pack", Version: "0.1.0", Description: "Tree test", Skills: CapabilityRefs{{ID: "review"}}}
	writeTestPack(t, packs, pack)
	tree, err := DependencyTreeForPack(packs, "tree-pack")
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Dependencies) != 1 || tree.Dependencies[0].Type != "skill" || tree.Dependencies[0].ID != "review" {
		t.Fatalf("unexpected tree: %#v", tree)
	}
}

func TestNewScaffoldsPack(t *testing.T) {
	temp := t.TempDir()
	path, err := New(NewOptions{Kind: "pack", ID: "platform-engineer", Dir: temp})
	if err != nil {
		t.Fatal(err)
	}
	pack, err := LoadPack(path)
	if err != nil {
		t.Fatal(err)
	}
	if pack.ID != "platform-engineer" || pack.Stability != "experimental" || pack.ReviewStatus != "draft" {
		t.Fatalf("unexpected scaffolded pack: %#v", pack)
	}
}

func TestRollbackRestoresPreviousReceipt(t *testing.T) {
	temp := t.TempDir()
	pack := testPack("/tmp/example-skill")
	pack.Capabilities = nil
	pack.Skills = CapabilityRefs{{ID: "remote", Source: "https://example.com/skill", Format: "agent-skill"}}
	plan := BuildInstallPlanWithOptions(pack, temp, "generic", "skills", InstallOptions{Mode: "reference", OnConflict: "skip"})
	result := ExecutePlan(plan, false)
	if _, err := WriteReceipt(temp, pack, result); err != nil {
		t.Fatal(err)
	}
	pack.Version = "0.2.0"
	plan = BuildInstallPlanWithOptions(pack, temp, "generic", "skills", InstallOptions{Mode: "reference", OnConflict: "skip"})
	result = ExecutePlan(plan, false)
	if _, err := WriteReceipt(temp, pack, result); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	if err := Rollback(temp, "example", &output); err != nil {
		t.Fatal(err)
	}
	receipt, err := LoadReceipt(filepath.Join(temp, "receipts", "example.json"))
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Pack.Version != "0.1.0" {
		t.Fatalf("expected rollback to 0.1.0, got %s", receipt.Pack.Version)
	}
}

func TestScanSkillsSkipsBackupDirectories(t *testing.T) {
	root := t.TempDir()
	skillMD := "---\nname: real-skill\ndescription: A live installed skill.\n---\n"

	live := filepath.Join(root, "real-skill")
	if err := os.MkdirAll(live, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(live, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
		t.Fatal(err)
	}

	// A backup left behind by `--on-conflict backup`: <dest>.bak.<timestamp>.
	backup := filepath.Join(root, "real-skill.bak.20260614120000")
	if err := os.MkdirAll(backup, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	if err := ScanSkills(root, &out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if strings.Contains(text, ".bak.") {
		t.Fatalf("scan must not report backup directories, got:\n%s", text)
	}
	lines := strings.Count(strings.TrimSpace(text), "\n") + 1
	if strings.TrimSpace(text) == "" || lines != 1 {
		t.Fatalf("expected exactly one live skill line, got %d:\n%s", lines, text)
	}
}

func testPack(skillSource string) Pack {
	return Pack{
		ID:          "example",
		Name:        "Example Pack",
		Version:     "0.1.0",
		Description: "A test pack.",
		Capabilities: []Capability{
			{Type: "skill", Name: "Example Skill", Source: skillSource, Format: "agent-skill", Entry: "SKILL.md"},
			{Type: "plugin", Name: "Example Plugin", Source: "https://example.com/plugin", Format: "anthropic-plugin", Entry: ".claude-plugin/plugin.json", Install: map[string]string{"method": "manual", "package": "example-plugin", "command": "echo install-plugin"}, RequiresExecution: true},
		},
	}
}
