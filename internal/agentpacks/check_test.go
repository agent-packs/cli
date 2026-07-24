package agentpacks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// checkTestSetup installs a copy-mode pack backed by a local skill so the
// check gates (pins, drift, policy) have real installed state to verify.
func checkTestSetup(t *testing.T) (registryPacks, target, skillDir string) {
	t.Helper()
	temp := t.TempDir()

	skillDir = filepath.Join(temp, "local-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: local-skill\ndescription: A local skill.\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	registryPacks = filepath.Join(temp, "registry", "packs")
	if err := os.MkdirAll(registryPacks, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestPack(t, registryPacks, Pack{
		ID: "check-pack", Name: "Check Pack", Version: "0.1.0", Description: "check test",
		Capabilities: []Capability{
			{Type: "skill", Name: "local-skill", Source: skillDir, Format: "agent-skill", Entry: "SKILL.md"},
		},
	})

	target = filepath.Join(temp, "home")
	var out strings.Builder
	err := InstallWithOptions(registryPacks, target, "check-pack", target, "codex", "skills",
		false, false, InstallOptions{Mode: "copy", OnConflict: "overwrite"}, &out)
	if err != nil {
		t.Fatalf("install failed: %v\n%s", err, out.String())
	}
	return registryPacks, target, skillDir
}

func TestCheckWarnsOnUnpinnedThenPassesAfterPin(t *testing.T) {
	registryPacks, target, _ := checkTestSetup(t)

	var before strings.Builder
	if err := Check(registryPacks, target, "", false, &before); err != nil {
		t.Fatalf("expected unpinned check to pass with warnings, got: %v\n%s", err, before.String())
	}
	if !strings.Contains(before.String(), "UNPINNED") || !strings.Contains(before.String(), "check passed") {
		t.Fatalf("expected UNPINNED warning and pass summary, got:\n%s", before.String())
	}

	if err := PinPack(registryPacks, target, "check-pack", false, &strings.Builder{}); err != nil {
		t.Fatalf("pin failed: %v", err)
	}
	var after strings.Builder
	if err := Check(registryPacks, target, "", false, &after); err != nil {
		t.Fatalf("expected pinned check to pass, got: %v\n%s", err, after.String())
	}
	if strings.Contains(after.String(), "UNPINNED") {
		t.Fatalf("expected no UNPINNED warnings after pinning, got:\n%s", after.String())
	}
}

func TestCheckFailsWhenInstalledSkillIsTampered(t *testing.T) {
	registryPacks, target, _ := checkTestSetup(t)

	installed := filepath.Join(target, ".codex", "skills", "local-skill", "SKILL.md")
	if err := os.WriteFile(installed, []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := Check(registryPacks, target, "", false, &out); err == nil {
		t.Fatalf("expected check to fail after tamper, output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "DRIFTED") || !strings.Contains(out.String(), "check failed") {
		t.Fatalf("expected DRIFTED failure, got:\n%s", out.String())
	}
}

func TestCheckEnforcesPolicyAcrossInstalledPacks(t *testing.T) {
	registryPacks, target, skillDir := checkTestSetup(t)

	policyPath := filepath.Join(filepath.Dir(registryPacks), "deny.json")
	policy := TrustPolicy{DenySources: []string{skillDir}}
	data, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	if err := Check(registryPacks, target, policyPath, false, &out); err == nil {
		t.Fatalf("expected policy violation to fail check, output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "denied source") {
		t.Fatalf("expected denied-source violation, got:\n%s", out.String())
	}
}

func TestCheckJSONReportsStructuredResults(t *testing.T) {
	registryPacks, target, _ := checkTestSetup(t)

	var out strings.Builder
	if err := Check(registryPacks, target, "", true, &out); err != nil {
		t.Fatalf("expected JSON check to pass, got: %v\n%s", err, out.String())
	}
	var report CheckReport
	if err := json.Unmarshal([]byte(out.String()), &report); err != nil {
		t.Fatalf("invalid JSON report: %v\n%s", err, out.String())
	}
	if !report.OK || len(report.Packs) != 1 || report.Packs[0].ID != "check-pack" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if report.Warnings == 0 {
		t.Fatalf("expected unpinned warning in report: %+v", report)
	}
}

// A CI gate must not pass when there is nothing to verify: running check from
// the wrong directory (or before any install) has to fail loudly, not exit 0.
func TestCheckFailsWithNoInstalledPacks(t *testing.T) {
	temp := t.TempDir()
	var out strings.Builder
	err := Check(filepath.Join(temp, "packs"), temp, "", false, &out)
	if err == nil {
		t.Fatalf("expected empty target to fail the gate, output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "no installed packs found") {
		t.Fatalf("unexpected output:\n%s", out.String())
	}
}
