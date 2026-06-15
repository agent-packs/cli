package targets

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/agent-packs/cli/internal/model"
	"github.com/agent-packs/cli/internal/util"
)

func customTargetsPath(home string) string {
	return filepath.Join(util.ExpandHome(home), "custom-targets.json")
}

func LoadCustomTargets(home string) (map[string]model.TargetSpec, error) {
	path := customTargetsPath(home)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var specs map[string]model.TargetSpec
	if err := json.Unmarshal(data, &specs); err != nil {
		return nil, err
	}
	return specs, nil
}

func saveCustomTargets(home string, specs map[string]model.TargetSpec) error {
	path := customTargetsPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(specs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func AddCustomTarget(home, id, name, globalSkills, projectSkills string) error {
	if id == "" || globalSkills == "" {
		return fmt.Errorf("id and --global are required")
	}
	specs, err := LoadCustomTargets(home)
	if err != nil {
		return err
	}
	if specs == nil {
		specs = map[string]model.TargetSpec{}
	}
	if projectSkills == "" {
		projectSkills = globalSkills
	}
	if name == "" {
		name = id
	}
	specs[id] = model.TargetSpec{ID: id, Name: name, GlobalSkills: globalSkills, ProjectSkills: projectSkills}
	return saveCustomTargets(home, specs)
}

func RemoveCustomTarget(home, id string) error {
	specs, err := LoadCustomTargets(home)
	if err != nil {
		return err
	}
	if specs == nil || specs[id].ID == "" {
		return fmt.Errorf("custom target not found: %s", id)
	}
	delete(specs, id)
	return saveCustomTargets(home, specs)
}

func ListCustomTargets(home string, out io.Writer) error {
	specs, err := LoadCustomTargets(home)
	if err != nil {
		return err
	}
	if len(specs) == 0 {
		fmt.Fprintln(out, "No custom targets registered.")
		return nil
	}
	ids := make([]string, 0, len(specs))
	for id := range specs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	fmt.Fprintln(out, "id\tname\tglobal skills\tproject skills")
	for _, id := range ids {
		spec := specs[id]
		fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", id, spec.Name, spec.GlobalSkills, spec.ProjectSkills)
	}
	return nil
}

// RegisterCustomTargets merges custom targets into the global TargetMatrix and SkillTargets.
// Call this at startup before any installs.
func RegisterCustomTargets(home string) {
	specs, err := LoadCustomTargets(home)
	if err != nil || specs == nil {
		return
	}
	for id, spec := range specs {
		TargetMatrix[id] = spec
		SkillTargets[id] = spec.GlobalSkills
	}
}
