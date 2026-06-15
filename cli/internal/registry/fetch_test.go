package registry

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// makeRegistryRepo creates a local git repo that looks like agent-packs/registry
// (a packs/ dir at the root) and returns its path for use as a file:// remote.
func makeRegistryRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	if err := os.MkdirAll(filepath.Join(repo, "packs"), 0o755); err != nil {
		t.Fatal(err)
	}
	pack := `{"id":"sample","name":"Sample","version":"0.1.0","description":"d","capabilities":[]}`
	if err := os.WriteFile(filepath.Join(repo, "packs", "sample.json"), []byte(pack), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "init")
	return repo
}

func TestEnsureLocalRegistryFetchesAndCaches(t *testing.T) {
	repo := makeRegistryRepo(t)
	t.Setenv("AGENT_PACKS_REGISTRY_REPO", "file://"+repo)
	t.Setenv("AGENT_PACKS_REGISTRY_REF", "main")

	cache := t.TempDir()
	packs, err := EnsureLocalRegistry(cache)
	if err != nil {
		t.Fatalf("ensure failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(packs, "sample.json")); err != nil {
		t.Fatalf("expected fetched pack in cache: %v", err)
	}

	// Cached pack must load through the normal registry path.
	loaded, err := LoadPacks(packs)
	if err != nil || len(loaded) != 1 || loaded[0].ID != "sample" {
		t.Fatalf("expected to load 1 sample pack, got %v (err %v)", loaded, err)
	}
}

func TestRefreshLocalRegistryPicksUpNewPacks(t *testing.T) {
	repo := makeRegistryRepo(t)
	t.Setenv("AGENT_PACKS_REGISTRY_REPO", "file://"+repo)
	t.Setenv("AGENT_PACKS_REGISTRY_REF", "main")
	cache := t.TempDir()

	if _, err := EnsureLocalRegistry(cache); err != nil {
		t.Fatal(err)
	}

	// Add a second pack upstream, then refresh.
	pack := `{"id":"second","name":"Second","version":"0.1.0","description":"d","capabilities":[]}`
	if err := os.WriteFile(filepath.Join(repo, "packs", "second.json"), []byte(pack), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "second"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	if err := RefreshLocalRegistry(cache); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	packs := filepath.Join(cache, "registry", "packs")
	loaded, err := LoadPacks(packs)
	if err != nil || len(loaded) != 2 {
		t.Fatalf("expected 2 packs after refresh, got %d (err %v)", len(loaded), err)
	}
}
