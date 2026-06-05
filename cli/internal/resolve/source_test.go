package resolve

import (
	"strings"
	"testing"
)

func TestParseGitHubCommit(t *testing.T) {
	repo, commit, subpath, kind := ParseGitSource("https://github.com/example/repo/commit/0123456789abcdef0123456789abcdef01234567/skills/foo")
	if kind != "github-commit" {
		t.Fatalf("expected github-commit, got %s", kind)
	}
	if repo != "https://github.com/example/repo.git" {
		t.Fatalf("unexpected repo: %s", repo)
	}
	if commit != "0123456789abcdef0123456789abcdef01234567" {
		t.Fatalf("unexpected commit: %s", commit)
	}
	if subpath != "skills/foo" {
		t.Fatalf("unexpected subpath: %s", subpath)
	}
	resolution := ResolveSource("https://github.com/example/repo/commit/0123456789abcdef0123456789abcdef01234567/skills/foo")
	if !resolution.Pinned || resolution.Warning != "" {
		t.Fatalf("expected pinned commit resolution: %#v", resolution)
	}
}

func TestParseGitLabTree(t *testing.T) {
	repo, branch, subpath, kind := ParseGitSource("https://gitlab.com/group/project/-/tree/main/skills/review")
	if kind != "gitlab-tree" {
		t.Fatalf("expected gitlab-tree, got %s", kind)
	}
	if !strings.Contains(repo, "gitlab.com/group/project.git") {
		t.Fatalf("unexpected repo: %s", repo)
	}
	if branch != "main" {
		t.Fatalf("unexpected branch: %s", branch)
	}
	if subpath != "skills/review" {
		t.Fatalf("unexpected subpath: %s", subpath)
	}
}

func TestResolveSourceAliasMovingRef(t *testing.T) {
	moving := ResolveSource("https://github.com/example/repo/tree/main/skills/foo")
	if moving.Pinned || !strings.Contains(moving.Warning, "moving ref") {
		t.Fatalf("unexpected moving resolution: %#v", moving)
	}
}
