package install

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sandeshh/agent-packs/cli/internal/model"
	"github.com/sandeshh/agent-packs/cli/internal/plan"
	"github.com/sandeshh/agent-packs/cli/internal/registry"
	"github.com/sandeshh/agent-packs/cli/internal/resolve"
	"github.com/sandeshh/agent-packs/cli/internal/util"
)

func Install(registryPath, home, packRef, target, agent, only string, executePlugins, dryRun bool, out io.Writer) error {
	return InstallWithOptions(registryPath, home, packRef, target, agent, only, executePlugins, dryRun,
		model.InstallOptions{Mode: "copy", OnConflict: "overwrite", Scope: "target"}, out)
}

func InstallWithOptions(registryPath, home, packRef, target, agent, only string, executePlugins, dryRun bool, options model.InstallOptions, out io.Writer) error {
	pack, sourceRegistry, err := registry.ResolvePack(registryPath, home, packRef)
	if err != nil {
		return err
	}
	expanded, err := registry.ExpandPack(sourceRegistry, pack, map[string]bool{})
	if err != nil {
		return err
	}
	absTarget, err := filepath.Abs(util.ExpandHome(target))
	if err != nil {
		return err
	}
	installPlan := plan.BuildInstallPlanWithOptions(expanded, absTarget, agent, only, options)
	if dryRun {
		plan.PrintPlan(installPlan, out)
		return nil
	}
	if err := os.MkdirAll(absTarget, 0o755); err != nil {
		return err
	}
	packDir := filepath.Join(absTarget, "packs", expanded.ID)
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		return err
	}
	if err := util.WriteJSON(filepath.Join(packDir, "agent-pack.json"), expanded); err != nil {
		return err
	}
	if pack.Path != "" {
		_ = util.CopyFile(pack.Path, filepath.Join(packDir, "source-registry-entry.json"))
	}
	result := ExecutePlan(installPlan, executePlugins)
	receiptPath, err := WriteReceipt(absTarget, expanded, result)
	if err != nil {
		return err
	}
	if err := WriteLockfile(packDir, expanded); err != nil {
		return err
	}
	plan.PrintPlan(result, out)
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Receipt: %s\n", receiptPath)
	for _, item := range result.Capabilities {
		if item.Status == "failed" {
			return model.ErrInstallFailed
		}
	}
	return nil
}

func Upgrade(registryPath, home, packRef, target string, executePlugins bool, out io.Writer) error {
	absTarget, err := filepath.Abs(util.ExpandHome(target))
	if err != nil {
		return err
	}
	packID := packRef
	if strings.Contains(packRef, "/") {
		parts := strings.SplitN(packRef, "/", 2)
		packID = parts[1]
	}
	receiptPath := filepath.Join(absTarget, "receipts", packID+".json")
	receipt, err := LoadReceipt(receiptPath)
	if err != nil {
		return fmt.Errorf("no installed receipt for %s: %w", packID, err)
	}
	options := model.InstallOptions{
		Mode:       receipt.Plan.Mode,
		OnConflict: receipt.Plan.OnConflict,
		Scope:      receipt.Plan.Scope,
	}
	only := "all"
	skillCount, pluginCount := 0, 0
	for _, item := range receipt.Plan.Capabilities {
		switch item.Type {
		case "skill":
			skillCount++
		case "plugin":
			pluginCount++
		}
	}
	if skillCount > 0 && pluginCount == 0 {
		only = "skills"
	} else if pluginCount > 0 && skillCount == 0 {
		only = "plugins"
	}
	fmt.Fprintf(out, "Upgrading %s (mode=%s, conflict=%s, scope=%s)\n", packRef, options.Mode, options.OnConflict, options.Scope)
	return InstallWithOptions(registryPath, home, packRef, target, receipt.Plan.Agent, only, executePlugins, false, options, out)
}

func Rollback(target, packID string, out io.Writer) error {
	absTarget, err := filepath.Abs(util.ExpandHome(target))
	if err != nil {
		return err
	}
	historyReceipt, err := latestHistoryFile(filepath.Join(absTarget, "receipts", "history"), packID, ".json")
	if err != nil {
		return err
	}
	previous, err := LoadReceipt(historyReceipt)
	if err != nil {
		return err
	}
	current, err := LoadReceipt(filepath.Join(absTarget, "receipts", packID+".json"))
	if err == nil {
		for _, item := range current.Plan.Capabilities {
			if item.Type == "skill" && item.Destination != "" && item.Status == "installed" {
				_ = os.RemoveAll(item.Destination)
			}
		}
	}
	result := ExecutePlan(previous.Plan, false)
	if _, err := WriteReceiptWithoutSnapshot(absTarget, previous.Pack, result); err != nil {
		return err
	}
	packDir := filepath.Join(absTarget, "packs", packID)
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		return err
	}
	if err := util.WriteJSON(filepath.Join(packDir, "agent-pack.json"), previous.Pack); err != nil {
		return err
	}
	if err := WriteLockfile(packDir, previous.Pack); err != nil {
		return err
	}
	fmt.Fprintf(out, "Rolled back %s using %s\n", packID, historyReceipt)
	return nil
}

func ExecutePlan(installPlan model.Plan, executePlugins bool) model.Plan {
	results := make([]model.PlanItem, 0, len(installPlan.Capabilities))
	for _, item := range installPlan.Capabilities {
		switch item.Type {
		case "skill":
			results = append(results, installSkill(item, installPlan.Target))
		case "plugin":
			results = append(results, installPlugin(item, executePlugins))
		default:
			item.Status = "recorded"
			results = append(results, item)
		}
	}
	installPlan.Capabilities = results
	return installPlan
}

func WriteReceipt(target string, pack model.Pack, installPlan model.Plan) (string, error) {
	if err := SnapshotInstall(target, pack.ID); err != nil {
		return "", err
	}
	return WriteReceiptWithoutSnapshot(target, pack, installPlan)
}

func WriteReceiptWithoutSnapshot(target string, pack model.Pack, installPlan model.Plan) (string, error) {
	receiptsDir := filepath.Join(target, "receipts")
	if err := os.MkdirAll(receiptsDir, 0o755); err != nil {
		return "", err
	}
	receiptPath := filepath.Join(receiptsDir, pack.ID+".json")
	receipt := model.Receipt{InstalledAt: time.Now().UTC().Format(time.RFC3339Nano), Pack: pack, Plan: installPlan}
	return receiptPath, util.WriteJSON(receiptPath, receipt)
}

func SnapshotInstall(target, packID string) error {
	timestamp := time.Now().UTC().Format("20060102150405.000000000")
	receiptPath := filepath.Join(target, "receipts", packID+".json")
	if _, err := os.Stat(receiptPath); err == nil {
		historyDir := filepath.Join(target, "receipts", "history")
		if err := os.MkdirAll(historyDir, 0o755); err != nil {
			return err
		}
		if err := util.CopyFile(receiptPath, filepath.Join(historyDir, packID+"-"+timestamp+".json")); err != nil {
			return err
		}
	}
	packDir := filepath.Join(target, "packs", packID)
	if _, err := os.Stat(packDir); err == nil {
		historyDir := filepath.Join(target, "packs", ".history", packID+"-"+timestamp)
		if err := util.CopyDir(packDir, historyDir); err != nil {
			return err
		}
	}
	return nil
}

func latestHistoryFile(dir, packID, suffix string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("no rollback history for %s: %w", packID, err)
	}
	matches := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, packID+"-") && strings.HasSuffix(name, suffix) {
			matches = append(matches, filepath.Join(dir, name))
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no rollback history for %s", packID)
	}
	sort.Strings(matches)
	return matches[len(matches)-1], nil
}

func LoadReceipt(path string) (model.Receipt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Receipt{}, err
	}
	var receipt model.Receipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return model.Receipt{}, err
	}
	return receipt, nil
}

func WriteLockfile(packDir string, pack model.Pack) error {
	lock := model.Lockfile{GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), Pack: pack.ID, Version: pack.Version}
	for _, capability := range pack.Capabilities {
		entry := model.LockEntry{
			Type: capability.Type, Name: capability.Name, Source: capability.Source,
			UpstreamSource: capability.UpstreamSource, Version: capability.Version,
			Revision:   resolve.ResolveSourceLive(capability.Source).Revision,
			ResolvedAt: time.Now().UTC().Format(time.RFC3339Nano),
			Integrity:  capability.Integrity,
			Digest:     resolve.DigestCapability(capability),
		}
		lock.Capabilities = append(lock.Capabilities, entry)
	}
	return util.WriteJSON(filepath.Join(packDir, "agent-pack.lock"), lock)
}

func LoadLockfile(path string) (model.Lockfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Lockfile{}, err
	}
	var lock model.Lockfile
	if err := json.Unmarshal(data, &lock); err != nil {
		return model.Lockfile{}, err
	}
	return lock, nil
}

func ListInstalled(target string, out io.Writer) error {
	receiptsDir := filepath.Join(util.ExpandHome(target), "receipts")
	entries, err := os.ReadDir(receiptsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(out, "No packs installed.")
			return nil
		}
		return err
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		receipt, err := LoadReceipt(filepath.Join(receiptsDir, entry.Name()))
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "%s\t%s\t%s\n", receipt.Pack.ID, receipt.Pack.Version, receipt.InstalledAt)
		count++
	}
	if count == 0 {
		fmt.Fprintln(out, "No packs installed.")
	}
	return nil
}

func ListInstalledReceipts(target string) ([]model.InstalledSummary, error) {
	receiptsDir := filepath.Join(util.ExpandHome(target), "receipts")
	entries, err := os.ReadDir(receiptsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	summaries := []model.InstalledSummary{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		receipt, err := LoadReceipt(filepath.Join(receiptsDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, model.InstalledSummary{
			ID: receipt.Pack.ID, Version: receipt.Pack.Version, InstalledAt: receipt.InstalledAt,
		})
	}
	return summaries, nil
}

func Uninstall(target, packID string, out io.Writer) error {
	absTarget, err := filepath.Abs(util.ExpandHome(target))
	if err != nil {
		return err
	}
	receiptPath := filepath.Join(absTarget, "receipts", packID+".json")
	receipt, err := LoadReceipt(receiptPath)
	if err != nil {
		return err
	}
	for _, item := range receipt.Plan.Capabilities {
		if item.Type == "skill" && item.Destination != "" && item.Status == "installed" {
			if err := os.RemoveAll(item.Destination); err != nil {
				return err
			}
			fmt.Fprintf(out, "Removed skill: %s\n", item.Destination)
		} else if item.Type == "plugin" {
			fmt.Fprintf(out, "Plugin requires native uninstall/manual cleanup: %s\n", item.Name)
		}
	}
	_ = os.RemoveAll(filepath.Join(absTarget, "packs", packID))
	if err := os.Remove(receiptPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	fmt.Fprintf(out, "Uninstalled %s\n", packID)
	return nil
}

func Outdated(registryPath, target string, out io.Writer) error {
	packsDir := filepath.Join(util.ExpandHome(target), "packs")
	entries, err := os.ReadDir(packsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(out, "No packs installed.")
			return nil
		}
		return err
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		lockPath := filepath.Join(packsDir, entry.Name(), "agent-pack.lock")
		lock, err := LoadLockfile(lockPath)
		if err != nil {
			fmt.Fprintf(out, "%s\tstatus=missing-lock\n", entry.Name())
			count++
			continue
		}
		registryPack, findErr := registry.FindPack(registryPath, lock.Pack)
		packVersionStatus := "current"
		if findErr != nil {
			packVersionStatus = "registry-missing"
		} else if lock.Version != registryPack.Version {
			packVersionStatus = "outdated"
			fmt.Fprintf(out, "%s\tpack-version\t%s\tlocked=%s\tregistry=%s\n", lock.Pack, packVersionStatus, lock.Version, registryPack.Version)
			count++
		}
		for _, capability := range lock.Capabilities {
			current := resolve.ResolveSourceLive(capability.Source)
			status := "current"
			if current.Warning != "" {
				status = "unresolved"
			}
			if capability.Revision != "" && current.Revision != "" && capability.Revision != current.Revision {
				status = "outdated"
			}
			fmt.Fprintf(out, "%s\t%s\t%s\tlocked=%s\tcurrent=%s\n", lock.Pack, capability.Name, status, capability.Revision, current.Revision)
			count++
		}
	}
	if count == 0 {
		fmt.Fprintln(out, "No packs installed.")
	}
	return nil
}

func OutdatedReport(registryPath, target string) (model.OutdatedReport, error) {
	report := model.OutdatedReport{}
	packsDir := filepath.Join(util.ExpandHome(target), "packs")
	entries, err := os.ReadDir(packsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return report, nil
		}
		return report, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		lockPath := filepath.Join(packsDir, entry.Name(), "agent-pack.lock")
		lock, err := LoadLockfile(lockPath)
		if err != nil {
			report.Entries = append(report.Entries, model.OutdatedEntry{
				Pack: entry.Name(), Kind: "pack", Status: "missing-lock",
			})
			continue
		}
		registryPack, findErr := registry.FindPack(registryPath, lock.Pack)
		if findErr == nil && lock.Version != registryPack.Version {
			report.Entries = append(report.Entries, model.OutdatedEntry{
				Pack: lock.Pack, Kind: "pack-version", Status: "outdated",
				Locked: lock.Version, Registry: registryPack.Version,
			})
		}
		for _, capability := range lock.Capabilities {
			current := resolve.ResolveSourceLive(capability.Source)
			status := "current"
			if current.Warning != "" {
				status = "unresolved"
			}
			if capability.Revision != "" && current.Revision != "" && capability.Revision != current.Revision {
				status = "outdated"
			}
			report.Entries = append(report.Entries, model.OutdatedEntry{
				Pack: lock.Pack, Kind: "capability", Name: capability.Name, Status: status,
				Locked: capability.Revision, Current: current.Revision,
			})
		}
	}
	return report, nil
}

func PackDiff(registryPath, target, packRef string, out io.Writer) error {
	pack, err := registry.FindPack(registryPath, packRef)
	if err != nil {
		return err
	}
	expanded, err := registry.ExpandPack(registryPath, pack, map[string]bool{})
	if err != nil {
		return err
	}
	lock, err := LoadLockfile(filepath.Join(util.ExpandHome(target), "packs", expanded.ID, "agent-pack.lock"))
	if err != nil {
		return err
	}
	diffCount := 0
	current := map[string]model.Capability{}
	for _, capability := range expanded.Capabilities {
		current[capability.Type+":"+capability.Name] = capability
	}
	seen := map[string]bool{}
	for _, entry := range lock.Capabilities {
		key := entry.Type + ":" + entry.Name
		seen[key] = true
		capability, ok := current[key]
		if !ok {
			fmt.Fprintf(out, "removed\t%s\n", key)
			diffCount++
			continue
		}
		if capability.Source != entry.Source {
			fmt.Fprintf(out, "changed\t%s\t%s -> %s\n", key, entry.Source, capability.Source)
			diffCount++
		}
	}
	for key := range current {
		if !seen[key] {
			fmt.Fprintf(out, "added\t%s\n", key)
			diffCount++
		}
	}
	if diffCount == 0 {
		fmt.Fprintf(out, "No differences for %s.\n", expanded.ID)
	}
	return nil
}

func CacheInfo(home string, out io.Writer) error {
	abs, err := filepath.Abs(util.ExpandHome(home))
	if err != nil {
		return err
	}
	for _, dir := range []string{"sources", "cache", "locks", "registries"} {
		path := filepath.Join(abs, dir)
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
		fmt.Fprintf(out, "%s\t%s\n", dir, path)
	}
	return nil
}

func CachePrune(home string, clean bool, out io.Writer) error {
	base := util.ExpandHome(home)
	dirs := []string{"cache", "locks"}
	if clean {
		dirs = append(dirs, "sources")
	}
	for _, dir := range dirs {
		path := filepath.Join(base, dir)
		if err := os.RemoveAll(path); err != nil {
			return err
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
		fmt.Fprintf(out, "cleaned\t%s\n", path)
	}
	return nil
}

func Update(home string, all bool, out io.Writer) error {
	config, err := registry.LoadRegistryConfig(home)
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
		if _, err := registry.ResolveRegistry(home, name); err != nil {
			fmt.Fprintf(out, "FAIL  %s: %s\n", name, err)
		} else {
			fmt.Fprintf(out, "OK    %s updated\n", name)
		}
	}
	return nil
}

func installSkill(item model.PlanItem, target string) model.PlanItem {
	if item.Action == "reference" {
		item.Status = "referenced"
		item.Reason = "referenced from source; not copied into target"
		return item
	}
	source, cleanup, err := resolve.MaterializeSkillSource(item.Source, target)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		item.Status = "pending"
		item.Reason = err.Error()
		return item
	}
	if item.Action == "symlink" {
		return symlinkSkillFromSource(item, source)
	}
	return copySkillFromSource(item, source)
}

func symlinkSkillFromSource(item model.PlanItem, source string) model.PlanItem {
	destination, err := filepath.Abs(util.ExpandHome(item.Destination))
	if err != nil {
		item.Status = "failed"
		item.Reason = err.Error()
		return item
	}
	if _, err := os.Stat(source); err != nil {
		item.Status = "pending"
		item.Reason = err.Error()
		return item
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		item.Status = "failed"
		item.Reason = err.Error()
		return item
	}
	if ok := handleDestinationConflict(destination, item.OnConflict, &item); !ok {
		return item
	}
	if err := os.Symlink(source, destination); err != nil {
		item.Status = "failed"
		item.Reason = err.Error()
		return item
	}
	entry := item.Entry
	if entry == "" {
		entry = "SKILL.md"
	}
	if err := resolve.VerifySkillIntegrity(source, entry, item.ExpectedChecksum, item.ExpectedSignature); err != nil {
		item.Status = "failed"
		item.Reason = err.Error()
		return item
	}
	item.Status = "installed"
	return item
}

func copySkillFromSource(item model.PlanItem, source string) model.PlanItem {
	destination, err := filepath.Abs(util.ExpandHome(item.Destination))
	if err != nil {
		item.Status = "failed"
		item.Reason = err.Error()
		return item
	}
	info, err := os.Stat(source)
	if err != nil {
		item.Status = "pending"
		item.Reason = fmt.Sprintf("skill source unavailable: %v", err)
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
	if ok := handleDestinationConflict(destination, item.OnConflict, &item); !ok {
		return item
	}
	if info.IsDir() {
		err = util.CopyDir(source, destination)
	} else {
		err = os.MkdirAll(destination, 0o755)
		if err == nil {
			err = util.CopyFile(source, filepath.Join(destination, filepath.Base(entryPath)))
		}
	}
	if err != nil {
		item.Status = "failed"
		item.Reason = err.Error()
		return item
	}
	if err := resolve.VerifySkillIntegrity(destination, entry, item.ExpectedChecksum, item.ExpectedSignature); err != nil {
		item.Status = "failed"
		item.Reason = err.Error()
		return item
	}
	item.Status = "installed"
	return item
}

func handleDestinationConflict(destination, onConflict string, item *model.PlanItem) bool {
	if _, err := os.Lstat(destination); os.IsNotExist(err) {
		return true
	}
	switch onConflict {
	case "skip":
		item.Status = "skipped"
		item.Reason = "destination exists"
		return false
	case "backup":
		backup := destination + ".bak." + time.Now().UTC().Format("20060102150405")
		if err := os.Rename(destination, backup); err != nil {
			item.Status = "failed"
			item.Reason = err.Error()
			return false
		}
		item.Reason = "existing destination backed up to " + backup
		return true
	case "overwrite":
		if err := os.RemoveAll(destination); err != nil {
			item.Status = "failed"
			item.Reason = err.Error()
			return false
		}
		return true
	default:
		item.Status = "failed"
		item.Reason = "invalid conflict policy: " + onConflict
		return false
	}
}
