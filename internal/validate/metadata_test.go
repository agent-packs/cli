package validate

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/agent-packs/cli/internal/model"
)

func TestMetadataCoverageCountsRequirementsRefsAndFreshness(t *testing.T) {
	now := time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC)
	packs := []model.Pack{
		{
			ID:           "complete",
			LastVerified: "2026-06-01",
			Requirements: model.Requirements{
				AgentPacks: ">=0.1.0",
			},
			Skills: model.CapabilityRefs{
				packRefFromJSON(t, `{"id":"skill-one","trust":"community"}`),
			},
			Plugins: model.CapabilityRefs{
				packRefFromJSON(t, `{"id":"plugin-one","trust":"official"}`),
			},
		},
		{
			ID:           "bare",
			LastVerified: "2025-01-01",
			Skills: model.CapabilityRefs{
				packRefFromJSON(t, `"skill-two"`),
			},
			Plugins: model.CapabilityRefs{
				packRefFromJSON(t, `"plugin-two"`),
			},
		},
		{
			ID:           "invalid-date",
			LastVerified: "yesterday",
		},
		{
			ID: "missing-date",
		},
	}

	report := MetadataCoverage(t.TempDir(), packs, now)
	if report.Packs != 4 {
		t.Fatalf("packs = %d; want 4", report.Packs)
	}
	if got := report.Fields[0]; got.Name != "requirements" || got.Present != 1 || got.Total != 4 || len(got.Missing) != 3 {
		t.Fatalf("unexpected requirements coverage: %#v", got)
	}
	if report.Refs.ObjectSkillRefs != 1 || report.Refs.BareSkillRefs != 1 || report.Refs.ObjectPluginRefs != 1 || report.Refs.BarePluginRefs != 1 {
		t.Fatalf("unexpected ref coverage: %#v", report.Refs)
	}
	if report.Refs.PacksWithBareRefs != 1 || report.Refs.Packs[0] != "bare" {
		t.Fatalf("expected bare pack to be tracked, got %#v", report.Refs)
	}
	if report.Freshness.Fresh != 1 || report.Freshness.Stale != 1 || report.Freshness.Invalid != 1 || report.Freshness.Missing != 1 {
		t.Fatalf("unexpected freshness coverage: %#v", report.Freshness)
	}
}

func packRefFromJSON(t *testing.T, raw string) model.CapabilityRef {
	t.Helper()
	var ref model.CapabilityRef
	if err := json.Unmarshal([]byte(raw), &ref); err != nil {
		t.Fatalf("unmarshal ref: %v", err)
	}
	return ref
}

func TestMetadataChecksWarnWithoutFailingPublishReport(t *testing.T) {
	report := MetadataCoverage(t.TempDir(), []model.Pack{{
		ID:           "missing-requirements",
		LastVerified: "2026-06-01",
		Skills: model.CapabilityRefs{
			packRefFromJSON(t, `"skill-one"`),
		},
	}}, time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC))

	checks := metadataChecks(report)
	wantWarns := map[string]bool{"requirements": false, "provenance": false}
	for _, check := range checks {
		if _, ok := wantWarns[check.Target]; ok && check.Status == "WARN" {
			wantWarns[check.Target] = true
		}
	}
	for target, seen := range wantWarns {
		if !seen {
			t.Fatalf("expected WARN metadata check for %s in %#v", target, checks)
		}
	}
}
