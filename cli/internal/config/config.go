package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sandeshh/agent-packs/cli/internal/model"
	"github.com/sandeshh/agent-packs/cli/internal/targets"
	"github.com/sandeshh/agent-packs/cli/internal/util"
	"gopkg.in/yaml.v3"
)

const DefaultFilename = ".agent-packs.yaml"

type ProjectConfig struct {
	Agent      string `yaml:"agent,omitempty" json:"agent,omitempty"`
	Mode       string `yaml:"mode,omitempty" json:"mode,omitempty"`
	OnConflict string `yaml:"onConflict,omitempty" json:"onConflict,omitempty"`
	Scope      string `yaml:"scope,omitempty" json:"scope,omitempty"`
	Registry   string `yaml:"registry,omitempty" json:"registry,omitempty"`
	Target     string `yaml:"target,omitempty" json:"target,omitempty"`
}

type InitOptions struct {
	Agent      string
	Mode       string
	OnConflict string
	Scope      string
	Registry   string
	Target     string
	Force      bool
}

func Init(projectDir string, opts InitOptions) (string, error) {
	projectDir = util.ExpandHome(projectDir)
	if projectDir == "" {
		projectDir = "."
	}
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(projectDir, DefaultFilename)
	if _, err := os.Stat(path); err == nil && !opts.Force {
		return "", fmt.Errorf("%s already exists; pass --force to overwrite", path)
	}
	cfg := ProjectConfig{
		Agent:      defaultString(opts.Agent, "codex"),
		Mode:       defaultString(opts.Mode, "reference"),
		OnConflict: defaultString(opts.OnConflict, "skip"),
		Scope:      defaultString(opts.Scope, "project"),
		Registry:   opts.Registry,
		Target:     defaultString(opts.Target, ".agent-packs"),
	}
	cfg.Agent = targets.NormalizeAgent(cfg.Agent)
	if !targets.ValidAgent(cfg.Agent) {
		return "", fmt.Errorf("invalid agent %q", cfg.Agent)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	header := "# Agent Packs project configuration\n# https://github.com/sandeshh/agent-packs\n\n"
	if err := os.WriteFile(path, append([]byte(header), data...), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func LoadProjectConfig(projectDir string) (ProjectConfig, error) {
	path := filepath.Join(util.ExpandHome(projectDir), DefaultFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		return ProjectConfig{}, err
	}
	var cfg ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ProjectConfig{}, err
	}
	if cfg.Agent != "" {
		cfg.Agent = targets.NormalizeAgent(cfg.Agent)
	}
	return cfg, nil
}

func MergeInstallOptions(cfg ProjectConfig, options model.InstallOptions) model.InstallOptions {
	if cfg.Mode != "" && options.Mode == "" {
		options.Mode = cfg.Mode
	}
	if cfg.OnConflict != "" && options.OnConflict == "" {
		options.OnConflict = cfg.OnConflict
	}
	if cfg.Scope != "" && options.Scope == "" {
		options.Scope = cfg.Scope
	}
	return options
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
