package validate

import (
	"encoding/json"
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
		errs := ValidateSkillManifest(filepath.Base(filepath.Dir(path)), manifest)
		data, err := os.ReadFile(path)
		if err == nil {
			errs = append(errs, scanBlockedKeys(string(data))...)
		}
		return errs
	}
	if strings.HasSuffix(filepath.ToSlash(path), "/.claude-plugin/plugin.json") {
		manifest, err := registry.LoadPluginManifest(path)
		if err != nil {
			return []string{err.Error()}
		}
		errs := ValidatePluginManifest(manifest)
		data, err := os.ReadFile(path)
		if err == nil {
			errs = append(errs, scanBlockedKeys(string(data))...)
		}
		return errs
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
		// File-backed fragment capabilities may carry inline content instead
		// of a source file; one of the two is required.
		if capabilityAllowsInlineContent(capability.Type) {
			if capability.Content == "" {
				errs = append(errs, prefix+".source or .content is required")
			}
		} else {
			errs = append(errs, prefix+".source is required")
		}
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

	// Scan env variables
	if capability.Env != nil {
		for k, v := range capability.Env {
			kLower := strings.ToLower(k)
			isSecretKey := strings.HasSuffix(kLower, "key") ||
				strings.HasSuffix(kLower, "token") ||
				strings.HasSuffix(kLower, "secret") ||
				strings.HasSuffix(kLower, "password") ||
				strings.HasSuffix(kLower, "pwd")
			if isSecretKey && v != "" && !isPlaceholder(v) && !isEnvVarRef(v) {
				errs = append(errs, fmt.Sprintf("%s.env.%s contains a literal credentials value: %q (use a placeholder like '<your-key>' or env reference)", prefix, k, redactSecret(v)))
			}
		}
	}

	// Scan args for secrets
	for i, arg := range capability.Args {
		if isActualSecret(arg) {
			errs = append(errs, fmt.Sprintf("%s.args[%d] contains a secret-looking value: %q", prefix, i, redactSecret(arg)))
		}
	}

	// Scan inline capability content
	if capability.Content != "" {
		// Scan for strong secret tokens in any inline content
		for _, errStr := range scanBlockedKeys(capability.Content) {
			errs = append(errs, fmt.Sprintf("%s.content contains a blocked secret: %s", prefix, errStr))
		}

		// For settings/mcp, unmarshal and do deeper key-value placeholder checks
		if capability.Type == "settings" || capability.Type == "mcp" {
			var settingsMap map[string]any
			if err := json.Unmarshal([]byte(capability.Content), &settingsMap); err == nil {
				errs = append(errs, scanMapForCredentials(settingsMap, prefix+".content")...)
			}
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

// strongSecretPattern matches actual inline credential patterns (OpenAI, GitHub).
var strongSecretPattern = regexp.MustCompile(`(?i)\b(?:sk-[a-zA-Z0-9_-]{20,}|ghp_[a-zA-Z0-9]{36,})\b`)

func scanBlockedKeys(content string) []string {
	var findings []string
	matches := strongSecretPattern.FindAllString(content, -1)
	if len(matches) > 0 {
		seen := map[string]bool{}
		for _, m := range matches {
			mLower := strings.ToLower(m)
			if !seen[mLower] {
				seen[mLower] = true
				findings = append(findings, "blocked inline credential secret: "+redactSecret(m))
			}
		}
	}
	return findings
}

func scanSkillAPIKeys(skillPath string) []string {
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return nil
	}
	return scanBlockedKeys(string(data))
}

func redactSecret(val string) string {
	if len(val) <= 8 {
		return "[redacted]"
	}
	if strings.HasPrefix(val, "sk-") && len(val) > 7 {
		return "sk-..." + val[len(val)-4:]
	}
	if strings.HasPrefix(val, "ghp_") && len(val) > 8 {
		return "ghp_..." + val[len(val)-4:]
	}
	return val[:3] + "..." + val[len(val)-4:]
}

func isPlaceholder(val string) bool {
	val = strings.ToLower(val)
	return strings.Contains(val, "your") ||
		strings.Contains(val, "todo") ||
		strings.Contains(val, "placeholder") ||
		strings.Contains(val, "<") ||
		strings.Contains(val, ">") ||
		val == ""
}

func isEnvVarRef(val string) bool {
	val = strings.TrimSpace(val)
	isRef := strings.HasPrefix(val, "$") ||
		(strings.HasPrefix(val, "%") && strings.HasSuffix(val, "%") && len(val) > 2) ||
		(strings.HasPrefix(val, "{{") && strings.HasSuffix(val, "}}") && len(val) > 4)

	if !isRef {
		return false
	}

	// 1. If it contains a strong secret token (sk-..., ghp_...), it's never safe
	if strongSecretPattern.MatchString(val) {
		return false
	}

	// 2. Extract fallback if there is one
	var fallback string
	if strings.HasPrefix(val, "${") && strings.HasSuffix(val, "}") {
		inner := val[2 : len(val)-1]
		if idx := strings.Index(inner, ":-"); idx != -1 {
			fallback = inner[idx+2:]
		} else if idx := strings.Index(inner, "-"); idx != -1 {
			fallback = inner[idx+1:]
		} else if idx := strings.Index(inner, ":"); idx != -1 {
			fallback = inner[idx+1:]
		}
	} else if strings.HasPrefix(val, "{{") && strings.HasSuffix(val, "}}") {
		inner := val[2 : len(val)-2]
		if idx := strings.Index(inner, ":-"); idx != -1 {
			fallback = inner[idx+2:]
		} else if idx := strings.Index(inner, "-"); idx != -1 {
			fallback = inner[idx+1:]
		} else if idx := strings.Index(inner, ":"); idx != -1 {
			fallback = inner[idx+1:]
		}
	} else if strings.HasPrefix(val, "$") {
		// Plain $VAR reference.
		// If it's a plain reference but contains characters not typical in a variable name (like hyphens, colons, etc.),
		// it might be a fallback or a literal secret.
		name := val[1:]
		if strings.HasPrefix(name, "(") && strings.HasSuffix(name, ")") {
			name = name[1 : len(name)-1]
		}
		isStandardName := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`).MatchString(name) ||
			regexp.MustCompile(`^(?i)env:[a-zA-Z_][a-zA-Z0-9_]*$`).MatchString(name)
		if !isStandardName {
			fallback = val
		}
	}

	// 3. Scan the fallback/literal candidate against all secret patterns
	if fallback != "" {
		if isUUID(fallback) || isSHA(fallback) {
			return true
		}
		for _, pat := range secretPatterns {
			if pat.MatchString(fallback) {
				return false
			}
		}
	}

	return true
}

// secretPatterns lists regexes that match actual secrets, not just names of env vars.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\bsk-[a-zA-Z0-9_-]{20,}\b`),
	regexp.MustCompile(`\bghp_[a-zA-Z0-9]{36,}\b`),
	regexp.MustCompile(`\b[a-f0-9]{32,}\b`),
	regexp.MustCompile(`\b[a-zA-Z0-9_-]{32,}\b`),
}

func isUUID(val string) bool {
	return len(val) == 36 && regexp.MustCompile(`^(?i)[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`).MatchString(val)
}

func isSHA(val string) bool {
	return (len(val) == 40 && regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(val)) ||
		(len(val) == 64 && regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(val))
}

func isActualSecret(val string) bool {
	if isPlaceholder(val) || isEnvVarRef(val) {
		return false
	}
	if isUUID(val) || isSHA(val) {
		return false
	}
	// Ignore standard URL formats or paths
	if strings.HasPrefix(val, "http://") || strings.HasPrefix(val, "https://") || strings.HasPrefix(val, "/") || strings.Contains(val, ".") {
		return false
	}
	for _, pat := range secretPatterns {
		if pat.MatchString(val) {
			return true
		}
	}
	return false
}

func scanMapForCredentials(m map[string]any, prefix string) []string {
	var errs []string
	for k, v := range m {
		kLower := strings.ToLower(k)
		if strVal, ok := v.(string); ok {
			isSecretKey := strings.HasSuffix(kLower, "key") ||
				strings.HasSuffix(kLower, "token") ||
				strings.HasSuffix(kLower, "secret") ||
				strings.HasSuffix(kLower, "password") ||
				strings.HasSuffix(kLower, "pwd")
			if isSecretKey && strVal != "" && !isPlaceholder(strVal) && !isEnvVarRef(strVal) {
				errs = append(errs, fmt.Sprintf("%s.%s contains a literal credentials value: %q (use a placeholder like '<your-key>')", prefix, k, redactSecret(strVal)))
			} else if isActualSecret(strVal) {
				errs = append(errs, fmt.Sprintf("%s.%s contains a secret-looking value: %q", prefix, k, redactSecret(strVal)))
			}
		} else if nestedMap, ok := v.(map[string]any); ok {
			errs = append(errs, scanMapForCredentials(nestedMap, prefix+"."+k)...)
		} else if listVal, ok := v.([]any); ok {
			errs = append(errs, scanListForCredentials(listVal, prefix+"."+k)...)
		}
	}
	return errs
}

func scanListForCredentials(list []any, prefix string) []string {
	var errs []string
	for i, item := range list {
		itemPrefix := fmt.Sprintf("%s[%d]", prefix, i)
		if strItem, ok := item.(string); ok {
			if isActualSecret(strItem) {
				errs = append(errs, fmt.Sprintf("%s contains a secret-looking value: %q", itemPrefix, redactSecret(strItem)))
			}
		} else if nestedMap, ok := item.(map[string]any); ok {
			errs = append(errs, scanMapForCredentials(nestedMap, itemPrefix)...)
		} else if nestedList, ok := item.([]any); ok {
			errs = append(errs, scanListForCredentials(nestedList, itemPrefix)...)
		}
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
		errs = append(errs, scanSkillAPIKeys(skillPath)...)
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
			errs = append(errs, scanSkillAPIKeys(skillPath)...)
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
		if capability.Source == "" && !(isMergeCapability(capability) && capability.Content != "") {
			fmt.Fprintf(out, "FAIL  missing source: %s\n", key)
			fail = true
		}
		if capability.Source == "" {
			continue
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

func isMergeCapability(capability model.Capability) bool {
	return capability.Type == "memory" || capability.Type == "settings"
}

func capabilityAllowsInlineContent(capType string) bool {
	return capType == "memory" || capType == "settings" || capType == "command" || capType == "hook" || capType == "subagent" || capType == "prompt" || capType == "template"
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
