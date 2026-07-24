package validate

import (
	"fmt"
	"time"

	"github.com/agent-packs/cli/internal/model"
	"github.com/agent-packs/cli/internal/registry"
)

const metadataFreshnessThresholdDays = 90

func MetadataCoverage(registryPath string, packs []model.Pack, now time.Time) model.MetadataCoverageReport {
	report := model.MetadataCoverageReport{
		Packs: len(packs),
		Fields: []model.MetadataFieldCoverage{
			fieldCoverage(packs, "requirements", hasRequirements),
			fieldCoverage(packs, "useCases", hasUseCases),
			fieldCoverage(packs, "examplePrompts", hasExamplePrompts),
		},
		Freshness: model.MetadataFreshnessCoverage{ThresholdDays: metadataFreshnessThresholdDays},
	}
	for _, pack := range packs {
		hasBare := false
		for _, ref := range pack.Skills {
			if ref.IsObjectRef() {
				report.Refs.ObjectSkillRefs++
			} else {
				report.Refs.BareSkillRefs++
				hasBare = true
			}
		}
		for _, ref := range pack.Plugins {
			if ref.IsObjectRef() {
				report.Refs.ObjectPluginRefs++
			} else {
				report.Refs.BarePluginRefs++
				hasBare = true
			}
		}
		if hasBare {
			report.Refs.PacksWithBareRefs++
			report.Refs.Packs = append(report.Refs.Packs, pack.ID)
		}
		switch registry.ComputeProvenance(registryPath, pack).PinStatus {
		case "pinned", "registry":
			report.Pinning.FullyPinned++
		case "partial":
			report.Pinning.Partial++
			report.Pinning.UnpinnedPacks = append(report.Pinning.UnpinnedPacks, pack.ID)
		default:
			report.Pinning.Unpinned++
			report.Pinning.UnpinnedPacks = append(report.Pinning.UnpinnedPacks, pack.ID)
		}
		addFreshness(&report.Freshness, pack, now)
	}
	return report
}

func fieldCoverage(packs []model.Pack, name string, present func(model.Pack) bool) model.MetadataFieldCoverage {
	coverage := model.MetadataFieldCoverage{Name: name, Total: len(packs)}
	for _, pack := range packs {
		if present(pack) {
			coverage.Present++
		} else {
			coverage.Missing = append(coverage.Missing, pack.ID)
		}
	}
	return coverage
}

func hasRequirements(pack model.Pack) bool {
	return pack.Requirements.AgentPacks != "" || len(pack.Requirements.Tools) > 0
}

func hasUseCases(pack model.Pack) bool {
	return len(pack.UseCases) > 0
}

func hasExamplePrompts(pack model.Pack) bool {
	return len(pack.ExamplePrompts) > 0
}

func addFreshness(freshness *model.MetadataFreshnessCoverage, pack model.Pack, now time.Time) {
	if pack.LastVerified == "" {
		freshness.Missing++
		freshness.MissingPacks = append(freshness.MissingPacks, pack.ID)
		return
	}
	verified, err := time.Parse("2006-01-02", pack.LastVerified)
	if err != nil {
		freshness.Invalid++
		freshness.InvalidPacks = append(freshness.InvalidPacks, pack.ID)
		return
	}
	if now.Sub(verified) > time.Duration(metadataFreshnessThresholdDays)*24*time.Hour {
		freshness.Stale++
		freshness.StalePacks = append(freshness.StalePacks, pack.ID)
		return
	}
	freshness.Fresh++
}

func metadataChecks(report model.MetadataCoverageReport) []model.CheckEntry {
	checks := []model.CheckEntry{}
	for _, field := range report.Fields {
		status := "OK"
		if field.Present < field.Total {
			status = "WARN"
		}
		checks = append(checks, model.CheckEntry{
			Kind:    "metadata",
			Target:  field.Name,
			Status:  status,
			Message: fmt.Sprintf("%d/%d packs declare %s", field.Present, field.Total, field.Name),
		})
	}
	refStatus := "OK"
	if report.Refs.PacksWithBareRefs > 0 {
		refStatus = "WARN"
	}
	checks = append(checks, model.CheckEntry{
		Kind:    "metadata",
		Target:  "provenance",
		Status:  refStatus,
		Message: fmt.Sprintf("%d/%d packs use bare skill/plugin refs", report.Refs.PacksWithBareRefs, report.Packs),
	})
	pinStatus := "OK"
	if report.Pinning.FullyPinned < report.Packs {
		pinStatus = "WARN"
	}
	checks = append(checks, model.CheckEntry{
		Kind:    "metadata",
		Target:  "pinning",
		Status:  pinStatus,
		Message: fmt.Sprintf("%d/%d packs pin every remote source to a commit or checksum", report.Pinning.FullyPinned, report.Packs),
	})
	freshStatus := "OK"
	if report.Freshness.Stale > 0 || report.Freshness.Invalid > 0 || report.Freshness.Missing > 0 {
		freshStatus = "WARN"
	}
	checks = append(checks, model.CheckEntry{
		Kind:    "metadata",
		Target:  "freshness",
		Status:  freshStatus,
		Message: fmt.Sprintf("%d/%d packs verified within %d days", report.Freshness.Fresh, report.Packs, report.Freshness.ThresholdDays),
	})
	return checks
}
