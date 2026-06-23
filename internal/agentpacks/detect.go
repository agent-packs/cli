package agentpacks

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agent-packs/cli/internal/config"
	reg "github.com/agent-packs/cli/internal/registry"
)

// ProjectDetection is the result of inspecting a project directory for smart
// init: the most likely agent in use and packs recommended from the detected
// stack. Detection is read-only — it never installs or executes anything.
type ProjectDetection struct {
	Agent string
	Stack []string
	Packs []string
}

// agentSignals maps an agent ID to project-local paths that indicate the agent
// is configured for this repo. Order is most-specific first.
var agentSignals = []struct {
	agent string
	paths []string
}{
	{"claude", []string{".claude"}},
	{"cursor", []string{".cursor"}},
	{"gemini", []string{".gemini", "GEMINI.md"}},
	{"copilot", []string{".github/copilot-instructions.md", ".github/instructions"}},
	{"opencode", []string{"opencode.json", ".opencode"}},
	{"codex", []string{".codex", "AGENTS.md"}},
}

// stackSignals maps a manifest file to capability tags it implies.
var stackSignals = []struct {
	file string
	tags []string
}{
	{"package.json", []string{"javascript", "typescript", "node", "frontend"}},
	{"tsconfig.json", []string{"typescript", "frontend"}},
	{"go.mod", []string{"go", "backend"}},
	{"Cargo.toml", []string{"rust"}},
	{"pyproject.toml", []string{"python"}},
	{"requirements.txt", []string{"python"}},
	{"Gemfile", []string{"ruby"}},
	{"pom.xml", []string{"java"}},
	{"build.gradle", []string{"java", "kotlin"}},
}

// DetectAgent returns the first agent whose project-local signal is present, or
// "" when none is detected. It only reads the given project directory.
func DetectAgent(projectDir string) string {
	for _, sig := range agentSignals {
		for _, p := range sig.paths {
			if _, err := os.Stat(filepath.Join(projectDir, p)); err == nil {
				return sig.agent
			}
		}
	}
	return ""
}

// DetectStack returns the deduplicated set of capability tags implied by the
// manifest files present in the project directory.
func DetectStack(projectDir string) []string {
	seen := map[string]bool{}
	tags := []string{}
	for _, sig := range stackSignals {
		if _, err := os.Stat(filepath.Join(projectDir, sig.file)); err == nil {
			for _, t := range sig.tags {
				if !seen[t] {
					seen[t] = true
					tags = append(tags, t)
				}
			}
		}
	}
	return tags
}

// RecommendPacks ranks registry packs by how many of their tags/categories
// overlap the detected stack, returning up to limit pack IDs (highest overlap
// first). Returns nil when the stack is empty or the registry can't be read.
func RecommendPacks(registryPath string, stack []string, limit int) []string {
	if len(stack) == 0 || limit <= 0 {
		return nil
	}
	packs, err := reg.LoadPacks(registryPath)
	if err != nil {
		return nil
	}
	stackSet := map[string]bool{}
	for _, t := range stack {
		stackSet[strings.ToLower(t)] = true
	}
	type scored struct {
		id    string
		score int
	}
	ranked := []scored{}
	for _, p := range packs {
		if p.Deprecated {
			continue
		}
		score := 0
		for _, tag := range append(append([]string{}, p.Tags...), p.Categories...) {
			if stackSet[strings.ToLower(tag)] {
				score++
			}
		}
		if score > 0 {
			ranked = append(ranked, scored{p.ID, score})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })
	ids := []string{}
	for _, r := range ranked {
		ids = append(ids, r.id)
		if len(ids) >= limit {
			break
		}
	}
	return ids
}

// DetectProject runs agent + stack + recommendation detection for smart init.
func DetectProject(projectDir, registryPath string, limit int) ProjectDetection {
	stack := DetectStack(projectDir)
	return ProjectDetection{
		Agent: DetectAgent(projectDir),
		Stack: stack,
		Packs: RecommendPacks(registryPath, stack, limit),
	}
}

// InitProjectWithDetection inspects the project (unless detect is false), folds
// any detected agent and recommended packs into opts, writes the config, and
// returns the path plus what was detected so the caller can summarize it.
func InitProjectWithDetection(projectDir, registryPath string, detect bool, opts config.InitOptions, agentExplicit bool) (string, ProjectDetection, error) {
	var det ProjectDetection
	if detect {
		det = DetectProject(projectDir, registryPath, 3)
		if det.Agent != "" && !agentExplicit {
			opts.Agent = det.Agent
		}
		if len(opts.Packs) == 0 {
			opts.Packs = det.Packs
		}
	}
	path, err := config.Init(projectDir, opts)
	return path, det, err
}
