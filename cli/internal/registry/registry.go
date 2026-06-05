package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sandeshh/agent-packs/cli/internal/model"
	"github.com/sandeshh/agent-packs/cli/internal/util"
)

func LoadPacks(registry string) ([]model.Pack, error) {
	entries, err := os.ReadDir(registry)
	if err != nil {
		return nil, err
	}
	var packs []model.Pack
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		pack, err := LoadPack(filepath.Join(registry, entry.Name()))
		if err != nil {
			return nil, err
		}
		packs = append(packs, pack)
	}
	sort.Slice(packs, func(i, j int) bool { return packs[i].ID < packs[j].ID })
	return packs, nil
}

func LoadPack(path string) (model.Pack, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Pack{}, err
	}
	var pack model.Pack
	if err := json.Unmarshal(data, &pack); err != nil {
		return model.Pack{}, err
	}
	pack.Path = path
	return pack, nil
}

func FindPack(registry, id string) (model.Pack, error) {
	packs, err := LoadPacks(registry)
	if err != nil {
		return model.Pack{}, err
	}
	for _, pack := range packs {
		if pack.ID == id {
			return pack, nil
		}
	}
	return model.Pack{}, fmt.Errorf("pack not found: %s", id)
}

func ResolvePack(defaultRegistry, home, ref string) (model.Pack, string, error) {
	if !strings.Contains(ref, "/") {
		pack, err := FindPack(defaultRegistry, ref)
		return pack, defaultRegistry, err
	}
	parts := strings.SplitN(ref, "/", 2)
	registryPath, err := ResolveRegistry(home, parts[0])
	if err != nil {
		return model.Pack{}, "", err
	}
	pack, err := FindPack(registryPath, parts[1])
	return pack, registryPath, err
}

func Search(registry, query string, out io.Writer) error {
	matches, err := MatchPacks(registry, query)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		fmt.Fprintln(out, "No packs found.")
		return model.ErrNotFound
	}
	for _, pack := range matches {
		fmt.Fprintf(out, "%s\t%s\t%s\n", pack.ID, pack.Name, strings.Join(pack.Tags, ", "))
	}
	return nil
}

func MatchPacks(registry, query string) ([]model.Pack, error) {
	packs, err := LoadPacks(registry)
	if err != nil {
		return nil, err
	}
	query = strings.ToLower(strings.TrimSpace(query))
	var matches []model.Pack
	for _, pack := range packs {
		if query == "" || packMatches(pack, query) {
			matches = append(matches, pack)
		}
	}
	return matches, nil
}

func Show(registry, id string, out io.Writer) error {
	pack, err := FindPack(registry, id)
	if err != nil {
		return err
	}
	license := pack.License
	if license == "" {
		license = "unknown"
	}
	fmt.Fprintf(out, "%s (%s)\n", pack.Name, pack.ID)
	fmt.Fprintln(out, pack.Description)
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Version: %s\n", pack.Version)
	fmt.Fprintf(out, "License: %s\n", license)
	fmt.Fprintf(out, "Tags: %s\n", strings.Join(pack.Tags, ", "))
	if len(pack.Packs) > 0 {
		fmt.Fprintf(out, "Includes packs: %s\n", strings.Join(pack.Packs, ", "))
	}
	if len(pack.Skills) > 0 {
		fmt.Fprintf(out, "Includes skills: %s\n", strings.Join(pack.Skills.IDs(), ", "))
	}
	if len(pack.Plugins) > 0 {
		fmt.Fprintf(out, "Includes plugins: %s\n", strings.Join(pack.Plugins.IDs(), ", "))
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Capabilities:")
	for _, capability := range pack.Capabilities {
		detail := capability.Format
		if detail == "" {
			detail = capability.Source
		}
		line := fmt.Sprintf("- %s: %s", capability.Type, capability.Name)
		if detail != "" {
			line += " " + detail
		}
		fmt.Fprintln(out, line)
	}
	return nil
}

func ExpandPack(registry string, pack model.Pack, seen map[string]bool) (model.Pack, error) {
	if seen[pack.ID] {
		return model.Pack{}, fmt.Errorf("pack composition cycle includes %s", pack.ID)
	}
	seen[pack.ID] = true
	out := pack
	out.Packs = append([]string{}, pack.Packs...)
	out.Skills = append(model.CapabilityRefs{}, pack.Skills...)
	out.Plugins = append(model.CapabilityRefs{}, pack.Plugins...)
	out.Capabilities = []model.Capability{}
	for _, childRef := range pack.Packs {
		child, err := FindPack(registry, childRef)
		if err != nil {
			return model.Pack{}, err
		}
		expanded, err := ExpandPack(registry, child, seen)
		if err != nil {
			return model.Pack{}, err
		}
		out.Capabilities = append(out.Capabilities, expanded.Capabilities...)
	}
	for _, skillRef := range pack.Skills {
		skill, err := ResolveCapabilityRef(registry, "skill", skillRef)
		if err != nil {
			return model.Pack{}, err
		}
		out.Capabilities = append(out.Capabilities, skill)
	}
	for _, pluginRef := range pack.Plugins {
		plugin, err := ResolveCapabilityRef(registry, "plugin", pluginRef)
		if err != nil {
			return model.Pack{}, err
		}
		out.Capabilities = append(out.Capabilities, plugin)
	}
	out.Capabilities = append(out.Capabilities, pack.Capabilities...)
	delete(seen, pack.ID)
	return out, nil
}

func ResolveCapabilityRef(registry, capabilityType string, ref model.CapabilityRef) (model.Capability, error) {
	if ref.ID == "" {
		return model.Capability{}, fmt.Errorf("%s reference id is required", capabilityType)
	}
	if ref.Source == "" {
		kind := capabilityType + "s"
		return FindCapability(registry, kind, ref.ID)
	}
	name := ref.Name
	if name == "" {
		name = ref.ID
	}
	upstreamSource := ref.UpstreamSource
	format := ref.Format
	entry := ref.Entry
	install := ref.Install
	if capabilityType == "skill" {
		if format == "" {
			format = "agent-skill"
		}
		if entry == "" {
			entry = "SKILL.md"
		}
	} else if capabilityType == "plugin" {
		if format == "" {
			format = "anthropic-plugin"
		}
		if entry == "" {
			entry = ".claude-plugin/plugin.json"
		}
		if install == nil {
			install = map[string]string{"method": "manual", "package": ref.ID}
		}
	} else {
		return model.Capability{}, fmt.Errorf("unsupported capability reference type: %s", capabilityType)
	}
	return model.Capability{
		Type: capabilityType, Name: name, Source: ref.Source, UpstreamSource: upstreamSource,
		Format: format, Version: ref.Version, Entry: entry, Homepage: ref.Homepage,
		Repository: ref.Repository, License: ref.License, Install: install, Trust: ref.Trust, Reference: true,
	}, nil
}

func FindCapability(registry, kind, id string) (model.Capability, error) {
	root := RegistryRoot(registry)
	if kind == "skills" {
		path := filepath.Join(root, kind, id, "SKILL.md")
		manifest, err := LoadSkillManifest(path)
		if err != nil {
			return model.Capability{}, fmt.Errorf("skill capability not found or invalid: %s", id)
		}
		return SkillCapability(id, path, manifest), nil
	}
	if kind == "plugins" {
		path := filepath.Join(root, kind, id, ".claude-plugin", "plugin.json")
		manifest, err := LoadPluginManifest(path)
		if err != nil {
			return model.Capability{}, fmt.Errorf("plugin capability not found or invalid: %s", id)
		}
		return PluginCapability(id, filepath.Dir(filepath.Dir(path)), manifest), nil
	}
	return model.Capability{}, fmt.Errorf("unsupported capability kind: %s", kind)
}

func SkillCapability(id, path string, manifest model.SkillManifest) model.Capability {
	upstreamSource := manifest.Metadata["agentpacks.upstreamSource"]
	source := manifest.Metadata["agentpacks.source"]
	if source == "" {
		source = upstreamSource
	}
	if source == "" {
		source = filepath.Dir(path)
	}
	return model.Capability{
		Type: "skill", Name: manifest.Name, Source: source, UpstreamSource: upstreamSource,
		Format: "agent-skill", Entry: "SKILL.md", License: manifest.License,
		Version: manifest.Metadata["agentpacks.version"], Reference: true,
	}
}

func PluginCapability(id, root string, manifest model.PluginManifest) model.Capability {
	name := manifest.DisplayName
	if name == "" {
		name = manifest.Name
	}
	source := manifest.Repository
	if source == "" {
		source = manifest.Homepage
	}
	if source == "" {
		source = root
	}
	return model.Capability{
		Type: "plugin", Name: name, Source: source, Format: "anthropic-plugin",
		Entry: ".claude-plugin/plugin.json", Version: manifest.Version,
		Homepage: manifest.Homepage, Repository: manifest.Repository, License: manifest.License,
		Install: map[string]string{"method": "manual", "package": manifest.Name}, Reference: true,
	}
}

func RegistryRoot(registry string) string {
	base := filepath.Base(registry)
	if base == "packs" {
		return filepath.Dir(registry)
	}
	if _, err := os.Stat(filepath.Join(registry, "packs")); err == nil {
		return registry
	}
	if _, err := os.Stat(filepath.Join(registry, "registry")); err == nil {
		return filepath.Join(registry, "registry")
	}
	return filepath.Dir(registry)
}

func GenerateIndex(registry, outputPath string, out io.Writer) error {
	packs, err := LoadPacks(registry)
	if err != nil {
		return err
	}
	index := model.RegistryIndex{GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	for _, pack := range packs {
		expanded, err := ExpandPack(registry, pack, map[string]bool{})
		if err != nil {
			return err
		}
		entry := model.IndexEntry{
			ID: pack.ID, Name: pack.Name, Version: pack.Version, Description: pack.Description,
			Tags: pack.Tags, Categories: pack.Categories, Tools: pack.Tools, Scope: pack.Scope,
			Skills: pack.Skills.IDs(), Plugins: pack.Plugins.IDs(), Capabilities: len(expanded.Capabilities),
		}
		index.Packs = append(index.Packs, entry)
	}
	if outputPath == "" {
		outputPath = filepath.Join(RegistryRoot(registry), "index.json")
	}
	if err := util.WriteJSON(outputPath, index); err != nil {
		return err
	}
	fmt.Fprintf(out, "Wrote %s\n", outputPath)
	return nil
}

func RegistryAdd(home, name, source string) error {
	config, err := LoadRegistryConfig(home)
	if err != nil {
		return err
	}
	if config.Registries == nil {
		config.Registries = map[string]string{}
	}
	config.Registries[name] = source
	return SaveRegistryConfig(home, config)
}

func RegistryRemove(home, name string) error {
	config, err := LoadRegistryConfig(home)
	if err != nil {
		return err
	}
	delete(config.Registries, name)
	return SaveRegistryConfig(home, config)
}

func RegistryList(home string, out io.Writer) error {
	config, err := LoadRegistryConfig(home)
	if err != nil {
		return err
	}
	if len(config.Registries) == 0 {
		fmt.Fprintln(out, "No registries configured.")
		return nil
	}
	names := []string{}
	for name := range config.Registries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(out, "%s\t%s\n", name, config.Registries[name])
	}
	return nil
}

func LoadRegistryConfig(home string) (model.RegistryConfig, error) {
	path := registryConfigPath(home)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return model.RegistryConfig{Registries: map[string]string{}}, nil
		}
		return model.RegistryConfig{}, err
	}
	var config model.RegistryConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return model.RegistryConfig{}, err
	}
	if config.Registries == nil {
		config.Registries = map[string]string{}
	}
	return config, nil
}

func SaveRegistryConfig(home string, config model.RegistryConfig) error {
	path := registryConfigPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return util.WriteJSON(path, config)
}

func ResolveRegistry(home, name string) (string, error) {
	config, err := LoadRegistryConfig(home)
	if err != nil {
		return "", err
	}
	source, ok := config.Registries[name]
	if !ok {
		return "", fmt.Errorf("registry not configured: %s", name)
	}
	localRoot, err := materializeRegistry(home, name, source)
	if err != nil {
		return "", err
	}
	return registryPacksPath(localRoot), nil
}

func materializeRegistry(home, name, source string) (string, error) {
	if util.IsLocalSource(source) {
		return util.ExpandHome(source), nil
	}
	cache := filepath.Join(util.ExpandHome(home), "registries", util.Slugify(name))
	if _, err := os.Stat(filepath.Join(cache, ".git")); err == nil {
		cmd := exec.Command("git", "-C", cache, "pull", "--ff-only")
		_ = cmd.Run()
		return cache, nil
	}
	_ = os.RemoveAll(cache)
	if err := os.MkdirAll(filepath.Dir(cache), 0o755); err != nil {
		return "", err
	}
	cmd := exec.Command("git", "clone", "--depth", "1", source, cache)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git clone failed: %s", strings.TrimSpace(stderr.String()))
	}
	return cache, nil
}

func registryPacksPath(root string) string {
	if _, err := os.Stat(filepath.Join(root, "registry", "packs")); err == nil {
		return filepath.Join(root, "registry", "packs")
	}
	return filepath.Join(root, "packs")
}

func registryConfigPath(home string) string {
	return filepath.Join(util.ExpandHome(home), "registries.json")
}

func packMatches(pack model.Pack, query string) bool {
	fields := []string{pack.ID, pack.Name, pack.Description}
	fields = append(fields, pack.Tags...)
	fields = append(fields, pack.Categories...)
	fields = append(fields, pack.Tools...)
	fields = append(fields, pack.Scope...)
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}
