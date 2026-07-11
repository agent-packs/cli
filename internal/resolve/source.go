package resolve

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"os/exec"
	"path"
	"regexp"
	"strings"

	"github.com/agent-packs/cli/internal/model"
	"github.com/agent-packs/cli/internal/util"
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

func parseGitHubTree(source string) (repo, branch, subpath string) {
	u, err := url.Parse(source)
	if err != nil || u.Host != "github.com" {
		return "", "", ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 4 || parts[2] != "tree" {
		return "", "", ""
	}
	subpath = ""
	if len(parts) > 4 {
		subpath = path.Join(parts[4:]...)
	}
	return "https://github.com/" + parts[0] + "/" + parts[1] + ".git", parts[3], subpath
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
		sub = path.Join(parts[4:]...)
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
				subpath = path.Join(parts[i+3:]...)
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
