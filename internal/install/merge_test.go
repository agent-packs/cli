package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agent-packs/cli/internal/model"
)

func mergePlan(target string, items ...model.PlanItem) model.Plan {
	return model.Plan{Pack: "mpack", Mode: "copy", Target: target, Capabilities: items}
}

func TestExecutePlanRecordedHookIsNotWritten(t *testing.T) {
	target := t.TempDir()
	dest := filepath.Join(target, ".agent-packs", "hooks", "pre-commit.json")
	item := model.PlanItem{
		Type: "hook", Name: "Pre Commit", Action: "record", FileKind: "json",
		Content: `{"event":"preCommit"}`, Destination: "", Status: "planned",
		Reason: "hook not written: pass --allow-hooks to apply",
	}
	result := ExecutePlan(mergePlan(target, item), false)
	got := result.Capabilities[0]
	if got.Status != "recorded" {
		t.Fatalf("want recorded, got %q", got.Status)
	}
	if got.Reason == "" || !strings.Contains(got.Reason, "allow-hooks") {
		t.Fatalf("recorded hook should keep its gate reason, got %q", got.Reason)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("recorded hook must not be written, stat err=%v", err)
	}
}

func TestExecutePlanInstallsMemoryBlock(t *testing.T) {
	target := t.TempDir()
	dest := filepath.Join(target, "CLAUDE.md")
	item := model.PlanItem{
		Type: "memory", Name: "rules", Action: "merge", FileKind: "markdown",
		BlockID: "mpack/rules", Content: "Use tabs.", Destination: dest, Status: "planned",
	}
	result := ExecutePlan(mergePlan(target, item), false)
	got := result.Capabilities[0]
	if got.Status != "installed" {
		t.Fatalf("want installed, got %q (%s)", got.Status, got.Reason)
	}
	if got.ContentHash == "" {
		t.Fatal("expected ContentHash to be recorded")
	}
	data, _ := os.ReadFile(dest)
	if !strings.Contains(string(data), "Use tabs.") {
		t.Fatalf("memory block not written: %s", data)
	}
}

func TestExecutePlanRecordsMemoryInReferenceMode(t *testing.T) {
	target := t.TempDir()
	item := model.PlanItem{
		Type: "memory", Name: "rules", Action: "record", FileKind: "markdown",
		BlockID: "mpack/rules", Content: "Use tabs.", Status: "planned",
	}
	result := ExecutePlan(mergePlan(target, item), false)
	if got := result.Capabilities[0].Status; got != "recorded" {
		t.Fatalf("reference/record mode want recorded, got %q", got)
	}
	if _, err := os.Stat(filepath.Join(target, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatal("record mode must not write any file")
	}
}

func TestExecutePlanSettingsFailsClosedOnMalformed(t *testing.T) {
	target := t.TempDir()
	dest := filepath.Join(target, "settings.json")
	if err := os.WriteFile(dest, []byte("{bad json"), 0o644); err != nil {
		t.Fatal(err)
	}
	item := model.PlanItem{
		Type: "settings", Name: "model", Action: "merge", FileKind: "json",
		Content: `{"model":"opus"}`, Destination: dest, Status: "planned",
	}
	result := ExecutePlan(mergePlan(target, item), false)
	if result.Capabilities[0].Status != "failed" {
		t.Fatalf("expected failed status on malformed settings, got %q", result.Capabilities[0].Status)
	}
	if got, _ := os.ReadFile(dest); string(got) != "{bad json" {
		t.Fatalf("malformed file must be left untouched, got %q", got)
	}
}

func TestMergeLifecycleInstallDriftUninstall(t *testing.T) {
	target := t.TempDir()
	dest := filepath.Join(target, "settings.json")
	// Pre-seed a user key that must survive the whole lifecycle.
	if err := os.WriteFile(dest, []byte(`{"theme":"dark"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	item := model.PlanItem{
		Type: "settings", Name: "model", Action: "merge", FileKind: "json",
		Content: `{"model":"opus"}`, Destination: dest, Status: "planned",
	}
	plan := mergePlan(target, item)
	result := ExecutePlan(plan, false)
	if _, err := WriteReceiptWithoutSnapshot(target, model.Pack{ID: "mpack"}, result, ""); err != nil {
		t.Fatal(err)
	}

	// Drift: clean immediately after install.
	var clean strings.Builder
	if err := DriftCheck(target, &clean); err != nil {
		t.Fatalf("unexpected drift after install: %v\n%s", err, clean.String())
	}

	// Drift: user edits the pack-owned value -> drifted.
	if err := os.WriteFile(dest, []byte(`{"theme":"dark","model":"sonnet"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var drifted strings.Builder
	if err := DriftCheck(target, &drifted); err == nil {
		t.Fatalf("expected drift error after edit, output:\n%s", drifted.String())
	} else if !strings.Contains(drifted.String(), "DRIFTED") {
		t.Fatalf("expected DRIFTED line, got:\n%s", drifted.String())
	}

	// Uninstall removes only the pack key; user key + value restored.
	if err := Uninstall(target, "mpack", &strings.Builder{}); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	data, _ := os.ReadFile(dest)
	if strings.Contains(string(data), "model") {
		t.Fatalf("pack key should be removed on uninstall, got %s", data)
	}
	if !strings.Contains(string(data), "dark") {
		t.Fatalf("user key must be preserved, got %s", data)
	}
}

func TestTOMLMergeLifecycleInstallDriftUninstall(t *testing.T) {
	target := t.TempDir()
	dest := filepath.Join(target, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("model = \"mine\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	item := model.PlanItem{
		Type: "settings", Name: "codex memories", Action: "merge", FileKind: "toml",
		Content: `{"features":{"memories":true}}`, Destination: dest, Status: "planned",
	}
	result := ExecutePlan(mergePlan(target, item), false)
	if got := result.Capabilities[0].Status; got != "installed" {
		t.Fatalf("want installed, got %q: %s", got, result.Capabilities[0].Reason)
	}
	if _, err := WriteReceiptWithoutSnapshot(target, model.Pack{ID: "mpack"}, result, ""); err != nil {
		t.Fatal(err)
	}
	var clean strings.Builder
	if err := DriftCheck(target, &clean); err != nil {
		t.Fatalf("unexpected drift after install: %v\n%s", err, clean.String())
	}
	if err := os.WriteFile(dest, []byte("model = \"mine\"\n\n[features]\nmemories = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var drifted strings.Builder
	if err := DriftCheck(target, &drifted); err == nil {
		t.Fatalf("expected drift error after edit, output:\n%s", drifted.String())
	}
	if err := Uninstall(target, "mpack", &strings.Builder{}); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	data, _ := os.ReadFile(dest)
	if strings.Contains(string(data), "memories") {
		t.Fatalf("pack TOML key should be removed, got %s", data)
	}
	if !strings.Contains(string(data), `model = "mine"`) {
		t.Fatalf("user TOML key should be preserved, got %s", data)
	}
}

func TestManagedCommandLifecycleInstallDriftUninstall(t *testing.T) {
	target := t.TempDir()
	dest := filepath.Join(target, ".claude", "commands", "review-pr.md")
	item := model.PlanItem{
		Type: "command", Name: "Review PR", Action: "copy", FileKind: "markdown",
		Content: "Review this pull request.", Destination: dest, Status: "planned",
	}
	result := ExecutePlan(mergePlan(target, item), false)
	got := result.Capabilities[0]
	if got.Status != "installed" {
		t.Fatalf("want installed, got %q: %s", got.Status, got.Reason)
	}
	if got.ContentHash == "" {
		t.Fatal("expected command content hash to be recorded")
	}
	if _, err := WriteReceiptWithoutSnapshot(target, model.Pack{ID: "mpack"}, result, ""); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(dest)
	if string(data) != "Review this pull request." {
		t.Fatalf("command file not written: %q", data)
	}
	var clean strings.Builder
	if err := DriftCheck(target, &clean); err != nil {
		t.Fatalf("unexpected drift after install: %v\n%s", err, clean.String())
	}
	if err := os.WriteFile(dest, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	var drifted strings.Builder
	if err := DriftCheck(target, &drifted); err == nil {
		t.Fatalf("expected drift after command edit, output:\n%s", drifted.String())
	}
	if err := Uninstall(target, "mpack", &strings.Builder{}); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("command file should be removed on uninstall, stat err=%v", err)
	}
}

func TestManagedHookInstallsFromSourceFile(t *testing.T) {
	target := t.TempDir()
	source := filepath.Join(target, "hook.json")
	if err := os.WriteFile(source, []byte(`{"event":"beforeCommit"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(target, ".agent-packs", "hooks", "before-commit.json")
	item := model.PlanItem{
		Type: "hook", Name: "Before Commit", Action: "copy", FileKind: "json",
		Source: source, Destination: dest, Status: "planned",
	}
	result := ExecutePlan(mergePlan(target, item), false)
	got := result.Capabilities[0]
	if got.Status != "installed" {
		t.Fatalf("want installed, got %q: %s", got.Status, got.Reason)
	}
	data, _ := os.ReadFile(dest)
	if string(data) != `{"event":"beforeCommit"}` {
		t.Fatalf("hook file not written from source: %q", data)
	}
}

func TestManagedToolDescriptorLifecycle(t *testing.T) {
	target := t.TempDir()
	dest := filepath.Join(target, ".agent-packs", "tools", "browser-verify.json")
	item := model.PlanItem{
		Type: "tool", Name: "Browser Verify", Action: "copy", FileKind: "json",
		Content: `{"name":"browser-verify","description":"descriptor only"}`, Destination: dest, Status: "planned",
	}
	result := ExecutePlan(mergePlan(target, item), false)
	got := result.Capabilities[0]
	if got.Status != "installed" {
		t.Fatalf("want installed, got %q: %s", got.Status, got.Reason)
	}
	if got.ContentHash == "" {
		t.Fatal("expected tool content hash to be recorded")
	}
	if _, err := WriteReceiptWithoutSnapshot(target, model.Pack{ID: "mpack"}, result, ""); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(dest); string(data) != item.Content {
		t.Fatalf("tool descriptor not written: %q", data)
	}
	if err := Uninstall(target, "mpack", &strings.Builder{}); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("tool descriptor should be removed on uninstall, stat err=%v", err)
	}
}
