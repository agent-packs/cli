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
		Tools:        []string{"codex", "claude-code"},
		Scope:        []string{"global", "project"},
		Tags:         []string{"starter"},
		Skills:       model.CapabilityRefs{{ID: opts.ID + "-skill"}},
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
