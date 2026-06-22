package model

type OutdatedReport struct {
	Entries []OutdatedEntry `json:"entries"`
}

type OutdatedEntry struct {
	Pack     string `json:"pack"`
	Kind     string `json:"kind"`
	Name     string `json:"name,omitempty"`
	Status   string `json:"status"`
	Locked   string `json:"locked,omitempty"`
	Current  string `json:"current,omitempty"`
	Registry string `json:"registry,omitempty"`
}

type InstalledSummary struct {
	ID          string `json:"id"`
	Version     string `json:"version"`
	InstalledAt string `json:"installedAt"`
}

type CompatibilityResult struct {
	Pack    string   `json:"pack"`
	Agent   string   `json:"agent"`
	OK      bool     `json:"ok"`
	Tools   []string `json:"tools,omitempty"`
	Message string   `json:"message,omitempty"`
}

type DependencyTree struct {
	Pack         string           `json:"pack"`
	Version      string           `json:"version"`
	Dependencies []DependencyNode `json:"dependencies"`
}

type DependencyNode struct {
	Type           string           `json:"type"`
	ID             string           `json:"id,omitempty"`
	Name           string           `json:"name"`
	Source         string           `json:"source,omitempty"`
	UpstreamSource string           `json:"upstreamSource,omitempty"`
	Trust          string           `json:"trust,omitempty"`
	Format         string           `json:"format,omitempty"`
	Dependencies   []DependencyNode `json:"dependencies,omitempty"`
}

type PublishReport struct {
	Registry string                  `json:"registry"`
	OK       bool                    `json:"ok"`
	Checks   []CheckEntry            `json:"checks"`
	Metadata *MetadataCoverageReport `json:"metadata,omitempty"`
}

type CheckEntry struct {
	Kind    string `json:"kind"`
	Target  string `json:"target"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type MetadataCoverageReport struct {
	Packs     int                       `json:"packs"`
	Fields    []MetadataFieldCoverage   `json:"fields"`
	Refs      MetadataRefCoverage       `json:"refs"`
	Freshness MetadataFreshnessCoverage `json:"freshness"`
}

type MetadataFieldCoverage struct {
	Name    string   `json:"name"`
	Present int      `json:"present"`
	Total   int      `json:"total"`
	Missing []string `json:"missing,omitempty"`
}

type MetadataRefCoverage struct {
	BareSkillRefs     int      `json:"bareSkillRefs"`
	ObjectSkillRefs   int      `json:"objectSkillRefs"`
	BarePluginRefs    int      `json:"barePluginRefs"`
	ObjectPluginRefs  int      `json:"objectPluginRefs"`
	PacksWithBareRefs int      `json:"packsWithBareRefs"`
	Packs             []string `json:"packs,omitempty"`
}

type MetadataFreshnessCoverage struct {
	Fresh         int      `json:"fresh"`
	Stale         int      `json:"stale"`
	Invalid       int      `json:"invalid"`
	Missing       int      `json:"missing"`
	ThresholdDays int      `json:"thresholdDays"`
	StalePacks    []string `json:"stalePacks,omitempty"`
	InvalidPacks  []string `json:"invalidPacks,omitempty"`
	MissingPacks  []string `json:"missingPacks,omitempty"`
}

type DoctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok | warn | info
	Detail string `json:"detail,omitempty"`
}

type DoctorReport struct {
	OK     bool          `json:"ok"`
	Checks []DoctorCheck `json:"checks"`
}
