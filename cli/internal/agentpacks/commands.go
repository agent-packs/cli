package agentpacks

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sandeshh/agent-packs/cli/internal/registry"
	"github.com/sandeshh/agent-packs/cli/internal/util"
)

func Doctor(defaultRegistry, home string, out io.Writer) error {
	checks := []struct{ name, command string }{
		{"git", "git"}, {"go", "go"}, {"claude", "claude"}, {"codex", "codex"},
		{"gemini", "gemini"}, {"goose", "goose"}, {"opencode", "opencode"},
	}
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
	if err := os.MkdirAll(util.ExpandHome(home), 0o755); err != nil {
		fmt.Fprintf(out, "WARN  install home not writable: %s\n", home)
	} else {
		fmt.Fprintf(out, "OK    install home writable: %s\n", home)
	}
	return nil
}

func ScanSkills(root string, out io.Writer) error {
	root = util.ExpandHome(root)
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
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
