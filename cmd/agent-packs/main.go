package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/agent-packs/cli/internal/agentpacks"
	"github.com/agent-packs/cli/internal/output"
	"gopkg.in/yaml.v3"
)

func main() {
	registry := os.Getenv("AGENT_PACKS_REGISTRY")
	if registry == "" {
		registry = resolveDefaultRegistry()
	}
	defaultTarget := os.Getenv("AGENT_PACKS_HOME")
	if defaultTarget == "" {
		defaultTarget = ".agent-packs"
	}

	// Merge any user-registered custom targets into the target matrix.
	agentpacks.RegisterCustomTargets(defaultTarget)

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
	case "test-run":
		err = runTestRun(registry, defaultTarget, os.Args[2:])
	case "install":
		err = runInstall(registry, defaultTarget, os.Args[2:])
	case "skills":
		err = runStandaloneLifecycle(registry, defaultTarget, "skills", os.Args[2:])
	case "plugins":
		err = runStandaloneLifecycle(registry, defaultTarget, "plugins", os.Args[2:])
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
	case "pin":
		err = runPin(registry, defaultTarget, os.Args[2:])
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
		err = runInit(registry, os.Args[2:])
	case "new":
		err = runNew(os.Args[2:])
	case "status":
		err = runStatus(defaultTarget, os.Args[2:])
	case "completion":
		err = runCompletion(os.Args[2:])
	case "sync":
		err = runSync(registry, defaultTarget, os.Args[2:])
	case "freeze":
		err = runFreeze(defaultTarget, os.Args[2:])
	case "export":
		err = runExport(defaultTarget, os.Args[2:])
	case "target":
		err = runTarget(defaultTarget, os.Args[2:])
	case "why":
		err = runWhy(defaultTarget, os.Args[2:])
	case "tap":
		err = runTap(defaultTarget, os.Args[2:])
	case "untap":
		err = runUntap(defaultTarget, os.Args[2:])
	case "info":
		err = runInfo(registry, defaultTarget, os.Args[2:])
	case "home":
		err = runHome(registry, defaultTarget, os.Args[2:])
	case "analytics":
		err = runAnalytics(defaultTarget, os.Args[2:])
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
	flags := flag.NewFlagSet("search", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	tagFilter := flags.String("tag", "", "filter by tag")
	categoryFilter := flags.String("category", "", "filter by category")
	stabilityFilter := flags.String("stability", "", "filter by stability (experimental|stable|deprecated)")
	toolFilter := flags.String("tool", "", "filter by supported tool/agent")
	reviewStatusFilter := flags.String("review-status", "", "filter by review status (draft|reviewed|verified)")
	scopeFilter := flags.String("scope", "", "filter by scope (global|project)")
	details := flags.Bool("details", false, "show stability, review status, freshness, tools, scope, and install command")
	if err := flags.Parse(args); err != nil {
		return err
	}
	query := strings.Join(flags.Args(), " ")
	f := agentpacks.SearchFilter{
		Tag:          *tagFilter,
		Category:     *categoryFilter,
		Stability:    *stabilityFilter,
		Tool:         *toolFilter,
		ReviewStatus: *reviewStatusFilter,
		Scope:        *scopeFilter,
	}
	matches, err := agentpacks.FilteredMatchPacks(registry, query, f)
	if err != nil {
		return err
	}
	if asJSON {
		if len(matches) == 0 {
			return agentpacks.ErrNotFound
		}
		return output.Encode(os.Stdout, matches)
	}
	if len(matches) == 0 {
		fmt.Fprintln(os.Stdout, "No packs found.")
		return agentpacks.ErrNotFound
	}
	printSearchResults(os.Stdout, matches, *details)
	return nil
}

func printSearchResults(out io.Writer, matches []agentpacks.Pack, details bool) {
	for _, pack := range matches {
		if details {
			fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\t%s\t%s\tagent-packs install %s\n",
				pack.ID,
				pack.Name,
				pack.Stability,
				pack.ReviewStatus,
				pack.LastVerified,
				strings.Join(pack.Tools, ","),
				strings.Join(pack.Scope, ","),
				pack.ID,
			)
			continue
		}
		fmt.Fprintf(out, "%s\t%s\t%s\n", pack.ID, pack.Name, strings.Join(pack.Tags, ", "))
	}
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
	agent := flags.String("agent", envOrDefault("AGENT_PACKS_AGENT", "generic"), "target agent/tool ($AGENT_PACKS_AGENT)")
	targetTool := flags.String("target-tool", "", "target tool alias for --agent")
	only := flags.String("only", "all", "capability filter: all, skills, plugins, memory, settings, commands, hooks, or subagents")
	dryRun := flags.Bool("dry-run", false, "print installation plan without writing files")
	executePlugins := flags.Bool("execute-plugins", false, "run native plugin installation commands")
	allowHooks := flags.Bool("allow-hooks", false, "write hook capabilities in copy mode (the agent may run them automatically)")
	mode := flags.String("mode", envOrDefault("AGENT_PACKS_MODE", "reference"), "sync mode: reference, symlink, copy, or native ($AGENT_PACKS_MODE)")
	onConflict := flags.String("on-conflict", envOrDefault("AGENT_PACKS_ON_CONFLICT", "skip"), "conflict policy: skip, overwrite, or backup ($AGENT_PACKS_ON_CONFLICT)")
	project := flags.String("project", "", "project directory target")
	global := flags.Bool("global", false, "install into the configured global target")
	from := flags.String("from", "", "install packs listed in a YAML export file")
	minTrust := flags.String("min-trust", "", "minimum trust level: core, community, tap, or unverified")
	if err := flags.Parse(args); err != nil {
		return err
	}
	remaining := flags.Args()
	if *from != "" {
		extra, err := readPacksFromFile(*from)
		if err != nil {
			return err
		}
		remaining = append(extra, remaining...)
	}
	if len(remaining) < 1 {
		return fmt.Errorf("usage: agent-packs install <pack-id>... [--from file] [--target dir] [--agent name] [--only filter] [--dry-run] [--execute-plugins]")
	}
	if *targetTool != "" {
		*agent = *targetTool
	}
	*agent = agentpacks.NormalizeAgent(*agent)
	if !agentpacks.ValidAgent(*agent) {
		return fmt.Errorf("invalid agent %q: run `agent-packs doctor targets` for supported tools", *agent)
	}
	if *only != "all" && *only != "skills" && *only != "plugins" && *only != "memory" && *only != "settings" && *only != "commands" && *only != "hooks" && *only != "subagents" {
		return fmt.Errorf("invalid --only %q: expected all, skills, plugins, memory, settings, commands, hooks, or subagents", *only)
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
	options := agentpacks.InstallOptions{Mode: *mode, OnConflict: *onConflict, Scope: scope, AllowHooks: *allowHooks}
	for index, packRef := range remaining {
		printLifecycleHeader("Installing", packRef, index, len(remaining))
		if err := agentpacks.InstallWithMinTrust(registry, *target, packRef, installTarget, *agent, *only, *executePlugins, *dryRun, options, *minTrust, os.Stdout); err != nil {
			return err
		}
		agentpacks.AnalyticsTrack(defaultTarget, "install", packRef, *agent, "")
	}
	return nil
}

func runTestRun(registry, defaultTarget string, args []string) error {
	args = normalizeInstallArgs(args)
	flags := flag.NewFlagSet("test-run", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	agent := flags.String("agent", envOrDefault("AGENT_PACKS_AGENT", "generic"), "target agent/tool ($AGENT_PACKS_AGENT)")
	command := flags.String("command", "", "command to launch the agent (overrides default agent executable)")
	mode := flags.String("mode", "copy", "sync mode (defaults to copy for test-run)")
	executePlugins := flags.Bool("execute-plugins", false, "run native plugin installation commands")
	if err := flags.Parse(args); err != nil {
		return err
	}
	remaining := flags.Args()
	if len(remaining) != 1 {
		return fmt.Errorf("usage: agent-packs test-run <pack-id> [--agent name] [--command cmd]")
	}
	packID := remaining[0]

	*agent = agentpacks.NormalizeAgent(*agent)
	if !agentpacks.ValidAgent(*agent) {
		return fmt.Errorf("invalid agent %q: run `agent-packs doctor targets` for supported tools", *agent)
	}

	tempDir, err := os.MkdirTemp("", "agent-packs-test-*")
	if err != nil {
		return fmt.Errorf("failed to create sandbox directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	fmt.Fprintf(os.Stdout, "Creating sandbox at %s\n", tempDir)

	options := agentpacks.InstallOptions{Mode: *mode, OnConflict: "overwrite", Scope: "project", AllowHooks: true}

	printLifecycleHeader("Installing pack into sandbox", packID, 0, 1)
	if err := agentpacks.InstallWithMinTrust(registry, defaultTarget, packID, tempDir, *agent, "all", *executePlugins, false, options, "", os.Stdout); err != nil {
		return fmt.Errorf("failed to install pack in sandbox: %w", err)
	}

	cmdToRun := *command
	if cmdToRun == "" {
		cmdToRun = *agent
	}

	fmt.Fprintf(os.Stdout, "\nSandbox ready. Launching %q...\n(Exit the agent to destroy the sandbox)\n\n", cmdToRun)

	cmd := exec.Command("sh", "-c", cmdToRun)
	cmd.Dir = tempDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("agent process exited with error: %w", err)
	}

	fmt.Fprintf(os.Stdout, "\nAgent exited. Destroying sandbox %s\n", tempDir)
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

func runStandaloneLifecycle(registry, defaultTarget, kind string, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: agent-packs %s <install|list|upgrade|uninstall> ...", kind)
	}
	switch args[0] {
	case "install", "add":
		return runStandaloneInstall(registry, defaultTarget, kind, args[1:])
	case "list", "ls":
		return runStandaloneList(defaultTarget, kind, args[1:])
	case "upgrade", "update":
		return runStandaloneUpgrade(defaultTarget, kind, args[1:])
	case "uninstall", "remove", "rm":
		return runStandaloneUninstall(defaultTarget, kind, args[1:])
	default:
		return fmt.Errorf("usage: agent-packs %s <install|list|upgrade|uninstall> ...", kind)
	}
}

func runStandaloneInstall(registry, defaultTarget, kind string, args []string) error {
	flags := flag.NewFlagSet(kind+" install", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	agent := flags.String("agent", "generic", "target agent/tool")
	targetTool := flags.String("target-tool", "", "target tool alias for --agent")
	dryRun := flags.Bool("dry-run", false, "print installation plan without writing files")
	executePlugins := flags.Bool("execute-plugins", false, "run native plugin installation commands")
	modeDefault := "copy"
	if kind == "plugins" {
		modeDefault = "native"
	}
	mode := flags.String("mode", modeDefault, "sync mode: reference, symlink, copy, or native")
	onConflict := flags.String("on-conflict", "skip", "conflict policy: skip, overwrite, or backup")
	project := flags.String("project", "", "project directory target")
	global := flags.Bool("global", false, "install into the configured global target")
	method := flags.String("method", "", "plugin install method")
	pkg := flags.String("package", "", "plugin package name")
	marketplace := flags.String("marketplace", "", "plugin marketplace name")
	command := flags.String("command", "", "plugin install command")
	uninstallCommand := flags.String("uninstall-command", "", "plugin uninstall command")
	if err := flags.Parse(normalizeInstallArgs(args)); err != nil {
		return err
	}
	remaining := flags.Args()
	if len(remaining) < 1 {
		return fmt.Errorf("usage: agent-packs %s install <id-or-path>... [--target dir] [--agent name] [--mode mode] [--dry-run]", kind)
	}
	if *targetTool != "" {
		*agent = *targetTool
	}
	*agent = agentpacks.NormalizeAgent(*agent)
	if !agentpacks.ValidAgent(*agent) {
		return fmt.Errorf("invalid agent %q: run `agent-packs doctor targets` for supported tools", *agent)
	}
	if *mode != "reference" && *mode != "symlink" && *mode != "copy" && *mode != "native" {
		return fmt.Errorf("invalid --mode %q: expected reference, symlink, copy, or native", *mode)
	}
	if kind == "plugins" && *mode != "reference" && *mode != "native" {
		return fmt.Errorf("invalid --mode %q for plugins: expected reference or native", *mode)
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
	installOverrides := map[string]string{}
	if kind == "plugins" {
		installOverrides["method"] = *method
		installOverrides["package"] = *pkg
		installOverrides["marketplace"] = *marketplace
		installOverrides["command"] = *command
		installOverrides["uninstall"] = *uninstallCommand
	}
	for index, ref := range remaining {
		printLifecycleHeader("Installing "+singularStandaloneKind(kind), ref, index, len(remaining))
		if err := agentpacks.InstallStandaloneWithOverrides(registry, ref, kind, installTarget, *agent, *executePlugins, *dryRun, options, installOverrides, os.Stdout); err != nil {
			return err
		}
	}
	return nil
}

func runStandaloneList(defaultTarget, kind string, args []string) error {
	flags := flag.NewFlagSet(kind+" list", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	if len(flags.Args()) != 0 {
		return fmt.Errorf("usage: agent-packs %s list [--target dir]", kind)
	}
	return agentpacks.ListStandalone(*target, kind, os.Stdout)
}

func runStandaloneUpgrade(defaultTarget, kind string, args []string) error {
	flags := flag.NewFlagSet(kind+" upgrade", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	executePlugins := flags.Bool("execute-plugins", false, "run native plugin installation commands")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	remaining := flags.Args()
	if len(remaining) < 1 {
		return fmt.Errorf("usage: agent-packs %s upgrade <id>... [--target dir]", kind)
	}
	for index, id := range remaining {
		printLifecycleHeader("Upgrading "+singularStandaloneKind(kind), id, index, len(remaining))
		if err := agentpacks.UpgradeStandalone(*target, id, kind, *executePlugins, os.Stdout); err != nil {
			return err
		}
	}
	return nil
}

func runStandaloneUninstall(defaultTarget, kind string, args []string) error {
	flags := flag.NewFlagSet(kind+" uninstall", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	executePlugins := flags.Bool("execute-plugins", false, "run native plugin uninstall commands")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	remaining := flags.Args()
	if len(remaining) < 1 {
		return fmt.Errorf("usage: agent-packs %s uninstall <id>... [--target dir] [--execute-plugins]", kind)
	}
	for index, id := range remaining {
		printLifecycleHeader("Uninstalling "+singularStandaloneKind(kind), id, index, len(remaining))
		if err := agentpacks.UninstallStandalone(*target, id, kind, *executePlugins, os.Stdout); err != nil {
			return err
		}
	}
	return nil
}

func singularStandaloneKind(kind string) string {
	if kind == "skills" {
		return "skill"
	}
	if kind == "plugins" {
		return "plugin"
	}
	return "capability"
}

func runUninstall(defaultTarget string, args []string) error {
	flags := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	executePlugins := flags.Bool("execute-plugins", false, "run native plugin uninstall commands")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	remaining := flags.Args()
	if len(remaining) < 1 {
		return fmt.Errorf("usage: agent-packs uninstall <pack-id>... [--target dir] [--execute-plugins]")
	}
	for index, packRef := range remaining {
		printLifecycleHeader("Uninstalling", packRef, index, len(remaining))
		if err := agentpacks.UninstallWithOptions(*target, packRef, *executePlugins, os.Stdout); err != nil {
			return err
		}
	}
	return nil
}

func runDoctor(registry, defaultTarget string, args []string) error {
	asJSON, args := extractJSONFlag(args)
	if len(args) == 1 && args[0] == "targets" {
		if asJSON {
			return output.Encode(os.Stdout, agentpacks.TargetMatrixList())
		}
		return agentpacks.PrintTargetMatrix(os.Stdout)
	}
	if len(args) != 0 {
		return fmt.Errorf("usage: agent-packs doctor [targets] [--json]")
	}
	if asJSON {
		return agentpacks.DoctorJSON(registry, defaultTarget, os.Stdout)
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
	all := flags.Bool("all", false, "upgrade all installed packs")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	remaining := flags.Args()
	if *all {
		summaries, err := agentpacks.ListInstalledReceipts(*target)
		if err != nil {
			return err
		}
		if len(summaries) == 0 {
			fmt.Fprintln(os.Stdout, "No packs installed.")
			return nil
		}
		for index, s := range summaries {
			printLifecycleHeader("Upgrading", s.ID, index, len(summaries))
			if err := agentpacks.Upgrade(registry, *target, s.ID, *target, *executePlugins, os.Stdout); err != nil {
				return err
			}
		}
		return nil
	}
	if len(remaining) < 1 {
		return fmt.Errorf("usage: agent-packs upgrade <pack-id>... [--all] [--target dir] [--execute-plugins]")
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

func runInit(registry string, args []string) error {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	agent := flags.String("agent", "codex", "default target agent")
	mode := flags.String("mode", "reference", "default sync mode")
	onConflict := flags.String("on-conflict", "skip", "default conflict policy")
	scope := flags.String("scope", "project", "default install scope")
	registryPath := flags.String("registry", "", "default registry path")
	target := flags.String("target", ".agent-packs", "default install target")
	force := flags.Bool("force", false, "overwrite existing config")
	noDetect := flags.Bool("no-detect", false, "skip project detection; write flag defaults only")
	if err := flags.Parse(normalizeInitArgs(args)); err != nil {
		return err
	}
	agentExplicit := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "agent" {
			agentExplicit = true
		}
	})
	projectDir := "."
	if len(flags.Args()) > 0 {
		projectDir = flags.Args()[0]
	}
	detectRegistry := *registryPath
	if detectRegistry == "" {
		detectRegistry = registry
	}
	path, det, err := agentpacks.InitProjectWithDetection(projectDir, detectRegistry, !*noDetect, agentpacks.InitOptions{
		Agent: *agent, Mode: *mode, OnConflict: *onConflict, Scope: *scope,
		Registry: *registryPath, Target: *target, Force: *force,
	}, agentExplicit)
	if err != nil {
		return err
	}
	if !*noDetect {
		if det.Agent != "" {
			fmt.Printf("Detected agent: %s\n", det.Agent)
		}
		if len(det.Stack) > 0 {
			fmt.Printf("Detected stack: %s\n", strings.Join(det.Stack, ", "))
		}
		if len(det.Packs) > 0 {
			fmt.Printf("Recommending packs: %s\n", strings.Join(det.Packs, ", "))
		}
	}
	fmt.Printf("Wrote %s\n", path)
	return nil
}

func runNew(args []string) error {
	const newUsage = "usage: agent-packs new <pack|skill|plugin|command|hook|subagent|memory|settings> <id> [--name name] [--dir dir] [--force]"
	if len(args) < 1 {
		return fmt.Errorf(newUsage)
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
		return fmt.Errorf(newUsage)
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
	// Refresh the fetched default registry (Homebrew-style catalog update) when
	// the CLI is relying on the runtime-fetched copy rather than a local checkout.
	if os.Getenv("AGENT_PACKS_REGISTRY") == "" && localRegistryPath() == "" {
		if err := agentpacks.RefreshLocalRegistry(registryCacheDir()); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not refresh default registry: %v\n", err)
		} else {
			fmt.Fprintln(os.Stdout, "OK    default registry refreshed")
		}
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
		return fmt.Errorf("usage: agent-packs policy check <pack-id> <policy.json|preset>")
	}
	policyArg := args[2]
	// Resolve named presets from registry/policy/<name>.json
	if !strings.Contains(policyArg, string(filepath.Separator)) && !strings.HasSuffix(policyArg, ".json") {
		policyArg = filepath.Join(filepath.Dir(registry), "policy", policyArg+".json")
	}
	return agentpacks.PolicyCheck(registry, args[1], policyArg, os.Stdout)
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
	check := flags.Bool("check", false, "verify the index at --output is up to date without writing it (exit non-zero on drift)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if len(flags.Args()) != 0 {
		return fmt.Errorf("usage: agent-packs index [--output path] [--check]")
	}
	if *check {
		return agentpacks.CheckIndex(registry, *output, os.Stdout)
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
	if len(args) == 1 && args[0] == "--all" {
		return agentpacks.LintAll(registry, os.Stdout)
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: agent-packs lint <pack-id|--all>")
	}
	return agentpacks.Lint(registry, args[0], os.Stdout)
}

func runVerify(registry string, args []string) error {
	if len(args) == 1 && args[0] == "--all" {
		return agentpacks.VerifyAll(registry, os.Stdout)
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: agent-packs verify <pack-id|--all>")
	}
	return agentpacks.Verify(registry, args[0], os.Stdout)
}

func runWhy(defaultTarget string, args []string) error {
	flags := flag.NewFlagSet("why", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	remaining := flags.Args()
	if len(remaining) != 1 {
		return fmt.Errorf("usage: agent-packs why <skill-name> [--target dir]")
	}
	return agentpacks.Why(*target, remaining[0], os.Stdout)
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

// normalizeInitArgs reorders `init` args so flags precede the optional
// positional project directory, since Go's flag parser stops at the first
// non-flag argument.
func normalizeInitArgs(args []string) []string {
	valueFlags := map[string]bool{
		"--agent": true, "--mode": true, "--on-conflict": true,
		"--scope": true, "--registry": true, "--target": true,
	}
	flags := []string{}
	positionals := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if valueFlags[arg] {
			flags = append(flags, arg)
			if i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "--") {
			flags = append(flags, arg)
			continue
		}
		positionals = append(positionals, arg)
	}
	return append(flags, positionals...)
}

func normalizeInstallArgs(args []string) []string {
	flags := []string{}
	positionals := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--dry-run" || arg == "--execute-plugins" || arg == "--allow-hooks" || arg == "--global" {
			flags = append(flags, arg)
			continue
		}
		if arg == "--target" || arg == "--agent" || arg == "--target-tool" || arg == "--only" || arg == "--mode" || arg == "--on-conflict" || arg == "--project" || arg == "--scope" || arg == "--method" || arg == "--package" || arg == "--marketplace" || arg == "--command" || arg == "--uninstall-command" || arg == "--from" || arg == "--min-trust" {
			flags = append(flags, arg)
			if i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "--target=") || strings.HasPrefix(arg, "--agent=") || strings.HasPrefix(arg, "--target-tool=") || strings.HasPrefix(arg, "--only=") || strings.HasPrefix(arg, "--mode=") || strings.HasPrefix(arg, "--on-conflict=") || strings.HasPrefix(arg, "--project=") || strings.HasPrefix(arg, "--scope=") || strings.HasPrefix(arg, "--method=") || strings.HasPrefix(arg, "--package=") || strings.HasPrefix(arg, "--marketplace=") || strings.HasPrefix(arg, "--command=") || strings.HasPrefix(arg, "--uninstall-command=") || strings.HasPrefix(arg, "--from=") || strings.HasPrefix(arg, "--min-trust=") {
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
		if arg == "--execute-plugins" || arg == "--check" {
			flags = append(flags, arg)
			continue
		}
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

func runPin(registry, defaultTarget string, args []string) error {
	flags := flag.NewFlagSet("pin", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	check := flags.Bool("check", false, "verify the live source still matches recorded pins instead of rewriting them")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	remaining := flags.Args()
	if len(remaining) != 1 {
		return fmt.Errorf("usage: agent-packs pin <pack-id> [--check] [--target dir]")
	}
	return agentpacks.PinPack(registry, *target, remaining[0], *check, os.Stdout)
}

func runStatus(defaultTarget string, args []string) error {
	asJSON, args := extractJSONFlag(normalizeTargetArgs(args))
	flags := flag.NewFlagSet("status", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if asJSON {
		return agentpacks.DriftCheckJSON(*target, os.Stdout)
	}
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

    local subcommands="search show install info home sync freeze export skills plugins list outdated upgrade rollback uninstall why status audit verify lint diff tree deps compat scan import validate index registry tap untap target doctor new init publish policy licenses attribution resolve analytics version completion help"

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
            COMPREPLY=($(compgen -W "all skills plugins memory settings commands hooks subagents" -- "$cur"))
            return ;;
    esac

    case "${words[1]}" in
        install|show|audit|verify|lint|upgrade|rollback|uninstall|diff|deps|tree|compat|licenses|attribution|resolve|why)
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
        registry)
            COMPREPLY=($(compgen -W "add list remove" -- "$cur"))
            ;;
        target)
            COMPREPLY=($(compgen -W "add list remove" -- "$cur"))
            ;;
        skills|plugins)
            COMPREPLY=($(compgen -W "install list upgrade uninstall" -- "$cur"))
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
                'sync:install packs from .agent-packs.yaml packs list'
                'freeze:write installed pack IDs to .agent-packs.yaml'
                'export:write installed packs to a portable YAML file'
                'skills:manage standalone Agent Skills'
                'plugins:manage standalone plugins'
                'list:list installed packs'
                'outdated:check for available updates'
                'upgrade:upgrade an installed pack (--all for all)'
                'rollback:roll back a pack to a previous install'
                'uninstall:remove an installed pack'
                'why:show which pack provides a skill'
                'status:check installed skills for drift or tampering'
                'audit:generate a supply-chain SBOM for a pack'
                'verify:verify pack source references (--all for all)'
                'lint:lint a pack manifest (--all for all)'
                'diff:diff an installed pack against the registry'
                'tree:show pack dependency tree'
                'compat:check pack compatibility with an agent'
                'validate:validate manifests against schema'
                'index:regenerate the registry index'
                'registry:manage remote registries'
                'target:manage custom agent tool targets'
                'doctor:diagnose installation environment'
                'new:scaffold a new pack, skill, plugin, command, hook, subagent, memory, or settings capability'
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
                install|show|audit|verify|lint|upgrade|rollback|uninstall|diff|deps|tree|compat|licenses|attribution|resolve|why)
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
                registry|target)
                    local sub; sub=(add list remove)
                    _describe 'subcommand' sub
                    ;;
            esac

            _arguments \
                '--agent=[target agent tool]:agent:(claude codex cursor gemini copilot opencode goose)' \
                '--target-tool=[target tool alias]:agent:(claude codex cursor gemini copilot opencode goose)' \
                '--mode=[install mode]:mode:(reference symlink copy native)' \
                '--on-conflict=[conflict policy]:policy:(skip overwrite backup)' \
                '--only=[capability filter]:filter:(all skills plugins memory settings commands hooks subagents)' \
                '--target=[install target directory]:directory:_directories' \
                '--dry-run[print plan without writing files]' \
                '--execute-plugins[run native plugin install commands]' \
                '--allow-hooks[write hook capabilities in copy mode]' \
                '--global[install into global target]' \
                '--json[output as JSON]'
            ;;
    esac
}

_agent_packs "$@"
`

const fishCompletion = `# agent-packs fish completion
# Place in ~/.config/fish/completions/agent-packs.fish

set -l __ap_subcommands search show install info home sync freeze export skills plugins list outdated upgrade rollback uninstall why status audit verify lint diff tree deps compat validate index registry tap untap target doctor new init publish policy licenses attribution resolve analytics version completion help

# Subcommand completions
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a search     -d 'Search the registry for packs'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a show       -d 'Show details of a pack'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a install    -d 'Install a pack'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a sync       -d 'Install packs from .agent-packs.yaml'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a freeze     -d 'Write installed packs to .agent-packs.yaml'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a export     -d 'Export installed packs to a portable file'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a skills     -d 'Manage standalone Agent Skills'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a plugins    -d 'Manage standalone plugins'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a list       -d 'List installed packs'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a outdated   -d 'Check for available updates'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a upgrade    -d 'Upgrade an installed pack (--all for all)'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a rollback   -d 'Roll back to a previous install'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a uninstall  -d 'Remove an installed pack'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a why        -d 'Show which pack provides a skill'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a status     -d 'Check installed skills for drift'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a audit      -d 'Generate a supply-chain SBOM'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a verify     -d 'Verify pack source references (--all for all)'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a lint       -d 'Lint a pack manifest (--all for all)'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a diff       -d 'Diff installed pack against registry'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a validate   -d 'Validate manifests against schema'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a index      -d 'Regenerate registry index'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a target     -d 'Manage custom agent tool targets'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a completion -d 'Output shell completion script'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a version    -d 'Show version'
complete -f -c agent-packs -n "not __fish_seen_subcommand_from $__ap_subcommands" -a help       -d 'Show usage'

# Pack ID completions for commands that take a pack argument
set -l __ap_pack_cmds install show audit verify lint upgrade rollback uninstall diff deps tree compat licenses attribution resolve why
complete -f -c agent-packs \
    -n "__fish_seen_subcommand_from $__ap_pack_cmds" \
    -a "(agent-packs search 2>/dev/null | awk '{print \$1}')"

# Shell name for completion subcommand
complete -f -c agent-packs -n "__fish_seen_subcommand_from completion" -a "bash zsh fish"
complete -f -c agent-packs -n "__fish_seen_subcommand_from skills plugins" -a "install list upgrade uninstall"
complete -f -c agent-packs -n "__fish_seen_subcommand_from registry target" -a "add list remove"

# Shared flags
complete -f -c agent-packs -l agent        -a "claude codex cursor gemini copilot opencode goose" -d 'Target agent tool'
complete -f -c agent-packs -l target-tool  -a "claude codex cursor gemini copilot opencode goose" -d 'Target tool alias'
complete -f -c agent-packs -l mode         -a "reference symlink copy native"                     -d 'Install mode'
complete -f -c agent-packs -l on-conflict  -a "skip overwrite backup"                             -d 'Conflict policy'
complete -f -c agent-packs -l only         -a "all skills plugins memory settings commands hooks subagents"  -d 'Capability filter'
complete -r -c agent-packs -l target       -d 'Installation target directory'
complete -f -c agent-packs -l dry-run      -d 'Print plan without writing files'
complete -f -c agent-packs -l execute-plugins -d 'Run native plugin install commands'
complete -f -c agent-packs -l allow-hooks     -d 'Write hook capabilities in copy mode'
complete -f -c agent-packs -l global       -d 'Install into global target'
complete -f -c agent-packs -l json         -d 'Output as JSON'
`

func runSync(registry, defaultTarget string, args []string) error {
	flags := flag.NewFlagSet("sync", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	agent := flags.String("agent", envOrDefault("AGENT_PACKS_AGENT", "generic"), "target agent/tool")
	mode := flags.String("mode", envOrDefault("AGENT_PACKS_MODE", "reference"), "sync mode: reference, symlink, copy, or native")
	project := flags.String("project", ".", "project directory containing .agent-packs.yaml")
	dryRun := flags.Bool("dry-run", false, "print what would be installed without writing files")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	if len(flags.Args()) != 0 {
		return fmt.Errorf("usage: agent-packs sync [--project dir] [--target dir] [--agent tool] [--mode mode] [--dry-run]")
	}
	*agent = agentpacks.NormalizeAgent(*agent)
	return agentpacks.Sync(registry, *target, *project, *target, *agent, *mode, *dryRun, os.Stdout)
}

func runFreeze(defaultTarget string, args []string) error {
	flags := flag.NewFlagSet("freeze", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	project := flags.String("project", ".", "project directory containing .agent-packs.yaml")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	if len(flags.Args()) != 0 {
		return fmt.Errorf("usage: agent-packs freeze [--target dir] [--project dir]")
	}
	return agentpacks.Freeze(*target, *project, os.Stdout)
}

func runExport(defaultTarget string, args []string) error {
	flags := flag.NewFlagSet("export", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	outFile := flags.String("output", "", "output file (default: stdout)")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	if len(flags.Args()) != 0 {
		return fmt.Errorf("usage: agent-packs export [--target dir] [--output file]")
	}
	out := io.Writer(os.Stdout)
	if *outFile != "" {
		f, err := os.Create(*outFile)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}
	return agentpacks.ExportPacks(*target, out)
}

func normalizeTargetCmdArgs(args []string) []string {
	knownFlags := map[string]bool{
		"--home": true, "--name": true, "--global": true, "--project": true,
	}
	flagsList := []string{}
	positionals := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if knownFlags[arg] {
			flagsList = append(flagsList, arg)
			if i+1 < len(args) {
				flagsList = append(flagsList, args[i+1])
				i++
			}
			continue
		}
		for k := range knownFlags {
			if strings.HasPrefix(arg, k+"=") {
				flagsList = append(flagsList, arg)
				goto next
			}
		}
		positionals = append(positionals, arg)
	next:
	}
	return append(flagsList, positionals...)
}

func runTarget(defaultTarget string, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: agent-packs target <add|list|remove> ...")
	}
	sub := args[0]
	normalized := normalizeTargetCmdArgs(args[1:])
	flags := flag.NewFlagSet("target "+sub, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	home := flags.String("home", defaultTarget, "agent-packs home directory")
	switch sub {
	case "add":
		name := flags.String("name", "", "display name")
		globalSkills := flags.String("global", "", "global skill directory (required)")
		projectSkills := flags.String("project", "", "project skill directory (defaults to --global)")
		if err := flags.Parse(normalized); err != nil {
			return err
		}
		remaining := flags.Args()
		if len(remaining) != 1 {
			return fmt.Errorf("usage: agent-packs target add <id> --global <path> [--project <path>] [--name <name>]")
		}
		return agentpacks.AddCustomTarget(*home, remaining[0], *name, *globalSkills, *projectSkills)
	case "remove":
		if err := flags.Parse(normalized); err != nil {
			return err
		}
		remaining := flags.Args()
		if len(remaining) != 1 {
			return fmt.Errorf("usage: agent-packs target remove <id>")
		}
		return agentpacks.RemoveCustomTarget(*home, remaining[0])
	case "list":
		if err := flags.Parse(normalized); err != nil {
			return err
		}
		return agentpacks.ListCustomTargets(*home, os.Stdout)
	default:
		return fmt.Errorf("unknown target command: %s", sub)
	}
}

func runTap(defaultTarget string, args []string) error {
	flags := flag.NewFlagSet("tap", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "agent-packs home directory")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	remaining := flags.Args()
	if len(remaining) == 0 || remaining[0] == "list" {
		return agentpacks.TapList(*target, os.Stdout)
	}
	// Support "tap add org/repo" or "tap org/repo" shorthand
	ref := remaining[0]
	if ref == "add" && len(remaining) > 1 {
		ref = remaining[1]
	}
	return agentpacks.Tap(*target, ref, os.Stdout)
}

func runUntap(defaultTarget string, args []string) error {
	flags := flag.NewFlagSet("untap", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "agent-packs home directory")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	remaining := flags.Args()
	if len(remaining) != 1 {
		return fmt.Errorf("usage: agent-packs untap <org/repo|name> [--target dir]")
	}
	return agentpacks.Untap(*target, remaining[0], os.Stdout)
}

func runInfo(registry, defaultTarget string, args []string) error {
	asJSON, args := extractJSONFlag(normalizeTargetArgs(args))
	flags := flag.NewFlagSet("info", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	remaining := flags.Args()
	if len(remaining) != 1 {
		return fmt.Errorf("usage: agent-packs info <pack-id> [--target dir] [--json]")
	}
	if asJSON {
		return agentpacks.PackInfoJSON(registry, *target, remaining[0], os.Stdout)
	}
	return agentpacks.PackInfo(registry, *target, remaining[0], os.Stdout)
}

func runHome(registry, defaultTarget string, args []string) error {
	flags := flag.NewFlagSet("home", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "installation target directory")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	remaining := flags.Args()
	if len(remaining) != 1 {
		return fmt.Errorf("usage: agent-packs home <pack-id> [--target dir]")
	}
	return agentpacks.PackHome(registry, *target, remaining[0])
}

func runAnalytics(defaultTarget string, args []string) error {
	flags := flag.NewFlagSet("analytics", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	target := flags.String("target", defaultTarget, "agent-packs home directory")
	if err := flags.Parse(normalizeTargetArgs(args)); err != nil {
		return err
	}
	remaining := flags.Args()
	if len(remaining) == 0 || remaining[0] == "status" {
		return agentpacks.AnalyticsStatus(*target, os.Stdout)
	}
	switch remaining[0] {
	case "enable":
		if err := agentpacks.AnalyticsEnable(*target); err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, "Analytics enabled. Anonymous install events will be sent.")
		fmt.Fprintln(os.Stdout, "Disable at any time with: agent-packs analytics disable")
		return nil
	case "disable":
		if err := agentpacks.AnalyticsDisable(*target); err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, "Analytics disabled.")
		return nil
	default:
		return fmt.Errorf("usage: agent-packs analytics <enable|disable|status>")
	}
}

func readPacksFromFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("--from: %w", err)
	}
	var f struct {
		Packs []string `yaml:"packs"`
	}
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("--from %s: %w", path, err)
	}
	return f.Packs, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func resolveDefaultRegistry() string {
	// 1. A registry that ships alongside the binary or sits in a source checkout
	//    wins — no network needed for dev or bundled installs.
	if local := localRegistryPath(); local != "" {
		return local
	}
	// 2. Otherwise fetch the canonical registry repo into the user cache. This is
	//    the normal path for `go install` and split CLI-only installs.
	cacheDir := registryCacheDir()
	if packs, err := agentpacks.EnsureLocalRegistry(cacheDir); err == nil {
		return packs
	}
	// Fall through to the cache path; LoadPacks surfaces an actionable error if
	// the fetch failed (offline, unreachable repo).
	return filepath.Join(cacheDir, "registry", "packs")
}

// localRegistryPath returns a registry shipped alongside the binary under the
// FHS share/ layout, or "" if none exists. The registry data otherwise lives in
// agent-packs/registry and is fetched at runtime; set AGENT_PACKS_REGISTRY to
// point at a local checkout for development or offline use.
func localRegistryPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	real, err := filepath.EvalSymlinks(exe)
	if err != nil {
		real = exe
	}
	// FHS: binary at <prefix>/bin/ → <prefix>/share/agent-packs/registry/packs.
	fhsPath := filepath.Join(filepath.Dir(real), "..", "share", "agent-packs", "registry", "packs")
	if fi, err := os.Stat(fhsPath); err == nil && fi.IsDir() {
		return fhsPath
	}
	return ""
}

func registryCacheDir() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "agent-packs")
	}
	return filepath.Join(os.TempDir(), "agent-packs-cache")
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  agent-packs search [query] [--tag t] [--category c] [--stability s] [--tool agent] [--review-status s] [--scope s] [--details] [--json]")
	fmt.Fprintln(os.Stderr, "  agent-packs show <pack-id> [--json]")
	fmt.Fprintln(os.Stderr, "  agent-packs test-run <pack-id> [--agent tool] [--command cmd]")
	fmt.Fprintln(os.Stderr, "  agent-packs install <pack-id[@version]>... [--from file] [--target dir] [--agent tool] [--only all|skills|plugins|memory|settings|commands|hooks|subagents] [--mode reference|symlink|copy|native] [--on-conflict skip|overwrite|backup] [--dry-run] [--execute-plugins] [--allow-hooks]")
	fmt.Fprintln(os.Stderr, "  agent-packs sync [--project dir] [--target dir] [--agent tool] [--mode mode]")
	fmt.Fprintln(os.Stderr, "  agent-packs freeze [--target dir] [--project dir]")
	fmt.Fprintln(os.Stderr, "  agent-packs export [--target dir] [--output file]")
	fmt.Fprintln(os.Stderr, "  agent-packs skills install|list|upgrade|uninstall ...")
	fmt.Fprintln(os.Stderr, "  agent-packs plugins install|list|upgrade|uninstall ... [--execute-plugins]")
	fmt.Fprintln(os.Stderr, "  agent-packs list [--target dir]")
	fmt.Fprintln(os.Stderr, "  agent-packs update|outdated|upgrade|cache ...")
	fmt.Fprintln(os.Stderr, "  agent-packs upgrade <pack-id>... [--target dir] [--execute-plugins]")
	fmt.Fprintln(os.Stderr, "  agent-packs rollback <pack-id>... [--target dir]")
	fmt.Fprintln(os.Stderr, "  agent-packs version [--json]")
	fmt.Fprintln(os.Stderr, "  agent-packs init [dir] [--agent tool] [--mode reference|symlink|copy|native] [--no-detect]")
	fmt.Fprintln(os.Stderr, "  agent-packs new <pack|skill|plugin|command|hook|subagent|memory|settings> <id> [--name name] [--dir dir] [--force]")
	fmt.Fprintln(os.Stderr, "  agent-packs audit <pack-id> [--json]")
	fmt.Fprintln(os.Stderr, "  agent-packs tree|deps <pack-id> [--json]")
	fmt.Fprintln(os.Stderr, "  agent-packs publish --check [--policy file] [--json]")
	fmt.Fprintln(os.Stderr, "  agent-packs policy check <pack-id> <policy.json|preset>")
	fmt.Fprintln(os.Stderr, "  agent-packs licenses|attribution|resolve <pack-id>")
	fmt.Fprintln(os.Stderr, "  agent-packs index [--output path] [--check]")
	fmt.Fprintln(os.Stderr, "  agent-packs diff <pack-id> [--target dir]")
	fmt.Fprintln(os.Stderr, "  agent-packs pin <pack-id> [--check] [--target dir]")
	fmt.Fprintln(os.Stderr, "  agent-packs compat <pack-id> [--agent tool]")
	fmt.Fprintln(os.Stderr, "  agent-packs scan [path]")
	fmt.Fprintln(os.Stderr, "  agent-packs import <skills-dir> [--target dir]")
	fmt.Fprintln(os.Stderr, "  agent-packs lint|verify|resolve <pack-id>")
	fmt.Fprintln(os.Stderr, "  agent-packs uninstall <pack-id>... [--target dir]")
	fmt.Fprintln(os.Stderr, "  agent-packs doctor [targets]")
	fmt.Fprintln(os.Stderr, "  agent-packs validate <file-or-directory>")
	fmt.Fprintln(os.Stderr, "  agent-packs status [--target dir] [--json]")
	fmt.Fprintln(os.Stderr, "  agent-packs target add|list|remove ...")
	fmt.Fprintln(os.Stderr, "  agent-packs completion <bash|zsh|fish>")
	fmt.Fprintln(os.Stderr, "  agent-packs registry add|list|remove ...")
	fmt.Fprintln(os.Stderr, "  agent-packs tap [list|add] [<org/repo>] [--target dir]")
	fmt.Fprintln(os.Stderr, "  agent-packs untap <org/repo|name> [--target dir]")
	fmt.Fprintln(os.Stderr, "  agent-packs info <pack-id> [--target dir] [--json]")
	fmt.Fprintln(os.Stderr, "  agent-packs home <pack-id>")
	fmt.Fprintln(os.Stderr, "  agent-packs analytics <enable|disable|status>")
}
