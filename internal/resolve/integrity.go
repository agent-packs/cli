package resolve

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DirChecksumPrefix marks a checksum that covers an entire directory tree
// rather than a single entry file.
const DirChecksumPrefix = "dirsha256:"

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

// HashDir returns a deterministic dirsha256: digest covering every regular
// file under dir: for each file, sorted by slash-separated relative path, the
// digest folds in the path and the file's sha256. Symlinks are skipped and
// .git directories are excluded, so the hash reflects tree content only.
func HashDir(dir string) (string, error) {
	type fileHash struct {
		rel string
		sum string
	}
	var files []fileHash
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		sum, err := HashFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, fileHash{rel: filepath.ToSlash(rel), sum: sum})
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })
	hasher := sha256.New()
	for _, file := range files {
		fmt.Fprintf(hasher, "%s\x00%s\n", file.rel, file.sum)
	}
	return DirChecksumPrefix + hex.EncodeToString(hasher.Sum(nil)), nil
}

// HashTree hashes a materialized skill source: a whole-directory digest for
// directories, a plain file digest for single-file sources.
func HashTree(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return HashDir(path)
	}
	return HashFile(path)
}

// VerifyTreeChecksum compares a directory tree against an expected
// dirsha256:... digest.
func VerifyTreeChecksum(dir, expected string) error {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return nil
	}
	sum, err := HashDir(dir)
	if err != nil {
		return err
	}
	if !strings.EqualFold(sum, expected) {
		return fmt.Errorf("tree checksum mismatch for %s: expected %s, got %s", dir, expected, sum)
	}
	return nil
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
