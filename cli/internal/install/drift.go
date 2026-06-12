package install

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/sandeshh/agent-packs/cli/internal/model"
	"github.com/sandeshh/agent-packs/cli/internal/util"
)

type driftItem struct {
	packID string
	name   string
	dest   string
	state  string // ok | missing | drifted
	detail string
}

func DriftCheck(target string, out io.Writer) error {
	absTarget, err := filepath.Abs(util.ExpandHome(target))
	if err != nil {
		return err
	}
	receiptsDir := filepath.Join(absTarget, "receipts")
	entries, err := os.ReadDir(receiptsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(out, "No installed packs found")
			return nil
		}
		return err
	}

	var items []driftItem
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		receipt, err := LoadReceipt(filepath.Join(receiptsDir, entry.Name()))
		if err != nil {
			continue
		}
		for _, item := range receipt.Plan.Capabilities {
			if item.Status != "installed" || item.Destination == "" {
				continue
			}
			items = append(items, checkDrift(receipt.Plan.Pack, item))
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].packID != items[j].packID {
			return items[i].packID < items[j].packID
		}
		return items[i].name < items[j].name
	})

	drifted := 0
	for _, it := range items {
		switch it.state {
		case "ok":
			fmt.Fprintf(out, "OK       %s/%s\n", it.packID, it.name)
		case "missing":
			fmt.Fprintf(out, "MISSING  %s/%s — destination %s not found\n", it.packID, it.name, it.dest)
			drifted++
		case "drifted":
			fmt.Fprintf(out, "DRIFTED  %s/%s — %s\n", it.packID, it.name, it.detail)
			drifted++
		}
	}

	if len(items) == 0 {
		fmt.Fprintln(out, "No tracked installed capabilities")
		return nil
	}

	fmt.Fprintln(out)
	if drifted > 0 {
		fmt.Fprintf(out, "%d/%d capabilities drifted or missing\n", drifted, len(items))
		return model.ErrInstallFailed
	}
	fmt.Fprintf(out, "All %d capabilities intact\n", len(items))
	return nil
}

func checkDrift(packID string, item model.PlanItem) driftItem {
	it := driftItem{packID: packID, name: item.Name, dest: item.Destination}
	dest := util.ExpandHome(item.Destination)

	if _, err := os.Stat(dest); err != nil {
		it.state = "missing"
		return it
	}

	switch item.Action {
	case "symlink":
		link, err := os.Readlink(dest)
		if err != nil {
			it.state = "missing"
			return it
		}
		want := util.ExpandHome(item.Source)
		if link != want {
			it.state = "drifted"
			it.detail = fmt.Sprintf("symlink → %s, expected → %s", link, want)
			return it
		}

	case "copy", "fetch-copy":
		if item.Type == "skill" {
			skillFile := filepath.Join(dest, "SKILL.md")
			destHash, err := hashFile(skillFile)
			if err != nil {
				it.state = "drifted"
				it.detail = "SKILL.md missing from installed directory"
				return it
			}
			if util.IsLocalSource(item.Source) {
				srcHash, err := hashFile(filepath.Join(util.ExpandHome(item.Source), "SKILL.md"))
				if err == nil && srcHash != destHash {
					it.state = "drifted"
					it.detail = fmt.Sprintf("content hash differs from source (dest=%.8s src=%.8s)", destHash, srcHash)
					return it
				}
			}
		}
	}

	it.state = "ok"
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
