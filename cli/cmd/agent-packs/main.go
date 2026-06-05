package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sandeshh/agent-packs/cli/internal/agentpacks"
)

func main() {
	root := repoRoot()
	registry := os.Getenv("AGENT_PACKS_REGISTRY")
	if registry == "" {
		registry = filepath.Join(root, "registry", "packs")
	}
	defaultTarget := os.Getenv("AGENT_PACKS_HOME")
	if defaultTarget == "" {
		defaultTarget = ".agent-packs"
	}

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "search":
		err = runSearch(registry, os.Args[2:])
	case "explore":
		err = runSearch(registry, os.Args[2:])
	case "show":
		err = runShow(registry, os.Args[2:])
	case "install":
		err = runInstall(registry, defaultTarget, os.Args[2:])
	case "list":
		err = runList(defaultTarget, os.Args[2:])
	case "outdated":
		err = runOutdated(defaultTarget, os.Args[2:])
	case "update":
		err = runUpdate(defaultTarget, os.Args[2:])
	case "cache":
		err = runCache(defaultTarget, os.Args[2:])
	case "scan":
		err = runScan(os.Args[2:])
	case "import":
		err = runImport(defaultTarget, os.Args[2:])
	case "lint":
		err = runLint(registry, os.Args[2:])
	case "verify":
		err = runVerify(registry, os.Args[2:])
	case "uninstall":
		err = runUninstall(defaultTarget, os.Args[2:])
	case "doctor":
		err = agentpacks.Doctor(registry, defaultTarget, os.Stdout)
	case "validate":
		err = runValidate(os.Args[2:])
	case "registry":
		err = runRegistry(defaultTarget, os.Args[2:])
	case "help", "--help", "-h":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(2)
	}

	if err != nil {
		if errors.Is(err, agentpacks.ErrNotFound) {
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runSearch(registry string, args []string) error {
	return agentpacks.Search(registry, strings.Join(args, " "), os.Stdout)
}

func runShow(registry string, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: agent-packs show <pack-id>")
	}
	return agentpacks.Show(registry, args[0], os.Stdout)
}

func runInstall(registry, defaultTarget string, args []string) error {
	args = normalizeInstallArgs(args)
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	agent := flags.String("agent", "generic", "target agent: claude, codex, or generic")
	only := flags.String("only", "all", "capability filter: all, skills, or plugins")
	dryRun := flags.Bool("dry-run", false, "print installation plan without writing files")
	executePlugins := flags.Bool("execute-plugins", false, "run native plugin installation commands")
	mode := flags.String("mode", "reference", "sync mode: reference, symlink, copy, or native")
	onConflict := flags.String("on-conflict", "skip", "conflict policy: skip, overwrite, or backup")
	project := flags.String("project", "", "project directory target")
	global := flags.Bool("global", false, "install into the configured global target")
	if err := flags.Parse(args); err != nil {
		return err
	}
	remaining := flags.Args()
	if len(remaining) != 1 {
		return fmt.Errorf("usage: agent-packs install <pack-id|registry/pack-id> [--target dir] [--agent name] [--only filter] [--dry-run] [--execute-plugins]")
	}
	if !validAgent(*agent) {
		return fmt.Errorf("invalid agent %q: expected claude, codex, or generic", *agent)
	}
	if *only != "all" && *only != "skills" && *only != "plugins" {
		return fmt.Errorf("invalid --only %q: expected all, skills, or plugins", *only)
	}
	if *mode != "reference" && *mode != "symlink" && *mode != "copy" && *mode != "native" {
		return fmt.Errorf("invalid --mode %q: expected reference, symlink, copy, or native", *mode)
	}
	if *onConflict != "skip" && *onConflict != "overwrite" && *onConflict != "backup" {
		return fmt.Errorf("invalid --on-conflict %q: expected skip, overwrite, or backup", *onConflict)
	}
	installTarget := *target
	scope := "target"
	if *project != "" {
		installTarget = *project
		scope = "project"
	}
	if *global {
		installTarget = *target
		scope = "global"
	}
	return agentpacks.InstallWithOptions(registry, *target, remaining[0], installTarget, *agent, *only, *executePlugins, *dryRun, agentpacks.InstallOptions{Mode: *mode, OnConflict: *onConflict, Scope: scope}, os.Stdout)
}

func runList(defaultTarget string, args []string) error {
	flags := flag.NewFlagSet("list", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	return agentpacks.ListInstalled(*target, os.Stdout)
}

func runUninstall(defaultTarget string, args []string) error {
	flags := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	remaining := flags.Args()
	if len(remaining) != 1 {
		return fmt.Errorf("usage: agent-packs uninstall <pack-id> [--target dir]")
	}
	return agentpacks.Uninstall(*target, remaining[0], os.Stdout)
}

func runValidate(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: agent-packs validate <file-or-directory>")
	}
	return agentpacks.ValidatePath(args[0], os.Stdout)
}

func runOutdated(defaultTarget string, args []string) error {
	flags := flag.NewFlagSet("outdated", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	return agentpacks.Outdated(*target, os.Stdout)
}

func runUpdate(defaultTarget string, args []string) error {
	flags := flag.NewFlagSet("update", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	all := flags.Bool("all", true, "update all configured registries")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	return agentpacks.Update(*target, *all, os.Stdout)
}

func runCache(defaultTarget string, args []string) error {
	flags := flag.NewFlagSet("cache", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	return agentpacks.CacheInfo(*target, os.Stdout)
}

func runScan(args []string) error {
	path := "."
	if len(args) > 1 {
		return fmt.Errorf("usage: agent-packs scan [path]")
	}
	if len(args) == 1 {
		path = args[0]
	}
	return agentpacks.ScanSkills(path, os.Stdout)
}

func runImport(defaultTarget string, args []string) error {
	flags := flag.NewFlagSet("import", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	remaining := flags.Args()
	if len(remaining) != 1 {
		return fmt.Errorf("usage: agent-packs import <skills-dir> [--target dir]")
	}
	return agentpacks.ImportSkills(remaining[0], *target, os.Stdout)
}

func runLint(registry string, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: agent-packs lint <pack-id>")
	}
	return agentpacks.Lint(registry, args[0], os.Stdout)
}

func runVerify(registry string, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: agent-packs verify <pack-id>")
	}
	return agentpacks.Verify(registry, args[0], os.Stdout)
}

func runRegistry(defaultTarget string, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: agent-packs registry <add|list|remove> ...")
	}
	sub := args[0]
	flags := flag.NewFlagSet("registry "+sub, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	rest := normalizeTargetArgs(args[1:])
	if err := flags.Parse(rest); err != nil {
		return err
	}
	remaining := flags.Args()
	switch sub {
	case "add":
		if len(remaining) != 2 {
			return fmt.Errorf("usage: agent-packs registry add <name> <source> [--target dir]")
		}
		return agentpacks.RegistryAdd(*target, remaining[0], remaining[1])
	case "list":
		if len(remaining) != 0 {
			return fmt.Errorf("usage: agent-packs registry list [--target dir]")
		}
		return agentpacks.RegistryList(*target, os.Stdout)
	case "remove":
		if len(remaining) != 1 {
			return fmt.Errorf("usage: agent-packs registry remove <name> [--target dir]")
		}
		return agentpacks.RegistryRemove(*target, remaining[0])
	default:
		return fmt.Errorf("unknown registry command: %s", sub)
	}
}

func normalizeInstallArgs(args []string) []string {
	flags := []string{}
	positionals := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--dry-run" || arg == "--execute-plugins" || arg == "--global" {
			flags = append(flags, arg)
			continue
		}
		if arg == "--target" || arg == "--agent" || arg == "--only" || arg == "--mode" || arg == "--on-conflict" || arg == "--project" {
			flags = append(flags, arg)
			if i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "--target=") || strings.HasPrefix(arg, "--agent=") || strings.HasPrefix(arg, "--only=") || strings.HasPrefix(arg, "--mode=") || strings.HasPrefix(arg, "--on-conflict=") || strings.HasPrefix(arg, "--project=") {
			flags = append(flags, arg)
			continue
		}
		positionals = append(positionals, arg)
	}
	return append(flags, positionals...)
}

func normalizeTargetArgs(args []string) []string {
	flags := []string{}
	positionals := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--target" {
			flags = append(flags, arg)
			if i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "--target=") {
			flags = append(flags, arg)
			continue
		}
		positionals = append(positionals, arg)
	}
	return append(flags, positionals...)
}

func validAgent(agent string) bool {
	_, ok := agentpacks.SkillTargets[agent]
	return ok
}

func repoRoot() string {
	executable, err := os.Executable()
	if err != nil {
		return "."
	}
	realPath, err := filepath.EvalSymlinks(executable)
	if err != nil {
		realPath = executable
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(realPath)))
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  agent-packs search [query]")
	fmt.Fprintln(os.Stderr, "  agent-packs show <pack-id>")
	fmt.Fprintln(os.Stderr, "  agent-packs install <pack-id|registry/pack-id> [--target dir] [--agent claude|codex|generic] [--only all|skills|plugins] [--mode reference|symlink|copy|native] [--on-conflict skip|overwrite|backup] [--project dir|--global] [--dry-run] [--execute-plugins]")
	fmt.Fprintln(os.Stderr, "  agent-packs list [--target dir]")
	fmt.Fprintln(os.Stderr, "  agent-packs update|outdated|cache ...")
	fmt.Fprintln(os.Stderr, "  agent-packs scan [path]")
	fmt.Fprintln(os.Stderr, "  agent-packs import <skills-dir> [--target dir]")
	fmt.Fprintln(os.Stderr, "  agent-packs lint|verify <pack-id>")
	fmt.Fprintln(os.Stderr, "  agent-packs uninstall <pack-id> [--target dir]")
	fmt.Fprintln(os.Stderr, "  agent-packs doctor")
	fmt.Fprintln(os.Stderr, "  agent-packs validate <file-or-directory>")
	fmt.Fprintln(os.Stderr, "  agent-packs registry add|list|remove ...")
}
