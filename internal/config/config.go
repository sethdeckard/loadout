package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sethdeckard/loadout/internal/domain"
	"github.com/sethdeckard/loadout/internal/fsx"
)

type TargetConfig struct {
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"`
}

func (t *TargetConfig) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*t = TargetConfig{}
		return nil
	}

	if len(data) > 0 && data[0] == '"' {
		var path string
		if err := json.Unmarshal(data, &path); err != nil {
			return err
		}
		*t = TargetConfig{
			Enabled: path != "",
			Path:    path,
		}
		return nil
	}

	var raw struct {
		Enabled *bool  `json:"enabled"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	cfg := TargetConfig{
		Path: raw.Path,
	}
	if raw.Enabled != nil {
		cfg.Enabled = *raw.Enabled
	} else if cfg.Path != "" {
		// Preserve current behavior for legacy configs missing "enabled".
		cfg.Enabled = true
	}
	*t = cfg
	return nil
}

type TargetPaths struct {
	Claude TargetConfig `json:"claude"`
	Codex  TargetConfig `json:"codex"`
}

type RepoActions struct {
	ImportAutoCommit bool `json:"import_auto_commit"`
	DeleteAutoCommit bool `json:"delete_auto_commit"`
}

func (r *RepoActions) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*r = RepoActions{
			ImportAutoCommit: true,
			DeleteAutoCommit: true,
		}
		return nil
	}

	var raw struct {
		ImportAutoCommit *bool `json:"import_auto_commit"`
		DeleteAutoCommit *bool `json:"delete_auto_commit"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	actions := RepoActions{
		ImportAutoCommit: true,
		DeleteAutoCommit: true,
	}
	if raw.ImportAutoCommit != nil {
		actions.ImportAutoCommit = *raw.ImportAutoCommit
	}
	if raw.DeleteAutoCommit != nil {
		actions.DeleteAutoCommit = *raw.DeleteAutoCommit
	}
	*r = actions
	return nil
}

type Config struct {
	RepoPath    string      `json:"repo_path"`
	Targets     TargetPaths `json:"targets"`
	RepoActions RepoActions `json:"repo_actions"`
}

func (c *Config) UnmarshalJSON(data []byte) error {
	type rawConfig struct {
		RepoPath    string       `json:"repo_path"`
		Targets     TargetPaths  `json:"targets"`
		RepoActions *RepoActions `json:"repo_actions"`
	}

	var raw rawConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	cfg := Config{
		RepoPath: raw.RepoPath,
		Targets:  raw.Targets,
		RepoActions: RepoActions{
			ImportAutoCommit: true,
			DeleteAutoCommit: true,
		},
	}
	if raw.RepoActions != nil {
		cfg.RepoActions = *raw.RepoActions
	}

	*c = cfg
	return nil
}

func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "loadout", "config.json")
}

func Default() Config {
	home, _ := os.UserHomeDir()
	return Config{
		RepoPath: "",
		Targets: TargetPaths{
			Claude: TargetConfig{
				Enabled: true,
				Path:    filepath.Join(home, ".claude", "skills"),
			},
			Codex: TargetConfig{
				Enabled: true,
				Path:    filepath.Join(home, ".codex", "skills"),
			},
		},
		RepoActions: RepoActions{
			ImportAutoCommit: true,
			DeleteAutoCommit: true,
		},
	}
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	return fsx.WriteJSONAtomic(path, cfg)
}

func (t TargetPaths) For(target domain.Target) TargetConfig {
	switch target {
	case domain.TargetClaude:
		return t.Claude
	case domain.TargetCodex:
		return t.Codex
	default:
		return TargetConfig{}
	}
}

func (t TargetPaths) Path(target domain.Target) string {
	return t.For(target).Path
}

func (t TargetPaths) Enabled(target domain.Target) bool {
	cfg := t.For(target)
	return cfg.Enabled && cfg.Path != ""
}

func (t TargetPaths) ConfiguredTargets() []domain.Target {
	var targets []domain.Target
	for _, target := range domain.AllTargets() {
		if t.Enabled(target) {
			targets = append(targets, target)
		}
	}
	return targets
}
