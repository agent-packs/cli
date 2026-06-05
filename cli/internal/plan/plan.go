package plan

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sandeshh/agent-packs/cli/internal/model"
	"github.com/sandeshh/agent-packs/cli/internal/targets"
	"github.com/sandeshh/agent-packs/cli/internal/util"
)

func BuildInstallPlan(pack model.Pack, target, agent, only string) model.Plan {
	return BuildInstallPlanWithOptions(pack, target, agent, only, model.InstallOptions{Mode: "copy", OnConflict: "overwrite", Scope: "target"})
}

func BuildInstallPlanWithOptions(pack model.Pack, target, agent, only string, options model.InstallOptions) model.Plan {
	options = normalizeInstallOptions(options)
	items := []model.PlanItem{}
	for _, capability := range selectCapabilities(pack.Capabilities, only) {
		items = append(items, planCapability(capability, target, agent, options))
	}
	return model.Plan{
		Pack: pack.ID, Version: pack.Version, Agent: agent, Target: target,
		Mode: options.Mode, OnConflict: options.OnConflict, Scope: options.Scope,
		Capabilities: items,
	}
}

func normalizeInstallOptions(options model.InstallOptions) model.InstallOptions {
	if options.Mode == "" {
		options.Mode = "reference"
	}
	if options.OnConflict == "" {
		options.OnConflict = "skip"
	}
	if options.Scope == "" {
		options.Scope = "target"
	}
	return options
}

func printPlanSummary(plan model.Plan, out io.Writer) {
	counts := map[string]int{}
	for _, item := range plan.Capabilities {
		counts[item.Action]++
	}
	if len(plan.Capabilities) == 0 {
		return
	}
	actions := []string{}
	for action := range counts {
		actions = append(actions, action)
	}
	sort.Strings(actions)
	parts := []string{}
	for _, action := range actions {
		parts = append(parts, fmt.Sprintf("%s=%d", action, counts[action]))
	}
	fmt.Fprintf(out, "Plan: %s\n", strings.Join(parts, ", "))
}

func PrintPlan(plan model.Plan, out io.Writer) {
	fmt.Fprintf(out, "Pack: %s\n", plan.Pack)
	fmt.Fprintf(out, "Agent: %s\n", plan.Agent)
	fmt.Fprintf(out, "Target: %s\n", plan.Target)
	fmt.Fprintf(out, "Mode: %s\n", plan.Mode)
	fmt.Fprintf(out, "Conflict: %s\n", plan.OnConflict)
	printPlanSummary(plan, out)
	fmt.Fprintln(out)
	if len(plan.Capabilities) == 0 {
		fmt.Fprintln(out, "No matching capabilities.")
		return
	}
	for _, item := range plan.Capabilities {
		fmt.Fprintf(out, "- %s: %s\n", item.Type, item.Name)
		fmt.Fprintf(out, "  action: %s\n", item.Action)
		if item.Destination != "" {
			fmt.Fprintf(out, "  destination: %s\n", item.Destination)
		}
		if item.Command != "" {
			fmt.Fprintf(out, "  command: %s\n", item.Command)
		}
		if item.Source != "" {
			fmt.Fprintf(out, "  source: %s\n", item.Source)
		}
		if item.UpstreamSource != "" && item.UpstreamSource != item.Source {
			fmt.Fprintf(out, "  upstreamSource: %s\n", item.UpstreamSource)
		}
	}
}

func selectCapabilities(capabilities []model.Capability, only string) []model.Capability {
	if only == "all" {
		return capabilities
	}
	wanted := ""
	if only == "skills" {
		wanted = "skill"
	} else if only == "plugins" {
		wanted = "plugin"
	}
	selected := []model.Capability{}
	for _, capability := range capabilities {
		if capability.Type == wanted {
			selected = append(selected, capability)
		}
	}
	return selected
}

func planCapability(capability model.Capability, target, agent string, options model.InstallOptions) model.PlanItem {
	expectedChecksum := capability.Integrity.Checksum
	switch capability.Type {
	case "skill":
		entry := capability.Entry
		if entry == "" {
			entry = "SKILL.md"
		}
		action := skillAction(capability, options)
		return model.PlanItem{
			Type: "skill", Name: capability.Name, Action: action,
			Mode: options.Mode, OnConflict: options.OnConflict,
			Source: capability.Source, UpstreamSource: capability.UpstreamSource,
			Entry: entry, Destination: skillDestination(capability, target, agent, options),
			ExpectedChecksum: expectedChecksum, Status: "planned",
		}
	case "plugin":
		action := "reference"
		if options.Mode != "reference" && !capability.Reference {
			action = "native-install"
		}
		return model.PlanItem{
			Type: "plugin", Name: capability.Name, Action: action,
			Mode: options.Mode, OnConflict: options.OnConflict,
			Source: capability.Source, UpstreamSource: capability.UpstreamSource,
			Format: capability.Format, Command: capability.Install["command"],
			Method: capability.Install["method"], Package: capability.Install["package"],
			Marketplace: capability.Install["marketplace"],
			ExpectedChecksum: expectedChecksum, Status: "planned",
		}
	default:
		return model.PlanItem{
			Type: capability.Type, Name: capability.Name, Action: "record",
			Source: capability.Source, ExpectedChecksum: expectedChecksum, Status: "planned",
		}
	}
}

func skillAction(capability model.Capability, options model.InstallOptions) string {
	if options.Mode == "reference" || options.Mode == "native" {
		return "reference"
	}
	if options.Mode == "symlink" {
		return "symlink"
	}
	if util.IsLocalSource(capability.Source) {
		return "copy"
	}
	return "fetch-copy"
}

func skillDestination(capability model.Capability, target, agent string, options model.InstallOptions) string {
	if options.Mode == "reference" || options.Mode == "native" {
		return ""
	}
	return filepath.Join(targets.SkillTargetRoot(target, agent, options.Scope), util.Slugify(capability.Name))
}
