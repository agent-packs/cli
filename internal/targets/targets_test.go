package targets

import (
	"path/filepath"
	"testing"
)

func TestNormalizeAgentClaudeCodeAlias(t *testing.T) {
	if got := NormalizeAgent("claude-code"); got != "claude" {
		t.Fatalf("expected claude, got %s", got)
	}
	if !ValidAgent("claude-code") {
		t.Fatal("claude-code alias should be valid")
	}
	want := filepath.Join("/tmp", ".claude", "skills")
	if root := SkillTargetRoot("/tmp", "claude-code", "global"); root != want {
		t.Fatalf("unexpected skill root: %s (want %s)", root, want)
	}
}
