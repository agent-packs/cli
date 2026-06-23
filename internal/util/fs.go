package util

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// AtomicWriteFile writes data to path via a temporary file in the same
// directory followed by an atomic rename, so a crash mid-write can never leave
// a user's file (e.g. settings.json) truncated or corrupt. Parent directories
// are created as needed.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".agent-packs-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// WithFileLock serializes read-modify-write access to path across concurrent
// agent-packs invocations using an adjacent ".lock" file (O_CREATE|O_EXCL with
// bounded retries). It is advisory: it only guards other WithFileLock callers,
// which is sufficient because every merge writer goes through it.
func WithFileLock(path string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lockPath := path + ".lock"
	var lock *os.File
	for attempt := 0; attempt < 200; attempt++ {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			lock = f
			break
		}
		if !os.IsExist(err) {
			return err
		}
		time.Sleep(25 * time.Millisecond)
	}
	if lock == nil {
		return fmt.Errorf("could not acquire lock for %s (held by another process?)", path)
	}
	defer func() {
		lock.Close()
		os.Remove(lockPath)
	}()
	return fn()
}

func ExpandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func WriteJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func CopyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	output, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer output.Close()
	_, err = io.Copy(output, input)
	return err
}

func CopyDir(source, destination string) error {
	return filepath.WalkDir(source, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return CopyFile(path, target)
	})
}

func Slugify(value string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			builder.WriteRune('-')
			lastDash = true
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return "capability"
	}
	return slug
}

func IsLocalSource(source string) bool {
	return !strings.HasPrefix(source, "http://") &&
		!strings.HasPrefix(source, "https://") &&
		!strings.HasPrefix(source, "git@") &&
		!strings.HasPrefix(source, "ssh://")
}

func ValueOrUnknown(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}
