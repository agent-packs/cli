package registry

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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
// failed clone never destroys a usable cached copy. Without an explicit
// AGENT_PACKS_REGISTRY_REF, the latest release tag is preferred over the moving
// default branch so the trust root is a fixed, inspectable revision; when the
// registry has no release tags yet the default branch is used.
func fetchRegistry(dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	repo := registryRepo()
	ref := registryRef()
	explicitRef := strings.TrimSpace(os.Getenv("AGENT_PACKS_REGISTRY_REF")) != ""
	if !explicitRef {
		if tag := latestReleaseTag(repo); tag != "" {
			ref = tag
		}
	}
	if err := cloneRegistry(repo, ref, dest); err != nil {
		// A stale cached tag or race with a tag move should not brick the
		// fetch: fall back to the configured default ref.
		if !explicitRef && ref != registryRef() {
			if fallbackErr := cloneRegistry(repo, registryRef(), dest); fallbackErr == nil {
				return nil
			}
		}
		return fmt.Errorf("failed to fetch registry from %s (%s): %s\n"+
			"  Set AGENT_PACKS_REGISTRY to a local checkout, or AGENT_PACKS_REGISTRY_REPO/_REF\n"+
			"  to point at a reachable registry.", repo, ref, err)
	}
	return nil
}

func cloneRegistry(repo, ref, dest string) error {
	tmp := dest + ".tmp"
	_ = os.RemoveAll(tmp)
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", ref, repo, tmp)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	_ = os.RemoveAll(dest)
	return os.Rename(tmp, dest)
}

// latestReleaseTag returns the highest semver-style tag (v1.2.3 or 1.2.3) of a
// remote repo, or "" when the repo has no such tags or cannot be queried.
func latestReleaseTag(repo string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--tags", "--refs", repo)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return ""
	}
	best := ""
	var bestParts []int
	for _, line := range strings.Split(stdout.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		tag := strings.TrimPrefix(fields[1], "refs/tags/")
		parts, ok := parseSemverTag(tag)
		if !ok {
			continue
		}
		if best == "" || semverLess(bestParts, parts) {
			best, bestParts = tag, parts
		}
	}
	return best
}

func parseSemverTag(tag string) ([]int, bool) {
	trimmed := strings.TrimPrefix(tag, "v")
	// Pre-release tags (1.2.3-rc.1) are skipped: releases only.
	segments := strings.Split(trimmed, ".")
	if len(segments) != 3 {
		return nil, false
	}
	parts := make([]int, 3)
	for i, segment := range segments {
		n, err := strconv.Atoi(segment)
		if err != nil || n < 0 {
			return nil, false
		}
		parts[i] = n
	}
	return parts, true
}

func semverLess(a, b []int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

// CheckoutCommit returns the HEAD commit of the git checkout containing path
// (typically a registry packs/ directory), or "" when path is not inside a git
// repository. It backs registry provenance recording in receipts and lockfiles.
func CheckoutCommit(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	cmd := exec.Command("git", "-C", path, "rev-parse", "HEAD")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(stdout.String())
}
