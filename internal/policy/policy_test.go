package policy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agent-packs/cli/internal/model"
)

// writeRegistry writes a single pack into a temp registry/packs dir and returns
// the packs path. Sources are local or pinned so policy/audit checks stay
// offline and deterministic.
func writeRegistry(t *testing.T, pack model.Pack) string {
	t.Helper()
	dir := t.TempDir()
	packs := filepath.Join(dir, "packs")
	if err := os.MkdirAll(packs, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(pack, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packs, pack.ID+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return packs
}

func writePolicy(t *testing.T, policy model.TrustPolicy) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")
	data, _ := json.MarshalIndent(policy, "", "  ")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func localSkillPack(source string) model.Pack {
	return model.Pack{
		ID: "p", Name: "P", Version: "1.0.0", Description: "d",
		Capabilities: []model.Capability{{Type: "skill", Name: "s", Source: source, Format: "agent-skill", Entry: "SKILL.md"}},
	}
}

func TestPolicyCheckAllowsLocalSource(t *testing.T) {
	// Local sources resolve without network and are treated as pinned.
	packs := writeRegistry(t, localSkillPack("/tmp/local-skill"))
	policyPath := writePolicy(t, model.TrustPolicy{RequirePinnedRefs: true})
	var out strings.Builder
	if err := PolicyCheck(packs, "p", policyPath, &out); err != nil {
		t.Fatalf("expected local source to satisfy policy, got %v (out: %s)", err, out.String())
	}
	if !strings.Contains(out.String(), "satisfies policy") {
		t.Fatalf("expected success message, got: %s", out.String())
	}
}

func TestPolicyCheckDeniesSource(t *testing.T) {
	packs := writeRegistry(t, localSkillPack("/tmp/forbidden/skill"))
	policyPath := writePolicy(t, model.TrustPolicy{DenySources: []string{"/tmp/forbidden"}})
	var out strings.Builder
	err := PolicyCheck(packs, "p", policyPath, &out)
	if err == nil {
		t.Fatal("expected denied source to fail policy")
	}
	if !strings.Contains(out.String(), "denied source") {
		t.Fatalf("expected denied-source message, got: %s", out.String())
	}
}

func TestPolicyCheckAllowSourcesRejectsUnlisted(t *testing.T) {
	packs := writeRegistry(t, localSkillPack("/tmp/other/skill"))
	policyPath := writePolicy(t, model.TrustPolicy{AllowSources: []string{"/tmp/approved"}})
	var out strings.Builder
	err := PolicyCheck(packs, "p", policyPath, &out)
	if err == nil {
		t.Fatal("expected unlisted source to fail when AllowSources is set")
	}
	if !strings.Contains(out.String(), "source not allowed") {
		t.Fatalf("expected source-not-allowed message, got: %s", out.String())
	}
}

func TestPolicyCheckRequirePinnedRejectsMovingRef(t *testing.T) {
	pack := model.Pack{
		ID: "p", Name: "P", Version: "1.0.0", Description: "d",
		Capabilities: []model.Capability{{
			Type: "skill", Name: "s", Format: "agent-skill", Entry: "SKILL.md",
			// github tree on a branch ref => not pinned, no network needed.
			Source: "https://github.com/owner/repo/tree/main/skills/s",
		}},
	}
	packs := writeRegistry(t, pack)
	policyPath := writePolicy(t, model.TrustPolicy{RequirePinnedRefs: true})
	var out strings.Builder
	err := PolicyCheck(packs, "p", policyPath, &out)
	if err == nil {
		t.Fatal("expected moving ref to fail RequirePinnedRefs")
	}
	if !strings.Contains(out.String(), "not pinned") {
		t.Fatalf("expected not-pinned message, got: %s", out.String())
	}
}

func TestPolicyCheckNativeCommandBlocked(t *testing.T) {
	pack := model.Pack{
		ID: "p", Name: "P", Version: "1.0.0", Description: "d",
		Capabilities: []model.Capability{{
			Type: "plugin", Name: "pl", Source: "/tmp/local-plugin", Format: "anthropic-plugin",
			Install: map[string]string{"method": "shell", "command": "do-install"},
		}},
	}
	packs := writeRegistry(t, pack)

	blocked := writePolicy(t, model.TrustPolicy{AllowNativeCommands: false})
	var out strings.Builder
	if err := PolicyCheck(packs, "p", blocked, &out); err == nil {
		t.Fatal("expected native command to be blocked")
	}
	if !strings.Contains(out.String(), "native command blocked") {
		t.Fatalf("expected native-command-blocked message, got: %s", out.String())
	}

	allowed := writePolicy(t, model.TrustPolicy{AllowNativeCommands: true})
	var out2 strings.Builder
	if err := PolicyCheck(packs, "p", allowed, &out2); err != nil {
		t.Fatalf("native command should be allowed with AllowNativeCommands, got %v (out: %s)", err, out2.String())
	}
}

func TestBuildAuditReportLocalSourceIsOK(t *testing.T) {
	packs := writeRegistry(t, localSkillPack("/tmp/local-skill"))
	report, err := BuildAuditReport(packs, "p")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.OK {
		t.Fatalf("expected local-source audit to be OK, violations: %v", report.Violations)
	}
	if len(report.Components) != 1 || report.Components[0].Name != "s" {
		t.Fatalf("unexpected components: %+v", report.Components)
	}
}

func TestMatchesAny(t *testing.T) {
	cases := []struct {
		value    string
		patterns []string
		want     bool
	}{
		{"/tmp/forbidden/x", []string{"/tmp/forbidden"}, true},
		{"https://github.com/owner/repo", []string{"https://github.com/owner/*"}, true},
		{"/tmp/safe/x", []string{"/tmp/forbidden"}, false},
		{"anything", nil, false},
	}
	for _, c := range cases {
		if got := matchesAny(c.value, c.patterns); got != c.want {
			t.Errorf("matchesAny(%q, %v) = %v, want %v", c.value, c.patterns, got, c.want)
		}
	}
}
