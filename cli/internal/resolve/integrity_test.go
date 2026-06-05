package resolve

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyChecksum(t *testing.T) {
	temp := t.TempDir()
	path := filepath.Join(temp, "SKILL.md")
	content := []byte("# Example\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sum, err := HashFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyChecksum(path, sum); err != nil {
		t.Fatalf("expected matching checksum: %v", err)
	}
	if err := VerifyChecksum(path, "sha256:deadbeef"); err == nil {
		t.Fatal("expected checksum mismatch error")
	}
}

func TestVerifySkillEntry(t *testing.T) {
	temp := t.TempDir()
	skillDir := filepath.Join(temp, "skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(path, []byte("# Skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum, err := HashFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifySkillEntry(skillDir, "SKILL.md", sum); err != nil {
		t.Fatalf("expected valid skill entry: %v", err)
	}
	if err := VerifySkillEntry(skillDir, "SKILL.md", ""); err != nil {
		t.Fatalf("empty checksum should skip verification: %v", err)
	}
}
