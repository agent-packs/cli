package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitWritesProjectConfig(t *testing.T) {
	dir := t.TempDir()
	path, err := Init(dir, InitOptions{Agent: "claude-code", Mode: "reference"})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != DefaultFilename {
		t.Fatalf("unexpected path: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "agent: claude") {
		t.Fatalf("expected normalized agent in config: %s", string(data))
	}
}
