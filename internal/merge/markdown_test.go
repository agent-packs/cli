package merge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func countBlocks(content, blockID string) int {
	return strings.Count(content, beginMarker(blockID))
}

func TestMarkdownAppendCreatesBlockInNewFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "CLAUDE.md")
	res, err := ApplyMarkdownBlock(path, "pack/mem", "Use tabs.")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Changed {
		t.Fatal("expected Changed=true")
	}
	got := read(t, path)
	if countBlocks(got, "pack/mem") != 1 {
		t.Fatalf("expected one block, got:\n%s", got)
	}
	if !strings.Contains(got, "Use tabs.") {
		t.Fatalf("body missing:\n%s", got)
	}
}

func TestMarkdownAppendPreservesUserContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	user := "# My notes\n\nKeep this.\n"
	if err := os.WriteFile(path, []byte(user), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyMarkdownBlock(path, "pack/mem", "Block body."); err != nil {
		t.Fatalf("apply: %v", err)
	}
	got := read(t, path)
	if !strings.HasPrefix(got, user) {
		t.Fatalf("user content not preserved at head:\n%q", got)
	}
}

func TestMarkdownAppendIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	for i := 0; i < 3; i++ {
		if _, err := ApplyMarkdownBlock(path, "pack/mem", "Same body."); err != nil {
			t.Fatalf("apply %d: %v", i, err)
		}
	}
	got := read(t, path)
	if n := countBlocks(got, "pack/mem"); n != 1 {
		t.Fatalf("expected exactly one block after repeats, got %d:\n%s", n, got)
	}
	if c := strings.Count(got, "Same body."); c != 1 {
		t.Fatalf("expected body once, got %d", c)
	}
}

func TestMarkdownAppendUpdatesBodyOnChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	if _, err := ApplyMarkdownBlock(path, "pack/mem", "v1 body"); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyMarkdownBlock(path, "pack/mem", "v2 body"); err != nil {
		t.Fatal(err)
	}
	got := read(t, path)
	if countBlocks(got, "pack/mem") != 1 {
		t.Fatalf("expected single block:\n%s", got)
	}
	if strings.Contains(got, "v1 body") || !strings.Contains(got, "v2 body") {
		t.Fatalf("body not updated in place:\n%s", got)
	}
}

func TestMarkdownDistinctPacksCoexist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	if _, err := ApplyMarkdownBlock(path, "a/mem", "alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyMarkdownBlock(path, "b/mem", "beta"); err != nil {
		t.Fatal(err)
	}
	got := read(t, path)
	if !strings.Contains(got, "alpha") || !strings.Contains(got, "beta") {
		t.Fatalf("both blocks should be present:\n%s", got)
	}
}

func TestMarkdownRetractRemovesOnlyOwnBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	user := "# Notes\n\nUser line.\n"
	if err := os.WriteFile(path, []byte(user), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyMarkdownBlock(path, "a/mem", "alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyMarkdownBlock(path, "b/mem", "beta"); err != nil {
		t.Fatal(err)
	}
	if err := RetractMarkdownBlock(path, "a/mem"); err != nil {
		t.Fatalf("retract: %v", err)
	}
	got := read(t, path)
	if strings.Contains(got, "alpha") {
		t.Fatalf("alpha block should be gone:\n%s", got)
	}
	if !strings.Contains(got, "beta") || !strings.Contains(got, "User line.") {
		t.Fatalf("beta and user content must survive:\n%s", got)
	}
}

func TestMarkdownRetractToOriginal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	user := "# Notes\n\nUser line.\n"
	if err := os.WriteFile(path, []byte(user), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyMarkdownBlock(path, "a/mem", "alpha"); err != nil {
		t.Fatal(err)
	}
	if err := RetractMarkdownBlock(path, "a/mem"); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); got != user {
		t.Fatalf("retract did not round-trip to original:\nwant %q\ngot  %q", user, got)
	}
}

func TestMarkdownRetractMissingIsNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	if err := RetractMarkdownBlock(path, "a/mem"); err != nil {
		t.Fatalf("retract on missing file should be no-op, got %v", err)
	}
}

func TestMarkdownExtractRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	if _, err := ApplyMarkdownBlock(path, "a/mem", "the body"); err != nil {
		t.Fatal(err)
	}
	body, found, err := ExtractMarkdownBlock(path, "a/mem")
	if err != nil || !found {
		t.Fatalf("extract: found=%v err=%v", found, err)
	}
	if body != "the body" {
		t.Fatalf("want %q, got %q", "the body", body)
	}
}
