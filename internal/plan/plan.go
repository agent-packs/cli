package plan

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agent-packs/cli/internal/model"
	"github.com/agent-packs/cli/internal/targets"
	"github.com/agent-packs/cli/internal/util"
)

func BuildInstallPlan(pack model.Pack, target, agent, only string) model.Plan {
	return BuildInstallPlanWithOptions(pack, target, agent, only, model.InstallOptions{Mode: "copy", OnConflict: "overwrite", Scope: "target"})
}

func BuildInstallPlanWithOptions(pack model.Pack, target, agent, only string, options model.InstallOptions) model.Plan {
	options = normalizeInstallOptions(options)
	items := []model.PlanItem{}
	for _, capability := range selectCapabilities(pack.Capabilities, only) {
		items = append(items, planCapability(pack.ID, capability, target, agent, options))
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
		if isManagedFileType(item.Type) && item.Content != "" {
			fmt.Fprintf(out, "  preview: %s\n", previewContent(item.Content))
		}
		if item.Reason != "" {
			fmt.Fprintf(out, "  note: %s\n", item.Reason)
		}
	}
}

// isManagedFileType reports whether a capability is materialized as a single
// managed file (copied verbatim), as opposed to a merge-into-file capability.
func isManagedFileType(capType string) bool {
	switch capType {
	case "command", "hook", "subagent", "prompt", "template":
		return true
	}
	return false
}

// previewContent renders a single-line, length-capped preview of inline
// capability content so install/dry-run output shows what a hook or command
// will write without dumping a full file.
func previewContent(content string) string {
	flat := strings.Join(strings.Fields(content), " ")
	const max = 120
	if len(flat) > max {
		return flat[:max] + "…"
	}
	return flat
}

func selectCapabilities(capabilities []model.Capability, only string) []model.Capability {
	if only == "all" {
		return capabilities
	}
	wanted := ""
	switch only {
	case "skills":
		wanted = "skill"
	case "plugins":
		wanted = "plugin"
	case "memory":
		wanted = "memory"
	case "settings":
		wanted = "settings"
	case "commands":
		wanted = "command"
	case "hooks":
		wanted = "hook"
	case "subagents":
		wanted = "subagent"
	case "prompts":
		wanted = "prompt"
	case "templates":
		wanted = "template"
	}
	selected := []model.Capability{}
	for _, capability := range capabilities {
		if capability.Type == wanted {
			selected = append(selected, capability)
		}
	}
	return selected
}

func planCapability(packID string, capability model.Capability, target, agent string, options model.InstallOptions) model.PlanItem {
	expectedChecksum := capability.Integrity.Checksum
	switch capability.Type {
	case "memory", "settings":
		return planMergeCapability(packID, capability, target, agent, options)
	case "mcp":
		return planMCPCapability(packID, capability, target, agent, options)
	case "command", "hook", "subagent", "prompt", "template":
		return planManagedFileCapability(packID, capability, target, agent, options)
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
			ExpectedChecksum: expectedChecksum, ExpectedSignature: capability.Integrity.Signature,
			Status: "planned",
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
			UninstallCommand: capability.Install["uninstall"],
			Method:           capability.Install["method"], Package: capability.Install["package"],
			Marketplace:      capability.Install["marketplace"],
			ExpectedChecksum: expectedChecksum, Status: "planned",
		}
	default:
		return model.PlanItem{
			Type: capability.Type, Name: capability.Name, Action: "record",
			Source: capability.Source, ExpectedChecksum: expectedChecksum, Status: "planned",
		}
	}
}

func planManagedFileCapability(packID string, capability model.Capability, target, agent string, options model.InstallOptions) model.PlanItem {
	path, kind, ok := targets.FileTargetRoot(capability.Type, target, agent, options.Scope)
	if !ok {
		return model.PlanItem{
			Type: capability.Type, Name: capability.Name, Action: "skip", Status: "unsupported",
			Reason: fmt.Sprintf("%s capabilities are not supported for agent %q at %s scope", capability.Type, agent, options.Scope),
		}
	}
	if override, found := capability.AgentTargets[targets.NormalizeAgent(agent)]; found && override.Destination != "" {
		path = filepath.Join(target, override.Destination)
		if override.Format != "" {
			kind = override.Format
		}
	}
	path = expandManagedFileDestination(path, capability.Name, kind)
	item := model.PlanItem{
		Type: capability.Type, Name: capability.Name,
		Mode: options.Mode, OnConflict: options.OnConflict,
		Source: capability.Source, UpstreamSource: capability.UpstreamSource,
		Entry: capability.Entry, Destination: path,
		FileKind: kind, Content: capability.Content,
		BlockID:           packID + "/" + util.Slugify(capability.Name),
		ExpectedChecksum:  capability.Integrity.Checksum,
		ExpectedSignature: capability.Integrity.Signature,
		Status:            "planned",
	}
	if options.Mode == "reference" {
		item.Action = "record"
		item.Destination = ""
		return item
	}
	if capability.Type == "hook" && !options.AllowHooks {
		item.Action = "record"
		item.Destination = ""
		item.Reason = "hook not written: installing a hook lets the agent run it automatically; pass --allow-hooks to apply"
		return item
	}
	item.Action = "copy"
	return item
}

func expandManagedFileDestination(path, name, kind string) string {
	if !strings.Contains(path, "*") {
		return path
	}
	ext := filepath.Ext(path)
	if ext == "" {
		ext = extensionForKind(kind)
	}
	return strings.Replace(path, "*"+ext, util.Slugify(name)+ext, 1)
}

func extensionForKind(kind string) string {
	switch kind {
	case "json":
		return ".json"
	case "yaml":
		return ".yaml"
	case "toml":
		return ".toml"
	default:
		return ".md"
	}
}

// planMergeCapability plans a memory/settings capability that merges a fragment
// into a shared agent file. Unsupported (agent, type, scope) combinations are
// recorded as a skip so plan/dry-run output is honest. In reference mode (the
// default) the item is only recorded, never written — actually merging into a
// user's file requires an explicit non-reference mode (e.g. --mode copy).
func planMergeCapability(packID string, capability model.Capability, target, agent string, options model.InstallOptions) model.PlanItem {
	path, kind, ok := targets.FileTargetRoot(capability.Type, target, agent, options.Scope)
	if !ok {
		return model.PlanItem{
			Type: capability.Type, Name: capability.Name, Action: "skip", Status: "unsupported",
			Reason: fmt.Sprintf("%s capabilities are not supported for agent %q at %s scope", capability.Type, agent, options.Scope),
		}
	}
	if override, found := capability.AgentTargets[targets.NormalizeAgent(agent)]; found && override.Destination != "" {
		path = filepath.Join(target, override.Destination)
		if override.Format != "" {
			kind = override.Format
		}
	}
	content := capability.Content
	if capability.Type == "memory" && targets.NormalizeAgent(agent) == "copilot" && capability.ApplyTo != "" {
		content = renderCopilotInstruction(capability.ApplyTo, content)
		path = filepath.Join(target, ".github", "instructions", util.Slugify(capability.Name)+".instructions.md")
	}
	item := model.PlanItem{
		Type: capability.Type, Name: capability.Name,
		Mode: options.Mode, OnConflict: options.OnConflict,
		Source: capability.Source, UpstreamSource: capability.UpstreamSource,
		FileKind: kind, Content: content, MergeKey: capability.MergeKey,
		BlockID: packID + "/" + util.Slugify(capability.Name),
		Status:  "planned",
	}
	if options.Mode == "reference" {
		item.Action = "record"
		return item
	}
	item.Action = "merge"
	item.Destination = path
	return item
}

func renderCopilotInstruction(applyTo, content string) string {
	return fmt.Sprintf("---\napplyTo: %q\n---\n\n%s", applyTo, strings.TrimRight(content, "\n"))
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

func planMCPCapability(packID string, capability model.Capability, target, agent string, options model.InstallOptions) model.PlanItem {
	path, kind, ok := targets.FileTargetRoot("settings", target, agent, options.Scope)
	if !ok {
		return model.PlanItem{
			Type: capability.Type, Name: capability.Name, Action: "skip", Status: "unsupported",
			Reason: fmt.Sprintf("MCP servers are configured via settings, which are not supported for agent %q at %s scope", agent, options.Scope),
		}
	}
	if override, found := capability.AgentTargets[targets.NormalizeAgent(agent)]; found && override.Destination != "" {
		path = filepath.Join(target, override.Destination)
		if override.Format != "" {
			kind = override.Format
		}
	}

	var args []string
	for _, arg := range capability.Args {
		args = append(args, strings.ReplaceAll(arg, "${PROJECT_DIR}", target))
	}
	var env map[string]string
	if len(capability.Env) > 0 {
		env = make(map[string]string)
		for k, v := range capability.Env {
			env[k] = strings.ReplaceAll(v, "${PROJECT_DIR}", target)
		}
	}

	serverDef := map[string]any{
		"command": capability.Command,
		"args":    args,
	}
	if len(env) > 0 {
		serverDef["env"] = env
	}
	mcpConfig := map[string]any{
		capability.ServerName: serverDef,
	}
	contentBytes, _ := json.Marshal(mcpConfig)

	item := model.PlanItem{
		Type: capability.Type, Name: capability.Name,
		Mode: options.Mode, OnConflict: options.OnConflict,
		Source: capability.Source, UpstreamSource: capability.UpstreamSource,
		FileKind: kind, Content: string(contentBytes), MergeKey: "mcpServers",
		BlockID: packID + "/mcp/" + util.Slugify(capability.ServerName),
		Status:  "planned",
	}

	action := "merge"
	if options.Mode == "reference" {
		action = "record"
	}
	if options.ExecuteMCPs && capability.Install != nil && capability.Install["command"] != "" {
		if action == "merge" {
			action = "native-install-and-merge"
		} else {
			action = "native-install"
		}
		item.Command = capability.Install["command"]
		item.UninstallCommand = capability.Install["uninstall"]
		item.Method = capability.Install["method"]
		item.Package = capability.Install["package"]
		item.Marketplace = capability.Install["marketplace"]
	}

	item.Action = action
	if action != "record" && action != "native-install" {
		item.Destination = path
	}
	return item
}
