package install

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agent-packs/cli/internal/model"
	"github.com/agent-packs/cli/internal/plan"
	"github.com/agent-packs/cli/internal/registry"
	"github.com/agent-packs/cli/internal/targets"
	"github.com/agent-packs/cli/internal/util"
)

func InstallStandalone(registryPath, ref, kind, target, agent string, executePlugins, dryRun bool, options model.InstallOptions, out io.Writer) error {
	return InstallStandaloneWithOverrides(registryPath, ref, kind, target, agent, executePlugins, dryRun, options, nil, out)
}

func InstallStandaloneWithOverrides(registryPath, ref, kind, target, agent string, executePlugins, dryRun bool, options model.InstallOptions, installOverrides map[string]string, out io.Writer) error {
	capability, id, err := resolveStandaloneCapability(registryPath, ref, kind)
	if err != nil {
		return err
	}
	if kind == "plugins" && len(installOverrides) > 0 {
		if capability.Install == nil {
			capability.Install = map[string]string{}
		}
		for key, value := range installOverrides {
			if value != "" {
				capability.Install[key] = value
			}
		}
	}
	absTarget, err := filepath.Abs(util.ExpandHome(target))
	if err != nil {
		return err
	}
	pack := standalonePack(kind, id, capability)
	installPlan := plan.BuildInstallPlanWithOptions(pack, absTarget, agent, kind, options)
	if dryRun {
		plan.PrintPlan(installPlan, out)
		return nil
	}
	if err := os.MkdirAll(absTarget, 0o755); err != nil {
		return err
	}
	result := ExecutePlan(installPlan, executePlugins)
	receiptPath, err := WriteStandaloneReceipt(absTarget, kind, id, pack, result)
	if err != nil {
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

func UpgradeStandalone(target, id, kind string, executePlugins bool, out io.Writer) error {
	absTarget, err := filepath.Abs(util.ExpandHome(target))
	if err != nil {
		return err
	}
	receipt, err := LoadStandaloneReceipt(absTarget, kind, id)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Upgrading %s %s (mode=%s, conflict=%s, scope=%s)\n", singularKind(kind), id, receipt.Plan.Mode, receipt.Plan.OnConflict, receipt.Plan.Scope)
	result := ExecutePlan(receipt.Plan, executePlugins)
	if _, err := WriteStandaloneReceipt(absTarget, kind, id, receipt.Pack, result); err != nil {
		return err
	}
	plan.PrintPlan(result, out)
	for _, item := range result.Capabilities {
		if item.Status == "failed" {
			return model.ErrInstallFailed
		}
	}
	return nil
}

// listedItem is one row of `skills list` / `plugins list`, annotated with where
// the capability came from so that items installed outside the standalone
// receipt path are still surfaced.
type listedItem struct {
	ID          string
	Version     string
	Source      string
	InstalledAt string
}

// Discovery precedence: a capability tracked by a standalone receipt wins over
// the same id seen in a pack receipt, which in turn wins over a bare on-disk
// directory with no receipt at all.
const (
	sourceManaged  = 1
	sourcePack     = 2
	sourceExternal = 3
)

func ListStandalone(target, kind string, out io.Writer) error {
	absTarget := util.ExpandHome(target)
	items := map[string]listedItem{}
	priority := map[string]int{}
	add := func(item listedItem, prio int) {
		if existing, ok := priority[item.ID]; ok && existing <= prio {
			return
		}
		items[item.ID] = item
		priority[item.ID] = prio
	}

	for _, item := range standaloneReceiptItems(absTarget, kind) {
		add(item, sourceManaged)
	}
	for _, item := range packReceiptItems(absTarget, kind) {
		add(item, sourcePack)
	}
	if kind == "skills" {
		for _, item := range diskSkillItems(absTarget) {
			add(item, sourceExternal)
		}
	}

	if len(items) == 0 {
		fmt.Fprintf(out, "No %s installed.\n", kind)
		return nil
	}
	ids := make([]string, 0, len(items))
	for id := range items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	fmt.Fprintln(out, "id\tversion\tsource\tinstalled")
	for _, id := range ids {
		item := items[id]
		fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", item.ID, dashIfEmpty(item.Version), item.Source, dashIfEmpty(item.InstalledAt))
	}
	return nil
}

// standaloneReceiptItems lists capabilities tracked under receipts/<kind>/.
func standaloneReceiptItems(target, kind string) []listedItem {
	dir := standaloneReceiptsDir(target, kind)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	items := []listedItem{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		receipt, err := LoadReceipt(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		items = append(items, listedItem{
			ID:          strings.TrimSuffix(entry.Name(), ".json"),
			Version:     receipt.Pack.Version,
			Source:      "managed",
			InstalledAt: receipt.InstalledAt,
		})
	}
	return items
}

// packReceiptItems lists capabilities of the requested kind that were
// materialized as part of a pack install (receipts/<pack-id>.json).
func packReceiptItems(target, kind string) []listedItem {
	dir := filepath.Join(target, "receipts")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	want := singularKind(kind)
	items := []listedItem{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		receipt, err := LoadReceipt(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		versions := capabilityVersions(receipt.Pack)
		for _, capability := range receipt.Plan.Capabilities {
			if capability.Type != want {
				continue
			}
			id := util.Slugify(capability.Name)
			items = append(items, listedItem{
				ID:          id,
				Version:     versions[capability.Name],
				Source:      "pack:" + receipt.Pack.ID,
				InstalledAt: receipt.InstalledAt,
			})
		}
	}
	return items
}

// diskSkillItems discovers skill directories present under any tool's skill
// root that are not tracked by a receipt (e.g. installed by hand).
func diskSkillItems(target string) []listedItem {
	items := []listedItem{}
	seen := map[string]bool{}
	for _, root := range targets.AllSkillRoots(target) {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if _, err := os.Stat(filepath.Join(root, entry.Name(), "SKILL.md")); err != nil {
				continue
			}
			id := entry.Name()
			if seen[id] {
				continue
			}
			seen[id] = true
			items = append(items, listedItem{ID: id, Source: "external"})
		}
	}
	return items
}

// externalUninstallHint returns guidance when id is present but not tracked by
// a standalone receipt, so uninstall can point at the right command instead of
// silently failing or deleting capabilities owned by a pack.
func externalUninstallHint(target, kind, id string) string {
	for _, item := range packReceiptItems(target, kind) {
		if item.ID == util.Slugify(id) {
			packID := strings.TrimPrefix(item.Source, "pack:")
			return fmt.Sprintf("it was installed by pack %q — run `agent-packs uninstall %s`", packID, packID)
		}
	}
	if kind == "skills" {
		for _, item := range diskSkillItems(target) {
			if item.ID == id {
				return "it exists on disk outside agent-packs — remove the skill directory manually"
			}
		}
	}
	return ""
}

func capabilityVersions(pack model.Pack) map[string]string {
	versions := map[string]string{}
	for _, capability := range pack.Capabilities {
		versions[capability.Name] = capability.Version
	}
	return versions
}

func dashIfEmpty(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func UninstallStandalone(target, id, kind string, executePlugins bool, out io.Writer) error {
	absTarget, err := filepath.Abs(util.ExpandHome(target))
	if err != nil {
		return err
	}
	receiptPath := standaloneReceiptPath(absTarget, kind, id)
	receipt, err := LoadReceipt(receiptPath)
	if err != nil {
		if os.IsNotExist(err) {
			if hint := externalUninstallHint(absTarget, kind, id); hint != "" {
				return fmt.Errorf("%s %q was not installed via `agent-packs %s install`; %s", singularKind(kind), id, kind, hint)
			}
			return fmt.Errorf("%s %q is not installed (no receipt at %s)", singularKind(kind), id, receiptPath)
		}
		return err
	}
	for _, item := range receipt.Plan.Capabilities {
		if item.Type == "skill" && item.Destination != "" && item.Status == "installed" {
			if err := os.RemoveAll(item.Destination); err != nil {
				return err
			}
			fmt.Fprintf(out, "Removed skill: %s\n", item.Destination)
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
	if err := os.Remove(receiptPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	fmt.Fprintf(out, "Uninstalled %s %s\n", singularKind(kind), id)
	return nil
}

func WriteStandaloneReceipt(target, kind, id string, pack model.Pack, installPlan model.Plan) (string, error) {
	receiptPath := standaloneReceiptPath(target, kind, id)
	if err := os.MkdirAll(filepath.Dir(receiptPath), 0o755); err != nil {
		return "", err
	}
	receipt := model.Receipt{InstalledAt: time.Now().UTC().Format(time.RFC3339Nano), Pack: pack, Plan: installPlan}
	return receiptPath, util.WriteJSON(receiptPath, receipt)
}

func LoadStandaloneReceipt(target, kind, id string) (model.Receipt, error) {
	return LoadReceipt(standaloneReceiptPath(target, kind, id))
}

func resolveStandaloneCapability(registryPath, ref, kind string) (model.Capability, string, error) {
	if kind != "skills" && kind != "plugins" {
		return model.Capability{}, "", fmt.Errorf("unsupported standalone capability kind: %s", kind)
	}
	expanded := util.ExpandHome(ref)
	if _, err := os.Stat(expanded); err == nil {
		capability, err := capabilityFromLocalPath(expanded, kind)
		if err != nil {
			return model.Capability{}, "", err
		}
		capability.Reference = false
		return capability, util.Slugify(capability.Name), nil
	}
	capability, err := registry.FindCapability(registryPath, kind, ref)
	if err != nil {
		return model.Capability{}, "", err
	}
	capability.Reference = false
	return capability, ref, nil
}

func capabilityFromLocalPath(path, kind string) (model.Capability, error) {
	if kind == "skills" {
		skillPath := path
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			skillPath = filepath.Join(path, "SKILL.md")
		}
		manifest, err := registry.LoadSkillManifest(skillPath)
		if err != nil {
			return model.Capability{}, err
		}
		capability := registry.SkillCapability(util.Slugify(manifest.Name), skillPath, manifest)
		capability.Source = filepath.Dir(skillPath)
		return capability, nil
	}
	root := path
	manifestPath := filepath.Join(root, ".claude-plugin", "plugin.json")
	if filepath.Base(path) == "plugin.json" {
		manifestPath = path
		root = filepath.Dir(filepath.Dir(path))
	}
	manifest, err := registry.LoadPluginManifest(manifestPath)
	if err != nil {
		return model.Capability{}, err
	}
	capability := registry.PluginCapability(manifest.Name, root, manifest)
	capability.Source = root
	return capability, nil
}

func standalonePack(kind, id string, capability model.Capability) model.Pack {
	return model.Pack{
		ID:           id,
		Name:         capability.Name,
		Version:      util.ValueOrUnknown(capability.Version),
		Description:  fmt.Sprintf("Standalone %s managed by agent-packs.", singularKind(kind)),
		Capabilities: []model.Capability{capability},
	}
}

func standaloneReceiptPath(target, kind, id string) string {
	return filepath.Join(standaloneReceiptsDir(target, kind), util.Slugify(id)+".json")
}

func standaloneReceiptsDir(target, kind string) string {
	return filepath.Join(target, "receipts", kind)
}

func singularKind(kind string) string {
	if kind == "skills" {
		return "skill"
	}
	if kind == "plugins" {
		return "plugin"
	}
	return "capability"
}
