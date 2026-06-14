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
