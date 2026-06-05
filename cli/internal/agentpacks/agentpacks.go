package agentpacks

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

type Pack struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Version      string       `json:"version"`
	Description  string       `json:"description"`
	License      string       `json:"license,omitempty"`
	Tags         []string     `json:"tags,omitempty"`
	Packs        []string     `json:"packs,omitempty"`
	Skills       []string     `json:"skills,omitempty"`
	Plugins      []string     `json:"plugins,omitempty"`
	Capabilities []Capability `json:"capabilities,omitempty"`
	Path         string       `json:"-"`
}

type Capability struct {
	Type              string            `json:"type"`
	Name              string            `json:"name"`
	Source            string            `json:"source"`
	Format            string            `json:"format,omitempty"`
	Version           string            `json:"version,omitempty"`
	Entry             string            `json:"entry,omitempty"`
	Homepage          string            `json:"homepage,omitempty"`
	Repository        string            `json:"repository,omitempty"`
	License           string            `json:"license,omitempty"`
	Install           map[string]string `json:"install,omitempty"`
	Targets           []string          `json:"targets,omitempty"`
	Integrity         Integrity         `json:"integrity,omitempty"`
	RequiresExecution bool              `json:"requiresExecution,omitempty"`
	Trust             string            `json:"trust,omitempty"`
}

type Integrity struct {
	Checksum  string `json:"checksum,omitempty"`
	Signature string `json:"signature,omitempty"`
}

type SkillManifest struct {
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	License       string            `json:"license,omitempty"`
	Compatibility string            `json:"compatibility,omitempty"`
	AllowedTools  string            `json:"allowed-tools,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Body          string            `json:"-"`
}

type PluginManifest struct {
	Name           string         `json:"name"`
	DisplayName    string         `json:"displayName,omitempty"`
	Version        string         `json:"version,omitempty"`
	Description    string         `json:"description,omitempty"`
	Author         map[string]any `json:"author,omitempty"`
	Homepage       string         `json:"homepage,omitempty"`
	Repository     string         `json:"repository,omitempty"`
	License        string         `json:"license,omitempty"`
	Keywords       []string       `json:"keywords,omitempty"`
	DefaultEnabled *bool          `json:"defaultEnabled,omitempty"`
	Skills         any            `json:"skills,omitempty"`
	Commands       any            `json:"commands,omitempty"`
	Agents         any            `json:"agents,omitempty"`
	Hooks          any            `json:"hooks,omitempty"`
	MCPServers     any            `json:"mcpServers,omitempty"`
	LSPServers     any            `json:"lspServers,omitempty"`
	Experimental   map[string]any `json:"experimental,omitempty"`
}

type Plan struct {
	Pack         string     `json:"pack"`
	Version      string     `json:"version"`
	Agent        string     `json:"agent"`
	Target       string     `json:"target"`
	Capabilities []PlanItem `json:"capabilities"`
}

type PlanItem struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Action      string `json:"action"`
	Source      string `json:"source,omitempty"`
	Entry       string `json:"entry,omitempty"`
	Destination string `json:"destination,omitempty"`
	Status      string `json:"status"`
	Format      string `json:"format,omitempty"`
	Command     string `json:"command,omitempty"`
	Method      string `json:"method,omitempty"`
	Package     string `json:"package,omitempty"`
	Marketplace string `json:"marketplace,omitempty"`
	Reason      string `json:"reason,omitempty"`
	ExitCode    *int   `json:"exit_code,omitempty"`
	Stdout      string `json:"stdout,omitempty"`
	Stderr      string `json:"stderr,omitempty"`
}

type Receipt struct {
	InstalledAt string `json:"installed_at"`
	Pack        Pack   `json:"pack"`
	Plan        Plan   `json:"plan"`
}

type Lockfile struct {
	GeneratedAt  string      `json:"generated_at"`
	Pack         string      `json:"pack"`
	Version      string      `json:"version"`
	Capabilities []LockEntry `json:"capabilities"`
}

type LockEntry struct {
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	Source    string    `json:"source"`
	Version   string    `json:"version,omitempty"`
	Integrity Integrity `json:"integrity,omitempty"`
	Digest    string    `json:"digest"`
}

type RegistryConfig struct {
	Registries map[string]string `json:"registries"`
}

var SkillTargets = map[string]string{
	"claude":  ".claude/skills",
	"codex":   ".codex/skills",
	"generic": "skills",
}

var (
	ErrNotFound      = errors.New("not found")
	ErrInstallFailed = errors.New("install failed")
)

func LoadPacks(registry string) ([]Pack, error) {
	entries, err := os.ReadDir(registry)
	if err != nil {
		return nil, err
	}
	var packs []Pack
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

func LoadPack(path string) (Pack, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Pack{}, err
	}
	var pack Pack
	if err := json.Unmarshal(data, &pack); err != nil {
		return Pack{}, err
	}
	pack.Path = path
	return pack, nil
}

func FindPack(registry, id string) (Pack, error) {
	packs, err := LoadPacks(registry)
	if err != nil {
		return Pack{}, err
	}
	for _, pack := range packs {
		if pack.ID == id {
			return pack, nil
		}
	}
	return Pack{}, fmt.Errorf("pack not found: %s", id)
}

func ResolvePack(defaultRegistry, home, ref string) (Pack, string, error) {
	if !strings.Contains(ref, "/") {
		pack, err := FindPack(defaultRegistry, ref)
		return pack, defaultRegistry, err
	}
	parts := strings.SplitN(ref, "/", 2)
	registryPath, err := ResolveRegistry(home, parts[0])
	if err != nil {
		return Pack{}, "", err
	}
	pack, err := FindPack(registryPath, parts[1])
	return pack, registryPath, err
}

func Search(registry, query string, out io.Writer) error {
	packs, err := LoadPacks(registry)
	if err != nil {
		return err
	}
	query = strings.ToLower(strings.TrimSpace(query))
	var matches []Pack
	for _, pack := range packs {
		if query == "" || packMatches(pack, query) {
			matches = append(matches, pack)
		}
	}
	if len(matches) == 0 {
		fmt.Fprintln(out, "No packs found.")
		return ErrNotFound
	}
	for _, pack := range matches {
		fmt.Fprintf(out, "%s\t%s\t%s\n", pack.ID, pack.Name, strings.Join(pack.Tags, ", "))
	}
	return nil
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
		fmt.Fprintf(out, "Includes skills: %s\n", strings.Join(pack.Skills, ", "))
	}
	if len(pack.Plugins) > 0 {
		fmt.Fprintf(out, "Includes plugins: %s\n", strings.Join(pack.Plugins, ", "))
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

func BuildInstallPlan(pack Pack, target, agent, only string) Plan {
	items := []PlanItem{}
	for _, capability := range selectCapabilities(pack.Capabilities, only) {
		items = append(items, planCapability(capability, target, agent))
	}
	return Plan{Pack: pack.ID, Version: pack.Version, Agent: agent, Target: target, Capabilities: items}
}

func PrintPlan(plan Plan, out io.Writer) {
	fmt.Fprintf(out, "Pack: %s\n", plan.Pack)
	fmt.Fprintf(out, "Agent: %s\n", plan.Agent)
	fmt.Fprintf(out, "Target: %s\n", plan.Target)
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
	}
}

func Install(registry, home, packRef, target, agent, only string, executePlugins, dryRun bool, out io.Writer) error {
	pack, sourceRegistry, err := ResolvePack(registry, home, packRef)
	if err != nil {
		return err
	}
	expanded, err := ExpandPack(sourceRegistry, pack, map[string]bool{})
	if err != nil {
		return err
	}
	absTarget, err := filepath.Abs(expandHome(target))
	if err != nil {
		return err
	}
	plan := BuildInstallPlan(expanded, absTarget, agent, only)
	if dryRun {
		PrintPlan(plan, out)
		return nil
	}
	if err := os.MkdirAll(absTarget, 0o755); err != nil {
		return err
	}
	packDir := filepath.Join(absTarget, "packs", expanded.ID)
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(packDir, "agent-pack.json"), expanded); err != nil {
		return err
	}
	if pack.Path != "" {
		_ = copyFile(pack.Path, filepath.Join(packDir, "source-registry-entry.json"))
	}
	result := ExecutePlan(plan, executePlugins)
	receiptPath, err := WriteReceipt(absTarget, expanded, result)
	if err != nil {
		return err
	}
	if err := WriteLockfile(packDir, expanded); err != nil {
		return err
	}
	PrintPlan(result, out)
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Receipt: %s\n", receiptPath)
	for _, item := range result.Capabilities {
		if item.Status == "failed" {
			return ErrInstallFailed
		}
	}
	return nil
}

func ExpandPack(registry string, pack Pack, seen map[string]bool) (Pack, error) {
	if seen[pack.ID] {
		return Pack{}, fmt.Errorf("pack composition cycle includes %s", pack.ID)
	}
	seen[pack.ID] = true
	out := pack
	out.Packs = append([]string{}, pack.Packs...)
	out.Skills = append([]string{}, pack.Skills...)
	out.Plugins = append([]string{}, pack.Plugins...)
	out.Capabilities = []Capability{}
	for _, childRef := range pack.Packs {
		child, err := FindPack(registry, childRef)
		if err != nil {
			return Pack{}, err
		}
		expanded, err := ExpandPack(registry, child, seen)
		if err != nil {
			return Pack{}, err
		}
		out.Capabilities = append(out.Capabilities, expanded.Capabilities...)
	}
	for _, skillRef := range pack.Skills {
		skill, err := FindCapability(registry, "skills", skillRef)
		if err != nil {
			return Pack{}, err
		}
		out.Capabilities = append(out.Capabilities, skill)
	}
	for _, pluginRef := range pack.Plugins {
		plugin, err := FindCapability(registry, "plugins", pluginRef)
		if err != nil {
			return Pack{}, err
		}
		out.Capabilities = append(out.Capabilities, plugin)
	}
	out.Capabilities = append(out.Capabilities, pack.Capabilities...)
	delete(seen, pack.ID)
	return out, nil
}

func ExecutePlan(plan Plan, executePlugins bool) Plan {
	results := make([]PlanItem, 0, len(plan.Capabilities))
	for _, item := range plan.Capabilities {
		switch item.Type {
		case "skill":
			results = append(results, installSkill(item, plan.Target))
		case "plugin":
			results = append(results, installPlugin(item, executePlugins))
		default:
			item.Status = "recorded"
			results = append(results, item)
		}
	}
	plan.Capabilities = results
	return plan
}

func WriteReceipt(target string, pack Pack, plan Plan) (string, error) {
	receiptsDir := filepath.Join(target, "receipts")
	if err := os.MkdirAll(receiptsDir, 0o755); err != nil {
		return "", err
	}
	receiptPath := filepath.Join(receiptsDir, pack.ID+".json")
	receipt := Receipt{InstalledAt: time.Now().UTC().Format(time.RFC3339Nano), Pack: pack, Plan: plan}
	return receiptPath, writeJSON(receiptPath, receipt)
}

func ListInstalled(target string, out io.Writer) error {
	receiptsDir := filepath.Join(expandHome(target), "receipts")
	entries, err := os.ReadDir(receiptsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(out, "No packs installed.")
			return nil
		}
		return err
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		receipt, err := LoadReceipt(filepath.Join(receiptsDir, entry.Name()))
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "%s\t%s\t%s\n", receipt.Pack.ID, receipt.Pack.Version, receipt.InstalledAt)
		count++
	}
	if count == 0 {
		fmt.Fprintln(out, "No packs installed.")
	}
	return nil
}

func Uninstall(target, packID string, out io.Writer) error {
	absTarget, err := filepath.Abs(expandHome(target))
	if err != nil {
		return err
	}
	receiptPath := filepath.Join(absTarget, "receipts", packID+".json")
	receipt, err := LoadReceipt(receiptPath)
	if err != nil {
		return err
	}
	for _, item := range receipt.Plan.Capabilities {
		if item.Type == "skill" && item.Destination != "" && item.Status == "installed" {
			if err := os.RemoveAll(item.Destination); err != nil {
				return err
			}
			fmt.Fprintf(out, "Removed skill: %s\n", item.Destination)
		} else if item.Type == "plugin" {
			fmt.Fprintf(out, "Plugin requires native uninstall/manual cleanup: %s\n", item.Name)
		}
	}
	_ = os.RemoveAll(filepath.Join(absTarget, "packs", packID))
	if err := os.Remove(receiptPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	fmt.Fprintf(out, "Uninstalled %s\n", packID)
	return nil
}

func LoadReceipt(path string) (Receipt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Receipt{}, err
	}
	var receipt Receipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

func Doctor(defaultRegistry, home string, out io.Writer) error {
	checks := []struct{ name, command string }{{"git", "git"}, {"go", "go"}, {"claude", "claude"}, {"codex", "codex"}}
	for _, check := range checks {
		if _, err := exec.LookPath(check.command); err != nil {
			fmt.Fprintf(out, "WARN  %s not found\n", check.name)
		} else {
			fmt.Fprintf(out, "OK    %s found\n", check.name)
		}
	}
	if _, err := os.Stat(defaultRegistry); err != nil {
		fmt.Fprintf(out, "WARN  registry unavailable: %s\n", defaultRegistry)
	} else {
		fmt.Fprintf(out, "OK    registry available: %s\n", defaultRegistry)
	}
	if err := os.MkdirAll(expandHome(home), 0o755); err != nil {
		fmt.Fprintf(out, "WARN  install home not writable: %s\n", home)
	} else {
		fmt.Fprintf(out, "OK    install home writable: %s\n", home)
	}
	return nil
}

func ValidatePath(path string, out io.Writer) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	paths := []string{}
	if info.IsDir() {
		err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && (strings.HasSuffix(p, ".json") || filepath.Base(p) == "SKILL.md") {
				paths = append(paths, p)
			}
			return nil
		})
		if err != nil {
			return err
		}
	} else {
		paths = append(paths, path)
	}
	failed := false
	for _, p := range paths {
		if isCapabilityManifestPath(p) {
			errors := ValidateCapabilityManifestPath(p)
			if len(errors) > 0 {
				fmt.Fprintf(out, "FAIL  %s\n", p)
				for _, msg := range errors {
					fmt.Fprintf(out, "  - %s\n", msg)
				}
				failed = true
			} else {
				fmt.Fprintf(out, "OK    %s\n", p)
			}
			continue
		}
		pack, err := LoadPack(p)
		if err != nil {
			fmt.Fprintf(out, "FAIL  %s: %s\n", p, err)
			failed = true
			continue
		}
		errors := ValidatePack(pack)
		if len(errors) > 0 {
			fmt.Fprintf(out, "FAIL  %s\n", p)
			for _, msg := range errors {
				fmt.Fprintf(out, "  - %s\n", msg)
			}
			failed = true
		} else {
			fmt.Fprintf(out, "OK    %s\n", p)
		}
	}
	if failed {
		return ErrInstallFailed
	}
	return nil
}

func FindCapability(registry, kind, id string) (Capability, error) {
	root := registryRoot(registry)
	if kind == "skills" {
		path := filepath.Join(root, kind, id, "SKILL.md")
		manifest, err := LoadSkillManifest(path)
		if err != nil {
			return Capability{}, fmt.Errorf("skill capability not found or invalid: %s", id)
		}
		return SkillCapability(id, path, manifest), nil
	}
	if kind == "plugins" {
		path := filepath.Join(root, kind, id, ".claude-plugin", "plugin.json")
		manifest, err := LoadPluginManifest(path)
		if err != nil {
			return Capability{}, fmt.Errorf("plugin capability not found or invalid: %s", id)
		}
		return PluginCapability(id, filepath.Dir(filepath.Dir(path)), manifest), nil
	}
	return Capability{}, fmt.Errorf("unsupported capability kind: %s", kind)
}

func SkillCapability(id, path string, manifest SkillManifest) Capability {
	source := manifest.Metadata["agentpacks.source"]
	if source == "" {
		source = filepath.Dir(path)
	}
	return Capability{Type: "skill", Name: manifest.Name, Source: source, Format: "agent-skill", Entry: "SKILL.md", License: manifest.License, Version: manifest.Metadata["agentpacks.version"]}
}

func PluginCapability(id, root string, manifest PluginManifest) Capability {
	name := manifest.DisplayName
	if name == "" {
		name = manifest.Name
	}
	return Capability{Type: "plugin", Name: name, Source: root, Format: "anthropic-plugin", Entry: ".claude-plugin/plugin.json", Version: manifest.Version, Homepage: manifest.Homepage, Repository: manifest.Repository, License: manifest.License, Install: map[string]string{"method": "manual", "package": manifest.Name}}
}

func LoadSkillManifest(path string) (SkillManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SkillManifest{}, err
	}
	frontmatter, body, err := parseSkillMarkdown(string(data))
	if err != nil {
		return SkillManifest{}, err
	}
	manifest := SkillManifest{Metadata: map[string]string{}, Body: body}
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

func LoadPluginManifest(path string) (PluginManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PluginManifest{}, err
	}
	var manifest PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return PluginManifest{}, err
	}
	return manifest, nil
}

func isCapabilityManifestPath(path string) bool {
	return strings.HasSuffix(filepath.ToSlash(path), "/SKILL.md") || strings.HasSuffix(filepath.ToSlash(path), "/.claude-plugin/plugin.json")
}

func registryRoot(registry string) string {
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

func ValidateCapabilityManifestPath(path string) []string {
	if strings.HasSuffix(filepath.ToSlash(path), "/SKILL.md") {
		manifest, err := LoadSkillManifest(path)
		if err != nil {
			return []string{err.Error()}
		}
		return ValidateSkillManifest(filepath.Base(filepath.Dir(path)), manifest)
	}
	if strings.HasSuffix(filepath.ToSlash(path), "/.claude-plugin/plugin.json") {
		manifest, err := LoadPluginManifest(path)
		if err != nil {
			return []string{err.Error()}
		}
		return ValidatePluginManifest(manifest)
	}
	return []string{"unsupported capability manifest path"}
}

func ValidateSkillManifest(directoryName string, manifest SkillManifest) []string {
	var errs []string
	if !validSkillName(manifest.Name) {
		errs = append(errs, "name must be 1-64 lowercase letters, numbers, and hyphens; no leading/trailing/consecutive hyphens")
	}
	if manifest.Name != "" && manifest.Name != directoryName {
		errs = append(errs, "name must match parent directory name")
	}
	if len(manifest.Description) < 1 || len(manifest.Description) > 1024 {
		errs = append(errs, "description must be 1-1024 characters")
	}
	if manifest.Compatibility != "" && len(manifest.Compatibility) > 500 {
		errs = append(errs, "compatibility must be 1-500 characters when provided")
	}
	return errs
}

func ValidatePluginManifest(manifest PluginManifest) []string {
	var errs []string
	if !validPluginName(manifest.Name) {
		errs = append(errs, "name is required and must not contain spaces or path separators")
	}
	if manifest.Version != "" && !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+`).MatchString(manifest.Version) {
		errs = append(errs, "version should be semantic version format")
	}
	pathFields := map[string]any{"skills": manifest.Skills, "commands": manifest.Commands, "agents": manifest.Agents, "hooks": manifest.Hooks}
	for field, value := range pathFields {
		errs = append(errs, validatePluginPathField(field, value)...)
	}
	if manifest.Experimental != nil {
		for field, value := range manifest.Experimental {
			errs = append(errs, validatePluginPathField("experimental."+field, value)...)
		}
	}
	return errs
}

func ValidateCapability(capability Capability, prefix string) []string {
	var errs []string
	if capability.Type == "" {
		errs = append(errs, prefix+".type is required")
	}
	if capability.Name == "" {
		errs = append(errs, prefix+".name is required")
	}
	if capability.Source == "" {
		errs = append(errs, prefix+".source is required")
	}
	if capability.Type == "skill" {
		if capability.Format != "agent-skill" {
			errs = append(errs, prefix+".format must be agent-skill")
		}
		if capability.Entry == "" {
			errs = append(errs, prefix+".entry is required")
		}
	}
	if capability.Type == "plugin" {
		if capability.Format == "" {
			errs = append(errs, prefix+".format is required")
		}
		if capability.Install == nil || capability.Install["method"] == "" {
			errs = append(errs, prefix+".install.method is required")
		}
		if capability.Install != nil && capability.Install["command"] != "" && !capability.RequiresExecution {
			errs = append(errs, prefix+".requiresExecution must be true when install.command is set")
		}
	}
	return errs
}

func ValidatePack(pack Pack) []string {
	var errs []string
	if pack.ID == "" || !regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`).MatchString(pack.ID) {
		errs = append(errs, "id must be kebab-case")
	}
	if pack.Name == "" {
		errs = append(errs, "name is required")
	}
	if pack.Version == "" {
		errs = append(errs, "version is required")
	}
	if pack.Description == "" {
		errs = append(errs, "description is required")
	}
	if len(pack.Capabilities) == 0 && len(pack.Packs) == 0 && len(pack.Skills) == 0 && len(pack.Plugins) == 0 {
		errs = append(errs, "capabilities, packs, skills, or plugins is required")
	}
	for i, capability := range pack.Capabilities {
		errs = append(errs, ValidateCapability(capability, fmt.Sprintf("capabilities[%d]", i))...)
	}
	return errs
}

func WriteLockfile(packDir string, pack Pack) error {
	lock := Lockfile{GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), Pack: pack.ID, Version: pack.Version}
	for _, capability := range pack.Capabilities {
		entry := LockEntry{Type: capability.Type, Name: capability.Name, Source: capability.Source, Version: capability.Version, Integrity: capability.Integrity, Digest: digestCapability(capability)}
		lock.Capabilities = append(lock.Capabilities, entry)
	}
	return writeJSON(filepath.Join(packDir, "agent-pack.lock"), lock)
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

func LoadRegistryConfig(home string) (RegistryConfig, error) {
	path := registryConfigPath(home)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return RegistryConfig{Registries: map[string]string{}}, nil
		}
		return RegistryConfig{}, err
	}
	var config RegistryConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return RegistryConfig{}, err
	}
	if config.Registries == nil {
		config.Registries = map[string]string{}
	}
	return config, nil
}

func SaveRegistryConfig(home string, config RegistryConfig) error {
	path := registryConfigPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeJSON(path, config)
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
	if isLocalSource(source) {
		return expandHome(source), nil
	}
	cache := filepath.Join(expandHome(home), "registries", slugify(name))
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
	return filepath.Join(expandHome(home), "registries.json")
}

func packMatches(pack Pack, query string) bool {
	fields := []string{pack.ID, pack.Name, pack.Description}
	fields = append(fields, pack.Tags...)
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}

func selectCapabilities(capabilities []Capability, only string) []Capability {
	if only == "all" {
		return capabilities
	}
	wanted := ""
	if only == "skills" {
		wanted = "skill"
	} else if only == "plugins" {
		wanted = "plugin"
	}
	selected := []Capability{}
	for _, capability := range capabilities {
		if capability.Type == wanted {
			selected = append(selected, capability)
		}
	}
	return selected
}

func planCapability(capability Capability, target, agent string) PlanItem {
	switch capability.Type {
	case "skill":
		entry := capability.Entry
		if entry == "" {
			entry = "SKILL.md"
		}
		action := "fetch-copy"
		if isLocalSource(capability.Source) {
			action = "copy"
		}
		return PlanItem{Type: "skill", Name: capability.Name, Action: action, Source: capability.Source, Entry: entry, Destination: filepath.Join(skillTargetRoot(target, agent), slugify(capability.Name)), Status: "planned"}
	case "plugin":
		return PlanItem{Type: "plugin", Name: capability.Name, Action: "native-install", Source: capability.Source, Format: capability.Format, Command: capability.Install["command"], Method: capability.Install["method"], Package: capability.Install["package"], Marketplace: capability.Install["marketplace"], Status: "planned"}
	default:
		return PlanItem{Type: capability.Type, Name: capability.Name, Action: "record", Source: capability.Source, Status: "planned"}
	}
}

func skillTargetRoot(target, agent string) string {
	root, ok := SkillTargets[agent]
	if !ok {
		root = SkillTargets["generic"]
	}
	return filepath.Join(target, root)
}

func installSkill(item PlanItem, target string) PlanItem {
	source, cleanup, err := materializeSkillSource(item.Source, target)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		item.Status = "pending"
		item.Reason = err.Error()
		return item
	}
	return copySkillFromSource(item, source)
}

func materializeSkillSource(source, target string) (string, func(), error) {
	if isLocalSource(source) {
		abs, err := filepath.Abs(expandHome(source))
		return abs, nil, err
	}
	cache := filepath.Join(target, "cache", "sources", slugify(source))
	_ = os.RemoveAll(cache)
	if err := os.MkdirAll(filepath.Dir(cache), 0o755); err != nil {
		return "", nil, err
	}
	repo, branch, subpath := parseGitHubTree(source)
	if repo == "" {
		repo, branch = source, ""
	}
	args := []string{"clone", "--depth", "1"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, repo, cache)
	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", nil, fmt.Errorf("remote or missing skill source; fetch failed: %s", strings.TrimSpace(stderr.String()))
	}
	return filepath.Join(cache, subpath), nil, nil
}

func parseGitHubTree(source string) (repo, branch, subpath string) {
	u, err := url.Parse(source)
	if err != nil || u.Host != "github.com" {
		return "", "", ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 5 || parts[2] != "tree" {
		return "", "", ""
	}
	return "https://github.com/" + parts[0] + "/" + parts[1] + ".git", parts[3], filepath.Join(parts[4:]...)
}

func copySkillFromSource(item PlanItem, source string) PlanItem {
	destination, err := filepath.Abs(expandHome(item.Destination))
	if err != nil {
		item.Status = "failed"
		item.Reason = err.Error()
		return item
	}
	info, err := os.Stat(source)
	if err != nil {
		item.Status = "pending"
		item.Reason = "remote or missing skill source; fetch support is not implemented yet"
		return item
	}
	entry := item.Entry
	if entry == "" {
		entry = "SKILL.md"
	}
	entryPath := source
	if info.IsDir() {
		entryPath = filepath.Join(source, entry)
	}
	if _, err := os.Stat(entryPath); err != nil {
		item.Status = "failed"
		item.Reason = fmt.Sprintf("skill entry not found: %s", entry)
		return item
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		item.Status = "failed"
		item.Reason = err.Error()
		return item
	}
	if err := os.RemoveAll(destination); err != nil {
		item.Status = "failed"
		item.Reason = err.Error()
		return item
	}
	if info.IsDir() {
		err = copyDir(source, destination)
	} else {
		err = os.MkdirAll(destination, 0o755)
		if err == nil {
			err = copyFile(source, filepath.Join(destination, filepath.Base(entryPath)))
		}
	}
	if err != nil {
		item.Status = "failed"
		item.Reason = err.Error()
		return item
	}
	item.Status = "installed"
	return item
}

func installPlugin(item PlanItem, executePlugins bool) PlanItem {
	if !executePlugins {
		item.Status = "pending"
		item.Reason = "plugin command execution requires --execute-plugins"
		return item
	}
	if item.Command == "" {
		item.Status = "pending"
		item.Reason = "plugin install command is not specified"
		return item
	}
	cmd := exec.Command("sh", "-c", item.Command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		exitCode = 1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
	}
	item.ExitCode = &exitCode
	item.Stdout = strings.TrimSpace(stdout.String())
	item.Stderr = strings.TrimSpace(stderr.String())
	if err != nil {
		item.Status = "failed"
	} else {
		item.Status = "installed"
	}
	return item
}

func digestCapability(capability Capability) string {
	data, _ := json.Marshal(capability)
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func isLocalSource(source string) bool {
	return !strings.HasPrefix(source, "http://") && !strings.HasPrefix(source, "https://") && !strings.HasPrefix(source, "git@") && !strings.HasPrefix(source, "ssh://")
}

func slugify(value string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			builder.WriteRune('-')
			lastDash = true
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return "capability"
	}
	return slug
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

func validSkillName(name string) bool {
	if len(name) < 1 || len(name) > 64 {
		return false
	}
	if strings.Contains(name, "--") {
		return false
	}
	return regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`).MatchString(name)
}

func validPluginName(name string) bool {
	return name != "" && !strings.ContainsAny(name, "/\\ ")
}

func validatePluginPathField(field string, value any) []string {
	if value == nil {
		return nil
	}
	var errs []string
	check := func(path string) {
		if path == "" {
			errs = append(errs, field+" path must not be empty")
			return
		}
		if strings.Contains(path, "..") || strings.HasPrefix(path, "/") {
			errs = append(errs, field+" path must stay within plugin root")
		}
		if !strings.HasPrefix(path, "./") {
			errs = append(errs, field+" path should be relative and start with ./")
		}
	}
	switch typed := value.(type) {
	case string:
		check(typed)
	case []any:
		for _, item := range typed {
			if s, ok := item.(string); ok {
				check(s)
			} else {
				errs = append(errs, field+" entries must be strings")
			}
		}
	case map[string]any:
		// Inline component objects are valid for hooks, MCP, LSP, etc.
	default:
		errs = append(errs, field+" must be a string, array, or object")
	}
	return errs
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	output, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer output.Close()
	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	return output.Close()
}

func copyDir(source, destination string) error {
	return filepath.WalkDir(source, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}
