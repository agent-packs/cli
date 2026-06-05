package install

import (
	"strings"
	"testing"

	"github.com/sandeshh/agent-packs/cli/internal/model"
)

func TestBuildPluginCommandClaudeMarketplace(t *testing.T) {
	item := model.PlanItem{
		Method: "claude-marketplace", Package: "code-review", Marketplace: "claude-plugins-official",
	}
	command, err := buildPluginCommand(item)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(command, "claude plugin install code-review@claude-plugins-official") {
		t.Fatalf("unexpected command: %s", command)
	}
}

func TestBuildPluginCommandManualRequiresCommand(t *testing.T) {
	_, err := buildPluginCommand(model.PlanItem{Method: "manual"})
	if err == nil {
		t.Fatal("expected error for missing manual command")
	}
}
