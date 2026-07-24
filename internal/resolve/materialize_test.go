package resolve

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// hmac-sha256 was removed: a shared-secret MAC is not a signature. Any
// remaining manifests that declare one must fail verification loudly instead
// of granting false assurance.
func TestVerifySignatureRejectsHMAC(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SKILL.md")
	if err := os.WriteFile(path, []byte("# Skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := VerifySignature(path, "hmac-sha256:deadbeef")
	if err == nil || !strings.Contains(err.Error(), "unsupported signature format") {
		t.Fatalf("expected unsupported-format error, got %v", err)
	}
}

func TestParseArchiveSource(t *testing.T) {
	if !isArchiveSource("https://example.com/skills/foo.tar.gz") {
		t.Fatal("expected tar.gz archive")
	}
	if !isArchiveSource("https://example.com/skills/foo.zip") {
		t.Fatal("expected zip archive")
	}
}

// Pinned sources reference commit SHAs, which cannot be cloned with
// --branch; cloneCommit must fetch them directly.
func TestCloneCommitFetchesBySHA(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	temp := t.TempDir()
	origin := filepath.Join(temp, "origin")
	if err := os.MkdirAll(origin, 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = origin
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s", args, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", ".")
	run("config", "uploadpack.allowAnySHA1InWant", "true")
	if err := os.WriteFile(filepath.Join(origin, "SKILL.md"), []byte("# Skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "SKILL.md")
	run("commit", "-q", "-m", "init")
	sha := run("rev-parse", "HEAD")

	dest := filepath.Join(temp, "dest")
	if err := cloneCommit(origin, sha, dest); err != nil {
		t.Fatalf("cloneCommit by SHA failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "SKILL.md")); err != nil {
		t.Fatalf("expected SKILL.md in fetched commit: %v", err)
	}
}
