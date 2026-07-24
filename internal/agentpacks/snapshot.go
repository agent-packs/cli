package agentpacks

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agent-packs/cli/internal/model"
	"github.com/agent-packs/cli/internal/registry"
	"github.com/agent-packs/cli/internal/resolve"
	"github.com/agent-packs/cli/internal/targets"
	"github.com/agent-packs/cli/internal/util"
)

// SnapshotOptions configures Snapshot. Project is the directory to inspect,
// Agent overrides detection, ID names the resulting pack, Output is the
// manifest path (default <project>/agent-pack.json), and Pin controls whether
// moving upstream refs are resolved to commit SHAs over the network.
type SnapshotOptions struct {
	Project string
	Agent   string
	ID      string
	Output  string
	Pin     bool
}

// Snapshot turns a project's existing agent setup into a pack manifest: every
// skill and command found in the detected agent's project directories becomes
// a capability. Skills installed from a registry carry their upstream source
// in SKILL.md frontmatter — those are recorded (and pinned to a commit SHA
// when Pin is set); everything else is recorded as a project-relative source
// with a content checksum, so installs can verify it and `status` can catch
// drift. The manifest is written to opts.Output and the path returned.
func Snapshot(opts SnapshotOptions, out io.Writer) (string, error) {
	project, err := filepath.Abs(util.ExpandHome(opts.Project))
	if err != nil {
		return "", err
	}
	agent := opts.Agent
	if agent == "" {
		agent = DetectAgent(project)
	}
	if agent == "" {
		return "", fmt.Errorf("no agent directory detected in %s; pass --agent (run `agent-packs doctor targets` for supported tools)", project)
	}
	agent = targets.NormalizeAgent(agent)
	if !targets.ValidAgent(agent) {
		return "", fmt.Errorf("invalid agent %q: run `agent-packs doctor targets` for supported tools", agent)
	}

	id := opts.ID
	if id == "" {
		id = util.Slugify(filepath.Base(project)) + "-pack"
	}

	var capabilities []model.Capability
	skillsRoot := targets.SkillTargetRoot(project, agent, "project")
	skillCaps, err := snapshotSkills(project, skillsRoot, opts.Pin, out)
	if err != nil {
		return "", err
	}
	capabilities = append(capabilities, skillCaps...)
	if commandsRoot, _, ok := targets.FileTargetRoot("command", project, agent, "project"); ok {
		// Command destinations are glob patterns (e.g. .claude/commands/*.md);
		// scan the directory that contains them.
		if strings.Contains(filepath.Base(commandsRoot), "*") {
			commandsRoot = filepath.Dir(commandsRoot)
		}
		commandCaps, err := snapshotCommands(project, commandsRoot)
		if err != nil {
			return "", err
		}
		capabilities = append(capabilities, commandCaps...)
	}
	if len(capabilities) == 0 {
		return "", fmt.Errorf("no skills or commands found for agent %q under %s", agent, project)
	}

	pack := model.Pack{
		ID:      id,
		Name:    titleFromID(id),
		Version: "0.1.0",
		Description: fmt.Sprintf("Snapshot of the %s capabilities in %s, taken %s by agent-packs snapshot. Edit this description before sharing.",
			agent, filepath.Base(project), time.Now().UTC().Format("2006-01-02")),
		LastVerified: time.Now().UTC().Format("2006-01-02"),
		ReviewStatus: "draft",
		Trust:        "unverified",
		Requirements: model.Requirements{AgentPacks: ">=0.1.0", Tools: map[string]string{agent: "latest"}},
		Tools:        []string{agent},
		Scope:        []string{"project"},
		Capabilities: capabilities,
	}

	output := opts.Output
	if output == "" {
		output = filepath.Join(project, "agent-pack.json")
	}
	if err := util.WriteJSON(output, pack); err != nil {
		return "", err
	}

	remote, pinned, local := 0, 0, 0
	for _, capability := range pack.Capabilities {
		if util.IsLocalSource(capability.Source) {
			local++
			continue
		}
		remote++
		if resolve.ResolveSource(capability.Source).Pinned {
			pinned++
		}
	}
	fmt.Fprintf(out, "Wrote %s: %d capabilities (%d remote, %d pinned; %d local with checksums)\n",
		output, len(pack.Capabilities), remote, pinned, local)
	fmt.Fprintln(out, "\nNext steps:")
	fmt.Fprintf(out, "  - review and edit the manifest (description, id, anything you don't want to share)\n")
	fmt.Fprintf(out, "  - commit it, then teammates run: agent-packs install ./%s (from the project root)\n", relOrSelf(project, output))
	fmt.Fprintf(out, "  - gate drift in CI with: agent-packs check\n")
	return output, nil
}

func snapshotSkills(project, skillsRoot string, pin bool, out io.Writer) ([]model.Capability, error) {
	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var capabilities []model.Capability
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillPath := filepath.Join(skillsRoot, entry.Name(), "SKILL.md")
		manifest, err := registry.LoadSkillManifest(skillPath)
		if err != nil {
			continue
		}
		capability := model.Capability{
			Type:    "skill",
			Name:    manifest.Name,
			Format:  "agent-skill",
			Entry:   "SKILL.md",
			License: manifest.License,
		}
		if capability.Name == "" {
			capability.Name = entry.Name()
		}
		upstream := manifest.Metadata["agentpacks.upstreamSource"]
		source := manifest.Metadata["agentpacks.source"]
		if source == "" {
			source = upstream
		}
		if source != "" && !util.IsLocalSource(source) {
			capability.Source = source
			capability.UpstreamSource = upstream
			if pin {
				if pinnedURL, moving := pinTreeSource(source); pinnedURL != "" {
					capability.Source = pinnedURL
					if capability.UpstreamSource == "" {
						capability.UpstreamSource = moving
					}
				} else if !resolve.ResolveSource(source).Pinned {
					fmt.Fprintf(out, "WARN  %s: could not pin %s; recording the moving ref\n", capability.Name, source)
				}
			}
		} else {
			rel, err := filepath.Rel(project, filepath.Dir(skillPath))
			if err != nil {
				return nil, err
			}
			capability.Source = filepath.ToSlash(rel)
			sum, err := resolve.HashFile(skillPath)
			if err != nil {
				return nil, err
			}
			capability.Integrity = model.Integrity{Checksum: sum}
		}
		capabilities = append(capabilities, capability)
	}
	sort.Slice(capabilities, func(i, j int) bool { return capabilities[i].Name < capabilities[j].Name })
	return capabilities, nil
}

func snapshotCommands(project, commandsRoot string) ([]model.Capability, error) {
	entries, err := os.ReadDir(commandsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var capabilities []model.Capability
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(commandsRoot, entry.Name())
		rel, err := filepath.Rel(project, path)
		if err != nil {
			return nil, err
		}
		sum, err := resolve.HashFile(path)
		if err != nil {
			return nil, err
		}
		capabilities = append(capabilities, model.Capability{
			Type:      "command",
			Name:      strings.TrimSuffix(entry.Name(), ".md"),
			Source:    filepath.ToSlash(rel),
			Format:    "markdown",
			Integrity: model.Integrity{Checksum: sum},
		})
	}
	sort.Slice(capabilities, func(i, j int) bool { return capabilities[i].Name < capabilities[j].Name })
	return capabilities, nil
}

// pinTreeSource resolves a moving-ref GitHub/GitLab tree URL to the commit SHA
// currently at its head. Returns ("", "") when the source is already pinned or
// cannot be resolved; otherwise (pinned URL, original moving URL).
func pinTreeSource(source string) (string, string) {
	resolution := resolve.ResolveSource(source)
	if resolution.Pinned {
		return "", ""
	}
	repo, ref, _, kind := resolve.ParseGitSource(source)
	if repo == "" || ref == "" || (kind != "github-tree" && kind != "gitlab-tree") {
		return "", ""
	}
	live := resolve.ResolveSourceLive(source)
	if live.Revision == "" || live.Revision == ref {
		return "", ""
	}
	marker := "/tree/" + ref
	index := strings.Index(source, marker)
	if index < 0 {
		return "", ""
	}
	return source[:index] + "/tree/" + live.Revision + source[index+len(marker):], source
}

func titleFromID(id string) string {
	words := strings.Split(strings.ReplaceAll(id, "-", " "), " ")
	for i, word := range words {
		if word == "" {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func relOrSelf(base, path string) string {
	if rel, err := filepath.Rel(base, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return path
}
