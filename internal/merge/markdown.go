package merge

import (
	"fmt"
	"os"
	"strings"

	"github.com/agent-packs/cli/internal/util"
)

// Managed-block markers. A block is delimited by a BEGIN/END comment pair keyed
// by a stable blockID (e.g. "<pack-id>/<capability-slug>"), so re-installing
// replaces the block in place and uninstall removes exactly it.
func beginMarker(blockID string) string {
	return fmt.Sprintf("<!-- BEGIN agent-packs:%s -->", blockID)
}

func endMarker(blockID string) string {
	return fmt.Sprintf("<!-- END agent-packs:%s -->", blockID)
}

// ApplyMarkdownBlock idempotently writes a managed block containing content into
// the markdown file at path, creating the file (and parents) if absent. If a
// block with the same blockID already exists, its body is replaced in place;
// otherwise the block is appended. User content outside the markers is never
// touched.
func ApplyMarkdownBlock(path, blockID, content string) (Result, error) {
	existing, err := readFileAllowMissing(path)
	if err != nil {
		return Result{}, err
	}
	block := renderBlock(blockID, content)
	updated, found := replaceBlock(existing, blockID, block)
	if !found {
		updated = appendBlock(existing, block)
	}
	res := Result{ContentHash: HashString(content)}
	if updated != existing {
		if err := util.AtomicWriteFile(path, []byte(updated), 0o644); err != nil {
			return Result{}, err
		}
		res.Changed = true
	}
	return res, nil
}

// RetractMarkdownBlock removes the managed block identified by blockID from the
// file at path, trimming the surrounding blank line it introduced. A missing
// file or absent block is a no-op.
func RetractMarkdownBlock(path, blockID string) error {
	existing, err := readFileAllowMissing(path)
	if err != nil {
		return err
	}
	updated, found := replaceBlock(existing, blockID, "")
	if !found {
		return nil
	}
	// Collapse the blank-line gap left where the block stood and normalize the
	// file to a single trailing newline so retract round-trips to the original.
	updated = strings.ReplaceAll(updated, "\n\n\n", "\n\n")
	updated = strings.Trim(updated, "\n")
	if updated != "" {
		updated += "\n"
	}
	return util.AtomicWriteFile(path, []byte(updated), 0o644)
}

// ExtractMarkdownBlock returns the body of the managed block identified by
// blockID, and whether it was found.
func ExtractMarkdownBlock(path, blockID string) (string, bool, error) {
	existing, err := readFileAllowMissing(path)
	if err != nil {
		return "", false, err
	}
	begin := beginMarker(blockID)
	end := endMarker(blockID)
	bi := strings.Index(existing, begin)
	if bi < 0 {
		return "", false, nil
	}
	ei := strings.Index(existing[bi:], end)
	if ei < 0 {
		return "", false, nil
	}
	bodyStart := bi + len(begin)
	body := existing[bodyStart : bi+ei]
	return strings.Trim(body, "\n"), true, nil
}

func renderBlock(blockID, content string) string {
	return beginMarker(blockID) + "\n" + strings.TrimRight(content, "\n") + "\n" + endMarker(blockID)
}

// replaceBlock replaces the existing block (markers and body) with replacement.
// When replacement is empty the block is removed entirely. The bool reports
// whether a block was present.
func replaceBlock(content, blockID, replacement string) (string, bool) {
	begin := beginMarker(blockID)
	end := endMarker(blockID)
	bi := strings.Index(content, begin)
	if bi < 0 {
		return content, false
	}
	rel := strings.Index(content[bi:], end)
	if rel < 0 {
		return content, false
	}
	ei := bi + rel + len(end)
	return content[:bi] + replacement + content[ei:], true
}

func appendBlock(existing, block string) string {
	if strings.TrimSpace(existing) == "" {
		return block + "\n"
	}
	sep := "\n\n"
	if strings.HasSuffix(existing, "\n\n") {
		sep = ""
	} else if strings.HasSuffix(existing, "\n") {
		sep = "\n"
	}
	return existing + sep + block + "\n"
}

func readFileAllowMissing(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}
