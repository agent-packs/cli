package author

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/agent-packs/cli/internal/model"
)

type NewOptions struct {
	Kind  string
	ID    string
	Name  string
	Dir   string
	Force bool
}

func New(opts NewOptions) (string, error) {
	if opts.ID == "" {
		return "", fmt.Errorf("id is required")
	}
	if opts.Name == "" {
		opts.Name = titleFromID(opts.ID)
	}
	if opts.Dir == "" {
		opts.Dir = "."
	}
	switch opts.Kind {
	case "pack":
		return newPack(opts)
	case "skill":
		return newSkill(opts)
	case "plugin":
		return newPlugin(opts)
	case "command":
		return newCommand(opts)
	case "hook":
		return newHook(opts)
	case "memory":
		return newMemory(opts)
	case "settings":
		return newSettings(opts)
	case "subagent":
		return newSubagent(opts)
	case "prompt":
		return newPrompt(opts)
	case "template":
		return newTemplate(opts)
	case "tool":
		return newTool(opts)
	default:
		return "", fmt.Errorf("unknown authoring kind: %s", opts.Kind)
	}
}

func newPack(opts NewOptions) (string, error) {
	pack := model.Pack{
		ID: opts.ID, Name: opts.Name, Version: "0.1.0",
		Description:  "Describe what this Agent Pack gives an agent.",
		License:      "Apache-2.0",
		Stability:    "experimental",
		ReviewStatus: "draft",
		Trust:        "community",
		Requirements: model.Requirements{
			AgentPacks: ">=0.1.0",
			Tools: map[string]string{
				"codex":       "latest",
				"claude-code": "latest",
			},
		},
		UseCases: []string{
			"Describe the primary job-to-be-done this pack helps a local coding agent complete.",
			"Describe a review, debugging, implementation, or planning workflow this pack improves.",
		},
		ExamplePrompts: []string{
			"Use this pack to review my current task and propose the safest next implementation step.",
			"Use this pack to inspect this codebase area and produce a source-grounded action plan.",
		},
		Tools:      []string{"codex", "claude-code"},
		Scope:      []string{"global", "project"},
		Tags:       []string{"starter"},
		Categories: []string{"engineering"},
		Skills:     model.CapabilityRefs{{ID: opts.ID + "-skill", Trust: "community"}},
	}
	path := filepath.Join(opts.Dir, opts.ID+".json")
	return path, writeJSON(path, pack, opts.Force)
}

func newSkill(opts NewOptions) (string, error) {
	path := filepath.Join(opts.Dir, opts.ID, "SKILL.md")
	body := fmt.Sprintf("---\nname: %s\ndescription: Describe when an agent should use this skill.\nlicense: Apache-2.0\n---\n\n# %s\n\nUse this skill when...\n", opts.ID, opts.Name)
	return path, writeText(path, body, opts.Force)
}

func newPlugin(opts NewOptions) (string, error) {
	path := filepath.Join(opts.Dir, opts.ID, ".claude-plugin", "plugin.json")
	manifest := map[string]any{
		"name": opts.ID, "displayName": opts.Name, "version": "0.1.0",
		"description": "Describe what this plugin adds to the agent.",
		"license":     "Apache-2.0",
	}
	return path, writeJSON(path, manifest, opts.Force)
}

// scaffoldCapability is a trimmed view of model.Capability for emitting a clean
// starter manifest: omitempty on the optional fields keeps empty source and
// integrity out of the generated file. Authors paste it into a pack's
// capabilities[]. It unmarshals back into model.Capability unchanged.
type scaffoldCapability struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Format  string `json:"format,omitempty"`
	Content string `json:"content,omitempty"`
	License string `json:"license,omitempty"`
}

func newCapabilityFile(opts NewOptions, capType, format, content string) (string, error) {
	cap := scaffoldCapability{
		Type:    capType,
		Name:    opts.Name,
		Format:  format,
		Content: content,
		License: "Apache-2.0",
	}
	path := filepath.Join(opts.Dir, opts.ID+".json")
	return path, writeJSON(path, cap, opts.Force)
}

func newCommand(opts NewOptions) (string, error) {
	return newCapabilityFile(opts, "command", "markdown",
		"Describe the prompt or instructions this command runs.")
}

func newHook(opts NewOptions) (string, error) {
	return newCapabilityFile(opts, "hook", "json",
		`{"event":"preCommit","steps":["describe the automation this hook runs"]}`)
}

func newMemory(opts NewOptions) (string, error) {
	return newCapabilityFile(opts, "memory", "markdown",
		"Durable guidance to merge into the agent's instruction file.")
}

func newSettings(opts NewOptions) (string, error) {
	return newCapabilityFile(opts, "settings", "json",
		`{"describe":"settings fragment to deep-merge into the agent settings file"}`)
}

func newSubagent(opts NewOptions) (string, error) {
	content := fmt.Sprintf("---\nname: %s\ndescription: Describe when the agent should delegate to this subagent.\ntools: Read, Grep, Glob\n---\n\nYou are %s. Describe the subagent's role and how it should behave.\n", opts.ID, opts.Name)
	return newCapabilityFile(opts, "subagent", "markdown", content)
}

func newPrompt(opts NewOptions) (string, error) {
	content := fmt.Sprintf("# %s\n\nDescribe the reusable prompt this capability provides.\n", opts.Name)
	return newCapabilityFile(opts, "prompt", "markdown", content)
}

func newTemplate(opts NewOptions) (string, error) {
	content := fmt.Sprintf("# %s\n\nStarter template body. Replace with the scaffold this template provides.\n", opts.Name)
	return newCapabilityFile(opts, "template", "markdown", content)
}

func newTool(opts NewOptions) (string, error) {
	return newCapabilityFile(opts, "tool", "json",
		`{"name":"`+opts.ID+`","description":"Describe the portable tool descriptor. This file is not executed by Agent Packs."}`)
}

func writeJSON(path string, value any, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("file exists: %s", path)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func writeText(path, body string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("file exists: %s", path)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body), 0o644)
}

func titleFromID(id string) string {
	words := strings.FieldsFunc(id, func(r rune) bool { return r == '-' || r == '_' })
	for i, word := range words {
		runes := []rune(word)
		if len(runes) == 0 {
			continue
		}
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}
