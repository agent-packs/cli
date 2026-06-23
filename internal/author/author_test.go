package author

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/agent-packs/cli/internal/model"
	"github.com/agent-packs/cli/internal/validate"
)

func TestNewScaffoldsValidCapabilities(t *testing.T) {
	for _, kind := range []string{"command", "hook", "memory", "settings", "subagent"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			path, err := New(NewOptions{Kind: kind, ID: "demo-" + kind, Dir: dir})
			if err != nil {
				t.Fatalf("New(%s): %v", kind, err)
			}
			if want := filepath.Join(dir, "demo-"+kind+".json"); path != want {
				t.Fatalf("path = %q, want %q", path, want)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var capability model.Capability
			if err := json.Unmarshal(data, &capability); err != nil {
				t.Fatalf("scaffolded %s is not valid JSON: %v", kind, err)
			}
			if capability.Type != kind {
				t.Fatalf("type = %q, want %q", capability.Type, kind)
			}
			if errs := validate.ValidateCapability(capability, "capability"); len(errs) != 0 {
				t.Fatalf("scaffolded %s failed validation: %v", kind, errs)
			}
		})
	}
}

func TestNewUnknownKind(t *testing.T) {
	if _, err := New(NewOptions{Kind: "bogus", ID: "x", Dir: t.TempDir()}); err == nil {
		t.Fatal("expected error for unknown kind")
	}
}
