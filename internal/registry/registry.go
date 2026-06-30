package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/agent-packs/cli/internal/model"
	"github.com/agent-packs/cli/internal/targets"
	"github.com/agent-packs/cli/internal/util"
)

func LoadPacks(registry string) ([]model.Pack, error) {
	entries, err := os.ReadDir(registry)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("registry not found at %s\n"+
				"  The CLI ships its registry alongside the binary (Homebrew, the install.sh\n"+
				"  script, and the Docker image all set this up). If you built from source or\n"+
				"  used `go install`, point AGENT_PACKS_REGISTRY at a checkout's registry/packs:\n"+
				"    export AGENT_PACKS_REGISTRY=/path/to/agent-packs/registry/packs", registry)
		}
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
	packID, versionPin := splitVersionPin(ref)
	if !strings.Contains(packID, "/") {
		pack, err := FindPack(defaultRegistry, packID)
		if err != nil {
			return model.Pack{}, "", err
		}
		if versionPin != "" && pack.Version != versionPin {
			return model.Pack{}, "", fmt.Errorf("pack %s: version %s not available (registry has %s)", packID, versionPin, pack.Version)
		}
		return pack, defaultRegistry, nil
	}
	parts := strings.SplitN(packID, "/", 2)
	registryPath, err := ResolveRegistry(home, parts[0])
	if err != nil {
		return model.Pack{}, "", err
	}
	pack, err := FindPack(registryPath, parts[1])
	if err != nil {
		return model.Pack{}, "", err
	}
	if versionPin != "" && pack.Version != versionPin {
		return model.Pack{}, "", fmt.Errorf("pack %s: version %s not available (registry has %s)", packID, versionPin, pack.Version)
	}
	return pack, registryPath, nil
}

func splitVersionPin(ref string) (string, string) {
	if idx := strings.Index(ref, "@"); idx >= 0 {
		return ref[:idx], ref[idx+1:]
	}
	return ref, ""
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

// SearchFilter holds optional facet filters for MatchPacks.
type SearchFilter struct {
	Tag            string
	Category       string
	Stability      string
	Tool           string
	ReviewStatus   string
	Scope          string
	Trust          string
	CompatibleWith string
	CompatStatus   string
}

func MatchPacks(registry, query string) ([]model.Pack, error) {
	return FilteredMatchPacks(registry, query, SearchFilter{})
}

func FilteredMatchPacks(registry, query string, f SearchFilter) ([]model.Pack, error) {
	packs, err := LoadPacks(registry)
	if err != nil {
		return nil, err
	}
	query = strings.ToLower(strings.TrimSpace(query))
	var matches []model.Pack
	for _, pack := range packs {
		if query != "" && !packMatches(pack, query) {
			continue
		}
		if f.Tag != "" && !containsString(pack.Tags, f.Tag) {
			continue
		}
		if f.Category != "" && !containsString(pack.Categories, f.Category) {
			continue
		}
		if f.Stability != "" && pack.Stability != f.Stability {
			continue
		}
		if f.Tool != "" && !targets.PackSupportsTool(pack.Tools, f.Tool) {
			continue
		}
		if f.ReviewStatus != "" && !strings.EqualFold(pack.ReviewStatus, f.ReviewStatus) {
			continue
		}
		if f.Scope != "" && !containsString(pack.Scope, f.Scope) {
			continue
		}
		if f.Trust != "" && !strings.EqualFold(pack.Trust, f.Trust) {
			continue
		}
		if f.CompatibleWith != "" {
			evidence, ok := compatibilityForAgent(pack.Compatibility, f.CompatibleWith)
			if !ok {
				continue
			}
			if f.CompatStatus != "" && !strings.EqualFold(evidence.Status, f.CompatStatus) {
				continue
			}
		}
		matches = append(matches, pack)
	}
	return matches, nil
}

func containsString(slice []string, s string) bool {
	s = strings.ToLower(s)
	for _, v := range slice {
		if strings.ToLower(v) == s {
			return true
		}
	}
	return false
}

func compatibilityForAgent(compat model.Compatibility, agent string) (model.CompatibilityEvidence, bool) {
	if len(compat) == 0 {
		return model.CompatibilityEvidence{}, false
	}
	candidates := []string{
		strings.ToLower(strings.TrimSpace(agent)),
		targets.NormalizeAgent(agent),
	}
	if strings.EqualFold(agent, "claude") {
		candidates = append(candidates, "claude-code")
	}
	if strings.EqualFold(agent, "claude-code") {
		candidates = append(candidates, "claude")
	}
	for _, candidate := range candidates {
		for key, evidence := range compat {
			if strings.EqualFold(key, candidate) {
				return evidence, true
			}
		}
	}
	return model.CompatibilityEvidence{}, false
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
	if pack.Deprecated || pack.Stability == "deprecated" {
		fmt.Fprintln(out, "DEPRECATED: This pack is deprecated.")
		if pack.Replacement != "" {
			fmt.Fprintf(out, "Replacement: %s\n", pack.Replacement)
		}
	}
	fmt.Fprintf(out, "Version: %s\n", pack.Version)
	fmt.Fprintf(out, "License: %s\n", license)
	fmt.Fprintf(out, "Tags: %s\n", strings.Join(pack.Tags, ", "))
	if len(pack.UseCases) > 0 {
		fmt.Fprintln(out, "Use cases:")
		for _, useCase := range pack.UseCases {
			fmt.Fprintf(out, "- %s\n", useCase)
		}
	}
	if len(pack.ExamplePrompts) > 0 {
		fmt.Fprintln(out, "Example prompts:")
		for _, prompt := range pack.ExamplePrompts {
			fmt.Fprintf(out, "- %s\n", prompt)
		}
	}
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
	return expandPackInner(registry, pack, seen, map[string]bool{})
}

// expandPackInner carries two separate maps:
//   - seen: DFS ancestry set for cycle detection (with backtracking via delete)
//   - contributed: packs already fully expanded, to deduplicate diamond dependencies
func expandPackInner(registry string, pack model.Pack, seen, contributed map[string]bool) (model.Pack, error) {
	if seen[pack.ID] {
		return model.Pack{}, fmt.Errorf("pack composition cycle includes %s", pack.ID)
	}
	seen[pack.ID] = true
	out := pack
	out.Packs = append([]string{}, pack.Packs...)
	out.Skills = append(model.CapabilityRefs{}, pack.Skills...)
	out.Plugins = append(model.CapabilityRefs{}, pack.Plugins...)
	out.Commands = append(model.CapabilityRefs{}, pack.Commands...)
	out.Hooks = append(model.CapabilityRefs{}, pack.Hooks...)
	out.Subagents = append(model.CapabilityRefs{}, pack.Subagents...)
	out.Prompts = append(model.CapabilityRefs{}, pack.Prompts...)
	out.Templates = append(model.CapabilityRefs{}, pack.Templates...)
	out.ToolRefs = append(model.CapabilityRefs{}, pack.ToolRefs...)
	out.Memory = append(model.CapabilityRefs{}, pack.Memory...)
	out.Settings = append(model.CapabilityRefs{}, pack.Settings...)
	out.MCP = append(model.CapabilityRefs{}, pack.MCP...)
	out.Capabilities = []model.Capability{}
	for _, childRef := range pack.Packs {
		if contributed[childRef] {
			continue
		}
		child, err := FindPack(registry, childRef)
		if err != nil {
			return model.Pack{}, err
		}
		expanded, err := expandPackInner(registry, child, seen, contributed)
		if err != nil {
			return model.Pack{}, err
		}
		contributed[childRef] = true
		out.Capabilities = append(out.Capabilities, expanded.Capabilities...)
	}
	// Requires are implicit skill dependencies — resolved before explicit Skills.
	for _, reqID := range pack.Requires {
		if contributed["skill:"+reqID] {
			continue
		}
		skill, err := FindCapability(registry, "skills", reqID)
		if err != nil {
			return model.Pack{}, fmt.Errorf("required skill %q not found: %w", reqID, err)
		}
		contributed["skill:"+reqID] = true
		out.Capabilities = append(out.Capabilities, skill)
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
	for _, group := range []struct {
		typ  string
		refs model.CapabilityRefs
	}{
		{"command", pack.Commands},
		{"hook", pack.Hooks},
		{"subagent", pack.Subagents},
		{"prompt", pack.Prompts},
		{"template", pack.Templates},
		{"tool", pack.ToolRefs},
		{"memory", pack.Memory},
		{"settings", pack.Settings},
		{"mcp", pack.MCP},
	} {
		for _, ref := range group.refs {
			capability, err := ResolveCapabilityRef(registry, group.typ, ref)
			if err != nil {
				return model.Pack{}, err
			}
			out.Capabilities = append(out.Capabilities, capability)
		}
	}
	out.Capabilities = append(out.Capabilities, normalizeLocalSources(pack.Capabilities, pack.Path)...)
	delete(seen, pack.ID)
	return out, nil
}

func normalizeLocalSources(capabilities []model.Capability, packPath string) []model.Capability {
	if packPath == "" {
		return append([]model.Capability{}, capabilities...)
	}
	base := filepath.Dir(packPath)
	out := make([]model.Capability, 0, len(capabilities))
	for _, capability := range capabilities {
		if capability.Source != "" &&
			util.IsLocalSource(capability.Source) &&
			!filepath.IsAbs(util.ExpandHome(capability.Source)) {
			capability.Source = filepath.Join(base, capability.Source)
		}
		out = append(out, capability)
	}
	return out
}

func ResolveCapabilityRef(registry, capabilityType string, ref model.CapabilityRef) (model.Capability, error) {
	if ref.ID == "" {
		return model.Capability{}, fmt.Errorf("%s reference id is required", capabilityType)
	}
	if ref.Source == "" {
		kind := pluralCapabilityKind(capabilityType)
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
	}
	return model.Capability{
		Type: capabilityType, Name: name, Source: ref.Source, UpstreamSource: upstreamSource,
		Format: format, Version: ref.Version, Entry: entry, Homepage: ref.Homepage,
		Repository: ref.Repository, License: ref.License, Install: install, Trust: ref.Trust, Reference: true,
	}, nil
}

func pluralCapabilityKind(capabilityType string) string {
	switch capabilityType {
	case "memory", "settings", "mcp":
		return capabilityType
	default:
		return capabilityType + "s"
	}
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
	capType := singularCapabilityKind(kind)
	if capType == "" {
		return model.Capability{}, fmt.Errorf("unsupported capability kind: %s", kind)
	}
	path := filepath.Join(root, kind, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Capability{}, fmt.Errorf("%s capability not found or invalid: %s", capType, id)
	}
	var capability model.Capability
	if err := json.Unmarshal(data, &capability); err != nil {
		return model.Capability{}, fmt.Errorf("%s capability not found or invalid: %s", capType, id)
	}
	if capability.Type == "" {
		capability.Type = capType
	}
	if capability.Type != capType {
		return model.Capability{}, fmt.Errorf("%s capability %q has type %q", capType, id, capability.Type)
	}
	if capability.Name == "" {
		capability.Name = id
	}
	capability.Reference = true
	return capability, nil
}

func singularCapabilityKind(kind string) string {
	switch kind {
	case "commands":
		return "command"
	case "hooks":
		return "hook"
	case "subagents":
		return "subagent"
	case "prompts":
		return "prompt"
	case "templates":
		return "template"
	case "tools":
		return "tool"
	case "memory":
		return "memory"
	case "settings":
		return "settings"
	case "mcp":
		return "mcp"
	default:
		return ""
	}
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

// buildIndex regenerates the registry index in memory (without a generatedAt
// stamp). The generatedAt field is left blank for callers to set.
func buildIndex(registry string) (model.RegistryIndex, error) {
	packs, err := LoadPacks(registry)
	if err != nil {
		return model.RegistryIndex{}, err
	}
	index := model.RegistryIndex{}
	for _, pack := range packs {
		expanded, err := ExpandPack(registry, pack, map[string]bool{})
		if err != nil {
			return model.RegistryIndex{}, err
		}
		entry := model.IndexEntry{
			ID: pack.ID, Name: pack.Name, Version: pack.Version, Description: pack.Description,
			Maintainers: pack.Maintainers, Stability: pack.Stability, Deprecated: pack.Deprecated,
			Replacement: pack.Replacement, LastVerified: pack.LastVerified, ReviewStatus: pack.ReviewStatus,
			UseCases: pack.UseCases, ExamplePrompts: pack.ExamplePrompts,
			Tags: pack.Tags, Categories: pack.Categories, Tools: pack.Tools, Scope: pack.Scope,
			Skills: pack.Skills.IDs(), Plugins: pack.Plugins.IDs(), Capabilities: len(expanded.Capabilities),
			CapabilityTypes: capabilityTypeCounts(expanded.Capabilities), Trust: pack.Trust,
			Compatibility: pack.Compatibility, Freshness: freshnessStatus(pack.LastVerified, time.Now().UTC()),
		}
		index.Packs = append(index.Packs, entry)
	}
	return index, nil
}

func capabilityTypeCounts(capabilities []model.Capability) map[string]int {
	counts := map[string]int{}
	for _, capability := range capabilities {
		counts[capability.Type]++
	}
	if len(counts) == 0 {
		return nil
	}
	return counts
}

func freshnessStatus(lastVerified string, now time.Time) string {
	if lastVerified == "" {
		return "missing"
	}
	verifiedAt, err := time.Parse("2006-01-02", lastVerified)
	if err != nil {
		return "invalid"
	}
	if now.Sub(verifiedAt) > 90*24*time.Hour {
		return "stale"
	}
	return "fresh"
}

func indexOutputPath(registry, outputPath string) string {
	if outputPath == "" {
		return filepath.Join(RegistryRoot(registry), "index.json")
	}
	return outputPath
}

func GenerateIndex(registry, outputPath string, out io.Writer) error {
	index, err := buildIndex(registry)
	if err != nil {
		return err
	}
	outputPath = indexOutputPath(registry, outputPath)
	// Keep the index byte-stable across regenerations: only stamp a new
	// generatedAt when the substantive content actually changed. This avoids
	// noisy one-line git diffs (and spurious "stale index" churn) every time
	// the index is rebuilt from an unchanged registry.
	index.GeneratedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if existing, err := loadIndex(outputPath); err == nil {
		if existing.GeneratedAt != "" && samePacks(existing.Packs, index.Packs) {
			index.GeneratedAt = existing.GeneratedAt
		}
	}
	if err := util.WriteJSON(outputPath, index); err != nil {
		return err
	}
	fmt.Fprintf(out, "Wrote %s\n", outputPath)
	return nil
}

// CheckIndex regenerates the index in memory and compares it against the file
// at outputPath, ignoring the generatedAt field. It never writes the file. When
// the on-disk index matches it prints a success line and returns nil; on drift
// it prints a concise per-pack diff summary and returns model.ErrInstallFailed.
func CheckIndex(registry, outputPath string, out io.Writer) error {
	generated, err := buildIndex(registry)
	if err != nil {
		return err
	}
	outputPath = indexOutputPath(registry, outputPath)
	existing, err := loadIndex(outputPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(out, "FAIL  %s does not exist; run: agent-packs index --output %s\n", outputPath, outputPath)
			return model.ErrInstallFailed
		}
		return err
	}
	diffs := diffIndexPacks(existing.Packs, generated.Packs)
	if len(diffs) == 0 {
		fmt.Fprintf(out, "OK    %s is up to date (%d packs)\n", outputPath, len(generated.Packs))
		return nil
	}
	fmt.Fprintf(out, "FAIL  %s is out of date (run: agent-packs index --output %s)\n", outputPath, outputPath)
	for _, d := range diffs {
		fmt.Fprintf(out, "  - %s\n", d)
	}
	return model.ErrInstallFailed
}

// diffIndexPacks returns a concise, deterministic summary of how the on-disk
// index (existing) differs from a freshly regenerated one. The generatedAt
// field is not part of IndexEntry, so it is inherently ignored.
func diffIndexPacks(existing, generated []model.IndexEntry) []string {
	existingByID := map[string]model.IndexEntry{}
	for _, e := range existing {
		existingByID[e.ID] = e
	}
	generatedByID := map[string]model.IndexEntry{}
	for _, g := range generated {
		generatedByID[g.ID] = g
	}
	ids := map[string]bool{}
	for id := range existingByID {
		ids[id] = true
	}
	for id := range generatedByID {
		ids[id] = true
	}
	sortedIDs := make([]string, 0, len(ids))
	for id := range ids {
		sortedIDs = append(sortedIDs, id)
	}
	sort.Strings(sortedIDs)
	var diffs []string
	for _, id := range sortedIDs {
		e, inExisting := existingByID[id]
		g, inGenerated := generatedByID[id]
		switch {
		case inExisting && !inGenerated:
			diffs = append(diffs, fmt.Sprintf("%s: present in index but no longer in registry", id))
		case !inExisting && inGenerated:
			diffs = append(diffs, fmt.Sprintf("%s: missing from index (new pack)", id))
		default:
			for _, field := range changedIndexFields(e, g) {
				diffs = append(diffs, fmt.Sprintf("%s: field %q differs", id, field))
			}
		}
	}
	return diffs
}

// changedIndexFields reports which top-level IndexEntry fields differ between
// two entries with the same id, by comparing each field's canonical JSON form.
func changedIndexFields(a, b model.IndexEntry) []string {
	fields := map[string][2]any{
		"name":            {a.Name, b.Name},
		"version":         {a.Version, b.Version},
		"description":     {a.Description, b.Description},
		"maintainers":     {a.Maintainers, b.Maintainers},
		"stability":       {a.Stability, b.Stability},
		"deprecated":      {a.Deprecated, b.Deprecated},
		"replacement":     {a.Replacement, b.Replacement},
		"lastVerified":    {a.LastVerified, b.LastVerified},
		"reviewStatus":    {a.ReviewStatus, b.ReviewStatus},
		"useCases":        {a.UseCases, b.UseCases},
		"examplePrompts":  {a.ExamplePrompts, b.ExamplePrompts},
		"tags":            {a.Tags, b.Tags},
		"categories":      {a.Categories, b.Categories},
		"tools":           {a.Tools, b.Tools},
		"scope":           {a.Scope, b.Scope},
		"skills":          {a.Skills, b.Skills},
		"plugins":         {a.Plugins, b.Plugins},
		"capabilities":    {a.Capabilities, b.Capabilities},
		"capabilityTypes": {a.CapabilityTypes, b.CapabilityTypes},
		"trust":           {a.Trust, b.Trust},
		"compatibility":   {a.Compatibility, b.Compatibility},
		"freshness":       {a.Freshness, b.Freshness},
	}
	var changed []string
	for name, pair := range fields {
		if !sameIndexFieldValue(pair[0], pair[1]) {
			changed = append(changed, name)
		}
	}
	sort.Strings(changed)
	return changed
}

// sameIndexFieldValue compares two IndexEntry field values by canonical JSON,
// treating a nil slice and an empty slice as equal. This matches the equality
// semantics GenerateIndex uses (see samePacks): omitempty erases the nil-vs-
// empty distinction on disk, so the check must not flag it as drift.
func sameIndexFieldValue(a, b any) bool {
	aj := normalizeFieldJSON(a)
	bj := normalizeFieldJSON(b)
	return bytes.Equal(aj, bj)
}

func normalizeFieldJSON(v any) []byte {
	if s, ok := v.([]string); ok && len(s) == 0 {
		return []byte("[]")
	}
	data, _ := json.Marshal(v)
	if bytes.Equal(data, []byte("null")) {
		return []byte("[]")
	}
	return data
}

func loadIndex(path string) (model.RegistryIndex, error) {
	var index model.RegistryIndex
	data, err := os.ReadFile(path)
	if err != nil {
		return index, err
	}
	if err := json.Unmarshal(data, &index); err != nil {
		return index, err
	}
	return index, nil
}

// samePacks compares two pack-entry slices by their canonical JSON form so that
// nil vs empty-slice differences (which survive a load/regenerate round-trip but
// are erased by omitempty marshaling) don't count as a change.
func samePacks(a, b []model.IndexEntry) bool {
	aj, err1 := json.Marshal(a)
	bj, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return bytes.Equal(aj, bj)
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
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "git", "-C", cache, "pull", "--ff-only")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: registry %q may be stale: %s\n", name, strings.TrimSpace(stderr.String()))
		}
		return cache, nil
	}
	_ = os.RemoveAll(cache)
	if err := os.MkdirAll(filepath.Dir(cache), 0o755); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", source, cache)
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
	fields = append(fields, pack.Maintainers...)
	fields = append(fields, pack.Stability, pack.ReviewStatus)
	fields = append(fields, pack.UseCases...)
	fields = append(fields, pack.ExamplePrompts...)
	fields = append(fields, pack.Tags...)
	fields = append(fields, pack.Categories...)
	fields = append(fields, pack.Tools...)
	fields = append(fields, pack.Scope...)
	fields = appendCapabilityRefFields(fields, pack.Skills)
	fields = appendCapabilityRefFields(fields, pack.Plugins)
	fields = appendCapabilityRefFields(fields, pack.Commands)
	fields = appendCapabilityRefFields(fields, pack.Hooks)
	fields = appendCapabilityRefFields(fields, pack.Subagents)
	fields = appendCapabilityRefFields(fields, pack.Prompts)
	fields = appendCapabilityRefFields(fields, pack.Templates)
	fields = appendCapabilityRefFields(fields, pack.ToolRefs)
	fields = appendCapabilityRefFields(fields, pack.Memory)
	fields = appendCapabilityRefFields(fields, pack.Settings)
	fields = appendCapabilityRefFields(fields, pack.MCP)
	for _, capability := range pack.Capabilities {
		fields = append(fields, capability.Type, capability.Name, capability.Content, capability.Format)
	}
	haystack := strings.ToLower(strings.Join(fields, " "))
	if strings.Contains(haystack, query) {
		return true
	}
	tokens := searchTokens(query)
	if len(tokens) == 0 {
		return false
	}
	for _, token := range tokens {
		if !strings.Contains(haystack, token) {
			return false
		}
	}
	return true
}

func appendCapabilityRefFields(fields []string, refs model.CapabilityRefs) []string {
	for _, ref := range refs {
		fields = append(fields, ref.ID, ref.Name, ref.Format)
	}
	return fields
}

func searchTokens(query string) []string {
	return strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func DependencyTree(registryPath, packRef string) (model.DependencyTree, error) {
	pack, err := FindPack(registryPath, packRef)
	if err != nil {
		return model.DependencyTree{}, err
	}
	nodes, err := dependencyNodes(registryPath, pack, map[string]bool{})
	if err != nil {
		return model.DependencyTree{}, err
	}
	return model.DependencyTree{Pack: pack.ID, Version: pack.Version, Dependencies: nodes}, nil
}

func dependencyNodes(registryPath string, pack model.Pack, seen map[string]bool) ([]model.DependencyNode, error) {
	if seen[pack.ID] {
		return nil, fmt.Errorf("pack composition cycle includes %s", pack.ID)
	}
	seen[pack.ID] = true
	nodes := []model.DependencyNode{}
	for _, childRef := range pack.Packs {
		child, err := FindPack(registryPath, childRef)
		if err != nil {
			return nil, err
		}
		children, err := dependencyNodes(registryPath, child, seen)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, model.DependencyNode{
			Type: "pack", ID: child.ID, Name: child.Name, Source: child.Path, Dependencies: children,
		})
	}
	for _, ref := range pack.Skills {
		capability, err := ResolveCapabilityRef(registryPath, "skill", ref)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, capabilityNode("skill", ref.ID, capability))
	}
	for _, ref := range pack.Plugins {
		capability, err := ResolveCapabilityRef(registryPath, "plugin", ref)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, capabilityNode("plugin", ref.ID, capability))
	}
	for _, capability := range pack.Capabilities {
		nodes = append(nodes, capabilityNode(capability.Type, "", capability))
	}
	delete(seen, pack.ID)
	return nodes, nil
}

func capabilityNode(kind, id string, capability model.Capability) model.DependencyNode {
	return model.DependencyNode{
		Type: kind, ID: id, Name: capability.Name, Source: capability.Source,
		UpstreamSource: capability.UpstreamSource, Trust: capability.Trust, Format: capability.Format,
	}
}

// InfoResult holds the data surfaced by Info.
type InfoResult struct {
	Pack           model.Pack `json:"pack"`
	Installed      bool       `json:"installed"`
	InstalledAt    string     `json:"installedAt,omitempty"`
	DiskUsageBytes int64      `json:"diskUsageBytes,omitempty"`
}

func Info(registryPath, home, packRef string, out io.Writer) error {
	result, err := buildInfoResult(registryPath, home, packRef)
	if err != nil {
		return err
	}
	pack := result.Pack
	fmt.Fprintf(out, "%s: %s\n", pack.ID, pack.Name)
	fmt.Fprintln(out, pack.Description)
	fmt.Fprintln(out)

	trust := pack.Trust
	if trust == "" {
		trust = "unverified"
	}
	fmt.Fprintf(out, "Version:       %s\n", pack.Version)
	fmt.Fprintf(out, "Trust:         %s\n", trust)
	if pack.Stability != "" {
		fmt.Fprintf(out, "Stability:     %s\n", pack.Stability)
	}
	if pack.LastVerified != "" {
		fmt.Fprintf(out, "Last verified: %s\n", pack.LastVerified)
	}
	if pack.License != "" {
		fmt.Fprintf(out, "License:       %s\n", pack.License)
	}
	if len(pack.Tools) > 0 {
		fmt.Fprintf(out, "Works with:    %s\n", strings.Join(pack.Tools, ", "))
	}
	if len(pack.Tags) > 0 {
		fmt.Fprintf(out, "Tags:          %s\n", strings.Join(pack.Tags, ", "))
	}
	fmt.Fprintln(out)
	if len(pack.Packs) > 0 {
		fmt.Fprintf(out, "Includes packs (%d):\n", len(pack.Packs))
		for _, p := range pack.Packs {
			fmt.Fprintf(out, "  %s\n", p)
		}
	}
	if len(pack.Requires) > 0 {
		fmt.Fprintf(out, "Requires skills (%d):\n", len(pack.Requires))
		for _, r := range pack.Requires {
			fmt.Fprintf(out, "  %s\n", r)
		}
	}
	if len(pack.Skills) > 0 {
		fmt.Fprintf(out, "Skills (%d):\n", len(pack.Skills))
		for _, s := range pack.Skills {
			fmt.Fprintf(out, "  %s\n", s.ID)
		}
	}
	if len(pack.ConflictsWith) > 0 {
		fmt.Fprintf(out, "Conflicts with: %s\n", strings.Join(pack.ConflictsWith, ", "))
	}
	fmt.Fprintln(out)
	if result.Installed {
		fmt.Fprintf(out, "Installed: %s (at %s)\n", pack.Version, result.InstalledAt)
		if result.DiskUsageBytes > 0 {
			fmt.Fprintf(out, "Disk usage: %s\n", humanBytes(result.DiskUsageBytes))
		}
	} else {
		fmt.Fprintln(out, "Not installed.")
		fmt.Fprintf(out, "Install with: agent-packs install %s\n", pack.ID)
	}
	if pack.UpstreamSource != "" {
		fmt.Fprintf(out, "Homepage: %s\n", pack.UpstreamSource)
	}
	return nil
}

func InfoJSON(registryPath, home, packRef string, out io.Writer) error {
	result, err := buildInfoResult(registryPath, home, packRef)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func OpenHome(registryPath, home, packRef string) error {
	pack, err := FindPack(registryPath, packRef)
	if err != nil {
		return err
	}
	url := pack.UpstreamSource
	if url == "" {
		return fmt.Errorf("pack %q has no homepage URL (set upstreamSource in the pack manifest)", packRef)
	}
	opener := "open"
	if _, err := exec.LookPath("xdg-open"); err == nil {
		opener = "xdg-open"
	}
	cmd := exec.Command(opener, url)
	return cmd.Start()
}

func buildInfoResult(registryPath, home, packRef string) (InfoResult, error) {
	pack, err := FindPack(registryPath, packRef)
	if err != nil {
		return InfoResult{}, err
	}
	result := InfoResult{Pack: pack}
	receiptsDir := filepath.Join(util.ExpandHome(home), "receipts")
	receiptPath := filepath.Join(receiptsDir, pack.ID+".json")
	if data, err := os.ReadFile(receiptPath); err == nil {
		var receipt struct {
			InstalledAt string `json:"installed_at"`
		}
		if json.Unmarshal(data, &receipt) == nil {
			result.Installed = true
			result.InstalledAt = receipt.InstalledAt
		}
		skillsDir := filepath.Join(util.ExpandHome(home), "skills")
		result.DiskUsageBytes = dirSize(skillsDir)
	}
	return result, nil
}

func dirSize(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
