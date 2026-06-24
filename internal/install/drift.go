package install

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/agent-packs/cli/internal/merge"
	"github.com/agent-packs/cli/internal/model"
	"github.com/agent-packs/cli/internal/util"
)

// DriftItem is a single capability checked for drift; exported for JSON output.
type DriftItem struct {
	Pack   string `json:"pack"`
	Agent  string `json:"agent,omitempty"`
	Name   string `json:"name"`
	Dest   string `json:"dest"`
	State  string `json:"state"`
	Detail string `json:"detail,omitempty"`
}

func collectDriftItems(target string) ([]DriftItem, error) {
	absTarget, err := filepath.Abs(util.ExpandHome(target))
	if err != nil {
		return nil, err
	}
	receiptsDir := filepath.Join(absTarget, "receipts")
	entries, err := os.ReadDir(receiptsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var items []DriftItem
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		receipt, err := LoadReceipt(filepath.Join(receiptsDir, entry.Name()))
		if err != nil {
			continue
		}
		for _, item := range receipt.Plan.Capabilities {
			if item.Destination == "" {
				// Reference mode: the pack is installed but nothing is
				// materialized on disk, so there is no file to verify drift
				// against. Surface it so `status` doesn't look empty.
				items = append(items, DriftItem{Pack: receipt.Plan.Pack, Agent: receipt.Plan.Agent, Name: item.Name, State: "referenced"})
				continue
			}
			if item.Status != "installed" {
				continue
			}
			items = append(items, checkDrift(receipt.Plan.Pack, receipt.Plan.Agent, item))
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Pack != items[j].Pack {
			return items[i].Pack < items[j].Pack
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

func DriftCheck(target string, out io.Writer) error {
	items, err := collectDriftItems(target)
	if err != nil {
		return err
	}
	if items == nil {
		fmt.Fprintln(out, "No installed packs found")
		return nil
	}

	drifted := 0
	referenced := 0
	for _, it := range items {
		switch it.State {
		case "ok":
			fmt.Fprintf(out, "OK       %s/%s\n", it.Pack, it.Name)
		case "referenced":
			fmt.Fprintf(out, "REF      %s/%s — reference mode (not materialized)\n", it.Pack, it.Name)
			referenced++
		case "missing":
			fmt.Fprintf(out, "MISSING  %s/%s — destination %s not found\n", it.Pack, it.Name, it.Dest)
			drifted++
		case "drifted":
			fmt.Fprintf(out, "DRIFTED  %s/%s — %s\n", it.Pack, it.Name, it.Detail)
			drifted++
		}
	}

	fmt.Fprintln(out)
	if drifted > 0 {
		fmt.Fprintf(out, "%d/%d capabilities drifted or missing\n", drifted, len(items))
		return model.ErrInstallFailed
	}
	materialized := len(items) - referenced
	if materialized == 0 {
		fmt.Fprintf(out, "%d capabilities installed in reference mode (nothing materialized to verify)\n", referenced)
		return nil
	}
	fmt.Fprintf(out, "All %d materialized capabilities intact", materialized)
	if referenced > 0 {
		fmt.Fprintf(out, " (%d referenced)", referenced)
	}
	fmt.Fprintln(out)
	return nil
}

func DriftCheckJSON(target string, out io.Writer) error {
	items, err := collectDriftItems(target)
	if err != nil {
		return err
	}
	if items == nil {
		items = []DriftItem{}
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(items)
}

func checkDrift(packID, agent string, item model.PlanItem) DriftItem {
	it := DriftItem{Pack: packID, Agent: agent, Name: item.Name, Dest: item.Destination}
	dest := util.ExpandHome(item.Destination)

	if _, err := os.Stat(dest); err != nil {
		it.State = "missing"
		return it
	}

	if (item.Type == "memory" || item.Type == "settings") && item.FileKind != "" {
		return checkMergeDrift(it, item, dest)
	}

	switch item.Action {
	case "symlink":
		link, err := os.Readlink(dest)
		if err != nil {
			it.State = "missing"
			return it
		}
		want := util.ExpandHome(item.Source)
		if link != want {
			it.State = "drifted"
			it.Detail = fmt.Sprintf("symlink → %s, expected → %s", link, want)
			return it
		}

	case "copy", "fetch-copy":
		if item.Type == "skill" {
			skillFile := filepath.Join(dest, "SKILL.md")
			destHash, err := hashFile(skillFile)
			if err != nil {
				it.State = "drifted"
				it.Detail = "SKILL.md missing from installed directory"
				return it
			}
			if util.IsLocalSource(item.Source) {
				srcHash, err := hashFile(filepath.Join(util.ExpandHome(item.Source), "SKILL.md"))
				if err == nil && srcHash != destHash {
					it.State = "drifted"
					it.Detail = fmt.Sprintf("content hash differs from source (dest=%.8s src=%.8s)", destHash, srcHash)
					return it
				}
			}
		} else if isManagedFileType(item.Type) {
			destHash, err := hashFile(dest)
			if err != nil {
				it.State = "missing"
				return it
			}
			if item.ContentHash != "" && item.ContentHash != destHash {
				it.State = "drifted"
				it.Detail = "managed file content was edited"
				return it
			}
		}
	}

	it.State = "ok"
	return it
}

// checkMergeDrift verifies that a merged fragment is still present in its shared
// file and unchanged since install. It reports "missing" when the managed block
// or owned keys are gone, and "drifted" when the user has hand-edited inside the
// pack's managed region (content hash no longer matches the receipt).
func checkMergeDrift(it DriftItem, item model.PlanItem, dest string) DriftItem {
	switch item.FileKind {
	case "markdown":
		body, found, err := merge.ExtractMarkdownBlock(dest, item.BlockID)
		if err != nil || !found {
			it.State = "missing"
			it.Detail = "managed memory block not found"
			return it
		}
		if item.ContentHash != "" && merge.HashString(body) != item.ContentHash {
			it.State = "drifted"
			it.Detail = "managed memory block was edited"
			return it
		}
	case "json":
		present, hash, err := merge.OwnedKeysState(dest, item.OwnedKeys)
		if err != nil || !present {
			it.State = "missing"
			it.Detail = "pack-owned settings keys not found"
			return it
		}
		if item.ContentHash != "" && hash != item.ContentHash {
			it.State = "drifted"
			it.Detail = "pack-owned settings values were changed"
			return it
		}
	case "toml":
		present, hash, err := merge.OwnedTOMLKeysState(dest, item.OwnedKeys)
		if err != nil || !present {
			it.State = "missing"
			it.Detail = "pack-owned settings keys not found"
			return it
		}
		if item.ContentHash != "" && hash != item.ContentHash {
			it.State = "drifted"
			it.Detail = "pack-owned settings values were changed"
			return it
		}
	}
	it.State = "ok"
	return it
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
