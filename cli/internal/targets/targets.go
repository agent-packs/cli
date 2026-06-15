package targets

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agent-packs/cli/internal/model"
)

var TargetMatrix = map[string]model.TargetSpec{
	"claude":   {ID: "claude", Name: "Claude Code", GlobalSkills: ".claude/skills", ProjectSkills: ".claude/skills"},
	"codex":    {ID: "codex", Name: "Codex", GlobalSkills: ".codex/skills", ProjectSkills: ".agents/skills"},
	"cursor":   {ID: "cursor", Name: "Cursor", GlobalSkills: ".cursor/skills", ProjectSkills: ".cursor/skills"},
	"gemini":   {ID: "gemini", Name: "Gemini CLI", GlobalSkills: ".gemini/skills", ProjectSkills: ".gemini/skills"},
	"copilot":  {ID: "copilot", Name: "GitHub Copilot", GlobalSkills: ".github/skills", ProjectSkills: ".github/skills"},
	"goose":    {ID: "goose", Name: "Goose", GlobalSkills: ".goose/skills", ProjectSkills: ".goose/skills"},
	"opencode": {ID: "opencode", Name: "OpenCode", GlobalSkills: ".opencode/skills", ProjectSkills: ".opencode/skills"},
	"generic":  {ID: "generic", Name: "Generic", GlobalSkills: "skills", ProjectSkills: "skills"},
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
	fmt.Fprintln(out, "tool\tname\tglobal skills\tproject skills\taliases")
	for _, id := range ids {
		spec := TargetMatrix[id]
		aliases := aliasNamesFor(id)
		fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\n", spec.ID, spec.Name, spec.GlobalSkills, spec.ProjectSkills, strings.Join(aliases, ", "))
	}
	return nil
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
