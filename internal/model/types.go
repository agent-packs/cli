package model

import (
	"encoding/json"
	"errors"
)

type Pack struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Version        string         `json:"version"`
	Description    string         `json:"description"`
	UpstreamSource string         `json:"upstreamSource,omitempty"`
	License        string         `json:"license,omitempty"`
	Maintainers    []string       `json:"maintainers,omitempty"`
	Stability      string         `json:"stability,omitempty"`
	Deprecated     bool           `json:"deprecated,omitempty"`
	Replacement    string         `json:"replacement,omitempty"`
	LastVerified   string         `json:"lastVerified,omitempty"`
	ReviewStatus   string         `json:"reviewStatus,omitempty"`
	Trust          string         `json:"trust,omitempty"`
	Requirements   Requirements   `json:"requirements,omitempty"`
	Tags           []string       `json:"tags,omitempty"`
	Categories     []string       `json:"categories,omitempty"`
	Tools          []string       `json:"tools,omitempty"`
	Scope          []string       `json:"scope,omitempty"`
	Packs          []string       `json:"packs,omitempty"`
	Requires       []string       `json:"requires,omitempty"`
	ConflictsWith  []string       `json:"conflictsWith,omitempty"`
	Skills         CapabilityRefs `json:"skills,omitempty"`
	Plugins        CapabilityRefs `json:"plugins,omitempty"`
	Capabilities   []Capability   `json:"capabilities,omitempty"`
	Path           string         `json:"-"`
}

type CapabilityRefs []CapabilityRef

type CapabilityRef struct {
	ID             string            `json:"id"`
	Name           string            `json:"name,omitempty"`
	Source         string            `json:"source,omitempty"`
	UpstreamSource string            `json:"upstreamSource,omitempty"`
	Format         string            `json:"format,omitempty"`
	Version        string            `json:"version,omitempty"`
	Entry          string            `json:"entry,omitempty"`
	Homepage       string            `json:"homepage,omitempty"`
	Repository     string            `json:"repository,omitempty"`
	License        string            `json:"license,omitempty"`
	Install        map[string]string `json:"install,omitempty"`
	Trust          string            `json:"trust,omitempty"`

	// bareString records that this ref was parsed from a bare JSON string
	// (e.g. "frontend-guidance") rather than a JSON object. Bare-string refs
	// carry no provenance metadata and are exempt from object-ref validation
	// such as the required `trust` field. It is unexported and not serialized.
	bareString bool
}

// IsObjectRef reports whether the ref was authored as a JSON object rather than
// a bare string. Object refs are subject to object-ref validation (e.g. a
// required, enum-constrained `trust` value); bare-string refs are not.
func (ref CapabilityRef) IsObjectRef() bool {
	return !ref.bareString
}

func (refs CapabilityRefs) IDs() []string {
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		ids = append(ids, ref.ID)
	}
	return ids
}

func (ref CapabilityRef) MarshalJSON() ([]byte, error) {
	if ref.Name == "" && ref.Source == "" && ref.UpstreamSource == "" && ref.Format == "" && ref.Version == "" && ref.Entry == "" && ref.Homepage == "" && ref.Repository == "" && ref.License == "" && len(ref.Install) == 0 && ref.Trust == "" {
		return json.Marshal(ref.ID)
	}
	type alias CapabilityRef
	return json.Marshal(alias(ref))
}

func (ref *CapabilityRef) UnmarshalJSON(data []byte) error {
	var id string
	if err := json.Unmarshal(data, &id); err == nil {
		ref.ID = id
		ref.bareString = true
		return nil
	}
	type alias CapabilityRef
	var object alias
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	*ref = CapabilityRef(object)
	return nil
}

type Capability struct {
	Type              string            `json:"type"`
	Name              string            `json:"name"`
	Source            string            `json:"source"`
	UpstreamSource    string            `json:"upstreamSource,omitempty"`
	Format            string            `json:"format,omitempty"`
	Version           string            `json:"version,omitempty"`
	Entry             string            `json:"entry,omitempty"`
	Homepage          string            `json:"homepage,omitempty"`
	Repository        string            `json:"repository,omitempty"`
	License           string            `json:"license,omitempty"`
	Install           map[string]string `json:"install,omitempty"`
	Targets           []string          `json:"targets,omitempty"`
	Integrity         Integrity         `json:"integrity,omitempty"`
	RequiresExecution bool              `json:"requiresExecution,omitempty"`
	Trust             string            `json:"trust,omitempty"`

	// Content is an inline fragment for merge-into-file capabilities (memory,
	// settings): a markdown block body, or a JSON object to deep-merge. When
	// empty, the fragment is read from Source instead.
	Content string `json:"content,omitempty"`
	// MergeKey is a dotted key path within a structured settings file under
	// which the fragment is merged (e.g. "mcpServers"). Empty merges at root.
	MergeKey string `json:"mergeKey,omitempty"`
	// ApplyTo scopes instruction fragments for agents with path-specific rule
	// files, such as GitHub Copilot's .github/instructions/*.instructions.md.
	ApplyTo string `json:"applyTo,omitempty"`
	// AgentTargets can override the selected destination for a specific agent.
	// It is intentionally narrow: the default target matrix remains the source
	// of truth, while packs can opt into a documented alternate file.
	AgentTargets map[string]AgentTarget `json:"agentTargets,omitempty"`

	Reference bool `json:"-"`
}

type AgentTarget struct {
	Destination string `json:"destination,omitempty"`
	Scope       string `json:"scope,omitempty"`
	Format      string `json:"format,omitempty"`
}

type Integrity struct {
	Checksum  string `json:"checksum,omitempty"`
	Signature string `json:"signature,omitempty"`
}

type Requirements struct {
	AgentPacks string            `json:"agentPacks,omitempty"`
	Tools      map[string]string `json:"tools,omitempty"`
}

type SkillManifest struct {
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	License       string            `json:"license,omitempty"`
	Compatibility string            `json:"compatibility,omitempty"`
	AllowedTools  string            `json:"allowed-tools,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Body          string            `json:"-"`
}

type PluginManifest struct {
	Name           string         `json:"name"`
	DisplayName    string         `json:"displayName,omitempty"`
	Version        string         `json:"version,omitempty"`
	Description    string         `json:"description,omitempty"`
	Author         map[string]any `json:"author,omitempty"`
	Homepage       string         `json:"homepage,omitempty"`
	Repository     string         `json:"repository,omitempty"`
	License        string         `json:"license,omitempty"`
	Keywords       []string       `json:"keywords,omitempty"`
	DefaultEnabled *bool          `json:"defaultEnabled,omitempty"`
	Skills         any            `json:"skills,omitempty"`
	Commands       any            `json:"commands,omitempty"`
	Agents         any            `json:"agents,omitempty"`
	Hooks          any            `json:"hooks,omitempty"`
	MCPServers     any            `json:"mcpServers,omitempty"`
	LSPServers     any            `json:"lspServers,omitempty"`
	Experimental   map[string]any `json:"experimental,omitempty"`
}

type InstallOptions struct {
	Mode       string
	OnConflict string
	Scope      string
	// AllowHooks gates writing hook capabilities in copy mode. Installing a hook
	// writes a file the target agent may execute automatically, so it is opt-in
	// (parallel to --execute-plugins). When false, hooks are recorded for
	// preview but not written.
	AllowHooks bool
}

type Plan struct {
	Pack         string     `json:"pack"`
	Version      string     `json:"version"`
	Agent        string     `json:"agent"`
	Target       string     `json:"target"`
	Mode         string     `json:"mode"`
	OnConflict   string     `json:"onConflict"`
	Scope        string     `json:"scope"`
	Capabilities []PlanItem `json:"capabilities"`
}

type PlanItem struct {
	Type              string `json:"type"`
	Name              string `json:"name"`
	Action            string `json:"action"`
	Mode              string `json:"mode,omitempty"`
	OnConflict        string `json:"onConflict,omitempty"`
	Source            string `json:"source,omitempty"`
	UpstreamSource    string `json:"upstreamSource,omitempty"`
	Entry             string `json:"entry,omitempty"`
	Destination       string `json:"destination,omitempty"`
	ExpectedChecksum  string `json:"expectedChecksum,omitempty"`
	ExpectedSignature string `json:"expectedSignature,omitempty"`
	Status            string `json:"status"`
	Format            string `json:"format,omitempty"`
	Command           string `json:"command,omitempty"`
	UninstallCommand  string `json:"uninstallCommand,omitempty"`
	Method            string `json:"method,omitempty"`
	Package           string `json:"package,omitempty"`
	Marketplace       string `json:"marketplace,omitempty"`
	Reason            string `json:"reason,omitempty"`
	ExitCode          *int   `json:"exit_code,omitempty"`
	Stdout            string `json:"stdout,omitempty"`
	Stderr            string `json:"stderr,omitempty"`

	// File-backed capability fields. For memory/settings, FileKind selects the
	// merge strategy and uninstall retracts the fragment from a shared file. For
	// command/hook, FileKind describes the managed copied file and uninstall
	// removes that file. Content carries inline data; BlockID is the stable
	// marker id for markdown blocks; MergeKey scopes structured merges;
	// ContentHash and OwnedKeys record what was installed for drift checks.
	FileKind    string   `json:"fileKind,omitempty"`
	Content     string   `json:"content,omitempty"`
	BlockID     string   `json:"blockId,omitempty"`
	MergeKey    string   `json:"mergeKey,omitempty"`
	ContentHash string   `json:"contentHash,omitempty"`
	OwnedKeys   []string `json:"ownedKeys,omitempty"`
}

type Receipt struct {
	InstalledAt string `json:"installed_at"`
	Pack        Pack   `json:"pack"`
	Plan        Plan   `json:"plan"`
}

type Lockfile struct {
	GeneratedAt  string      `json:"generated_at"`
	Pack         string      `json:"pack"`
	Version      string      `json:"version"`
	Capabilities []LockEntry `json:"capabilities"`
}

type LockEntry struct {
	Type           string    `json:"type"`
	Name           string    `json:"name"`
	Source         string    `json:"source"`
	UpstreamSource string    `json:"upstreamSource,omitempty"`
	Version        string    `json:"version,omitempty"`
	Revision       string    `json:"revision,omitempty"`
	ResolvedAt     string    `json:"resolvedAt"`
	Integrity      Integrity `json:"integrity,omitempty"`
	Digest         string    `json:"digest"`
}

type SourceResolution struct {
	Source   string
	Kind     string
	Revision string
	Pinned   bool
	Warning  string
}

type TrustPolicy struct {
	AllowSources        []string `json:"allowSources,omitempty"`
	DenySources         []string `json:"denySources,omitempty"`
	RequirePinnedRefs   bool     `json:"requirePinnedRefs,omitempty"`
	AllowNativeCommands bool     `json:"allowNativeCommands,omitempty"`
}

type RegistryIndex struct {
	GeneratedAt string       `json:"generatedAt"`
	Packs       []IndexEntry `json:"packs"`
}

type IndexEntry struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Description  string   `json:"description"`
	Maintainers  []string `json:"maintainers,omitempty"`
	Stability    string   `json:"stability,omitempty"`
	Deprecated   bool     `json:"deprecated,omitempty"`
	Replacement  string   `json:"replacement,omitempty"`
	LastVerified string   `json:"lastVerified,omitempty"`
	ReviewStatus string   `json:"reviewStatus,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	Categories   []string `json:"categories,omitempty"`
	Tools        []string `json:"tools,omitempty"`
	Scope        []string `json:"scope,omitempty"`
	Skills       []string `json:"skills,omitempty"`
	Plugins      []string `json:"plugins,omitempty"`
	Capabilities int      `json:"capabilities"`
}

type RegistryConfig struct {
	Registries map[string]string `json:"registries"`
}

type TargetSpec struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	GlobalSkills  string `json:"globalSkills"`
	ProjectSkills string `json:"projectSkills"`

	// Memory and Settings describe where merge-into-file capabilities land for
	// this agent. An empty FileDest (or empty scope field) means the agent does
	// not support that capability type at that scope, and installs skip+warn.
	Memory                  FileDest   `json:"memory"`
	Settings                FileDest   `json:"settings"`
	InstructionDestinations []FileDest `json:"instructionDestinations,omitempty"`
	SettingsDestinations    []FileDest `json:"settingsDestinations,omitempty"`
	CommandDestinations     []FileDest `json:"commandDestinations,omitempty"`
	HookDestinations        []FileDest `json:"hookDestinations,omitempty"`
	SubagentDestinations    []FileDest `json:"subagentDestinations,omitempty"`
	PromptDestinations      []FileDest `json:"promptDestinations,omitempty"`
	TemplateDestinations    []FileDest `json:"templateDestinations,omitempty"`
}

// FileDest locates a merge-into-file capability for an agent. Global and
// Project are paths relative to the target root for each scope; an empty string
// marks that scope unsupported. Kind selects the merge strategy
// ("markdown" or "json").
type FileDest struct {
	Global    string `json:"global,omitempty"`
	Project   string `json:"project,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Scope     string `json:"scope,omitempty"`
	Path      string `json:"path,omitempty"`
	Verified  bool   `json:"verified,omitempty"`
	SourceURL string `json:"sourceURL,omitempty"`
	Default   bool   `json:"default,omitempty"`
}

// PathFor returns the relative path for the given scope ("project" uses the
// project file, anything else uses the global file) and whether it is supported.
func (d FileDest) PathFor(scope string) (string, bool) {
	if d.Path != "" {
		destScope := d.Scope
		if destScope == "" {
			destScope = "target"
		}
		if scope == destScope || (scope == "target" && destScope == "global") {
			return d.Path, true
		}
		return "", false
	}
	rel := d.Global
	if scope == "project" {
		rel = d.Project
	}
	if rel == "" {
		return "", false
	}
	return rel, true
}

var (
	ErrNotFound      = errors.New("not found")
	ErrInstallFailed = errors.New("install failed")
)
