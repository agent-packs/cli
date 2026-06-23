package agentpacks

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, rel, body string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectAgentFromProjectDir(t *testing.T) {
	dir := t.TempDir()
	if got := DetectAgent(dir); got != "" {
		t.Fatalf("empty project should detect no agent, got %q", got)
	}
	writeFile(t, dir, ".claude/settings.json", "{}")
	if got := DetectAgent(dir); got != "claude" {
		t.Fatalf("want claude, got %q", got)
	}
}

func TestDetectStackFromManifests(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module x\n")
	tags := DetectStack(dir)
	found := map[string]bool{}
	for _, tag := range tags {
		found[tag] = true
	}
	if !found["go"] || !found["backend"] {
		t.Fatalf("go.mod should imply go+backend, got %v", tags)
	}
	if found["python"] {
		t.Fatalf("unexpected python tag, got %v", tags)
	}
}

func TestRecommendPacksRanksByOverlap(t *testing.T) {
	reg := t.TempDir()
	writeFile(t, reg, "go-pack.json", `{"id":"go-pack","name":"Go","version":"1.0.0","description":"d","tags":["go","backend"]}`)
	writeFile(t, reg, "py-pack.json", `{"id":"py-pack","name":"Py","version":"1.0.0","description":"d","tags":["python"]}`)
	got := RecommendPacks(reg, []string{"go", "backend"}, 3)
	if len(got) != 1 || got[0] != "go-pack" {
		t.Fatalf("want [go-pack], got %v", got)
	}
	if RecommendPacks(reg, nil, 3) != nil {
		t.Fatal("empty stack should recommend nothing")
	}
}
