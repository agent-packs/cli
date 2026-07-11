package resolve

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/agent-packs/cli/internal/model"
	"github.com/agent-packs/cli/internal/util"
)

const defaultHTTPTimeout = 2 * time.Minute
const defaultGitTimeout = 2 * time.Minute

// ResolveSourceLive resolves a source and, for moving git refs, queries the remote HEAD.
func ResolveSourceLive(source string) model.SourceResolution {
	resolution, _ := ResolveSourceLiveErr(source)
	return resolution
}

// ResolveSourceLiveErr is ResolveSourceLive with an explicit error when the
// remote could not be queried, so verification gates can fail closed instead of
// silently treating an unreachable or deleted source as unchanged.
func ResolveSourceLiveErr(source string) (model.SourceResolution, error) {
	resolution := ResolveSource(source)
	if resolution.Kind == "remote" || resolution.Pinned {
		return resolution, nil
	}
	repo, ref, _, kind := ParseGitSource(source)
	if repo == "" || ref == "" || isCommitSHA(ref) {
		return resolution, nil
	}
	if kind != "github-tree" && kind != "gitlab-tree" && kind != "git" {
		return resolution, nil
	}
	live, err := remoteRevision(repo, ref)
	if err != nil {
		return resolution, err
	}
	if live != "" {
		resolution.Revision = live
		resolution.Warning = fmt.Sprintf("source tracks moving ref %s (resolved to %s)", ref, live)
	}
	return resolution, nil
}

type revisionResult struct {
	revision string
	err      error
}

var (
	revisionCacheMu sync.Mutex
	revisionCache   = map[string]revisionResult{}
)

// remoteRevision resolves a ref to a commit SHA via git ls-remote, memoizing
// per (repo, ref) so commands that verify many capabilities from the same
// repository query the network once per run.
func remoteRevision(repo, ref string) (string, error) {
	key := repo + "\x00" + ref
	revisionCacheMu.Lock()
	if cached, ok := revisionCache[key]; ok {
		revisionCacheMu.Unlock()
		return cached.revision, cached.err
	}
	revisionCacheMu.Unlock()

	revision, err := remoteRevisionUncached(repo, ref)

	revisionCacheMu.Lock()
	revisionCache[key] = revisionResult{revision: revision, err: err}
	revisionCacheMu.Unlock()
	return revision, err
}

func remoteRevisionUncached(repo, ref string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultGitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "ls-remote", repo, ref)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git ls-remote failed: %s", strings.TrimSpace(stderr.String()))
	}
	line := strings.TrimSpace(stdout.String())
	if line == "" {
		return "", fmt.Errorf("ref %q not found in %s", ref, repo)
	}
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return "", fmt.Errorf("unexpected ls-remote output")
	}
	return parts[0], nil
}

func isArchiveSource(source string) bool {
	lower := strings.ToLower(source)
	return strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") || strings.HasSuffix(lower, ".zip")
}

// materializeEntry memoizes one materialized cache directory for the lifetime
// of the process, so commands that touch many capabilities from the same
// repository clone it once per run instead of once per capability.
type materializeEntry struct {
	once sync.Once
	err  error
}

var (
	materializeMu    sync.Mutex
	materializeCache = map[string]*materializeEntry{}
)

func MaterializeSkillSource(source, target string) (string, func(), error) {
	if util.IsLocalSource(source) {
		abs, err := filepath.Abs(util.ExpandHome(source))
		return abs, nil, err
	}
	if isArchiveSource(source) {
		cache := filepath.Join(target, "cache", "sources", util.Slugify(source))
		if err := os.MkdirAll(filepath.Dir(cache), 0o755); err != nil {
			return "", nil, err
		}
		return materializeArchiveSource(source, cache)
	}
	repo, ref, subpath, kind := ParseGitSource(source)
	if repo == "" {
		return "", nil, fmt.Errorf("unsupported remote source: %s", source)
	}
	// Cache one clone per (repo, ref): capabilities that reference different
	// subpaths of the same repository share a single checkout.
	cacheKey := util.Slugify(repo + "@" + ref)
	cache := filepath.Join(target, "cache", "sources", cacheKey)
	if err := os.MkdirAll(filepath.Dir(cache), 0o755); err != nil {
		return "", nil, err
	}

	materializeMu.Lock()
	entry, ok := materializeCache[cache]
	if !ok {
		entry = &materializeEntry{}
		materializeCache[cache] = entry
	}
	materializeMu.Unlock()
	entry.once.Do(func() {
		entry.err = materializeGitSource(repo, ref, kind, cache)
	})
	if entry.err != nil {
		return "", nil, entry.err
	}
	if subpath != "" {
		return filepath.Join(cache, subpath), nil, nil
	}
	return cache, nil, nil
}

func materializeGitSource(repo, ref, kind, cache string) error {
	// Clone into a temp dir first; replace the cache only on success so a
	// failed clone doesn't destroy a previously usable cached copy.
	tmp := cache + ".tmp"
	_ = os.RemoveAll(tmp)
	var cloneErr error
	// A tree URL whose ref is a commit SHA (a pinned source) cannot be cloned
	// with --branch; fetch the commit directly like github-commit sources.
	if kind == "github-commit" || isCommitSHA(ref) {
		cloneErr = cloneCommit(repo, ref, tmp)
	} else {
		cloneErr = cloneRef(repo, ref, tmp)
	}
	if cloneErr != nil {
		_ = os.RemoveAll(tmp)
		return cloneErr
	}
	_ = os.RemoveAll(cache)
	if err := os.Rename(tmp, cache); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("failed to update source cache: %w", err)
	}
	return nil
}

// cloneCommit fetches a single commit by SHA without cloning the full history,
// which works for any reachable commit on GitHub and GitLab (not just branch tips).
func cloneCommit(repo, commit, dest string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultGitTimeout)
	defer cancel()
	for _, args := range [][]string{
		{"init", dest},
		{"-C", dest, "remote", "add", "origin", repo},
	} {
		if out, err := exec.CommandContext(ctx, "git", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("git %s failed: %s", args[0], strings.TrimSpace(string(out)))
		}
	}
	fetchCtx, fetchCancel := context.WithTimeout(context.Background(), defaultGitTimeout)
	defer fetchCancel()
	var stderr bytes.Buffer
	fetch := exec.CommandContext(fetchCtx, "git", "-C", dest, "fetch", "--depth", "1", "origin", commit)
	fetch.Stderr = &stderr
	if err := fetch.Run(); err != nil {
		return fmt.Errorf("git fetch failed: %s", strings.TrimSpace(stderr.String()))
	}
	checkoutCtx, checkoutCancel := context.WithTimeout(context.Background(), defaultGitTimeout)
	defer checkoutCancel()
	stderr.Reset()
	checkout := exec.CommandContext(checkoutCtx, "git", "-C", dest, "checkout", "FETCH_HEAD")
	checkout.Stderr = &stderr
	if err := checkout.Run(); err != nil {
		return fmt.Errorf("git checkout failed: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func cloneRef(repo, ref, dest string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultGitTimeout)
	defer cancel()
	args := []string{"clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, repo, dest)
	cmd := exec.CommandContext(ctx, "git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func materializeArchiveSource(source, cache string) (string, func(), error) {
	client := &http.Client{Timeout: defaultHTTPTimeout}
	resp, err := client.Get(source)
	if err != nil {
		return "", nil, fmt.Errorf("archive download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", nil, fmt.Errorf("archive download failed: HTTP %s", resp.Status)
	}
	archivePath := cache + archiveSuffix(source)
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		return "", nil, err
	}
	file, err := os.Create(archivePath)
	if err != nil {
		return "", nil, err
	}
	if _, err := io.Copy(file, resp.Body); err != nil {
		file.Close()
		return "", nil, err
	}
	if err := file.Close(); err != nil {
		return "", nil, err
	}
	extractDir := cache + ".extracted"
	_ = os.RemoveAll(extractDir)
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return "", nil, err
	}
	if strings.HasSuffix(strings.ToLower(source), ".zip") {
		if err := unzipArchive(archivePath, extractDir); err != nil {
			return "", nil, err
		}
	} else {
		if err := untarGzArchive(archivePath, extractDir); err != nil {
			return "", nil, err
		}
	}
	return extractDir, nil, nil
}

func archiveSuffix(source string) string {
	if strings.HasSuffix(strings.ToLower(source), ".zip") {
		return ".zip"
	}
	return ".tar.gz"
}

func untarGzArchive(archivePath, dest string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dest, filepath.Join(dest, header.Name))
		if err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("archive entry %q attempts path traversal", header.Name)
		}
		target := filepath.Join(dest, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
}

func unzipArchive(archivePath, dest string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		rel, err := filepath.Rel(dest, filepath.Join(dest, file.Name))
		if err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("archive entry %q attempts path traversal", file.Name)
		}
		target := filepath.Join(dest, file.Name)
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		src, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			src.Close()
			return err
		}
		if _, err := io.Copy(out, src); err != nil {
			out.Close()
			src.Close()
			return err
		}
		out.Close()
		src.Close()
	}
	return nil
}

// VerifySignature checks integrity.signature against file content.
// Supported format: sha256:<hex> must match file hash.
//
// The previous hmac-sha256 format was removed: an HMAC is a shared-secret MAC,
// not a signature — anyone able to verify could also forge — and presenting it
// as a signature overstated the guarantee. Real signing (e.g. registry-index
// signatures) should use asymmetric keys.
func VerifySignature(path, signature string) error {
	signature = strings.TrimSpace(signature)
	if signature == "" {
		return nil
	}
	if strings.HasPrefix(signature, "sha256:") {
		return VerifyChecksum(path, signature)
	}
	return fmt.Errorf("unsupported signature format: %s", signature)
}

// VerifySkillIntegrity runs checksum and signature checks when declared.
// A dirsha256: checksum covers every file in the skill directory; a sha256:
// checksum covers the entry file only.
func VerifySkillIntegrity(sourceDir, entry, checksum, signature string) error {
	entryPath := sourceDir
	info, err := os.Stat(sourceDir)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if entry == "" {
			entry = "SKILL.md"
		}
		entryPath = filepath.Join(sourceDir, entry)
	}
	if strings.HasPrefix(strings.TrimSpace(checksum), DirChecksumPrefix) {
		if err := VerifyTreeChecksum(sourceDir, checksum); err != nil {
			return err
		}
	} else if err := VerifySkillEntry(sourceDir, entry, checksum); err != nil {
		return err
	}
	return VerifySignature(entryPath, signature)
}
