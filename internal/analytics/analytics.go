package analytics

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/agent-packs/cli/internal/util"
	"github.com/agent-packs/cli/internal/version"
)

const defaultEndpoint = "https://analytics.agent-packs.dev/v1/track"

type Config struct {
	Enabled     bool   `json:"enabled"`
	AnonymousID string `json:"anonymousId"`
	Endpoint    string `json:"endpoint,omitempty"`
}

func configPath(home string) string {
	return filepath.Join(util.ExpandHome(home), "analytics.json")
}

func LoadConfig(home string) (Config, error) {
	data, err := os.ReadFile(configPath(home))
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func saveConfig(home string, cfg Config) error {
	path := configPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func Enable(home string) error {
	cfg, err := LoadConfig(home)
	if err != nil {
		return err
	}
	cfg.Enabled = true
	if cfg.AnonymousID == "" {
		cfg.AnonymousID = generateID()
	}
	return saveConfig(home, cfg)
}

func Disable(home string) error {
	cfg, err := LoadConfig(home)
	if err != nil {
		return err
	}
	cfg.Enabled = false
	return saveConfig(home, cfg)
}

func Status(home string, out io.Writer) error {
	cfg, err := LoadConfig(home)
	if err != nil {
		return err
	}
	if cfg.Enabled {
		fmt.Fprintf(out, "Analytics: enabled (anonymous ID: %s)\n", cfg.AnonymousID)
		ep := cfg.Endpoint
		if ep == "" {
			ep = defaultEndpoint
		}
		fmt.Fprintf(out, "Endpoint:  %s\n", ep)
	} else {
		fmt.Fprintln(out, "Analytics: disabled")
		fmt.Fprintln(out, "Enable with: agent-packs analytics enable")
	}
	return nil
}

type event struct {
	AnonymousID string `json:"anonymousId"`
	Event       string `json:"event"`
	PackID      string `json:"packId"`
	ToolID      string `json:"toolId,omitempty"`
	PackVersion string `json:"packVersion,omitempty"`
	CLIVersion  string `json:"cliVersion"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	Timestamp   string `json:"timestamp"`
}

// Track fires an install/upgrade/uninstall event. The send is synchronous with
// a short timeout: a background goroutine would race process exit and the
// event would almost never be delivered from a short-lived CLI run. Failures
// are silently ignored — analytics must never break a command.
func Track(home, eventName, packID, toolID, packVersion string) {
	cfg, err := LoadConfig(home)
	if err != nil || !cfg.Enabled || cfg.AnonymousID == "" {
		return
	}
	ep := cfg.Endpoint
	if ep == "" {
		ep = defaultEndpoint
	}
	e := event{
		AnonymousID: cfg.AnonymousID,
		Event:       eventName,
		PackID:      packID,
		ToolID:      toolID,
		PackVersion: packVersion,
		CLIVersion:  version.String(),
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(e)
	if err != nil {
		return
	}
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest("POST", ep, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}
