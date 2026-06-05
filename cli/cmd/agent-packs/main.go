package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sandeshh/agent-packs/cli/internal/agentpacks"
	"github.com/sandeshh/agent-packs/cli/internal/output"
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
		err = runOutdated(registry, defaultTarget, os.Args[2:])
	case "upgrade":
		err = runUpgrade(registry, defaultTarget, os.Args[2:])
	case "rollback":
		err = runRollback(defaultTarget, os.Args[2:])
	case "audit":
		err = runAudit(registry, os.Args[2:])
	case "update":
		err = runUpdate(defaultTarget, os.Args[2:])
	case "cache":
		err = runCache(defaultTarget, os.Args[2:])
	case "policy":
		err = runPolicy(registry, os.Args[2:])
	case "licenses":
		err = runLicenses(registry, os.Args[2:])
	case "attribution":
		err = runAttribution(registry, os.Args[2:])
	case "index":
		err = runIndex(registry, os.Args[2:])
	case "tree", "deps":
		err = runTree(registry, os.Args[2:])
	case "diff":
		err = runDiff(registry, defaultTarget, os.Args[2:])
	case "compat":
		err = runCompat(registry, os.Args[2:])
	case "scan":
		err = runScan(os.Args[2:])
	case "import":
		err = runImport(defaultTarget, os.Args[2:])
	case "lint":
		err = runLint(registry, os.Args[2:])
	case "verify":
		err = runVerify(registry, os.Args[2:])
	case "resolve":
		err = runResolve(registry, os.Args[2:])
	case "uninstall":
		err = runUninstall(defaultTarget, os.Args[2:])
	case "doctor":
		err = runDoctor(registry, defaultTarget, os.Args[2:])
	case "validate":
		err = runValidate(os.Args[2:])
	case "registry":
		err = runRegistry(defaultTarget, os.Args[2:])
	case "version":
		err = runVersion(os.Args[2:])
	case "init":
		err = runInit(os.Args[2:])
	case "new":
		err = runNew(os.Args[2:])
	case "publish":
		err = runPublish(registry, os.Args[2:])
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

func extractJSONFlag(args []string) (bool, []string) {
	asJSON := false
	remaining := []string{}
	for _, arg := range args {
		if arg == "--json" {
			asJSON = true
			continue
		}
		remaining = append(remaining, arg)
	}
	return asJSON, remaining
}

func runSearch(registry string, args []string) error {
	asJSON, args := extractJSONFlag(args)
	query := strings.Join(args, " ")
	if asJSON {
		matches, err := agentpacks.MatchPacks(registry, query)
		if err != nil {
			return err
		}
		if len(matches) == 0 {
			return agentpacks.ErrNotFound
		}
		return output.Encode(os.Stdout, matches)
	}
	return agentpacks.Search(registry, query, os.Stdout)
}

func runShow(registry string, args []string) error {
	asJSON, args := extractJSONFlag(args)
	if len(args) != 1 {
		return fmt.Errorf("usage: agent-packs show <pack-id> [--json]")
	}
	if asJSON {
		pack, err := agentpacks.FindPack(registry, args[0])
		if err != nil {
			return err
		}
		return output.Encode(os.Stdout, pack)
	}
	return agentpacks.Show(registry, args[0], os.Stdout)
}

func runInstall(registry, defaultTarget string, args []string) error {
	args = normalizeInstallArgs(args)
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	agent := flags.String("agent", "generic", "target agent/tool")
	targetTool := flags.String("target-tool", "", "target tool alias for --agent")
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
	if *targetTool != "" {
		*agent = *targetTool
	}
	*agent = agentpacks.NormalizeAgent(*agent)
	if !agentpacks.ValidAgent(*agent) {
		return fmt.Errorf("invalid agent %q: run `agent-packs doctor targets` for supported tools", *agent)
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
	asJSON, args := extractJSONFlag(normalizeTargetArgs(args))
	flags := flag.NewFlagSet("list", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if asJSON {
		receipts, err := agentpacks.ListInstalledReceipts(*target)
		if err != nil {
			return err
		}
		return output.Encode(os.Stdout, receipts)
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

func runDoctor(registry, defaultTarget string, args []string) error {
	if len(args) == 1 && args[0] == "targets" {
		return agentpacks.PrintTargetMatrix(os.Stdout)
	}
	if len(args) != 0 {
		return fmt.Errorf("usage: agent-packs doctor [targets]")
	}
	return agentpacks.Doctor(registry, defaultTarget, os.Stdout)
}

func runValidate(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: agent-packs validate <file-or-directory>")
	}
	return agentpacks.ValidatePath(args[0], os.Stdout)
}

func runOutdated(registry, defaultTarget string, args []string) error {
	asJSON, args := extractJSONFlag(normalizeTargetArgs(args))
	flags := flag.NewFlagSet("outdated", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if asJSON {
		report, err := agentpacks.GetOutdatedReport(registry, *target)
		if err != nil {
			return err
		}
		return output.Encode(os.Stdout, report)
	}
	return agentpacks.Outdated(registry, *target, os.Stdout)
}

func runUpgrade(registry, defaultTarget string, args []string) error {
	flags := flag.NewFlagSet("upgrade", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	executePlugins := flags.Bool("execute-plugins", false, "run native plugin installation commands")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	remaining := flags.Args()
	if len(remaining) != 1 {
		return fmt.Errorf("usage: agent-packs upgrade <pack-id> [--target dir] [--execute-plugins]")
	}
	return agentpacks.Upgrade(registry, *target, remaining[0], *target, *executePlugins, os.Stdout)
}

func runRollback(defaultTarget string, args []string) error {
	flags := flag.NewFlagSet("rollback", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	remaining := flags.Args()
	if len(remaining) != 1 {
		return fmt.Errorf("usage: agent-packs rollback <pack-id> [--target dir]")
	}
	return agentpacks.Rollback(*target, remaining[0], os.Stdout)
}

func runAudit(registry string, args []string) error {
	asJSON, args := extractJSONFlag(args)
	if len(args) != 1 {
		return fmt.Errorf("usage: agent-packs audit <pack-id> [--json]")
	}
	if asJSON {
		return agentpacks.AuditJSON(registry, args[0], os.Stdout)
	}
	return agentpacks.Audit(registry, args[0], os.Stdout)
}

func runVersion(args []string) error {
	asJSON, _ := extractJSONFlag(args)
	if asJSON {
		return output.Encode(os.Stdout, map[string]string{"version": agentpacks.VersionString()})
	}
	fmt.Println(agentpacks.VersionString())
	return nil
}

func runInit(args []string) error {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	agent := flags.String("agent", "codex", "default target agent")
	mode := flags.String("mode", "reference", "default sync mode")
	onConflict := flags.String("on-conflict", "skip", "default conflict policy")
	scope := flags.String("scope", "project", "default install scope")
	registryPath := flags.String("registry", "", "default registry path")
	target := flags.String("target", ".agent-packs", "default install target")
	force := flags.Bool("force", false, "overwrite existing config")
	if err := flags.Parse(args); err != nil {
		return err
	}
	projectDir := "."
	if len(flags.Args()) > 0 {
		projectDir = flags.Args()[0]
	}
	path, err := agentpacks.InitProject(projectDir, agentpacks.InitOptions{
		Agent: *agent, Mode: *mode, OnConflict: *onConflict, Scope: *scope,
		Registry: *registryPath, Target: *target, Force: *force,
	})
	if err != nil {
		return err
	}
	fmt.Printf("Wrote %s\n", path)
	return nil
}

func runNew(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: agent-packs new <pack|skill|plugin> <id> [--name name] [--dir dir] [--force]")
	}
	kind := args[0]
	flags := flag.NewFlagSet("new "+kind, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	name := flags.String("name", "", "display name")
	dir := flags.String("dir", ".", "output directory")
	force := flags.Bool("force", false, "overwrite existing files")
	if err := flags.Parse(normalizeNewArgs(args[1:])); err != nil {
		return err
	}
	remaining := flags.Args()
	if len(remaining) != 1 {
		return fmt.Errorf("usage: agent-packs new <pack|skill|plugin> <id> [--name name] [--dir dir] [--force]")
	}
	path, err := agentpacks.New(agentpacks.NewOptions{Kind: kind, ID: remaining[0], Name: *name, Dir: *dir, Force: *force})
	if err != nil {
		return err
	}
	fmt.Printf("Wrote %s\n", path)
	return nil
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
	if len(args) > 0 && (args[0] == "prune" || args[0] == "clean") {
		flags := flag.NewFlagSet("cache "+args[0], flag.ContinueOnError)
		flags.SetOutput(os.Stderr)
		target := flags.String("target", defaultTarget, "installation target directory")
		if err := flags.Parse(normalizeTargetArgs(args[1:])); err != nil {
			return err
		}
		return agentpacks.CachePrune(*target, args[0] == "clean", os.Stdout)
	}
	flags := flag.NewFlagSet("cache", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	return agentpacks.CacheInfo(*target, os.Stdout)
}

func runPolicy(registry string, args []string) error {
	if len(args) != 3 || args[0] != "check" {
		return fmt.Errorf("usage: agent-packs policy check <pack-id> <policy.json>")
	}
	return agentpacks.PolicyCheck(registry, args[1], args[2], os.Stdout)
}

func runLicenses(registry string, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: agent-packs licenses <pack-id>")
	}
	return agentpacks.Licenses(registry, args[0], os.Stdout)
}

func runAttribution(registry string, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: agent-packs attribution <pack-id>")
	}
	return agentpacks.Attribution(registry, args[0], os.Stdout)
}

func runIndex(registry string, args []string) error {
	flags := flag.NewFlagSet("index", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	output := flags.String("output", "", "output index path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if len(flags.Args()) != 0 {
		return fmt.Errorf("usage: agent-packs index [--output path]")
	}
	return agentpacks.GenerateIndex(registry, *output, os.Stdout)
}

func runTree(registry string, args []string) error {
	asJSON, args := extractJSONFlag(args)
	if len(args) != 1 {
		return fmt.Errorf("usage: agent-packs tree <pack-id> [--json]")
	}
	tree, err := agentpacks.DependencyTreeForPack(registry, args[0])
	if err != nil {
		return err
	}
	if asJSON {
		return output.Encode(os.Stdout, tree)
	}
	printDependencyTree(tree)
	return nil
}

func printDependencyTree(tree agentpacks.DependencyTree) {
	fmt.Printf("%s@%s\n", tree.Pack, tree.Version)
	for i, node := range tree.Dependencies {
		printDependencyNode(node, "", i == len(tree.Dependencies)-1)
	}
}

func printDependencyNode(node agentpacks.DependencyNode, prefix string, last bool) {
	branch := "+- "
	nextPrefix := prefix + "|  "
	if last {
		branch = "`- "
		nextPrefix = prefix + "   "
	}
	label := node.Type + ":" + node.Name
	if node.ID != "" {
		label += " (" + node.ID + ")"
	}
	if node.Trust != "" {
		label += " [" + node.Trust + "]"
	}
	fmt.Println(prefix + branch + label)
	if node.Source != "" {
		fmt.Println(nextPrefix + "source: " + node.Source)
	}
	for i, child := range node.Dependencies {
		printDependencyNode(child, nextPrefix, i == len(node.Dependencies)-1)
	}
}

func runPublish(registry string, args []string) error {
	asJSON, args := extractJSONFlag(args)
	flags := flag.NewFlagSet("publish", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	check := flags.Bool("check", false, "run contributor publish checks")
	policyPath := flags.String("policy", filepath.Join(filepath.Dir(registry), "policy", "default.json"), "policy file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if !*check || len(flags.Args()) != 0 {
		return fmt.Errorf("usage: agent-packs publish --check [--policy file] [--json]")
	}
	if asJSON {
		report, err := agentpacks.PublishReportForRegistry(registry, *policyPath)
		if err != nil {
			return err
		}
		return output.Encode(os.Stdout, report)
	}
	return agentpacks.PublishCheck(registry, *policyPath, os.Stdout)
}

func runDiff(registry, defaultTarget string, args []string) error {
	flags := flag.NewFlagSet("diff", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	remaining := flags.Args()
	if len(remaining) != 1 {
		return fmt.Errorf("usage: agent-packs diff <pack-id> [--target dir]")
	}
	return agentpacks.PackDiff(registry, *target, remaining[0], os.Stdout)
}

func runCompat(registry string, args []string) error {
	asJSON, args := extractJSONFlag(normalizeAgentArgs(args))
	flags := flag.NewFlagSet("compat", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	agent := flags.String("agent", "generic", "target agent/tool")
	if err := flags.Parse(args); err != nil {
		return err
	}
	remaining := flags.Args()
	if len(remaining) != 1 {
		return fmt.Errorf("usage: agent-packs compat <pack-id> [--agent tool] [--json]")
	}
	normalized := agentpacks.NormalizeAgent(*agent)
	if asJSON {
		result, err := agentpacks.CompatibilityReport(registry, remaining[0], normalized)
		if err != nil {
			return err
		}
		return output.Encode(os.Stdout, result)
	}
	return agentpacks.Compatibility(registry, remaining[0], normalized, os.Stdout)
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

func runResolve(registry string, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: agent-packs resolve <pack-id>")
	}
	return agentpacks.ResolveSources(registry, args[0], os.Stdout)
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
		if arg == "--target" || arg == "--agent" || arg == "--target-tool" || arg == "--only" || arg == "--mode" || arg == "--on-conflict" || arg == "--project" {
			flags = append(flags, arg)
			if i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "--target=") || strings.HasPrefix(arg, "--agent=") || strings.HasPrefix(arg, "--target-tool=") || strings.HasPrefix(arg, "--only=") || strings.HasPrefix(arg, "--mode=") || strings.HasPrefix(arg, "--on-conflict=") || strings.HasPrefix(arg, "--project=") {
			flags = append(flags, arg)
			continue
		}
		positionals = append(positionals, arg)
	}
	return append(flags, positionals...)
}

func normalizeAgentArgs(args []string) []string {
	flags := []string{}
	positionals := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--agent" {
			flags = append(flags, arg)
			if i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "--agent=") {
			flags = append(flags, arg)
			continue
		}
		positionals = append(positionals, arg)
	}
	return append(flags, positionals...)
}

func normalizeNewArgs(args []string) []string {
	flags := []string{}
	positionals := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--force" {
			flags = append(flags, arg)
			continue
		}
		if arg == "--name" || arg == "--dir" {
			flags = append(flags, arg)
			if i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "--name=") || strings.HasPrefix(arg, "--dir=") {
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
	fmt.Fprintln(os.Stderr, "  agent-packs install <pack-id|registry/pack-id> [--target dir] [--agent tool|--target-tool tool] [--only all|skills|plugins] [--mode reference|symlink|copy|native] [--on-conflict skip|overwrite|backup] [--project dir|--global] [--dry-run] [--execute-plugins]")
	fmt.Fprintln(os.Stderr, "  agent-packs list [--target dir]")
	fmt.Fprintln(os.Stderr, "  agent-packs update|outdated|upgrade|cache ...")
	fmt.Fprintln(os.Stderr, "  agent-packs upgrade <pack-id> [--target dir] [--execute-plugins]")
	fmt.Fprintln(os.Stderr, "  agent-packs rollback <pack-id> [--target dir]")
	fmt.Fprintln(os.Stderr, "  agent-packs version [--json]")
	fmt.Fprintln(os.Stderr, "  agent-packs init [dir] [--agent tool] [--mode reference|symlink|copy|native]")
	fmt.Fprintln(os.Stderr, "  agent-packs new <pack|skill|plugin> <id> [--name name] [--dir dir] [--force]")
	fmt.Fprintln(os.Stderr, "  agent-packs audit <pack-id> [--json]")
	fmt.Fprintln(os.Stderr, "  agent-packs tree|deps <pack-id> [--json]")
	fmt.Fprintln(os.Stderr, "  agent-packs publish --check [--policy file] [--json]")
	fmt.Fprintln(os.Stderr, "  agent-packs policy check <pack-id> <policy.json>")
	fmt.Fprintln(os.Stderr, "  agent-packs licenses|attribution|resolve <pack-id>")
	fmt.Fprintln(os.Stderr, "  agent-packs index [--output path]")
	fmt.Fprintln(os.Stderr, "  agent-packs diff <pack-id> [--target dir]")
	fmt.Fprintln(os.Stderr, "  agent-packs compat <pack-id> [--agent tool]")
	fmt.Fprintln(os.Stderr, "  agent-packs scan [path]")
	fmt.Fprintln(os.Stderr, "  agent-packs import <skills-dir> [--target dir]")
	fmt.Fprintln(os.Stderr, "  agent-packs lint|verify|resolve <pack-id>")
	fmt.Fprintln(os.Stderr, "  agent-packs uninstall <pack-id> [--target dir]")
	fmt.Fprintln(os.Stderr, "  agent-packs doctor [targets]")
	fmt.Fprintln(os.Stderr, "  agent-packs validate <file-or-directory>")
	fmt.Fprintln(os.Stderr, "  agent-packs registry add|list|remove ...")
}
