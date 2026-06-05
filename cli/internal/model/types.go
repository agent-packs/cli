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
	Tags           []string       `json:"tags,omitempty"`
	Categories     []string       `json:"categories,omitempty"`
	Tools          []string       `json:"tools,omitempty"`
	Scope          []string       `json:"scope,omitempty"`
	Packs          []string       `json:"packs,omitempty"`
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
	Reference         bool              `json:"-"`
}

type Integrity struct {
	Checksum  string `json:"checksum,omitempty"`
	Signature string `json:"signature,omitempty"`
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
	Method            string `json:"method,omitempty"`
	Package           string `json:"package,omitempty"`
	Marketplace       string `json:"marketplace,omitempty"`
	Reason            string `json:"reason,omitempty"`
	ExitCode          *int   `json:"exit_code,omitempty"`
	Stdout            string `json:"stdout,omitempty"`
	Stderr            string `json:"stderr,omitempty"`
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
	ID            string
	Name          string
	GlobalSkills  string
	ProjectSkills string
}

var (
	ErrNotFound      = errors.New("not found")
	ErrInstallFailed = errors.New("install failed")
)
