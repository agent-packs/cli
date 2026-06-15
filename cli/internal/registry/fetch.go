package registry

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DefaultRegistryRepo is the canonical registry the CLI fetches when no local
// registry is available. Both the repo and the ref can be overridden via
// AGENT_PACKS_REGISTRY_REPO / AGENT_PACKS_REGISTRY_REF (useful for forks, CI,
// and pinning to a release tag for reproducibility).
const (
	DefaultRegistryRepo = "https://github.com/agent-packs/registry.git"
	DefaultRegistryRef  = "main"
)

func registryRepo() string {
	if v := strings.TrimSpace(os.Getenv("AGENT_PACKS_REGISTRY_REPO")); v != "" {
		return v
	}
	return DefaultRegistryRepo
}

func registryRef() string {
	if v := strings.TrimSpace(os.Getenv("AGENT_PACKS_REGISTRY_REF")); v != "" {
		return v
	}
	return DefaultRegistryRef
}

// EnsureLocalRegistry returns the path to a local registry "packs" directory,
// cloning the canonical registry repo into cacheDir on first use. Subsequent
// calls reuse the cached checkout; use RefreshLocalRegistry to pull updates.
func EnsureLocalRegistry(cacheDir string) (string, error) {
	dest := filepath.Join(cacheDir, "registry")
	packs := filepath.Join(dest, "packs")
	if fi, err := os.Stat(packs); err == nil && fi.IsDir() {
		return packs, nil
	}
	if err := fetchRegistry(dest); err != nil {
		return "", err
	}
	return packs, nil
}

// RefreshLocalRegistry re-fetches the cached registry so the next command sees
// the latest packs. This backs `agent-packs update` (Homebrew-style catalog
// refresh) for the default registry.
func RefreshLocalRegistry(cacheDir string) error {
	return fetchRegistry(filepath.Join(cacheDir, "registry"))
}

// fetchRegistry clones the registry repo into dest, replacing it atomically so a
// failed clone never destroys a usable cached copy.
func fetchRegistry(dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".tmp"
	_ = os.RemoveAll(tmp)
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", registryRef(), registryRepo(), tmp)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("failed to fetch registry from %s (%s): %s\n"+
			"  Set AGENT_PACKS_REGISTRY to a local checkout, or AGENT_PACKS_REGISTRY_REPO/_REF\n"+
			"  to point at a reachable registry.", registryRepo(), registryRef(), strings.TrimSpace(stderr.String()))
	}
	_ = os.RemoveAll(dest)
	return os.Rename(tmp, dest)
}
