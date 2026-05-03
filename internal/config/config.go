package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/sethdeckard/loadout/internal/domain"
	"github.com/sethdeckard/loadout/internal/fsx"
)

type TargetConfig struct {
	Enabled bool   `json:"enabled" toml:"enabled"`
	Path    string `json:"path" toml:"path"`
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
	Claude TargetConfig `json:"claude" toml:"claude"`
	Codex  TargetConfig `json:"codex" toml:"codex"`
}

type RepoActions struct {
	ImportAutoCommit bool `json:"import_auto_commit" toml:"import_auto_commit"`
	DeleteAutoCommit bool `json:"delete_auto_commit" toml:"delete_auto_commit"`
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
	RepoPath    string      `json:"repo_path" toml:"repo_path"`
	Targets     TargetPaths `json:"targets" toml:"targets"`
	RepoActions RepoActions `json:"repo_actions" toml:"repo_actions"`
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
	return filepath.Join(home, ".config", "loadout", "config.toml")
}

// LegacyJSONPath returns the well-known path of the pre-TOML config file.
// Used by HasExistingConfig to detect a legacy install at the default
// location; Load uses legacyPathFor instead so its behavior follows its
// argument.
func LegacyJSONPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "loadout", "config.json")
}

// legacyPathFor derives the legacy JSON path that corresponds to a given TOML
// config path by stem replacement, so Load(tempDir/config.toml) migrates
// tempDir/config.json. If path does not end in .toml, the suffix is appended
// rather than replaced so the helper still returns a sibling file.
func legacyPathFor(path string) string {
	if strings.HasSuffix(path, ".toml") {
		return strings.TrimSuffix(path, ".toml") + ".json"
	}
	return path + ".json"
}

// HasExistingConfig reports whether a config already exists at the default
// location, in either the new TOML format or the legacy JSON format. A
// non-IsNotExist stat error is propagated rather than treated as missing, so
// callers do not silently run init over an inaccessible config.
func HasExistingConfig() (bool, error) {
	for _, p := range []string{DefaultPath(), LegacyJSONPath()} {
		_, err := os.Stat(p)
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return false, fmt.Errorf("stat %s: %w", p, err)
		}
	}
	return false, nil
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

// Load reads the config at path. If path does not exist but a sibling legacy
// JSON file does, Load reads the JSON, writes a TOML file at path, and
// returns the resulting config; the legacy JSON is left in place untouched.
// If neither file exists, Load returns an error wrapping os.ErrNotExist.
func Load(path string) (Config, error) {
	cfg := Config{
		RepoActions: RepoActions{
			ImportAutoCommit: true,
			DeleteAutoCommit: true,
		},
	}

	data, err := os.ReadFile(path)
	if err == nil {
		if _, err := toml.Decode(string(data), &cfg); err != nil {
			return Config{}, fmt.Errorf("decode toml: %w", err)
		}
		return cfg, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	legacyPath := legacyPathFor(path)
	legacyData, err := os.ReadFile(legacyPath)
	if err == nil {
		var legacyCfg Config
		if err := json.Unmarshal(legacyData, &legacyCfg); err != nil {
			return Config{}, fmt.Errorf("decode legacy config: %w", err)
		}
		if err := Save(path, legacyCfg); err != nil {
			return Config{}, fmt.Errorf("migrate legacy config to toml: %w", err)
		}
		return legacyCfg, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("read legacy config: %w", err)
	}

	return Config{}, fmt.Errorf("config not found at %s: %w", path, os.ErrNotExist)
}

func Save(path string, cfg Config) error {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return fmt.Errorf("encode toml: %w", err)
	}
	return fsx.WriteFileAtomic(path, buf.Bytes())
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
