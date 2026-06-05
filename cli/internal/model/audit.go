package model

type AuditReport struct {
	Pack       AuditPack        `json:"pack"`
	Components []AuditComponent `json:"components"`
	OK         bool             `json:"ok"`
	Violations []string         `json:"violations,omitempty"`
}

type AuditPack struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Version        string `json:"version"`
	License        string `json:"license"`
	UpstreamSource string `json:"upstreamSource,omitempty"`
}

type AuditComponent struct {
	Type           string `json:"type"`
	Name           string `json:"name"`
	Source         string `json:"source"`
	UpstreamSource string `json:"upstreamSource,omitempty"`
	Format         string `json:"format"`
	License        string `json:"license"`
	Trust          string `json:"trust,omitempty"`
	Resolution     struct {
		Kind     string `json:"kind"`
		Revision string `json:"revision,omitempty"`
		Pinned   bool   `json:"pinned"`
		Warning  string `json:"warning,omitempty"`
	} `json:"resolution"`
	Integrity struct {
		Checksum  string `json:"checksum,omitempty"`
		Signature string `json:"signature,omitempty"`
	} `json:"integrity,omitempty"`
	NativeCommand bool `json:"nativeCommand,omitempty"`
}
