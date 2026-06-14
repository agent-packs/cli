package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sandeshh/agent-packs/cli/internal/model"
)

func TestCheckTrustLevelBlocksBelowMinimum(t *testing.T) {
	pack := model.Pack{ID: "test-pack", Trust: ""}
	err := checkTrustLevel(pack, "community")
	if err == nil {
		t.Fatal("expected error when trust=unverified and min-trust=community")
	}
	if !strings.Contains(err.Error(), "unverified") || !strings.Contains(err.Error(), "community") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCheckTrustLevelAllowsAtOrAboveMinimum(t *testing.T) {
	cases := []struct{ trust, minTrust string }{
		{"core", "core"},
		{"core", "community"},
		{"community", "community"},
		{"community", "tap"},
		{"", "unverified"},
	}
	for _, c := range cases {
		pack := model.Pack{ID: "test-pack", Trust: c.trust}
		if err := checkTrustLevel(pack, c.minTrust); err != nil {
			t.Errorf("trust=%q min-trust=%q: unexpected error: %v", c.trust, c.minTrust, err)
		}
	}
}

func TestPackDiffReturnsHelpfulErrorWhenNotInstalled(t *testing.T) {
	temp := t.TempDir()
	registryPacks := filepath.Join(temp, "registry", "packs")
	if err := os.MkdirAll(registryPacks, 0o755); err != nil {
		t.Fatal(err)
	}
	pack := model.Pack{
		ID: "my-pack", Name: "My Pack", Version: "0.1.0",
		Capabilities: []model.Capability{{Type: "skill", Name: "S", Source: "/tmp/s"}},
	}
	data, _ := json.MarshalIndent(pack, "", "  ")
	if err := os.WriteFile(filepath.Join(registryPacks, "my-pack.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(temp, "home")
	err := PackDiff(registryPacks, target, "my-pack", os.Stdout)
	if err == nil {
		t.Fatal("expected error when pack is not installed")
	}
	if !strings.Contains(err.Error(), "not installed") || !strings.Contains(err.Error(), "agent-packs install") {
		t.Fatalf("expected helpful not-installed message, got: %v", err)
	}
}

func TestOutdatedReportSkipsInternalHistoryDirectory(t *testing.T) {
	temp := t.TempDir()
	target := filepath.Join(temp, "home")
	if err := os.MkdirAll(filepath.Join(target, "packs", ".history", "example-20260614120000"), 0o755); err != nil {
		t.Fatal(err)
	}
	registryPacks := filepath.Join(temp, "registry", "packs")
	if err := os.MkdirAll(registryPacks, 0o755); err != nil {
		t.Fatal(err)
	}

	report, err := OutdatedReport(registryPacks, target)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Entries) != 0 {
		t.Fatalf("expected internal history directory to be ignored, got %#v", report.Entries)
	}
}

func TestDriftCheckReportsReferenceModeInsteadOfEmpty(t *testing.T) {
	target := t.TempDir()
	plan := model.Plan{
		Pack: "ref-pack", Mode: "reference",
		Capabilities: []model.PlanItem{
			{Type: "skill", Name: "skill-a", Action: "reference", Status: "referenced"},
			{Type: "skill", Name: "skill-b", Action: "reference", Status: "referenced"},
		},
	}
	if _, err := WriteReceiptWithoutSnapshot(target, model.Pack{ID: "ref-pack"}, plan); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := DriftCheck(target, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := out.String()
	if strings.Contains(text, "No installed packs found") {
		t.Fatalf("reference-mode install must not report as empty:\n%s", text)
	}
	if !strings.Contains(text, "reference mode") || !strings.Contains(text, "ref-pack/skill-a") {
		t.Fatalf("expected reference-mode capabilities to be listed, got:\n%s", text)
	}
}

func TestDriftCheckEmptyTargetReportsNoPacks(t *testing.T) {
	target := t.TempDir()
	var out strings.Builder
	if err := DriftCheck(target, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "No installed packs found") {
		t.Fatalf("empty target should report no packs, got: %s", out.String())
	}
}

func TestDriftCheckFlagsMissingMaterializedCapability(t *testing.T) {
	target := t.TempDir()
	plan := model.Plan{
		Pack: "copy-pack", Mode: "copy",
		Capabilities: []model.PlanItem{
			{Type: "skill", Name: "gone", Action: "copy", Status: "installed",
				Destination: filepath.Join(target, "does-not-exist", "gone")},
		},
	}
	if _, err := WriteReceiptWithoutSnapshot(target, model.Pack{ID: "copy-pack"}, plan); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	err := DriftCheck(target, &out)
	if err == nil {
		t.Fatal("expected drift error when a materialized capability is missing")
	}
	if !strings.Contains(out.String(), "MISSING") {
		t.Fatalf("expected MISSING line, got: %s", out.String())
	}
}
