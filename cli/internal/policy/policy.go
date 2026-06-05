package policy

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sandeshh/agent-packs/cli/internal/model"
	"github.com/sandeshh/agent-packs/cli/internal/registry"
	"github.com/sandeshh/agent-packs/cli/internal/resolve"
	"github.com/sandeshh/agent-packs/cli/internal/util"
)

func PolicyCheck(registryPath, packRef, policyPath string, out io.Writer) error {
	policy, err := LoadTrustPolicy(policyPath)
	if err != nil {
		return err
	}
	pack, err := registry.FindPack(registryPath, packRef)
	if err != nil {
		return err
	}
	expanded, err := registry.ExpandPack(registryPath, pack, map[string]bool{})
	if err != nil {
		return err
	}
	failed := false
	for _, capability := range expanded.Capabilities {
		if matchesAny(capability.Source, policy.DenySources) {
			fmt.Fprintf(out, "FAIL  denied source: %s\n", capability.Source)
			failed = true
		}
		if len(policy.AllowSources) > 0 && !matchesAny(capability.Source, policy.AllowSources) {
			fmt.Fprintf(out, "FAIL  source not allowed: %s\n", capability.Source)
			failed = true
		}
		resolution := resolve.ResolveSource(capability.Source)
		if policy.RequirePinnedRefs && !resolution.Pinned && !util.IsLocalSource(capability.Source) {
			fmt.Fprintf(out, "FAIL  source is not pinned: %s\n", capability.Source)
			failed = true
		}
		if capability.Type == "plugin" && capability.Install != nil && capability.Install["command"] != "" && !policy.AllowNativeCommands {
			fmt.Fprintf(out, "FAIL  native command blocked by policy: %s\n", capability.Name)
			failed = true
		}
	}
	if failed {
		return model.ErrInstallFailed
	}
	fmt.Fprintf(out, "OK    %s satisfies policy\n", expanded.ID)
	return nil
}

func Audit(registryPath, packRef string, out io.Writer) error {
	pack, err := registry.FindPack(registryPath, packRef)
	if err != nil {
		return err
	}
	expanded, err := registry.ExpandPack(registryPath, pack, map[string]bool{})
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "SBOM: %s (%s) v%s\n", expanded.Name, expanded.ID, expanded.Version)
	fmt.Fprintf(out, "Generated for supply-chain audit\n\n")
	fmt.Fprintf(out, "Pack\n")
	fmt.Fprintf(out, "  id: %s\n", expanded.ID)
	fmt.Fprintf(out, "  version: %s\n", expanded.Version)
	fmt.Fprintf(out, "  license: %s\n", util.ValueOrUnknown(expanded.License))
	if expanded.UpstreamSource != "" {
		fmt.Fprintf(out, "  upstreamSource: %s\n", expanded.UpstreamSource)
	}
	fmt.Fprintf(out, "\nComponents (%d)\n", len(expanded.Capabilities))
	failed := false
	for i, capability := range expanded.Capabilities {
		resolution := resolve.ResolveSource(capability.Source)
		fmt.Fprintf(out, "\n[%d] %s:%s\n", i+1, capability.Type, capability.Name)
		fmt.Fprintf(out, "  source: %s\n", capability.Source)
		if capability.UpstreamSource != "" {
			fmt.Fprintf(out, "  upstreamSource: %s\n", capability.UpstreamSource)
		}
		fmt.Fprintf(out, "  format: %s\n", util.ValueOrUnknown(capability.Format))
		fmt.Fprintf(out, "  license: %s\n", util.ValueOrUnknown(capability.License))
		fmt.Fprintf(out, "  resolution.kind: %s\n", resolution.Kind)
		if resolution.Revision != "" {
			fmt.Fprintf(out, "  resolution.revision: %s\n", resolution.Revision)
		}
		fmt.Fprintf(out, "  resolution.pinned: %v\n", resolution.Pinned)
		if capability.Integrity.Checksum != "" {
			fmt.Fprintf(out, "  integrity.checksum: %s\n", capability.Integrity.Checksum)
		}
		if resolution.Warning != "" {
			fmt.Fprintf(out, "  WARN: %s\n", resolution.Warning)
			if !util.IsLocalSource(capability.Source) && (resolution.Kind == "remote" || !resolution.Pinned) {
				failed = true
			}
		}
	}
	if failed {
		return model.ErrInstallFailed
	}
	return nil
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
