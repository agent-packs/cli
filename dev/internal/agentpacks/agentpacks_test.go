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
	plan := BuildInstallPlan(pack, "/tmp/target", "codex", "skills")

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
	plan := BuildInstallPlan(pack, filepath.Join(temp, "target"), "codex", "skills")
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
	plan := BuildInstallPlan(pack, temp, "generic", "plugins")
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

func testPack(skillSource string) Pack {
	return Pack{
		ID:          "example",
		Name:        "Example Pack",
		Version:     "0.1.0",
		Description: "A test pack.",
		Capabilities: []Capability{
			{Type: "skill", Name: "Example Skill", Source: skillSource, Format: "agent-skill", Entry: "SKILL.md"},
			{Type: "plugin", Name: "Example Plugin", Source: "https://example.com/plugin", Format: "anthropic-plugin", Entry: ".claude-plugin/plugin.json", Install: map[string]string{"method": "manual", "package": "example-plugin", "command": "echo install-plugin"}},
		},
	}
}
