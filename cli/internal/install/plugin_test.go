package install

import (
	"strings"
	"testing"

	"github.com/agent-packs/cli/internal/model"
)

func TestBuildPluginCommandClaudeMarketplace(t *testing.T) {
	item := model.PlanItem{
		Method: "claude-marketplace", Package: "code-review", Marketplace: "claude-plugins-official",
	}
	args, command, err := buildPluginExec(item)
	if err != nil {
		t.Fatal(err)
	}
	// claude-marketplace without a pre-built command uses direct exec args to avoid shell injection.
	if len(args) > 0 {
		full := strings.Join(args, " ")
		if !strings.Contains(full, "claude plugin install code-review@claude-plugins-official") {
			t.Fatalf("unexpected args: %v", args)
		}
	} else {
		if !strings.Contains(command, "claude plugin install code-review@claude-plugins-official") {
			t.Fatalf("unexpected command: %s", command)
		}
	}
}

func TestBuildPluginCommandManualRequiresCommand(t *testing.T) {
	_, _, err := buildPluginExec(model.PlanItem{Method: "manual"})
	if err == nil {
		t.Fatal("expected error for missing manual command")
	}
}

func TestBuildPluginUninstallCommandUsesExplicitCommand(t *testing.T) {
	item := model.PlanItem{Method: "manual", UninstallCommand: "echo uninstall-plugin"}
	args, command, err := buildPluginUninstallExec(item)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 0 {
		t.Fatalf("expected shell command, got args: %v", args)
	}
	if command != "echo uninstall-plugin" {
		t.Fatalf("unexpected command: %s", command)
	}
}

func TestBuildPluginUninstallCommandClaudeMarketplace(t *testing.T) {
	item := model.PlanItem{
		Method: "claude-marketplace", Package: "code-review", Marketplace: "claude-plugins-official",
	}
	args, command, err := buildPluginUninstallExec(item)
	if err != nil {
		t.Fatal(err)
	}
	if command != "" {
		t.Fatalf("expected direct exec args, got command: %s", command)
	}
	full := strings.Join(args, " ")
	if !strings.Contains(full, "claude plugin uninstall code-review@claude-plugins-official") {
		t.Fatalf("unexpected args: %v", args)
	}
}
