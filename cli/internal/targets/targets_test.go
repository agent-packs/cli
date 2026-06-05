package targets

import "testing"

func TestNormalizeAgentClaudeCodeAlias(t *testing.T) {
	if got := NormalizeAgent("claude-code"); got != "claude" {
		t.Fatalf("expected claude, got %s", got)
	}
	if !ValidAgent("claude-code") {
		t.Fatal("claude-code alias should be valid")
	}
	if root := SkillTargetRoot("/tmp", "claude-code", "global"); root != "/tmp/.claude/skills" {
		t.Fatalf("unexpected skill root: %s", root)
	}
}
