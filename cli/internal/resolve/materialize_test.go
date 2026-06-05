package resolve

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
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
