package agentpacks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	Capabilities []Capability `json:"capabilities"`
	Path         string       `json:"-"`
}

type Capability struct {
	Type       string            `json:"type"`
	Name       string            `json:"name"`
	Source     string            `json:"source"`
	Format     string            `json:"format,omitempty"`
	Version    string            `json:"version,omitempty"`
	Entry      string            `json:"entry,omitempty"`
	Homepage   string            `json:"homepage,omitempty"`
	Repository string            `json:"repository,omitempty"`
	License    string            `json:"license,omitempty"`
	Install    map[string]string `json:"install,omitempty"`
	Targets    []string          `json:"targets,omitempty"`
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

type Config struct {
	Root     string
	Registry string
}

var SkillTargets = map[string]string{
	"claude":  ".claude/skills",
	"codex":   ".codex/skills",
	"generic": "skills",
}

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
		path := filepath.Join(registry, entry.Name())
		pack, err := LoadPack(path)
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

func Install(registry, packID, target, agent, only string, executePlugins, dryRun bool, out io.Writer) error {
	pack, err := FindPack(registry, packID)
	if err != nil {
		return err
	}
	absTarget, err := filepath.Abs(expandHome(target))
	if err != nil {
		return err
	}
	plan := BuildInstallPlan(pack, absTarget, agent, only)
	if dryRun {
		PrintPlan(plan, out)
		return nil
	}
	if err := os.MkdirAll(absTarget, 0o755); err != nil {
		return err
	}
	packDir := filepath.Join(absTarget, "packs", pack.ID)
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(packDir, "agent-pack.json"), pack); err != nil {
		return err
	}
	if err := copyFile(pack.Path, filepath.Join(packDir, "source-registry-entry.json")); err != nil {
		return err
	}

	result := ExecutePlan(plan, executePlugins)
	receiptPath, err := WriteReceipt(absTarget, pack, result)
	if err != nil {
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

func ExecutePlan(plan Plan, executePlugins bool) Plan {
	results := make([]PlanItem, 0, len(plan.Capabilities))
	for _, item := range plan.Capabilities {
		switch item.Type {
		case "skill":
			results = append(results, installSkill(item))
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

var (
	ErrNotFound      = errors.New("not found")
	ErrInstallFailed = errors.New("install failed")
)

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

func installSkill(item PlanItem) PlanItem {
	source, err := filepath.Abs(expandHome(item.Source))
	if err != nil {
		item.Status = "failed"
		item.Reason = err.Error()
		return item
	}
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
	var stdout bytes.Buffer
	var stderr bytes.Buffer
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

func expandHome(path string) string {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
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
