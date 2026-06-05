package resolve

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// VerifyChecksum compares file content against an expected sha256:... digest.
func VerifyChecksum(path, expected string) error {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return nil
	}
	sum, err := HashFile(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(sum, expected) {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", path, expected, sum)
	}
	return nil
}

// HashFile returns a sha256: hex digest for a file.
func HashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

// VerifySkillEntry checks the skill entry file when a checksum is declared.
func VerifySkillEntry(sourceDir, entry, expectedChecksum string) error {
	if expectedChecksum == "" {
		return nil
	}
	entryPath := sourceDir
	info, err := os.Stat(sourceDir)
	if err != nil {
		return err
	}
	if info.IsDir() {
		entryPath = filepath.Join(sourceDir, entry)
	}
	return VerifyChecksum(entryPath, expectedChecksum)
}
