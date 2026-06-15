package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/agent-packs/cli/internal/model"
)

func LoadSkillManifest(path string) (model.SkillManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.SkillManifest{}, err
	}
	frontmatter, body, err := parseSkillMarkdown(string(data))
	if err != nil {
		return model.SkillManifest{}, err
	}
	manifest := model.SkillManifest{Metadata: map[string]string{}, Body: body}
	currentMap := ""
	for _, raw := range strings.Split(frontmatter, "\n") {
		line := strings.TrimRight(raw, " 	")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "  ") && currentMap == "metadata" {
			key, value, ok := splitYAMLScalar(strings.TrimSpace(line))
			if ok {
				manifest.Metadata[key] = value
			}
			continue
		}
		currentMap = ""
		key, value, ok := splitYAMLScalar(line)
		if !ok {
			continue
		}
		switch key {
		case "name":
			manifest.Name = value
		case "description":
			manifest.Description = value
		case "license":
			manifest.License = value
		case "compatibility":
			manifest.Compatibility = value
		case "allowed-tools":
			manifest.AllowedTools = value
		case "metadata":
			currentMap = "metadata"
		}
	}
	return manifest, nil
}

func LoadPluginManifest(path string) (model.PluginManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.PluginManifest{}, err
	}
	var manifest model.PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return model.PluginManifest{}, err
	}
	return manifest, nil
}

func parseSkillMarkdown(content string) (string, string, error) {
	if !strings.HasPrefix(content, "---\n") && content != "---" {
		return "", "", fmt.Errorf("SKILL.md must start with YAML frontmatter")
	}
	parts := strings.SplitN(content, "\n---", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("SKILL.md frontmatter must be closed with ---")
	}
	frontmatter := strings.TrimPrefix(parts[0], "---\n")
	body := strings.TrimPrefix(parts[1], "\n")
	return frontmatter, body, nil
}

func splitYAMLScalar(line string) (string, string, bool) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	value = strings.Trim(value, `"`)
	return key, value, key != ""
}
