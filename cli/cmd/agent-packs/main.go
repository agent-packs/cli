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
	case "status":
		err = runStatus(defaultTarget, os.Args[2:])
	case "completion":
		err = runCompletion(os.Args[2:])
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
	if len(remaining) < 1 {
		return fmt.Errorf("usage: agent-packs install <pack-id|registry/pack-id>... [--target dir] [--agent name] [--only filter] [--dry-run] [--execute-plugins]")
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
	options := agentpacks.InstallOptions{Mode: *mode, OnConflict: *onConflict, Scope: scope}
	for index, packRef := range remaining {
		printLifecycleHeader("Installing", packRef, index, len(remaining))
		if err := agentpacks.InstallWithOptions(registry, *target, packRef, installTarget, *agent, *only, *executePlugins, *dryRun, options, os.Stdout); err != nil {
			return err
		}
	}
	return nil
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
	if len(remaining) < 1 {
		return fmt.Errorf("usage: agent-packs uninstall <pack-id>... [--target dir]")
	}
	for index, packRef := range remaining {
		printLifecycleHeader("Uninstalling", packRef, index, len(remaining))
		if err := agentpacks.Uninstall(*target, packRef, os.Stdout); err != nil {
			return err
		}
	}
	return nil
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
	if len(remaining) < 1 {
		return fmt.Errorf("usage: agent-packs upgrade <pack-id>... [--target dir] [--execute-plugins]")
	}
	for index, packRef := range remaining {
		printLifecycleHeader("Upgrading", packRef, index, len(remaining))
		if err := agentpacks.Upgrade(registry, *target, packRef, *target, *executePlugins, os.Stdout); err != nil {
			return err
		}
	}
	return nil
}

func runRollback(defaultTarget string, args []string) error {
	flags := flag.NewFlagSet("rollback", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	remaining := flags.Args()
	if len(remaining) < 1 {
		return fmt.Errorf("usage: agent-packs rollback <pack-id>... [--target dir]")
	}
	for index, packRef := range remaining {
		printLifecycleHeader("Rolling back", packRef, index, len(remaining))
		if err := agentpacks.Rollback(*target, packRef, os.Stdout); err != nil {
			return err
		}
	}
	return nil
}

func printLifecycleHeader(action, packRef string, index, total int) {
	if total <= 1 {
		return
	}
	if index > 0 {
		fmt.Println()
	}
	fmt.Printf("==> %s %s (%d/%d)\n", action, packRef, index+1, total)
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
		if arg == "--target" || arg == "--agent" || arg == "--target-tool" || arg == "--only" || arg == "--mode" || arg == "--on-conflict" || arg == "--project" || arg == "--scope" {
			flags = append(flags, arg)
			if i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "--target=") || strings.HasPrefix(arg, "--agent=") || strings.HasPrefix(arg, "--target-tool=") || strings.HasPrefix(arg, "--only=") || strings.HasPrefix(arg, "--mode=") || strings.HasPrefix(arg, "--on-conflict=") || strings.HasPrefix(arg, "--project=") || strings.HasPrefix(arg, "--scope=") {
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

func runStatus(defaultTarget string, args []string) error {
	asJSON, args := extractJSONFlag(normalizeTargetArgs(args))
	flags := flag.NewFlagSet("status", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	_ = asJSON
	return agentpacks.DriftCheck(*target, os.Stdout)
}

func runCompletion(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: agent-packs completion <bash|zsh|fish>")
	}
	switch args[0] {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	case "fish":
		fmt.Print(fishCompletion)
	default:
		return fmt.Errorf("unsupported shell %q: expected bash, zsh, or fish", args[0])
	}
	return nil
}

const bashCompletion = `# agent-packs bash completion
# Source this file or add to ~/.bash_completion.d/
_agent_packs() {
    local cur prev words cword
    _init_completion 2>/dev/null || {
        COMPREPLY=()
        cur="${COMP_WORDS[COMP_CWORD]}"
        prev="${COMP_WORDS[COMP_CWORD-1]}"
        words=("${COMP_WORDS[@]}")
        cword=$COMP_CWORD
    }

    local subcommands="search show install list outdated upgrade rollback uninstall status audit verify lint diff tree deps compat scan import validate index registry doctor new init publish policy licenses attribution resolve version completion help"

    if [[ $cword -eq 1 ]]; then
        COMPREPLY=($(compgen -W "$subcommands" -- "$cur"))
        return
    fi

    case "$prev" in
        --agent|--target-tool)
            COMPREPLY=($(compgen -W "claude codex cursor gemini copilot opencode goose" -- "$cur"))
            return ;;
        --mode)
            COMPREPLY=($(compgen -W "reference symlink copy native" -- "$cur"))
            return ;;
        --on-conflict)
            COMPREPLY=($(compgen -W "skip overwrite backup" -- "$cur"))
            return ;;
        --only)
            COMPREPLY=($(compgen -W "all skills plugins" -- "$cur"))
            return ;;
    esac

    case "${words[1]}" in
        install|show|audit|verify|lint|upgrade|rollback|uninstall|diff|deps|tree|compat|licenses|attribution|resolve)
            local packs
            packs=$(agent-packs search 2>/dev/null | awk '{print $1}')
            COMPREPLY=($(compgen -W "$packs" -- "$cur"))
            ;;
        completion)
            COMPREPLY=($(compgen -W "bash zsh fish" -- "$cur"))
            ;;
        policy)
            COMPREPLY=($(compgen -W "check" -- "$cur"))
            ;;
    esac
}
complete -F _agent_packs agent-packs
`

const zshCompletion = `#compdef agent-packs
# agent-packs zsh completion
# Place in a directory on your $fpath, e.g. ~/.zsh/completions/_agent-packs

_agent_packs() {
    local state line
    typeset -A opt_args

    _arguments -C \
        '1: :->command' \
        '*: :->args' && return 0

    case $state in
        command)
            local -a commands
            commands=(
                'search:search the registry for packs'
                'show:show details of a pack'
                'install:install a pack into an agent tool'
                'list:list installed packs'
                'outdated:check for available updates'
                'upgrade:upgrade an installed pack'
                'rollback:roll back a pack to a previous install'
                'uninstall:remove an installed pack'
                'status:check installed skills for drift or tampering'
                'audit:generate a supply-chain SBOM for a pack'
                'verify:verify pack source references'
                'lint:lint a pack manifest'
                'diff:diff an installed pack against the registry'
                'tree:show pack dependency tree'
                'compat:check pack compatibility with an agent'
                'validate:validate manifests against schema'
                'index:regenerate the registry index'
                'registry:manage remote registries'
                'doctor:diagnose installation environment'
                'new:scaffold a new pack, skill, or plugin'
                'init:create a project .agent-packs.yaml config'
                'publish:check registry packs for publish readiness'
                'policy:check packs against a trust policy'
                'licenses:show licenses for a pack'\''s capabilities'
                'attribution:show attribution for a pack'\''s capabilities'
                'version:show the agent-packs version'
                'completion:output shell completion script'
                'help:show usage'
            )
            _describe 'command' commands
            ;;
        args)
            local pack_ids
            case ${words[2]} in
                install|show|audit|verify|lint|upgrade|rollback|uninstall|diff|deps|tree|compat|licenses|attribution|resolve)
                    pack_ids=(${(f)"$(agent-packs search 2>/dev/null | awk '{print $1}')"})
                    _describe 'pack' pack_ids
                    ;;
                completion)
                    local shells; shells=(bash zsh fish)
                    _describe 'shell' shells
                    ;;
                policy)
                    local sub; sub=(check)
                    _describe 'subcommand' sub
                    ;;
            esac

            _arguments \
                '--agent=[target agent tool]:agent:(claude codex cursor gemini copilot opencode goose)' \
                '--target-tool=[target tool alias]:agent:(claude codex cursor gemini copilot opencode goose)' \
                '--mode=[install mode]:mode:(reference symlink copy native)' \
                '--on-conflict=[conflict policy]:policy:(skip overwrite backup)' \
                '--only=[capability filter]:filter:(all skills plugins)' \
                '--target=[install target directory]:directory:_directories' \
                '--dry-run[print plan without writing files]' \
                '--execute-plugins[run native plugin install commands]' \
                '--global[install into global target]' \
                '--json[output as JSON]'
            ;;
    esac
}

_agent_packs "$@"
`

const fishCompletion = `# agent-packs fish completion
# Place in ~/.config/fish/completions/agent-packs.fish

set -l __ap_subcommands search show install list outdated upgrade rollback uninstall status audit verify lint diff tree deps compat validate index registry doctor new init publish policy licenses attribution resolve version completion help

# Subcommand completions
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a search     -d 'Search the registry for packs'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a show       -d 'Show details of a pack'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a install    -d 'Install a pack'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a list       -d 'List installed packs'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a outdated   -d 'Check for available updates'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a upgrade    -d 'Upgrade an installed pack'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a rollback   -d 'Roll back to a previous install'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a uninstall  -d 'Remove an installed pack'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a status     -d 'Check installed skills for drift'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a audit      -d 'Generate a supply-chain SBOM'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a verify     -d 'Verify pack source references'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a lint       -d 'Lint a pack manifest'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a diff       -d 'Diff installed pack against registry'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a validate   -d 'Validate manifests against schema'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a index      -d 'Regenerate registry index'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a completion -d 'Output shell completion script'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a version    -d 'Show version'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a help       -d 'Show usage'

# Pack ID completions for commands that take a pack argument
set -l __ap_pack_cmds install show audit verify lint upgrade rollback uninstall diff deps tree compat licenses attribution resolve
complete -f -c agent-packs \
    -n "__fish_seen_subcommand_from $__ap_pack_cmds" \
    -a "(agent-packs search 2>/dev/null | awk '{print \$1}')"

# Shell name for completion subcommand
complete -f -c agent-packs -n "__fish_seen_subcommand_from completion" -a "bash zsh fish"

# Shared flags
complete -f -c agent-packs -l agent        -a "claude codex cursor gemini copilot opencode goose" -d 'Target agent tool'
complete -f -c agent-packs -l target-tool  -a "claude codex cursor gemini copilot opencode goose" -d 'Target tool alias'
complete -f -c agent-packs -l mode         -a "reference symlink copy native"                     -d 'Install mode'
complete -f -c agent-packs -l on-conflict  -a "skip overwrite backup"                             -d 'Conflict policy'
complete -f -c agent-packs -l only         -a "all skills plugins"                                 -d 'Capability filter'
complete -r -c agent-packs -l target       -d 'Installation target directory'
complete -f -c agent-packs -l dry-run      -d 'Print plan without writing files'
complete -f -c agent-packs -l execute-plugins -d 'Run native plugin install commands'
complete -f -c agent-packs -l global       -d 'Install into global target'
complete -f -c agent-packs -l json         -d 'Output as JSON'
`

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
	fmt.Fprintln(os.Stderr, "  agent-packs install <pack-id|registry/pack-id>... [--target dir] [--agent tool|--target-tool tool] [--only all|skills|plugins] [--mode reference|symlink|copy|native] [--on-conflict skip|overwrite|backup] [--project dir|--global] [--dry-run] [--execute-plugins]")
	fmt.Fprintln(os.Stderr, "  agent-packs list [--target dir]")
	fmt.Fprintln(os.Stderr, "  agent-packs update|outdated|upgrade|cache ...")
	fmt.Fprintln(os.Stderr, "  agent-packs upgrade <pack-id>... [--target dir] [--execute-plugins]")
	fmt.Fprintln(os.Stderr, "  agent-packs rollback <pack-id>... [--target dir]")
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
	fmt.Fprintln(os.Stderr, "  agent-packs uninstall <pack-id>... [--target dir]")
	fmt.Fprintln(os.Stderr, "  agent-packs doctor [targets]")
	fmt.Fprintln(os.Stderr, "  agent-packs validate <file-or-directory>")
	fmt.Fprintln(os.Stderr, "  agent-packs status [--target dir]")
	fmt.Fprintln(os.Stderr, "  agent-packs completion <bash|zsh|fish>")
	fmt.Fprintln(os.Stderr, "  agent-packs registry add|list|remove ...")
}
