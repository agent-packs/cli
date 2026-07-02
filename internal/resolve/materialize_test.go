package resolve

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifySignatureHMAC(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SKILL.md")
	content := []byte("# Skill\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	key := "test-key"
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(content)
	sig := "hmac-sha256:" + hex.EncodeToString(mac.Sum(nil))
	t.Setenv("AGENT_PACKS_TRUST_KEY", key)
	if err := VerifySignature(path, sig); err != nil {
		t.Fatal(err)
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
