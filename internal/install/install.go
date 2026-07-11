package install

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/agent-packs/cli/internal/merge"
	"github.com/agent-packs/cli/internal/model"
	"github.com/agent-packs/cli/internal/plan"
	"github.com/agent-packs/cli/internal/registry"
	"github.com/agent-packs/cli/internal/resolve"
	"github.com/agent-packs/cli/internal/util"
)

func Install(registryPath, home, packRef, target, agent, only string, executePlugins, dryRun bool, out io.Writer) error {
	return InstallWithOptions(registryPath, home, packRef, target, agent, only, executePlugins, dryRun,
		model.InstallOptions{Mode: "copy", OnConflict: "overwrite", Scope: "target"}, out)
}

func InstallWithOptions(registryPath, home, packRef, target, agent, only string, executePlugins, dryRun bool, options model.InstallOptions, out io.Writer) error {
	return InstallWithOptionsAndMinTrust(registryPath, home, packRef, target, agent, only, executePlugins, dryRun, options, "", out)
}

func InstallWithOptionsAndMinTrust(registryPath, home, packRef, target, agent, only string, executePlugins, dryRun bool, options model.InstallOptions, minTrust string, out io.Writer) error {
	pack, sourceRegistry, err := registry.ResolvePack(registryPath, home, packRef)
	if err != nil {
		return err
	}
	if pack.Deprecated || pack.Stability == "deprecated" {
		msg := fmt.Sprintf("WARN  pack %q is deprecated", pack.ID)
		if pack.Replacement != "" {
			msg += fmt.Sprintf(" — consider using %q instead", pack.Replacement)
		}
		fmt.Fprintln(out, msg)
	}
	if minTrust != "" {
		if err := checkTrustLevel(pack, minTrust); err != nil {
			return err
		}
	}
	if err := checkConflicts(pack, target); err != nil {
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
	// Retract any merge fragments from a prior install of this pack so a
	// re-install/upgrade starts from a clean file and recomputes ownership.
	if err := retractExistingMergeItems(absTarget, expanded.ID); err != nil {
		return err
	}
	result := ExecutePlan(installPlan, executePlugins)
	registryCommit := registry.CheckoutCommit(registryPath)
	receiptPath, err := WriteReceipt(absTarget, expanded, result, registryCommit)
	if err != nil {
		return err
	}
	if err := WriteLockfile(packDir, expanded, registryCommit); err != nil {
		return err
	}
	plan.PrintPlan(result, out)
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Receipt: %s\n", receiptPath)
	if options.Mode == "reference" {
		// Reference mode is deliberately a record-only install; make that
		// unmistakable so a first run doesn't read as a silent no-op.
		fmt.Fprintln(out, "Recorded only (reference mode): no files were installed into the agent.")
		fmt.Fprintln(out, "Re-run with --mode copy to materialize skills and managed files.")
	}
	for _, item := range result.Capabilities {
		if item.Status == "failed" {
			return model.ErrInstallFailed
		}
	}
	return nil
}

func Upgrade(registryPath, home, packRef, target string, executePlugins, executeMCPs bool, out io.Writer) error {
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
		Mode:        receipt.Plan.Mode,
		OnConflict:  receipt.Plan.OnConflict,
		Scope:       receipt.Plan.Scope,
		ExecuteMCPs: executeMCPs,
	}
	// Replay the capability filter recorded at install time. Older receipts
	// predate the field; default to "all" so an upgrade never silently drops
	// capability types the original install materialized.
	only := receipt.Plan.Only
	if only == "" {
		only = "all"
	}
	fmt.Fprintf(out, "Upgrading %s (mode=%s, conflict=%s, scope=%s, only=%s)\n", packRef, options.Mode, options.OnConflict, options.Scope, only)
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
	// Pre-flight: verify local skill sources before modifying anything so a
	// missing source doesn't leave the environment empty after removal.
	for _, item := range previous.Plan.Capabilities {
		if item.Type == "skill" && item.Action != "reference" && item.Source != "" && util.IsLocalSource(item.Source) {
			if _, statErr := os.Stat(util.ExpandHome(item.Source)); statErr != nil {
				return fmt.Errorf("rollback aborted: skill %q source %q unavailable: %w", item.Name, item.Source, statErr)
			}
		} else if isManagedFileType(item.Type) && item.Action != "record" && item.Source != "" && util.IsLocalSource(item.Source) {
			if _, statErr := os.Stat(util.ExpandHome(item.Source)); statErr != nil {
				return fmt.Errorf("rollback aborted: %s %q source %q unavailable: %w", item.Type, item.Name, item.Source, statErr)
			}
		}
	}
	current, err := LoadReceipt(filepath.Join(absTarget, "receipts", packID+".json"))
	if err == nil {
		for _, item := range current.Plan.Capabilities {
			if item.Type == "skill" && item.Destination != "" && item.Status == "installed" {
				_ = os.RemoveAll(item.Destination)
			} else if isManagedFileType(item.Type) && item.Destination != "" && item.Status == "installed" {
				_ = os.Remove(item.Destination)
			} else if item.Type == "memory" || item.Type == "settings" || item.Type == "mcp" {
				_ = retractMergeItem(item)
			}
		}
	}
	result := ExecutePlan(previous.Plan, false)
	if _, err := WriteReceiptWithoutSnapshot(absTarget, previous.Pack, result, previous.RegistryCommit); err != nil {
		return err
	}
	packDir := filepath.Join(absTarget, "packs", packID)
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		return err
	}
	if err := util.WriteJSON(filepath.Join(packDir, "agent-pack.json"), previous.Pack); err != nil {
		return err
	}
	if err := WriteLockfile(packDir, previous.Pack, previous.RegistryCommit); err != nil {
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
		case "memory", "settings":
			results = append(results, installMerge(item))
		case "mcp":
			results = append(results, installMCP(item))
		case "command", "hook", "subagent", "prompt", "template", "tool":
			results = append(results, installManagedFile(item))
		default:
			item.Status = "recorded"
			results = append(results, item)
		}
	}
	installPlan.Capabilities = results
	return installPlan
}

func WriteReceipt(target string, pack model.Pack, installPlan model.Plan, registryCommit string) (string, error) {
	if err := SnapshotInstall(target, pack.ID); err != nil {
		return "", err
	}
	return WriteReceiptWithoutSnapshot(target, pack, installPlan, registryCommit)
}

func WriteReceiptWithoutSnapshot(target string, pack model.Pack, installPlan model.Plan, registryCommit string) (string, error) {
	receiptsDir := filepath.Join(target, "receipts")
	if err := os.MkdirAll(receiptsDir, 0o755); err != nil {
		return "", err
	}
	receiptPath := filepath.Join(receiptsDir, pack.ID+".json")
	receipt := model.Receipt{
		InstalledAt:    time.Now().UTC().Format(time.RFC3339Nano),
		RegistryCommit: registryCommit,
		Pack:           pack,
		Plan:           installPlan,
	}
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

func WriteLockfile(packDir string, pack model.Pack, registryCommit string) error {
	lock := model.Lockfile{
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339Nano),
		RegistryCommit: registryCommit,
		Pack:           pack.ID,
		Version:        pack.Version,
	}
	for _, capability := range pack.Capabilities {
		entry := model.LockEntry{
			Type: capability.Type, Name: capability.Name, Source: capability.Source,
			UpstreamSource: capability.UpstreamSource, Version: capability.Version,
			Revision:   resolve.ResolveSource(capability.Source).Revision,
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
	return UninstallWithOptions(target, packID, false, false, out)
}

func UninstallWithOptions(target, packID string, executePlugins, executeMCPs bool, out io.Writer) error {
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
		} else if isManagedFileType(item.Type) && item.Destination != "" && item.Status == "installed" {
			if err := os.Remove(item.Destination); err != nil && !os.IsNotExist(err) {
				return err
			}
			fmt.Fprintf(out, "Removed %s: %s\n", item.Type, item.Destination)
		} else if item.Type == "memory" || item.Type == "settings" || item.Type == "mcp" {
			if err := retractMergeItem(item); err != nil {
				return err
			}
			if item.Status == "installed" {
				fmt.Fprintf(out, "Removed %s from: %s\n", item.Type, item.Destination)
			}
			if item.Type == "mcp" {
				result := uninstallMCP(item, executeMCPs) // executeMCPs is passed because it maps to execution allowed
				if result.Status == "uninstalled" {
					fmt.Fprintf(out, "Uninstalled MCP: %s\n", item.Name)
				} else if result.Status == "pending" {
					fmt.Fprintf(out, "MCP requires native uninstall/manual cleanup: %s\n", item.Name)
				}
			}
		} else if item.Type == "plugin" {
			result := uninstallPlugin(item, executePlugins)
			if result.Status == "uninstalled" {
				fmt.Fprintf(out, "Uninstalled plugin: %s\n", item.Name)
			} else if result.Status == "pending" {
				fmt.Fprintf(out, "Plugin requires native uninstall/manual cleanup: %s\n", item.Name)
			} else if result.Status == "failed" {
				return fmt.Errorf("plugin uninstall failed for %s: %s", item.Name, result.Reason)
			}
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
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
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
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
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
	lockPath := filepath.Join(util.ExpandHome(target), "packs", expanded.ID, "agent-pack.lock")
	lock, err := LoadLockfile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("pack %q is not installed — run `agent-packs install %s` first", expanded.ID, expanded.ID)
		}
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

// PinStatus is the pin verification state of one locked capability: "ok" when
// the live source still matches the recorded pin, "unpinned" when no pin was
// ever recorded, "changed" when the live revision or checksum drifted, and
// "unverifiable" when a recorded pin could not be re-resolved (unreachable or
// deleted source). Unverifiable pins fail verification: an attacker who takes
// a source offline must not produce a passing check.
type PinStatus struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Detail string `json:"detail,omitempty"`
}

// pinCheckWorkers bounds the parallelism of live pin verification. Each check
// may hit the network (ls-remote, clone), so a small pool keeps a many-
// capability pack fast without hammering the remotes.
const pinCheckWorkers = 4

// PinCheckResults re-resolves each locked capability's live source and reports
// whether it still matches the pins recorded in the pack lockfile.
func PinCheckResults(registryPath, target, packRef string) ([]PinStatus, error) {
	expanded, lock, _, err := loadPackLock(registryPath, target, packRef)
	if err != nil {
		return nil, err
	}
	capByKey := map[string]model.Capability{}
	for _, capability := range expanded.Capabilities {
		capByKey[capability.Type+":"+capability.Name] = capability
	}
	results := make([]PinStatus, len(lock.Capabilities))
	var wg sync.WaitGroup
	sem := make(chan struct{}, pinCheckWorkers)
	for i := range lock.Capabilities {
		entry := &lock.Capabilities[i]
		if entry.Revision == "" && entry.Integrity.Checksum == "" {
			results[i] = PinStatus{Name: entry.Name, State: "unpinned",
				Detail: fmt.Sprintf("run `agent-packs pin %s` to record a pin", expanded.ID)}
			continue
		}
		wg.Add(1)
		go func(i int, entry *model.LockEntry) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = checkPinEntry(entry, capByKey[entry.Type+":"+entry.Name], target)
		}(i, entry)
	}
	wg.Wait()
	return results, nil
}

// checkPinEntry verifies one recorded pin against the live source, failing
// closed: any recorded value that cannot be re-resolved is "unverifiable".
func checkPinEntry(entry *model.LockEntry, capability model.Capability, target string) PinStatus {
	live, resolveErr := resolvePin(entry, capability, target)
	if resolveErr != nil {
		return PinStatus{Name: entry.Name, State: "unverifiable",
			Detail: fmt.Sprintf("recorded pin could not be re-resolved: %v", resolveErr)}
	}
	if entry.Revision != "" && live.revision == "" {
		return PinStatus{Name: entry.Name, State: "unverifiable",
			Detail: "recorded revision pin, but the live revision could not be resolved"}
	}
	if entry.Integrity.Checksum != "" && live.checksum == "" {
		return PinStatus{Name: entry.Name, State: "unverifiable",
			Detail: "recorded checksum pin, but the live content could not be fetched"}
	}
	var details []string
	if entry.Revision != "" && live.revision != "" && live.revision != entry.Revision {
		details = append(details, fmt.Sprintf("revision %s → %s", shortHash(entry.Revision), shortHash(live.revision)))
	}
	if entry.Integrity.Checksum != "" && live.checksum != "" && live.checksum != entry.Integrity.Checksum {
		details = append(details, fmt.Sprintf("checksum %s → %s", shortHash(entry.Integrity.Checksum), shortHash(live.checksum)))
	}
	if len(details) > 0 {
		return PinStatus{Name: entry.Name, State: "changed", Detail: strings.Join(details, "; ")}
	}
	return PinStatus{Name: entry.Name, State: "ok"}
}

func loadPackLock(registryPath, target, packRef string) (model.Pack, model.Lockfile, string, error) {
	pack, err := registry.FindPack(registryPath, packRef)
	if err != nil {
		return model.Pack{}, model.Lockfile{}, "", err
	}
	expanded, err := registry.ExpandPack(registryPath, pack, map[string]bool{})
	if err != nil {
		return model.Pack{}, model.Lockfile{}, "", err
	}
	lockPath := filepath.Join(util.ExpandHome(target), "packs", expanded.ID, "agent-pack.lock")
	lock, err := LoadLockfile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return model.Pack{}, model.Lockfile{}, "", fmt.Errorf("pack %q is not installed — run `agent-packs install %s` first", expanded.ID, expanded.ID)
		}
		return model.Pack{}, model.Lockfile{}, "", err
	}
	return expanded, lock, lockPath, nil
}

// PinPack resolves each installed capability's source to a concrete revision
// and content checksum, recording them in the pack lockfile so future operations
// can detect drift or tampering. With check=true it re-resolves the live source
// and verifies it still matches the recorded pins instead of rewriting them.
func PinPack(registryPath, target, packRef string, check bool, out io.Writer) error {
	if check {
		results, err := PinCheckResults(registryPath, target, packRef)
		if err != nil {
			return err
		}
		failed := 0
		for _, result := range results {
			switch result.State {
			case "unpinned":
				fmt.Fprintf(out, "UNPINNED %s — %s\n", result.Name, result.Detail)
			case "changed":
				fmt.Fprintf(out, "CHANGED  %s — %s\n", result.Name, result.Detail)
				failed++
			case "unverifiable":
				fmt.Fprintf(out, "UNVERIFIABLE %s — %s\n", result.Name, result.Detail)
				failed++
			default:
				fmt.Fprintf(out, "OK       %s\n", result.Name)
			}
		}
		fmt.Fprintln(out)
		if failed > 0 {
			fmt.Fprintf(out, "%d/%d capabilities drifted from their pins or could not be verified\n", failed, len(results))
			return model.ErrInstallFailed
		}
		fmt.Fprintf(out, "All %d capabilities match their pins\n", len(results))
		return nil
	}

	expanded, lock, lockPath, err := loadPackLock(registryPath, target, packRef)
	if err != nil {
		return err
	}
	capByKey := map[string]model.Capability{}
	for _, capability := range expanded.Capabilities {
		capByKey[capability.Type+":"+capability.Name] = capability
	}

	pinned := 0
	var warnings []string
	for i := range lock.Capabilities {
		entry := &lock.Capabilities[i]
		live, err := resolvePin(entry, capByKey[entry.Type+":"+entry.Name], target)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", entry.Name, err))
		}
		if live.revision != "" {
			entry.Revision = live.revision
		}
		if live.checksum != "" {
			entry.Integrity.Checksum = live.checksum
		}
		entry.ResolvedAt = time.Now().UTC().Format(time.RFC3339Nano)
		pinned++
		fmt.Fprintf(out, "pinned   %s  rev=%s  checksum=%s\n", entry.Name, shortHash(entry.Revision), shortHash(entry.Integrity.Checksum))
	}
	if err := util.WriteJSON(lockPath, lock); err != nil {
		return err
	}
	for _, warning := range warnings {
		fmt.Fprintf(out, "WARN  could not fully resolve %s\n", warning)
	}
	fmt.Fprintf(out, "\nPinned %d capabilities into %s\n", pinned, lockPath)
	if len(warnings) > 0 {
		return fmt.Errorf("%d capability source(s) could not be resolved while pinning: %w", len(warnings), model.ErrInstallFailed)
	}
	return nil
}

// pinResolution carries the live values for one lock entry: the source's
// current revision and the content checksum of the materialized skill tree.
type pinResolution struct {
	revision string
	checksum string
}

// resolvePin returns the live revision and content checksum for a lock entry.
// A moving ref resolves to its current commit SHA via `git ls-remote`; a
// skill's content is hashed after materializing the source — as a whole-tree
// dirsha256 digest by default, or as an entry-file sha256 when the recorded
// pin used that older format. The error reports any live value that could not
// be resolved so callers can fail closed.
func resolvePin(entry *model.LockEntry, capability model.Capability, target string) (pinResolution, error) {
	var live pinResolution
	var errs []string
	resolution, err := resolve.ResolveSourceLiveErr(entry.Source)
	if err != nil {
		errs = append(errs, fmt.Sprintf("live revision unavailable: %v", err))
	} else {
		live.revision = resolution.Revision
	}
	if entry.Type == "skill" {
		dir, cleanup, err := resolve.MaterializeSkillSource(entry.Source, util.ExpandHome(target))
		if cleanup != nil {
			defer cleanup()
		}
		if err != nil {
			errs = append(errs, fmt.Sprintf("source unavailable for checksum: %v", err))
		} else {
			live.checksum, err = hashPinContent(dir, capability.Entry, entry.Integrity.Checksum)
			if err != nil {
				errs = append(errs, fmt.Sprintf("checksum unavailable: %v", err))
			}
		}
	}
	if len(errs) > 0 {
		return live, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return live, nil
}

// hashPinContent hashes a materialized skill source in the same format as the
// recorded pin: legacy sha256: pins hash the entry file only, everything else
// (including fresh pins) hashes the whole tree so every skill file is covered.
func hashPinContent(dir, entry, recorded string) (string, error) {
	if strings.HasPrefix(recorded, "sha256:") {
		if entry == "" {
			entry = "SKILL.md"
		}
		path := dir
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			path = filepath.Join(dir, entry)
		}
		return resolve.HashFile(path)
	}
	return resolve.HashTree(dir)
}

func shortHash(s string) string {
	if s == "" {
		return "-"
	}
	if len(s) > 12 {
		return s[:12]
	}
	return s
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

// installMerge materializes a memory/settings capability by injecting its
// fragment into the agent's shared file. In record mode (reference) nothing is
// written; unsupported items pass through untouched.
func installMerge(item model.PlanItem) model.PlanItem {
	if item.Status == "unsupported" {
		return item
	}
	if item.Action == "record" {
		item.Status = "recorded"
		item.Reason = "reference mode; not merged into target (use --mode copy to apply)"
		return item
	}
	content, err := mergeContent(item)
	if err != nil {
		item.Status = "failed"
		item.Reason = err.Error()
		return item
	}
	dest, err := filepath.Abs(util.ExpandHome(item.Destination))
	if err != nil {
		item.Status = "failed"
		item.Reason = err.Error()
		return item
	}
	err = util.WithFileLock(dest, func() error {
		switch item.FileKind {
		case "markdown":
			res, err := merge.ApplyMarkdownBlock(dest, item.BlockID, content)
			if err != nil {
				return err
			}
			item.ContentHash = res.ContentHash
		case "json":
			res, err := merge.ApplyJSONMerge(dest, item.MergeKey, []byte(content))
			if err != nil {
				return err
			}
			item.ContentHash = res.ContentHash
			item.OwnedKeys = res.OwnedKeys
		case "toml":
			res, err := merge.ApplyTOMLMerge(dest, item.MergeKey, []byte(content))
			if err != nil {
				return err
			}
			item.ContentHash = res.ContentHash
			item.OwnedKeys = res.OwnedKeys
		default:
			return fmt.Errorf("unknown merge file kind %q", item.FileKind)
		}
		return nil
	})
	if err != nil {
		item.Status = "failed"
		item.Reason = err.Error()
		return item
	}
	item.Status = "installed"
	return item
}

// retractMergeItem removes a previously merged fragment from the shared file,
// using the receipt's ownership record so only pack-added content is touched.
func retractMergeItem(item model.PlanItem) error {
	if item.Destination == "" || item.Status != "installed" {
		return nil
	}
	dest, err := filepath.Abs(util.ExpandHome(item.Destination))
	if err != nil {
		return err
	}
	return util.WithFileLock(dest, func() error {
		switch item.FileKind {
		case "markdown":
			return merge.RetractMarkdownBlock(dest, item.BlockID)
		case "json":
			return merge.RetractJSONMerge(dest, item.MergeKey, item.OwnedKeys)
		case "toml":
			return merge.RetractTOMLMerge(dest, item.MergeKey, item.OwnedKeys)
		default:
			return nil
		}
	})
}

// mergeContent returns the fragment to inject: the inline Content if set,
// otherwise the contents of the capability's local Source file.
func mergeContent(item model.PlanItem) (string, error) {
	if item.Content != "" {
		return item.Content, nil
	}
	if item.Source == "" {
		return "", fmt.Errorf("%s capability %q has neither inline content nor a source", item.Type, item.Name)
	}
	data, err := os.ReadFile(util.ExpandHome(item.Source))
	if err != nil {
		return "", fmt.Errorf("reading %s source: %w", item.Type, err)
	}
	return string(data), nil
}

// retractExistingMergeItems retracts the merge fragments recorded in an existing
// receipt for packID, so that a re-install or upgrade starts from a clean file
// and recomputes ownership correctly. Missing receipt is a no-op.
func retractExistingMergeItems(absTarget, packID string) error {
	receipt, err := LoadReceipt(filepath.Join(absTarget, "receipts", packID+".json"))
	if err != nil {
		return nil
	}
	for _, item := range receipt.Plan.Capabilities {
		if item.Type == "memory" || item.Type == "settings" {
			if err := retractMergeItem(item); err != nil {
				return err
			}
		} else if isManagedFileType(item.Type) && item.Destination != "" && item.Status == "installed" {
			if err := os.Remove(item.Destination); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

func isManagedFileType(capType string) bool {
	return capType == "command" || capType == "hook" || capType == "subagent" || capType == "prompt" || capType == "template" || capType == "tool"
}

func installManagedFile(item model.PlanItem) model.PlanItem {
	if item.Status == "unsupported" {
		return item
	}
	if item.Action == "record" {
		item.Status = "recorded"
		if item.Reason == "" {
			item.Reason = "reference mode; not copied into target (use --mode copy to apply)"
		}
		return item
	}
	content, mode, err := managedFileContent(item)
	if err != nil {
		item.Status = "failed"
		item.Reason = err.Error()
		return item
	}
	destination, err := filepath.Abs(util.ExpandHome(item.Destination))
	if err != nil {
		item.Status = "failed"
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
	if err := os.WriteFile(destination, []byte(content), mode); err != nil {
		item.Status = "failed"
		item.Reason = err.Error()
		return item
	}
	item.ContentHash = hashString(content)
	item.Status = "installed"
	return item
}

func managedFileContent(item model.PlanItem) (string, os.FileMode, error) {
	if item.Content != "" {
		return item.Content, 0o644, nil
	}
	if item.Source == "" {
		return "", 0, fmt.Errorf("%s capability %q has neither inline content nor a source", item.Type, item.Name)
	}
	path, cleanup, err := resolve.MaterializeSkillSource(item.Source, filepath.Dir(util.ExpandHome(item.Destination)))
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return "", 0, fmt.Errorf("materializing %s source: %w", item.Type, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", 0, fmt.Errorf("reading %s source: %w", item.Type, err)
	}
	if info.IsDir() {
		entry := item.Entry
		if entry == "" {
			return "", 0, fmt.Errorf("%s capability %q source is a directory; entry is required", item.Type, item.Name)
		}
		path = filepath.Join(path, entry)
		info, err = os.Stat(path)
		if err != nil {
			return "", 0, fmt.Errorf("reading %s source entry: %w", item.Type, err)
		}
		if info.IsDir() {
			return "", 0, fmt.Errorf("%s source entry must be a file: %s", item.Type, entry)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, fmt.Errorf("reading %s source: %w", item.Type, err)
	}
	mode := info.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	return string(data), mode, nil
}

func hashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum[:])
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
		// An unreachable source is a hard failure: a network error must not
		// let the install exit 0 with nothing materialized.
		item.Status = "failed"
		item.Reason = fmt.Sprintf("skill source unavailable: %v", err)
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
		item.Status = "failed"
		item.Reason = fmt.Sprintf("skill source unavailable: %v", err)
		return item
	}
	entry := item.Entry
	if entry == "" {
		entry = "SKILL.md"
	}
	// Verify integrity against the source before linking it into the agent's
	// live skill directory, so tampered content is never exposed to the agent.
	if err := resolve.VerifySkillIntegrity(source, entry, item.ExpectedChecksum, item.ExpectedSignature); err != nil {
		item.Status = "failed"
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
		item.Status = "failed"
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
	// Verify integrity against the materialized source before any file lands
	// in the destination, so a checksum mismatch never leaves tampered content
	// in the agent's live skill directory.
	if err := resolve.VerifySkillIntegrity(source, entry, item.ExpectedChecksum, item.ExpectedSignature); err != nil {
		item.Status = "failed"
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

// trustOrder ranks the registry schema's trust taxonomy (see
// internal/validate/trust.go): official > verified > community > unknown.
var trustOrder = map[string]int{"official": 3, "verified": 2, "community": 1, "unknown": 0, "": 0}

func checkTrustLevel(pack model.Pack, minTrust string) error {
	if _, ok := trustOrder[minTrust]; !ok {
		return fmt.Errorf("invalid --min-trust %q: expected official, verified, community, or unknown", minTrust)
	}
	packLevel := trustOrder[pack.Trust]
	required := trustOrder[minTrust]
	if packLevel < required {
		t := pack.Trust
		if t == "" {
			t = "unknown"
		}
		return fmt.Errorf("pack %q trust level %q is below required %q", pack.ID, t, minTrust)
	}
	return nil
}

func checkConflicts(pack model.Pack, target string) error {
	if len(pack.ConflictsWith) == 0 {
		return nil
	}
	receiptsDir := filepath.Join(target, "receipts")
	entries, err := os.ReadDir(receiptsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	installed := map[string]bool{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			id := strings.TrimSuffix(entry.Name(), ".json")
			installed[id] = true
		}
	}
	for _, conflict := range pack.ConflictsWith {
		if installed[conflict] {
			return fmt.Errorf("pack %q conflicts with installed pack %q — uninstall %q first", pack.ID, conflict, conflict)
		}
	}
	return nil
}
