package targets

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agent-packs/cli/internal/model"
)

// TargetMatrix maps each supported agent to its capability destinations. Memory
// (markdown) is broadly supported; Settings (JSON deep-merge) is wired only for
// agents with a clean, JSON config file we own. Empty FileDest fields mark
// unsupported (agent, type, scope) combinations, which installs skip+warn.
//
// NOTE: several non-Claude paths are best-effort per the design's per-agent
// matrix and should be confirmed against current agent docs before relying on
// them; Claude is the confirmed reference implementation.
var TargetMatrix = map[string]model.TargetSpec{
	"claude": {ID: "claude", Name: "Claude Code", GlobalSkills: ".claude/skills", ProjectSkills: ".claude/skills",
		Memory:   model.FileDest{Global: ".claude/CLAUDE.md", Project: "CLAUDE.md", Kind: "markdown"},
		Settings: model.FileDest{Global: ".claude/settings.json", Project: ".claude/settings.json", Kind: "json"}},
	"codex": {ID: "codex", Name: "Codex", GlobalSkills: ".codex/skills", ProjectSkills: ".agents/skills",
		Memory: model.FileDest{Global: ".codex/AGENTS.md", Project: "AGENTS.md", Kind: "markdown"}},
	"cursor": {ID: "cursor", Name: "Cursor", GlobalSkills: ".cursor/skills", ProjectSkills: ".cursor/skills"},
	"gemini": {ID: "gemini", Name: "Gemini CLI", GlobalSkills: ".gemini/skills", ProjectSkills: ".gemini/skills",
		Memory:   model.FileDest{Global: ".gemini/GEMINI.md", Project: "GEMINI.md", Kind: "markdown"},
		Settings: model.FileDest{Global: ".gemini/settings.json", Project: ".gemini/settings.json", Kind: "json"}},
	"copilot": {ID: "copilot", Name: "GitHub Copilot", GlobalSkills: ".github/skills", ProjectSkills: ".github/skills",
		Memory: model.FileDest{Project: ".github/copilot-instructions.md", Kind: "markdown"}},
	"goose": {ID: "goose", Name: "Goose", GlobalSkills: ".goose/skills", ProjectSkills: ".goose/skills",
		Memory: model.FileDest{Global: ".config/goose/.goosehints", Project: ".goosehints", Kind: "markdown"}},
	"opencode": {ID: "opencode", Name: "OpenCode", GlobalSkills: ".opencode/skills", ProjectSkills: ".opencode/skills",
		Memory:   model.FileDest{Global: ".config/opencode/AGENTS.md", Project: "AGENTS.md", Kind: "markdown"},
		Settings: model.FileDest{Global: ".config/opencode/opencode.json", Project: "opencode.json", Kind: "json"}},
	"generic": {ID: "generic", Name: "Generic", GlobalSkills: "skills", ProjectSkills: "skills",
		Memory: model.FileDest{Global: "AGENTS.md", Project: "AGENTS.md", Kind: "markdown"}},
}

// MergeFileDest returns the FileDest for a merge capability type ("memory" or
// "settings") on the given spec.
func MergeFileDest(spec model.TargetSpec, capType string) (model.FileDest, bool) {
	switch capType {
	case "memory":
		return spec.Memory, true
	case "settings":
		return spec.Settings, true
	default:
		return model.FileDest{}, false
	}
}

// FileTargetRoot resolves the absolute destination file and merge kind for a
// merge capability (memory/settings) on an agent at a scope. ok is false when
// the agent does not support that capability type at that scope, in which case
// the caller should skip+warn.
func FileTargetRoot(capType, target, agent, scope string) (path, kind string, ok bool) {
	spec, found := TargetMatrix[NormalizeAgent(agent)]
	if !found {
		spec = TargetMatrix["generic"]
	}
	dest, isMerge := MergeFileDest(spec, capType)
	if !isMerge {
		return "", "", false
	}
	rel, supported := dest.PathFor(scope)
	if !supported {
		return "", "", false
	}
	return filepath.Join(target, rel), dest.Kind, true
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
