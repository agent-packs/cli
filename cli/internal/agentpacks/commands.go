package agentpacks

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/agent-packs/cli/internal/config"
	"github.com/agent-packs/cli/internal/install"
	"github.com/agent-packs/cli/internal/model"
	"github.com/agent-packs/cli/internal/registry"
	"github.com/agent-packs/cli/internal/util"
	"gopkg.in/yaml.v3"
)

func BuildDoctorReport(defaultRegistry, home string) model.DoctorReport {
	report := model.DoctorReport{OK: true}
	add := func(name, status, detail string) {
		if status == "warn" {
			report.OK = false
		}
		report.Checks = append(report.Checks, model.DoctorCheck{Name: name, Status: status, Detail: detail})
	}

	binaries := []struct{ name, cmd string }{
		{"git", "git"}, {"go", "go"},
		{"claude", "claude"}, {"codex", "codex"}, {"cursor", "cursor"},
		{"gemini", "gemini"}, {"goose", "goose"}, {"opencode", "opencode"},
		{"gh", "gh"},
	}
	for _, b := range binaries {
		if _, err := exec.LookPath(b.cmd); err != nil {
			add(b.name+" binary", "warn", b.cmd+" not found in PATH")
		} else {
			add(b.name+" binary", "ok", "")
		}
	}

	if _, err := os.Stat(defaultRegistry); err != nil {
		add("registry", "warn", "unavailable: "+defaultRegistry)
	} else {
		add("registry", "ok", defaultRegistry)
	}

	if err := os.MkdirAll(util.ExpandHome(home), 0o755); err != nil {
		add("install home", "warn", "not writable: "+home)
	} else {
		add("install home", "ok", home)
	}

	summaries, err := install.ListInstalledReceipts(home)
	if err == nil {
		add("installed packs", "info", fmt.Sprintf("%d pack(s) in %s", len(summaries), home))
	}

	indexCheck := indexFreshnessCheck(defaultRegistry)
	add("registry index", indexCheck.status, indexCheck.detail)

	return report
}

type freshnessResult struct{ status, detail string }

func indexFreshnessCheck(registryDir string) freshnessResult {
	indexPath := filepath.Join(filepath.Dir(registryDir), "index.json")
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		return freshnessResult{"warn", "index not found: " + indexPath}
	}
	var idx model.RegistryIndex
	if err := json.Unmarshal(indexData, &idx); err != nil || idx.GeneratedAt == "" {
		return freshnessResult{"warn", "index unreadable"}
	}
	indexTime, err := time.Parse(time.RFC3339, idx.GeneratedAt)
	if err != nil {
		return freshnessResult{"warn", "index has invalid generatedAt"}
	}
	entries, _ := os.ReadDir(registryDir)
	var latestPack time.Time
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		info, err := entry.Info()
		if err == nil && info.ModTime().After(latestPack) {
			latestPack = info.ModTime()
		}
	}
	if !latestPack.IsZero() && latestPack.After(indexTime) {
		return freshnessResult{"warn", "index is stale (run: agent-packs index --output registry/index.json)"}
	}
	return freshnessResult{"ok", ""}
}

func Doctor(defaultRegistry, home string, out io.Writer) error {
	report := BuildDoctorReport(defaultRegistry, home)
	for _, c := range report.Checks {
		switch c.Status {
		case "ok":
			if c.Detail != "" {
				fmt.Fprintf(out, "OK    %s: %s\n", c.Name, c.Detail)
			} else {
				fmt.Fprintf(out, "OK    %s\n", c.Name)
			}
		case "warn":
			fmt.Fprintf(out, "WARN  %s: %s\n", c.Name, c.Detail)
		case "info":
			fmt.Fprintf(out, "INFO  %s: %s\n", c.Name, c.Detail)
		}
	}
	return nil
}

func DoctorJSON(defaultRegistry, home string, out io.Writer) error {
	report := BuildDoctorReport(defaultRegistry, home)
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func Why(target, name string, out io.Writer) error {
	absTarget, err := filepath.Abs(util.ExpandHome(target))
	if err != nil {
		return err
	}
	receiptsDir := filepath.Join(absTarget, "receipts")
	entries, err := os.ReadDir(receiptsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(out, "No packs installed.")
			return nil
		}
		return err
	}

	query := strings.ToLower(name)
	found := false
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		receipt, err := install.LoadReceipt(filepath.Join(receiptsDir, entry.Name()))
		if err != nil {
			continue
		}
		for _, item := range receipt.Plan.Capabilities {
			lname := strings.ToLower(item.Name)
			lbase := strings.ToLower(filepath.Base(item.Source))
			if lname == query || lbase == query || strings.Contains(lname, query) {
				fmt.Fprintf(out, "pack:   %s\n", receipt.Plan.Pack)
				fmt.Fprintf(out, "name:   %s\n", item.Name)
				fmt.Fprintf(out, "source: %s\n", item.Source)
				if item.Destination != "" {
					fmt.Fprintf(out, "dest:   %s\n", item.Destination)
				}
				fmt.Fprintln(out)
				found = true
			}
		}
	}
	if !found {
		fmt.Fprintf(out, "No installed pack provides %q\n", name)
	}
	return nil
}

func Sync(registryPath, home, projectDir, target, agent, mode string, dryRun bool, out io.Writer) error {
	cfg, err := config.LoadProjectConfig(projectDir)
	if err != nil {
		return fmt.Errorf("no .agent-packs.yaml in %s (run agent-packs init first): %w", projectDir, err)
	}
	if len(cfg.Packs) == 0 {
		fmt.Fprintln(out, "No packs configured in .agent-packs.yaml (add a 'packs' list)")
		return nil
	}

	summaries, _ := install.ListInstalledReceipts(target)
	installedIDs := make(map[string]bool, len(summaries))
	for _, s := range summaries {
		installedIDs[s.ID] = true
	}

	options := model.InstallOptions{Mode: mode, OnConflict: "skip"}
	for _, packID := range cfg.Packs {
		if installedIDs[packID] && !dryRun {
			fmt.Fprintf(out, "already installed: %s\n", packID)
			continue
		}
		if dryRun {
			fmt.Fprintf(out, "would install: %s\n", packID)
		} else {
			fmt.Fprintf(out, "installing: %s\n", packID)
		}
		if err := install.InstallWithOptions(registryPath, home, packID, target, agent, "all", false, dryRun, options, out); err != nil {
			return fmt.Errorf("failed to install %s: %w", packID, err)
		}
	}
	return nil
}

func Freeze(target, projectDir string, out io.Writer) error {
	summaries, err := install.ListInstalledReceipts(target)
	if err != nil {
		return err
	}
	cfg, _ := config.LoadProjectConfig(projectDir)
	cfg.Packs = make([]string, 0, len(summaries))
	for _, s := range summaries {
		cfg.Packs = append(cfg.Packs, s.ID)
	}
	sort.Strings(cfg.Packs)
	if err := config.SaveProjectConfig(projectDir, cfg); err != nil {
		return err
	}
	fmt.Fprintf(out, "Wrote %d pack(s) to %s/.agent-packs.yaml\n", len(cfg.Packs), projectDir)
	return nil
}

func ExportPacks(target string, out io.Writer) error {
	summaries, err := install.ListInstalledReceipts(target)
	if err != nil {
		return err
	}
	packs := make([]string, 0, len(summaries))
	for _, s := range summaries {
		packs = append(packs, s.ID)
	}
	sort.Strings(packs)
	type exportFile struct {
		Packs []string `yaml:"packs"`
	}
	enc := yaml.NewEncoder(out)
	return enc.Encode(exportFile{Packs: packs})
}

// backupDirPattern matches directories created by `--on-conflict backup`,
// named <dest>.bak.<14-digit-timestamp>.
var backupDirPattern = regexp.MustCompile(`\.bak\.\d{14}$`)

func ScanSkills(root string, out io.Writer) error {
	root = util.ExpandHome(root)
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip backup directories left by `--on-conflict backup`
		// (named <dest>.bak.<timestamp>) so they aren't reported as live skills.
		if d.IsDir() && backupDirPattern.MatchString(d.Name()) {
			return filepath.SkipDir
		}
		if !d.IsDir() && filepath.Base(path) == "SKILL.md" {
			manifest, err := registry.LoadSkillManifest(path)
			if err != nil {
				fmt.Fprintf(out, "WARN  %s: %s\n", path, err)
				return nil
			}
			fmt.Fprintf(out, "%s\t%s\n", manifest.Name, path)
		}
		return nil
	})
}

func ImportSkills(sourceDir, target string, out io.Writer) error {
	importDir := filepath.Join(util.ExpandHome(target), "sources", "imported")
	if err := os.MkdirAll(importDir, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(util.ExpandHome(sourceDir), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Base(path) != "SKILL.md" {
			return nil
		}
		manifest, err := registry.LoadSkillManifest(path)
		if err != nil {
			return nil
		}
		dest := filepath.Join(importDir, util.Slugify(manifest.Name))
		if err := os.RemoveAll(dest); err != nil {
			return err
		}
		if err := util.CopyDir(filepath.Dir(path), dest); err != nil {
			return err
		}
		fmt.Fprintf(out, "imported\t%s\t%s\n", manifest.Name, dest)
		return nil
	})
}
