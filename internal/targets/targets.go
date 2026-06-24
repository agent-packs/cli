package targets

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agent-packs/cli/internal/model"
)

// TargetMatrix maps each supported agent to its capability destinations. The
// legacy Memory/Settings fields keep old custom-target JSON compatible; the
// richer destination slices carry verification/source metadata and alternate
// documented files.
var TargetMatrix = map[string]model.TargetSpec{
	"claude": {ID: "claude", Name: "Claude Code", GlobalSkills: ".claude/skills", ProjectSkills: ".claude/skills",
		Memory:   model.FileDest{Global: ".claude/CLAUDE.md", Project: "CLAUDE.md", Kind: "markdown", Verified: true, SourceURL: "https://code.claude.com/docs/en/memory"},
		Settings: model.FileDest{Global: ".claude/settings.json", Project: ".claude/settings.json", Kind: "json", Verified: true, SourceURL: "https://code.claude.com/docs/en/settings"},
		InstructionDestinations: []model.FileDest{
			{Scope: "global", Path: ".claude/CLAUDE.md", Kind: "markdown", Verified: true, SourceURL: "https://code.claude.com/docs/en/memory", Default: true},
			{Scope: "project", Path: "CLAUDE.md", Kind: "markdown", Verified: true, SourceURL: "https://code.claude.com/docs/en/memory", Default: true},
			{Scope: "project", Path: ".claude/CLAUDE.md", Kind: "markdown", Verified: true, SourceURL: "https://code.claude.com/docs/en/memory"},
		},
		SettingsDestinations: []model.FileDest{
			{Scope: "global", Path: ".claude/settings.json", Kind: "json", Verified: true, SourceURL: "https://code.claude.com/docs/en/settings", Default: true},
			{Scope: "project", Path: ".claude/settings.json", Kind: "json", Verified: true, SourceURL: "https://code.claude.com/docs/en/settings", Default: true},
			{Scope: "project", Path: ".claude/settings.local.json", Kind: "json", Verified: true, SourceURL: "https://code.claude.com/docs/en/settings"},
		},
		CommandDestinations: []model.FileDest{
			{Scope: "global", Path: ".claude/commands/*.md", Kind: "markdown", Verified: true, SourceURL: "https://docs.anthropic.com/en/docs/claude-code/slash-commands", Default: true},
			{Scope: "project", Path: ".claude/commands/*.md", Kind: "markdown", Verified: true, SourceURL: "https://docs.anthropic.com/en/docs/claude-code/slash-commands", Default: true},
		},
		SubagentDestinations: []model.FileDest{
			{Scope: "global", Path: ".claude/agents/*.md", Kind: "markdown", Verified: true, SourceURL: "https://docs.anthropic.com/en/docs/claude-code/sub-agents", Default: true},
			{Scope: "project", Path: ".claude/agents/*.md", Kind: "markdown", Verified: true, SourceURL: "https://docs.anthropic.com/en/docs/claude-code/sub-agents", Default: true},
		}},
	"codex": {ID: "codex", Name: "Codex", GlobalSkills: ".codex/skills", ProjectSkills: ".agents/skills",
		Memory:   model.FileDest{Global: ".codex/AGENTS.md", Project: "AGENTS.md", Kind: "markdown", Verified: true, SourceURL: "https://developers.openai.com/codex/codex-manual.md"},
		Settings: model.FileDest{Global: ".codex/config.toml", Project: ".codex/config.toml", Kind: "toml", Verified: true, SourceURL: "https://developers.openai.com/codex/codex-manual.md"},
		InstructionDestinations: []model.FileDest{
			{Scope: "global", Path: ".codex/AGENTS.md", Kind: "markdown", Verified: true, SourceURL: "https://developers.openai.com/codex/codex-manual.md", Default: true},
			{Scope: "global", Path: ".codex/AGENTS.override.md", Kind: "markdown", Verified: true, SourceURL: "https://developers.openai.com/codex/codex-manual.md"},
			{Scope: "project", Path: "AGENTS.md", Kind: "markdown", Verified: true, SourceURL: "https://developers.openai.com/codex/codex-manual.md", Default: true},
			{Scope: "project", Path: "AGENTS.override.md", Kind: "markdown", Verified: true, SourceURL: "https://developers.openai.com/codex/codex-manual.md"},
		},
		SettingsDestinations: []model.FileDest{
			{Scope: "global", Path: ".codex/config.toml", Kind: "toml", Verified: true, SourceURL: "https://developers.openai.com/codex/codex-manual.md", Default: true},
			{Scope: "project", Path: ".codex/config.toml", Kind: "toml", Verified: true, SourceURL: "https://developers.openai.com/codex/codex-manual.md", Default: true},
		}},
	"cursor": {ID: "cursor", Name: "Cursor", GlobalSkills: ".cursor/skills", ProjectSkills: ".cursor/skills"},
	"gemini": {ID: "gemini", Name: "Gemini CLI", GlobalSkills: ".gemini/skills", ProjectSkills: ".gemini/skills",
		Memory:   model.FileDest{Global: ".gemini/GEMINI.md", Project: "GEMINI.md", Kind: "markdown", Verified: true, SourceURL: "https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/tools/memory.md"},
		Settings: model.FileDest{Global: ".gemini/settings.json", Project: ".gemini/settings.json", Kind: "json", Verified: true, SourceURL: "https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/cli/configuration.md"},
		InstructionDestinations: []model.FileDest{
			{Scope: "global", Path: ".gemini/GEMINI.md", Kind: "markdown", Verified: true, SourceURL: "https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/tools/memory.md", Default: true},
			{Scope: "project", Path: "GEMINI.md", Kind: "markdown", Verified: true, SourceURL: "https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/tools/memory.md", Default: true},
		},
		SettingsDestinations: []model.FileDest{
			{Scope: "global", Path: ".gemini/settings.json", Kind: "json", Verified: true, SourceURL: "https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/cli/configuration.md", Default: true},
			{Scope: "project", Path: ".gemini/settings.json", Kind: "json", Verified: true, SourceURL: "https://raw.githubusercontent.com/google-gemini/gemini-cli/main/docs/cli/configuration.md", Default: true},
		}},
	"copilot": {ID: "copilot", Name: "GitHub Copilot", GlobalSkills: ".github/skills", ProjectSkills: ".github/skills",
		Memory: model.FileDest{Project: ".github/copilot-instructions.md", Kind: "markdown", Verified: true, SourceURL: "https://docs.github.com/en/copilot/how-tos/copilot-on-github/customize-copilot/add-custom-instructions/add-repository-instructions"},
		InstructionDestinations: []model.FileDest{
			{Scope: "project", Path: ".github/copilot-instructions.md", Kind: "markdown", Verified: true, SourceURL: "https://docs.github.com/en/copilot/how-tos/copilot-on-github/customize-copilot/add-custom-instructions/add-repository-instructions", Default: true},
			{Scope: "project", Path: ".github/instructions/*.instructions.md", Kind: "markdown", Verified: true, SourceURL: "https://docs.github.com/en/copilot/how-tos/copilot-on-github/customize-copilot/add-custom-instructions/add-repository-instructions"},
			{Scope: "project", Path: "AGENTS.md", Kind: "markdown", Verified: true, SourceURL: "https://docs.github.com/en/copilot/how-tos/copilot-on-github/customize-copilot/add-custom-instructions/add-repository-instructions"},
		}},
	"goose": {ID: "goose", Name: "Goose", GlobalSkills: ".goose/skills", ProjectSkills: ".goose/skills",
		Memory: model.FileDest{Global: ".config/goose/.goosehints", Project: ".goosehints", Kind: "markdown"}},
	"opencode": {ID: "opencode", Name: "OpenCode", GlobalSkills: ".opencode/skills", ProjectSkills: ".opencode/skills",
		Memory:   model.FileDest{Global: ".config/opencode/AGENTS.md", Project: "AGENTS.md", Kind: "markdown", Verified: true, SourceURL: "https://opencode.ai/docs/rules"},
		Settings: model.FileDest{Global: ".config/opencode/opencode.json", Project: "opencode.json", Kind: "json", Verified: true, SourceURL: "https://opencode.ai/docs/config"},
		InstructionDestinations: []model.FileDest{
			{Scope: "global", Path: ".config/opencode/AGENTS.md", Kind: "markdown", Verified: true, SourceURL: "https://opencode.ai/docs/rules", Default: true},
			{Scope: "project", Path: "AGENTS.md", Kind: "markdown", Verified: true, SourceURL: "https://opencode.ai/docs/rules", Default: true},
		},
		SettingsDestinations: []model.FileDest{
			{Scope: "global", Path: ".config/opencode/opencode.json", Kind: "json", Verified: true, SourceURL: "https://opencode.ai/docs/config", Default: true},
			{Scope: "project", Path: "opencode.json", Kind: "json", Verified: true, SourceURL: "https://opencode.ai/docs/config", Default: true},
		}},
	"generic": {ID: "generic", Name: "Generic", GlobalSkills: "skills", ProjectSkills: "skills",
		Memory: model.FileDest{Global: "AGENTS.md", Project: "AGENTS.md", Kind: "markdown"},
		CommandDestinations: []model.FileDest{
			{Scope: "target", Path: ".agent-packs/commands/*.md", Kind: "markdown", Default: true},
			{Scope: "global", Path: ".agent-packs/commands/*.md", Kind: "markdown", Default: true},
			{Scope: "project", Path: ".agent-packs/commands/*.md", Kind: "markdown", Default: true},
		},
		HookDestinations: []model.FileDest{
			{Scope: "target", Path: ".agent-packs/hooks/*.json", Kind: "json", Default: true},
			{Scope: "global", Path: ".agent-packs/hooks/*.json", Kind: "json", Default: true},
			{Scope: "project", Path: ".agent-packs/hooks/*.json", Kind: "json", Default: true},
		},
		SubagentDestinations: []model.FileDest{
			{Scope: "target", Path: ".agent-packs/agents/*.md", Kind: "markdown", Default: true},
			{Scope: "global", Path: ".agent-packs/agents/*.md", Kind: "markdown", Default: true},
			{Scope: "project", Path: ".agent-packs/agents/*.md", Kind: "markdown", Default: true},
		},
		PromptDestinations: []model.FileDest{
			{Scope: "target", Path: ".agent-packs/prompts/*.md", Kind: "markdown", Default: true},
			{Scope: "global", Path: ".agent-packs/prompts/*.md", Kind: "markdown", Default: true},
			{Scope: "project", Path: ".agent-packs/prompts/*.md", Kind: "markdown", Default: true},
		},
		TemplateDestinations: []model.FileDest{
			{Scope: "target", Path: ".agent-packs/templates/*.md", Kind: "markdown", Default: true},
			{Scope: "global", Path: ".agent-packs/templates/*.md", Kind: "markdown", Default: true},
			{Scope: "project", Path: ".agent-packs/templates/*.md", Kind: "markdown", Default: true},
		}},
}

// MergeFileDest returns the FileDest for a merge capability type ("memory" or
// "settings") on the given spec.
func MergeFileDest(spec model.TargetSpec, capType string) (model.FileDest, bool) {
	switch capType {
	case "memory":
		if len(spec.InstructionDestinations) > 0 {
			return spec.InstructionDestinations[0], true
		}
		return spec.Memory, true
	case "settings":
		if len(spec.SettingsDestinations) > 0 {
			return spec.SettingsDestinations[0], true
		}
		return spec.Settings, true
	default:
		return model.FileDest{}, false
	}
}

// FileTargetRoot resolves the absolute destination file and file kind for a
// file-backed capability on an agent at a scope. ok is false when
// the agent does not support that capability type at that scope, in which case
// the caller should skip+warn.
func FileTargetRoot(capType, target, agent, scope string) (path, kind string, ok bool) {
	dest, ok := FileTargetDest(capType, agent, scope)
	if !ok {
		return "", "", false
	}
	return filepath.Join(target, destPathForScope(dest, scope)), dest.Kind, true
}

func FileTargetDest(capType, agent, scope string) (model.FileDest, bool) {
	spec, found := TargetMatrix[NormalizeAgent(agent)]
	if !found {
		spec = TargetMatrix["generic"]
	}
	var candidates []model.FileDest
	switch capType {
	case "memory":
		candidates = spec.InstructionDestinations
		if len(candidates) == 0 {
			candidates = []model.FileDest{spec.Memory}
		}
	case "settings":
		candidates = spec.SettingsDestinations
		if len(candidates) == 0 {
			candidates = []model.FileDest{spec.Settings}
		}
	case "command":
		candidates = spec.CommandDestinations
		if len(candidates) == 0 && NormalizeAgent(agent) != "generic" {
			candidates = TargetMatrix["generic"].CommandDestinations
		}
	case "hook":
		candidates = spec.HookDestinations
		if len(candidates) == 0 && NormalizeAgent(agent) != "generic" {
			candidates = TargetMatrix["generic"].HookDestinations
		}
	case "subagent":
		candidates = spec.SubagentDestinations
		if len(candidates) == 0 && NormalizeAgent(agent) != "generic" {
			candidates = TargetMatrix["generic"].SubagentDestinations
		}
	case "prompt":
		candidates = spec.PromptDestinations
		if len(candidates) == 0 && NormalizeAgent(agent) != "generic" {
			candidates = TargetMatrix["generic"].PromptDestinations
		}
	case "template":
		candidates = spec.TemplateDestinations
		if len(candidates) == 0 && NormalizeAgent(agent) != "generic" {
			candidates = TargetMatrix["generic"].TemplateDestinations
		}
	default:
		return model.FileDest{}, false
	}
	for _, dest := range candidates {
		if _, ok := dest.PathFor(scope); ok && dest.Default {
			return dest, true
		}
	}
	for _, dest := range candidates {
		if _, ok := dest.PathFor(scope); ok {
			return dest, true
		}
	}
	return model.FileDest{}, false
}

func destPathForScope(dest model.FileDest, scope string) string {
	rel, _ := dest.PathFor(scope)
	return rel
}

// Aliases maps common pack metadata tool IDs to canonical target matrix keys.
var Aliases = map[string]string{
	"claude-code":    "claude",
	"github-copilot": "copilot",
}

var SkillTargets = legacySkillTargets()

func legacySkillTargets() map[string]string {
	targets := map[string]string{}
	for id, spec := range TargetMatrix {
		targets[id] = spec.GlobalSkills
	}
	return targets
}

func NormalizeAgent(agent string) string {
	key := strings.ToLower(strings.TrimSpace(agent))
	if canonical, ok := Aliases[key]; ok {
		return canonical
	}
	return key
}

func ValidAgent(agent string) bool {
	_, ok := TargetMatrix[NormalizeAgent(agent)]
	return ok
}

func PackSupportsTool(packTools []string, agent string) bool {
	normalized := NormalizeAgent(agent)
	for _, tool := range packTools {
		if NormalizeAgent(tool) == normalized {
			return true
		}
	}
	return false
}

// AllSkillRoots returns the unique skill directories under target across every
// known tool (global and project scopes). It is used to discover skills that
// were installed outside the agent-packs receipt path.
func AllSkillRoots(target string) []string {
	seen := map[string]bool{}
	roots := []string{}
	for _, spec := range TargetMatrix {
		for _, rel := range []string{spec.GlobalSkills, spec.ProjectSkills} {
			root := filepath.Join(target, rel)
			if !seen[root] {
				seen[root] = true
				roots = append(roots, root)
			}
		}
	}
	sort.Strings(roots)
	return roots
}

func SkillTargetRoot(target, agent, scope string) string {
	spec, ok := TargetMatrix[NormalizeAgent(agent)]
	if !ok {
		spec = TargetMatrix["generic"]
	}
	root := spec.GlobalSkills
	if scope == "project" {
		root = spec.ProjectSkills
	}
	return filepath.Join(target, root)
}

func PrintTargetMatrix(out io.Writer) error {
	ids := []string{}
	for id := range TargetMatrix {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	fmt.Fprintln(out, "tool\tname\tglobal skills\tproject skills\tmemory\tsettings\taliases")
	for _, id := range ids {
		spec := TargetMatrix[id]
		aliases := aliasNamesFor(id)
		fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", spec.ID, spec.Name, spec.GlobalSkills, spec.ProjectSkills,
			supportCell(spec.Memory), supportCell(spec.Settings), strings.Join(aliases, ", "))
	}
	return nil
}

func TargetMatrixList() []model.TargetSpec {
	ids := []string{}
	for id := range TargetMatrix {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	specs := make([]model.TargetSpec, 0, len(ids))
	for _, id := range ids {
		specs = append(specs, TargetMatrix[id])
	}
	return specs
}

// supportCell renders a FileDest as a short support indicator for the matrix.
func supportCell(dest model.FileDest) string {
	if dest.Project == "" && dest.Global == "" {
		return "-"
	}
	if dest.Project != "" {
		return dest.Project
	}
	return dest.Global
}

func aliasNamesFor(canonical string) []string {
	names := []string{}
	for alias, target := range Aliases {
		if target == canonical {
			names = append(names, alias)
		}
	}
	sort.Strings(names)
	return names
}
