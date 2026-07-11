package agentpacks

import (
	"fmt"
	"io"

	"github.com/agent-packs/cli/internal/install"
	"github.com/agent-packs/cli/internal/model"
	"github.com/agent-packs/cli/internal/output"
	"github.com/agent-packs/cli/internal/policy"
)

// CheckPackReport is the CI-gate result for one installed pack.
type CheckPackReport struct {
	ID               string              `json:"id"`
	Pins             []install.PinStatus `json:"pins,omitempty"`
	PolicyViolations []string            `json:"policyViolations,omitempty"`
	Error            string              `json:"error,omitempty"`
}

// CheckReport aggregates pin verification, managed-file drift, and optional
// policy enforcement across every pack installed at a target.
type CheckReport struct {
	Target   string              `json:"target"`
	Policy   string              `json:"policy,omitempty"`
	Packs    []CheckPackReport   `json:"packs"`
	Drift    []install.DriftItem `json:"drift"`
	Failures int                 `json:"failures"`
	Warnings int                 `json:"warnings"`
	OK       bool                `json:"ok"`
	// Note carries a target-level failure explanation, e.g. when no packs are
	// installed at the target and the gate has nothing to verify.
	Note string `json:"note,omitempty"`
}

// BuildCheckReport runs every gate that `agent-packs check` enforces:
// recorded pins still match live sources, materialized files have not
// drifted, and (when a policy is given) every installed pack satisfies it.
// Verification failures are recorded in the report rather than returned as
// errors so one broken pack cannot hide the state of the others.
func BuildCheckReport(registry, target, policyPath string) (CheckReport, error) {
	report := CheckReport{Target: target, Policy: policyPath, Packs: []CheckPackReport{}, Drift: []install.DriftItem{}}

	installed, err := install.ListInstalledReceipts(target)
	if err != nil {
		return report, err
	}
	// An empty target fails closed: a CI gate that silently passes when run
	// from the wrong directory (or before any install) verifies nothing.
	if len(installed) == 0 {
		report.Note = fmt.Sprintf("no installed packs found at %s — nothing to verify (wrong --target?)", target)
		report.Failures++
		report.OK = false
		return report, nil
	}
	for _, summary := range installed {
		packReport := CheckPackReport{ID: summary.ID}
		pins, err := install.PinCheckResults(registry, target, summary.ID)
		if err != nil {
			packReport.Error = err.Error()
			report.Failures++
		} else {
			packReport.Pins = pins
			for _, pin := range pins {
				switch pin.State {
				case "changed", "unverifiable":
					report.Failures++
				case "unpinned":
					report.Warnings++
				}
			}
		}
		if policyPath != "" && packReport.Error == "" {
			_, violations, err := policy.PolicyViolations(registry, summary.ID, policyPath)
			if err != nil {
				packReport.Error = err.Error()
				report.Failures++
			} else {
				packReport.PolicyViolations = violations
				report.Failures += len(violations)
			}
		}
		report.Packs = append(report.Packs, packReport)
	}

	drift, err := install.CollectDriftItems(target)
	if err != nil {
		return report, err
	}
	for _, item := range drift {
		if item.State == "missing" || item.State == "drifted" {
			report.Failures++
		}
	}
	if drift != nil {
		report.Drift = drift
	}

	report.OK = report.Failures == 0
	return report, nil
}

// Check verifies pins, drift, and optional policy for every installed pack
// and fails (nonzero exit) when any gate is violated — one command suitable
// for CI. Unpinned capabilities and reference-mode installs are warnings.
func Check(registry, target, policyPath string, asJSON bool, out io.Writer) error {
	report, err := BuildCheckReport(registry, target, policyPath)
	if err != nil {
		return err
	}
	if asJSON {
		if err := output.Encode(out, report); err != nil {
			return err
		}
		if !report.OK {
			return checkFailed(report)
		}
		return nil
	}

	if len(report.Packs) == 0 {
		fmt.Fprintf(out, "check failed: %s\n", report.Note)
		return checkFailed(report)
	}

	fmt.Fprintln(out, "Pins")
	for _, pack := range report.Packs {
		if pack.Error != "" {
			fmt.Fprintf(out, "  ERROR    %s — %s\n", pack.ID, pack.Error)
			continue
		}
		for _, pin := range pack.Pins {
			switch pin.State {
			case "changed":
				fmt.Fprintf(out, "  CHANGED  %s/%s — %s\n", pack.ID, pin.Name, pin.Detail)
			case "unverifiable":
				fmt.Fprintf(out, "  UNVERIFIABLE %s/%s — %s\n", pack.ID, pin.Name, pin.Detail)
			case "unpinned":
				fmt.Fprintf(out, "  UNPINNED %s/%s — %s\n", pack.ID, pin.Name, pin.Detail)
			default:
				fmt.Fprintf(out, "  OK       %s/%s\n", pack.ID, pin.Name)
			}
		}
	}

	fmt.Fprintln(out, "Drift")
	for _, item := range report.Drift {
		switch item.State {
		case "missing":
			fmt.Fprintf(out, "  MISSING  %s/%s — destination %s not found\n", item.Pack, item.Name, item.Dest)
		case "drifted":
			fmt.Fprintf(out, "  DRIFTED  %s/%s — %s\n", item.Pack, item.Name, item.Detail)
		case "referenced":
			fmt.Fprintf(out, "  REF      %s/%s — reference mode (not materialized)\n", item.Pack, item.Name)
		default:
			fmt.Fprintf(out, "  OK       %s/%s\n", item.Pack, item.Name)
		}
	}

	if report.Policy != "" {
		fmt.Fprintln(out, "Policy")
		for _, pack := range report.Packs {
			if pack.Error != "" {
				continue
			}
			if len(pack.PolicyViolations) == 0 {
				fmt.Fprintf(out, "  OK       %s\n", pack.ID)
				continue
			}
			for _, violation := range pack.PolicyViolations {
				fmt.Fprintf(out, "  FAIL     %s — %s\n", pack.ID, violation)
			}
		}
	}

	fmt.Fprintln(out)
	if !report.OK {
		fmt.Fprintf(out, "check failed: %d failure(s), %d warning(s) across %d pack(s)\n", report.Failures, report.Warnings, len(report.Packs))
		return checkFailed(report)
	}
	fmt.Fprintf(out, "check passed: %d pack(s), %d warning(s)\n", len(report.Packs), report.Warnings)
	return nil
}

// checkFailed wraps the shared install-failure sentinel (so callers can still
// match it with errors.Is) behind a message that names the failed gate.
func checkFailed(report CheckReport) error {
	return fmt.Errorf("check failed with %d gate failure(s): %w", report.Failures, model.ErrInstallFailed)
}
