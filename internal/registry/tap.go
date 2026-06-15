package registry

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Tap adds a tap using the GitHub naming convention:
// "myorg/skills" → https://github.com/myorg/agent-packs-skills
// A full URL can also be passed directly.
func Tap(home, ref string, out io.Writer) error {
	name, source := tapNameAndSource(ref)
	cfg, err := LoadRegistryConfig(home)
	if err != nil {
		return err
	}
	if cfg.Registries == nil {
		cfg.Registries = map[string]string{}
	}
	if existing, ok := cfg.Registries[name]; ok {
		fmt.Fprintf(out, "Already tapped: %s -> %s\n", name, existing)
		return nil
	}
	cfg.Registries[name] = source
	if err := SaveRegistryConfig(home, cfg); err != nil {
		return err
	}
	fmt.Fprintf(out, "Tapped %s\n  %s\n", name, source)
	return nil
}

// Untap removes a previously added tap by name or org/repo ref.
func Untap(home, ref string, out io.Writer) error {
	name, _ := tapNameAndSource(ref)
	cfg, err := LoadRegistryConfig(home)
	if err != nil {
		return err
	}
	if _, ok := cfg.Registries[name]; !ok {
		return fmt.Errorf("tap not found: %s", name)
	}
	delete(cfg.Registries, name)
	if err := SaveRegistryConfig(home, cfg); err != nil {
		return err
	}
	fmt.Fprintf(out, "Untapped %s\n", name)
	return nil
}

// TapList lists all configured taps with their sources.
func TapList(home string, out io.Writer) error {
	cfg, err := LoadRegistryConfig(home)
	if err != nil {
		return err
	}
	if len(cfg.Registries) == 0 {
		fmt.Fprintln(out, "No taps configured.")
		return nil
	}
	names := make([]string, 0, len(cfg.Registries))
	for name := range cfg.Registries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(out, "%s\n  %s\n", name, cfg.Registries[name])
	}
	return nil
}

// tapNameAndSource converts a tap ref to (name, source).
// "myorg/skills"   → ("myorg/skills", "https://github.com/myorg/agent-packs-skills")
// "https://..."    → ("https://...", "https://...")
// "myorg/skills https://my.url" → split by space not supported here; callers split
func tapNameAndSource(ref string) (string, string) {
	if strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "/") || strings.HasPrefix(ref, "./") {
		return ref, ref
	}
	// org/repo format
	if strings.Contains(ref, "/") {
		parts := strings.SplitN(ref, "/", 2)
		org, repo := parts[0], parts[1]
		source := fmt.Sprintf("https://github.com/%s/agent-packs-%s", org, repo)
		return ref, source
	}
	return ref, ref
}
