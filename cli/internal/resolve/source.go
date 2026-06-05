package resolve

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sandeshh/agent-packs/cli/internal/model"
	"github.com/sandeshh/agent-packs/cli/internal/util"
)

var commitSHA = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

// ParseGitSource classifies a source URL into clone metadata.
func ParseGitSource(source string) (repo, ref, subpath, kind string) {
	if source == "" {
		return "", "", "", "missing"
	}
	if util.IsLocalSource(source) {
		return source, "", "", "local"
	}
	if repo, ref, subpath = parseGitHubTree(source); repo != "" {
		return repo, ref, subpath, "github-tree"
	}
	if repo, ref, subpath = parseGitHubCommit(source); repo != "" {
		return repo, ref, subpath, "github-commit"
	}
	if repo, ref, subpath = parseGitLabTree(source); repo != "" {
		return repo, ref, subpath, "gitlab-tree"
	}
	if repo, ref = parseGitURL(source); repo != "" {
		return repo, ref, "", "git"
	}
	return "", "", "", "remote"
}

func ResolveSource(source string) model.SourceResolution {
	if source == "" {
		return model.SourceResolution{Source: source, Kind: "missing", Warning: "source is empty"}
	}
	_, ref, _, kind := ParseGitSource(source)
	switch kind {
	case "local":
		revision := resolveLocalRevision(source)
		return model.SourceResolution{
			Source:   source,
			Kind:     "local",
			Revision: revision,
			Pinned:   revision != "",
			Warning:  localRevisionWarning(revision),
		}
	case "github-tree", "gitlab-tree":
		resolution := model.SourceResolution{Source: source, Kind: kind, Revision: ref, Pinned: isCommitSHA(ref)}
		if !resolution.Pinned {
			resolution.Warning = "source tracks a moving ref; pin to a commit for reproducibility"
		}
		return resolution
	case "github-commit":
		return model.SourceResolution{Source: source, Kind: kind, Revision: ref, Pinned: true}
	case "git":
		resolution := model.SourceResolution{Source: source, Kind: kind, Revision: ref, Pinned: ref != "" && isCommitSHA(ref)}
		if !resolution.Pinned {
			resolution.Warning = "git source uses a branch or tag ref; pin to a commit for reproducibility"
		}
		return resolution
	default:
		return model.SourceResolution{Source: source, Kind: "remote", Warning: "remote revision is unresolved; use a pinned commit when possible"}
	}
}

func MaterializeSkillSource(source, target string) (string, func(), error) {
	if util.IsLocalSource(source) {
		abs, err := filepath.Abs(util.ExpandHome(source))
		return abs, nil, err
	}
	cache := filepath.Join(target, "cache", "sources", util.Slugify(source))
	_ = os.RemoveAll(cache)
	if err := os.MkdirAll(filepath.Dir(cache), 0o755); err != nil {
		return "", nil, err
	}
	repo, ref, subpath, kind := ParseGitSource(source)
	if repo == "" {
		return "", nil, fmt.Errorf("unsupported remote source: %s", source)
	}
	args := []string{"clone", "--depth", "1"}
	if ref != "" && kind != "github-commit" {
		args = append(args, "--branch", ref)
	}
	if kind == "github-commit" {
		args = append(args, repo, cache)
		cmd := exec.Command("git", args...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return "", nil, fmt.Errorf("git clone failed: %s", strings.TrimSpace(stderr.String()))
		}
		checkout := exec.Command("git", "-C", cache, "checkout", ref)
		checkout.Stderr = &stderr
		if err := checkout.Run(); err != nil {
			return "", nil, fmt.Errorf("git checkout failed: %s", strings.TrimSpace(stderr.String()))
		}
	} else {
		args = append(args, repo, cache)
		cmd := exec.Command("git", args...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return "", nil, fmt.Errorf("git clone failed: %s", strings.TrimSpace(stderr.String()))
		}
	}
	if subpath != "" {
		return filepath.Join(cache, subpath), nil, nil
	}
	return cache, nil, nil
}

func parseGitHubTree(source string) (repo, branch, subpath string) {
	u, err := url.Parse(source)
	if err != nil || u.Host != "github.com" {
		return "", "", ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 5 || parts[2] != "tree" {
		return "", "", ""
	}
	return "https://github.com/" + parts[0] + "/" + parts[1] + ".git", parts[3], filepath.Join(parts[4:]...)
}

func parseGitHubCommit(source string) (repo, commit, subpath string) {
	u, err := url.Parse(source)
	if err != nil || u.Host != "github.com" {
		return "", "", ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 4 || parts[2] != "commit" {
		return "", "", ""
	}
	if !isCommitSHA(parts[3]) {
		return "", "", ""
	}
	sub := ""
	if len(parts) > 4 {
		sub = filepath.Join(parts[4:]...)
	}
	return "https://github.com/" + parts[0] + "/" + parts[1] + ".git", parts[3], sub
}

func parseGitLabTree(source string) (repo, branch, subpath string) {
	u, err := url.Parse(source)
	if err != nil || !strings.Contains(u.Host, "gitlab") {
		return "", "", ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, part := range parts {
		if part == "-" && i+3 < len(parts) && parts[i+1] == "tree" {
			project := strings.Join(parts[:i], "/")
			branch = parts[i+2]
			if i+3 < len(parts) {
				subpath = filepath.Join(parts[i+3:]...)
			}
			scheme := u.Scheme
			if scheme == "" {
				scheme = "https"
			}
			return scheme + "://" + u.Host + "/" + project + ".git", branch, subpath
		}
	}
	return "", "", ""
}

func parseGitURL(source string) (repo, ref string) {
	if strings.HasPrefix(source, "git@") {
		parts := strings.SplitN(strings.TrimPrefix(source, "git@"), ":", 2)
		if len(parts) == 2 {
			return "git@" + parts[0] + ":" + parts[1], ""
		}
		return source, ""
	}
	u, err := url.Parse(source)
	if err != nil {
		return "", ""
	}
	if strings.HasSuffix(u.Path, ".git") {
		return source, strings.TrimPrefix(u.Fragment, "ref=")
	}
	return "", ""
}

func resolveLocalRevision(source string) string {
	if source == "" || !util.IsLocalSource(source) {
		return ""
	}
	cmd := exec.Command("git", "-C", util.ExpandHome(source), "rev-parse", "HEAD")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(stdout.String())
}

func localRevisionWarning(revision string) string {
	if revision == "" {
		return "local source is not a git repository or revision could not be resolved"
	}
	return ""
}

func isCommitSHA(value string) bool {
	return commitSHA.MatchString(value)
}

func DigestCapability(capability model.Capability) string {
	data, _ := json.Marshal(capability)
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
