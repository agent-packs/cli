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
	Registry string       `json:"registry"`
	OK       bool         `json:"ok"`
	Checks   []CheckEntry `json:"checks"`
}

type CheckEntry struct {
	Kind    string `json:"kind"`
	Target  string `json:"target"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}
