package policy

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/agent-packs/cli/internal/model"
	"github.com/agent-packs/cli/internal/output"
	"github.com/agent-packs/cli/internal/registry"
	"github.com/agent-packs/cli/internal/resolve"
	"github.com/agent-packs/cli/internal/util"
)

func PolicyCheck(registryPath, packRef, policyPath string, out io.Writer) error {
	packID, violations, err := PolicyViolations(registryPath, packRef, policyPath)
	if err != nil {
		return err
	}
	for _, violation := range violations {
		fmt.Fprintf(out, "FAIL  %s\n", violation)
	}
	if len(violations) > 0 {
		return model.ErrInstallFailed
	}
	fmt.Fprintf(out, "OK    %s satisfies policy\n", packID)
	return nil
}

// PolicyViolations evaluates a pack against a trust policy and returns the
// expanded pack ID plus one message per violated rule, without printing.
func PolicyViolations(registryPath, packRef, policyPath string) (string, []string, error) {
	policy, err := LoadTrustPolicy(policyPath)
	if err != nil {
		return "", nil, err
	}
	pack, err := registry.FindPack(registryPath, packRef)
	if err != nil {
		return "", nil, err
	}
	expanded, err := registry.ExpandPack(registryPath, pack, map[string]bool{})
	if err != nil {
		return "", nil, err
	}
	var violations []string
	for _, capability := range expanded.Capabilities {
		if capability.Source != "" {
			if matchesAny(capability.Source, policy.DenySources) {
				violations = append(violations, "denied source: "+capability.Source)
			}
			if len(policy.AllowSources) > 0 && !matchesAny(capability.Source, policy.AllowSources) {
				violations = append(violations, "source not allowed: "+capability.Source)
			}
			resolution := resolve.ResolveSource(capability.Source)
			if policy.RequirePinnedRefs {
				if resolution.Kind == "remote" {
					violations = append(violations, "source revision unresolved: "+capability.Source)
				} else if !resolution.Pinned && !util.IsLocalSource(capability.Source) {
					violations = append(violations, "source is not pinned: "+capability.Source)
				}
			}
		}
		if capability.Type == "plugin" && capability.Install != nil && capability.Install["command"] != "" && !policy.AllowNativeCommands {
			violations = append(violations, "native command blocked by policy: "+capability.Name)
		}
	}
	return expanded.ID, violations, nil
}

func Audit(registryPath, packRef string, out io.Writer) error {
	return writeAudit(registryPath, packRef, out, false)
}

func AuditJSON(registryPath, packRef string, out io.Writer) error {
	return writeAudit(registryPath, packRef, out, true)
}

func writeAudit(registryPath, packRef string, out io.Writer, asJSON bool) error {
	report, err := BuildAuditReport(registryPath, packRef)
	if err != nil {
		return err
	}
	if asJSON {
		return output.Encode(out, report)
	}
	fmt.Fprintf(out, "SBOM: %s (%s) v%s\n", report.Pack.Name, report.Pack.ID, report.Pack.Version)
	fmt.Fprintf(out, "Generated for supply-chain audit\n\n")
	fmt.Fprintf(out, "Pack\n")
	fmt.Fprintf(out, "  id: %s\n", report.Pack.ID)
	fmt.Fprintf(out, "  version: %s\n", report.Pack.Version)
	fmt.Fprintf(out, "  license: %s\n", report.Pack.License)
	if report.Pack.UpstreamSource != "" {
		fmt.Fprintf(out, "  upstreamSource: %s\n", report.Pack.UpstreamSource)
	}
	fmt.Fprintf(out, "\nComponents (%d)\n", len(report.Components))
	for i, component := range report.Components {
		fmt.Fprintf(out, "\n[%d] %s:%s\n", i+1, component.Type, component.Name)
		fmt.Fprintf(out, "  source: %s\n", component.Source)
		if component.UpstreamSource != "" {
			fmt.Fprintf(out, "  upstreamSource: %s\n", component.UpstreamSource)
		}
		fmt.Fprintf(out, "  format: %s\n", component.Format)
		fmt.Fprintf(out, "  license: %s\n", component.License)
		fmt.Fprintf(out, "  resolution.kind: %s\n", component.Resolution.Kind)
		if component.Resolution.Revision != "" {
			fmt.Fprintf(out, "  resolution.revision: %s\n", component.Resolution.Revision)
		}
		fmt.Fprintf(out, "  resolution.pinned: %v\n", component.Resolution.Pinned)
		if component.Integrity.Checksum != "" {
			fmt.Fprintf(out, "  integrity.checksum: %s\n", component.Integrity.Checksum)
		}
		if component.Integrity.Signature != "" {
			fmt.Fprintf(out, "  integrity.signature: %s\n", component.Integrity.Signature)
		}
		if component.Resolution.Warning != "" {
			fmt.Fprintf(out, "  WARN: %s\n", component.Resolution.Warning)
		}
	}
	if !report.OK {
		return model.ErrInstallFailed
	}
	return nil
}

func BuildAuditReport(registryPath, packRef string) (model.AuditReport, error) {
	pack, err := registry.FindPack(registryPath, packRef)
	if err != nil {
		return model.AuditReport{}, err
	}
	expanded, err := registry.ExpandPack(registryPath, pack, map[string]bool{})
	if err != nil {
		return model.AuditReport{}, err
	}
	report := model.AuditReport{
		Pack: model.AuditPack{
			ID: expanded.ID, Name: expanded.Name, Version: expanded.Version,
			License: util.ValueOrUnknown(expanded.License), UpstreamSource: expanded.UpstreamSource,
		},
		OK: true,
	}
	for _, capability := range expanded.Capabilities {
		resolution := resolve.ResolveSourceLive(capability.Source)
		component := model.AuditComponent{
			Type: capability.Type, Name: capability.Name, Source: capability.Source,
			UpstreamSource: capability.UpstreamSource, Format: util.ValueOrUnknown(capability.Format),
			License: util.ValueOrUnknown(capability.License), Trust: capability.Trust,
			NativeCommand: capability.Install != nil && capability.Install["command"] != "",
		}
		component.Resolution.Kind = resolution.Kind
		component.Resolution.Revision = resolution.Revision
		component.Resolution.Pinned = resolution.Pinned
		component.Resolution.Warning = resolution.Warning
		component.Integrity.Checksum = capability.Integrity.Checksum
		component.Integrity.Signature = capability.Integrity.Signature
		report.Components = append(report.Components, component)
		if resolution.Warning != "" && !util.IsLocalSource(capability.Source) && (resolution.Kind == "remote" || !resolution.Pinned) {
			report.OK = false
			report.Violations = append(report.Violations, capability.Name+": "+resolution.Warning)
		}
	}
	return report, nil
}

func LoadTrustPolicy(path string) (model.TrustPolicy, error) {
	data, err := os.ReadFile(util.ExpandHome(path))
	if err != nil {
		return model.TrustPolicy{}, err
	}
	var policy model.TrustPolicy
	if err := json.Unmarshal(data, &policy); err != nil {
		return model.TrustPolicy{}, err
	}
	return policy, nil
}

func matchesAny(value string, patterns []string) bool {
	for _, pattern := range patterns {
		pattern = strings.TrimSuffix(pattern, "*")
		if strings.Contains(value, pattern) || strings.HasPrefix(value, pattern) {
			return true
		}
	}
	return false
}
