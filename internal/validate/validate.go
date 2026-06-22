package validate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/agent-packs/cli/internal/model"
	"github.com/agent-packs/cli/internal/policy"
	"github.com/agent-packs/cli/internal/registry"
	"github.com/agent-packs/cli/internal/resolve"
	"github.com/agent-packs/cli/internal/targets"
	"github.com/agent-packs/cli/internal/util"
)

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
			errs := ValidateCapabilityManifestPath(p)
			if len(errs) > 0 {
				fmt.Fprintf(out, "FAIL  %s\n", p)
				for _, msg := range errs {
					fmt.Fprintf(out, "  - %s\n", msg)
				}
				failed = true
			} else {
				fmt.Fprintf(out, "OK    %s\n", p)
			}
			continue
		}
		pack, err := registry.LoadPack(p)
		if err != nil {
			fmt.Fprintf(out, "FAIL  %s: %s\n", p, err)
			failed = true
			continue
		}
		errs := ValidatePackWithSchemaDir(pack, filepath.Dir(p))
		if len(errs) > 0 {
			fmt.Fprintf(out, "FAIL  %s\n", p)
			for _, msg := range errs {
				fmt.Fprintf(out, "  - %s\n", msg)
			}
			failed = true
		} else {
			fmt.Fprintf(out, "OK    %s\n", p)
		}
	}
	if failed {
		return model.ErrInstallFailed
	}
	return nil
}

func isCapabilityManifestPath(path string) bool {
	return strings.HasSuffix(filepath.ToSlash(path), "/SKILL.md") ||
		strings.HasSuffix(filepath.ToSlash(path), "/.claude-plugin/plugin.json")
}

func ValidateCapabilityManifestPath(path string) []string {
	if strings.HasSuffix(filepath.ToSlash(path), "/SKILL.md") {
		manifest, err := registry.LoadSkillManifest(path)
		if err != nil {
			return []string{err.Error()}
		}
		return ValidateSkillManifest(filepath.Base(filepath.Dir(path)), manifest)
	}
	if strings.HasSuffix(filepath.ToSlash(path), "/.claude-plugin/plugin.json") {
		manifest, err := registry.LoadPluginManifest(path)
		if err != nil {
			return []string{err.Error()}
		}
		return ValidatePluginManifest(manifest)
	}
	return []string{"unsupported capability manifest path"}
}

func ValidateSkillManifest(directoryName string, manifest model.SkillManifest) []string {
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

func ValidatePluginManifest(manifest model.PluginManifest) []string {
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

func ValidateCapability(capability model.Capability, prefix string) []string {
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
		if capability.Install != nil && capability.Install["uninstall"] != "" && !capability.RequiresExecution {
			errs = append(errs, prefix+".requiresExecution must be true when install.uninstall is set")
		}
	}
	return errs
}

func ValidatePack(pack model.Pack) []string {
	return ValidatePackWithSchemaDir(pack, "")
}

// ValidatePackWithSchemaDir validates a pack, sourcing the allowed category set
// from the registry JSON schema found by walking up from schemaDir. When
// schemaDir is empty (e.g. validating a standalone file with no registry
// context) the canonical fallback list is used.
func ValidatePackWithSchemaDir(pack model.Pack, schemaDir string) []string {
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
	if pack.Stability != "" && pack.Stability != "experimental" && pack.Stability != "stable" && pack.Stability != "deprecated" {
		errs = append(errs, "stability must be experimental, stable, or deprecated")
	}
	if pack.ReviewStatus != "" && pack.ReviewStatus != "draft" && pack.ReviewStatus != "reviewed" && pack.ReviewStatus != "verified" {
		errs = append(errs, "reviewStatus must be draft, reviewed, or verified")
	}
	if pack.Deprecated && pack.Replacement == "" {
		errs = append(errs, "replacement is required when deprecated is true")
	}
	if len(pack.Capabilities) == 0 && len(pack.Packs) == 0 && len(pack.Skills) == 0 && len(pack.Plugins) == 0 {
		errs = append(errs, "capabilities, packs, skills, or plugins is required")
	}
	for i, ref := range pack.Skills {
		errs = append(errs, ValidateCapabilityRef(ref, "skill", fmt.Sprintf("skills[%d]", i), schemaDir)...)
	}
	for i, ref := range pack.Plugins {
		errs = append(errs, ValidateCapabilityRef(ref, "plugin", fmt.Sprintf("plugins[%d]", i), schemaDir)...)
	}
	for i, capability := range pack.Capabilities {
		errs = append(errs, ValidateCapability(capability, fmt.Sprintf("capabilities[%d]", i))...)
	}
	errs = append(errs, validateCategories(pack.Categories, AllowedCategories(schemaDir))...)
	return errs
}

func ValidateCapabilityRef(ref model.CapabilityRef, capabilityType, prefix, schemaDir string) []string {
	var errs []string
	if ref.ID == "" {
		errs = append(errs, prefix+".id is required")
	}
	if capabilityType == "skill" && ref.Format != "" && ref.Format != "agent-skill" {
		errs = append(errs, prefix+".format must be agent-skill")
	}
	if capabilityType == "plugin" && ref.Format != "" && ref.Format != "anthropic-plugin" && ref.Format != "codex-plugin" && ref.Format != "other" {
		errs = append(errs, prefix+".format is not allowed for plugin")
	}
	if ref.Install != nil && ref.Install["method"] == "" {
		errs = append(errs, prefix+".install.method is required")
	}
	// Object refs (those authored as JSON objects rather than bare strings)
	// must declare a valid `trust` value. Bare-string refs carry no provenance
	// metadata and are exempt, matching the schema's oneOf[string, object].
	if ref.IsObjectRef() {
		errs = append(errs, validateTrust(ref.Trust, AllowedTrustLevels(schemaDir), prefix)...)
	}
	return errs
}

// injectionPatterns are regexes that flag potential prompt injection in skill content.
var injectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ignore\s+(all\s+|previous\s+|prior\s+|above\s+)?instructions`),
	regexp.MustCompile(`(?i)disregard\s+(all\s+|previous\s+|prior\s+)?instructions`),
	regexp.MustCompile(`(?i)forget\s+(all\s+|your\s+|previous\s+)?instructions`),
	regexp.MustCompile(`<\|endoftext\|>`),
	regexp.MustCompile(`<\|im_end\|>`),
	regexp.MustCompile(`(?i)you\s+are\s+now\s+(a\s+|an\s+)?(?:different|unrestricted|evil|jailbreak)`),
	regexp.MustCompile(`(?i)act\s+as\s+(?:a\s+|an\s+)?(?:different|unrestricted|evil|jailbreak|dan)`),
}

func scanSkillInjection(skillPath string) []string {
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return nil
	}
	content := string(data)
	var findings []string
	for _, pat := range injectionPatterns {
		if pat.MatchString(content) {
			findings = append(findings, "possible prompt injection pattern: "+pat.String())
		}
	}
	return findings
}

func Lint(registryPath, packRef string, out io.Writer) error {
	pack, err := registry.FindPack(registryPath, packRef)
	if err != nil {
		return err
	}
	// Scan local skills for prompt injection patterns.
	root := registry.RegistryRoot(registryPath)
	errs := ValidatePackWithSchemaDir(pack, root)
	for _, skillRef := range pack.Skills {
		skillPath := filepath.Join(root, "skills", skillRef.ID, "SKILL.md")
		errs = append(errs, scanSkillInjection(skillPath)...)
	}
	if len(errs) > 0 {
		for _, msg := range errs {
			fmt.Fprintf(out, "FAIL  %s: %s\n", pack.ID, msg)
		}
		return model.ErrInstallFailed
	}
	fmt.Fprintf(out, "OK    %s\n", pack.ID)
	return nil
}

func LintAll(registryPath string, out io.Writer) error {
	packs, err := registry.LoadPacks(registryPath)
	if err != nil {
		return err
	}
	root := registry.RegistryRoot(registryPath)
	failed := false
	for _, pack := range packs {
		errs := ValidatePackWithSchemaDir(pack, root)
		for _, skillRef := range pack.Skills {
			skillPath := filepath.Join(root, "skills", skillRef.ID, "SKILL.md")
			errs = append(errs, scanSkillInjection(skillPath)...)
		}
		if len(errs) > 0 {
			for _, msg := range errs {
				fmt.Fprintf(out, "FAIL  %s: %s\n", pack.ID, msg)
			}
			failed = true
		} else {
			fmt.Fprintf(out, "OK    %s\n", pack.ID)
		}
	}
	if failed {
		return model.ErrInstallFailed
	}
	return nil
}

func VerifyAll(registryPath string, out io.Writer) error {
	packs, err := registry.LoadPacks(registryPath)
	if err != nil {
		return err
	}
	failed := false
	for _, pack := range packs {
		var buf strings.Builder
		if err := Verify(registryPath, pack.ID, &buf); err != nil {
			fmt.Fprintf(out, "FAIL  %s\n", pack.ID)
			if msg := strings.TrimSpace(buf.String()); msg != "" {
				fmt.Fprintf(out, "      %s\n", strings.ReplaceAll(msg, "\n", "\n      "))
			}
			failed = true
		} else {
			fmt.Fprintf(out, "OK    %s\n", pack.ID)
		}
	}
	if failed {
		return model.ErrInstallFailed
	}
	return nil
}

func PublishCheck(registryPath, policyPath string, out io.Writer) error {
	report, err := PublishReport(registryPath, policyPath)
	if err != nil {
		return err
	}
	for _, check := range report.Checks {
		fmt.Fprintf(out, "%s\t%s\t%s", check.Status, check.Kind, check.Target)
		if check.Message != "" {
			fmt.Fprintf(out, "\t%s", check.Message)
		}
		fmt.Fprintln(out)
	}
	if !report.OK {
		return model.ErrInstallFailed
	}
	return nil
}

func PublishReport(registryPath, policyPath string) (model.PublishReport, error) {
	report := model.PublishReport{Registry: registryPath, OK: true}
	root := registry.RegistryRoot(registryPath)
	add := func(kind, target, status, message string) {
		if status == "FAIL" {
			report.OK = false
		}
		report.Checks = append(report.Checks, model.CheckEntry{Kind: kind, Target: target, Status: status, Message: message})
	}
	packs, err := registry.LoadPacks(registryPath)
	if err != nil {
		return report, err
	}
	metadata := MetadataCoverage(packs, time.Now().UTC())
	report.Metadata = &metadata
	for _, check := range metadataChecks(metadata) {
		report.Checks = append(report.Checks, check)
	}
	seen := map[string]string{}
	for _, pack := range packs {
		if previous, ok := seen[pack.ID]; ok {
			add("duplicate-id", pack.ID, "FAIL", "also defined in "+previous)
		} else {
			seen[pack.ID] = pack.Path
		}
		if errs := ValidatePackWithSchemaDir(pack, root); len(errs) > 0 {
			add("schema", pack.ID, "FAIL", strings.Join(errs, "; "))
		} else {
			add("schema", pack.ID, "OK", "")
		}
		var sink strings.Builder
		if err := Verify(registryPath, pack.ID, &sink); err != nil {
			add("verify", pack.ID, "FAIL", strings.TrimSpace(sink.String()))
		} else {
			add("verify", pack.ID, "OK", "")
		}
		audit, err := policy.BuildAuditReport(registryPath, pack.ID)
		if err != nil {
			add("audit", pack.ID, "FAIL", err.Error())
		} else if !audit.OK {
			add("audit", pack.ID, "WARN", strings.Join(audit.Violations, "; "))
		} else {
			add("audit", pack.ID, "OK", "")
		}
		if policyPath != "" {
			var policySink strings.Builder
			if err := policy.PolicyCheck(registryPath, pack.ID, policyPath, &policySink); err != nil {
				add("policy", pack.ID, "FAIL", strings.TrimSpace(policySink.String()))
			} else {
				add("policy", pack.ID, "OK", "")
			}
		}
	}
	for _, dir := range []string{"skills", "plugins", "schemas/examples"} {
		path := filepath.Join(root, dir)
		var sink strings.Builder
		if err := ValidatePath(path, &sink); err != nil {
			add("validate", dir, "FAIL", strings.TrimSpace(sink.String()))
		} else {
			add("validate", dir, "OK", "")
		}
	}
	return report, nil
}

func Verify(registryPath, packRef string, out io.Writer) error {
	pack, err := registry.FindPack(registryPath, packRef)
	if err != nil {
		return err
	}
	expanded, err := registry.ExpandPack(registryPath, pack, map[string]bool{})
	if err != nil {
		return err
	}
	fail := false
	seen := map[string]bool{}
	for _, capability := range expanded.Capabilities {
		key := capability.Type + ":" + capability.Name
		if seen[key] {
			fmt.Fprintf(out, "FAIL  duplicate capability: %s\n", key)
			fail = true
		}
		seen[key] = true
		if capability.Source == "" {
			fmt.Fprintf(out, "FAIL  missing source: %s\n", key)
			fail = true
		}
		resolution := resolve.ResolveSource(capability.Source)
		if resolution.Warning != "" {
			fmt.Fprintf(out, "WARN  %s: %s\n", key, resolution.Warning)
		}
	}
	if fail {
		return model.ErrInstallFailed
	}
	fmt.Fprintf(out, "OK    %s verified (%d capabilities)\n", expanded.ID, len(expanded.Capabilities))
	return nil
}

func ResolveSources(registryPath, packRef string, out io.Writer) error {
	pack, err := registry.FindPack(registryPath, packRef)
	if err != nil {
		return err
	}
	expanded, err := registry.ExpandPack(registryPath, pack, map[string]bool{})
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Pack: %s\n", expanded.ID)
	for _, capability := range expanded.Capabilities {
		resolution := resolve.ResolveSource(capability.Source)
		line := fmt.Sprintf("%s\t%s\t%s", capability.Type, capability.Name, resolution.Kind)
		if resolution.Revision != "" {
			line += "\t" + resolution.Revision
		}
		if resolution.Pinned {
			line += "\tpinned"
		}
		if resolution.Warning != "" {
			line += "\tWARN " + resolution.Warning
		}
		fmt.Fprintln(out, line)
	}
	return nil
}

func Licenses(registryPath, packRef string, out io.Writer) error {
	pack, err := registry.FindPack(registryPath, packRef)
	if err != nil {
		return err
	}
	expanded, err := registry.ExpandPack(registryPath, pack, map[string]bool{})
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Pack\t%s\t%s\n", expanded.ID, util.ValueOrUnknown(expanded.License))
	for _, capability := range expanded.Capabilities {
		fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", capability.Type, capability.Name, util.ValueOrUnknown(capability.License), capability.Source)
	}
	return nil
}

func Attribution(registryPath, packRef string, out io.Writer) error {
	pack, err := registry.FindPack(registryPath, packRef)
	if err != nil {
		return err
	}
	expanded, err := registry.ExpandPack(registryPath, pack, map[string]bool{})
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "# Attribution for %s\n\n", expanded.Name)
	for _, capability := range expanded.Capabilities {
		fmt.Fprintf(out, "- %s (%s): %s\n", capability.Name, capability.Type, capability.Source)
	}
	return nil
}

func Compatibility(registryPath, packRef, agent string, out io.Writer) error {
	result, err := CompatibilityReport(registryPath, packRef, agent)
	if err != nil {
		return err
	}
	if result.OK {
		fmt.Fprintf(out, "OK    %s is compatible with %s\n", result.Pack, result.Agent)
		return nil
	}
	if result.Message != "" {
		fmt.Fprintf(out, "WARN  %s\n", result.Message)
	}
	return model.ErrInstallFailed
}

func CompatibilityReport(registryPath, packRef, agent string) (model.CompatibilityResult, error) {
	pack, err := registry.FindPack(registryPath, packRef)
	if err != nil {
		return model.CompatibilityResult{}, err
	}
	normalized := targets.NormalizeAgent(agent)
	result := model.CompatibilityResult{Pack: pack.ID, Agent: normalized, Tools: pack.Tools, OK: true}
	if len(pack.Tools) > 0 && !targets.PackSupportsTool(pack.Tools, agent) {
		result.OK = false
		result.Message = fmt.Sprintf("%s not listed in pack tools: %s", normalized, strings.Join(pack.Tools, ", "))
	}
	if !targets.ValidAgent(agent) {
		result.OK = false
		result.Message = fmt.Sprintf("unsupported target tool: %s", agent)
		return result, model.ErrInstallFailed
	}
	return result, nil
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
		// Inline component objects are valid for hooks, LSP configs, etc.
	default:
		errs = append(errs, field+" must be a string, array, or object")
	}
	return errs
}
