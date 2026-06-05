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
