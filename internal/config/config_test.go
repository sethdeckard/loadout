package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeJSON(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := Config{
		RepoPath: filepath.Join(dir, "repo"),
		Targets: TargetPaths{
			Claude: TargetConfig{Enabled: true, Path: filepath.Join(dir, "claude")},
			Codex:  TargetConfig{Enabled: true, Path: filepath.Join(dir, "codex")},
		},
		RepoActions: RepoActions{
			ImportAutoCommit: false,
			DeleteAutoCommit: true,
		},
	}

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.RepoPath != cfg.RepoPath {
		t.Errorf("RepoPath = %q, want %q", got.RepoPath, cfg.RepoPath)
	}
	if got.Targets.Claude != cfg.Targets.Claude {
		t.Errorf("Targets.Claude = %+v, want %+v", got.Targets.Claude, cfg.Targets.Claude)
	}
	if got.Targets.Codex != cfg.Targets.Codex {
		t.Errorf("Targets.Codex = %+v, want %+v", got.Targets.Codex, cfg.Targets.Codex)
	}
	if got.RepoActions != cfg.RepoActions {
		t.Errorf("RepoActions = %+v, want %+v", got.RepoActions, cfg.RepoActions)
	}
}

func TestLoad_NeitherFileExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error when neither config.toml nor config.json exists")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("error = %v, want it to wrap os.ErrNotExist", err)
	}
}

func TestLoad_InvalidTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("repo_path = \"unterminated"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

func TestDefaultPath(t *testing.T) {
	p := DefaultPath()
	if p == "" {
		t.Fatal("DefaultPath() returned empty string")
	}
	if filepath.Ext(p) != ".toml" {
		t.Errorf("DefaultPath() = %q, want .toml extension", p)
	}
}

func TestLegacyJSONPath(t *testing.T) {
	p := LegacyJSONPath()
	if p == "" {
		t.Fatal("LegacyJSONPath() returned empty string")
	}
	if filepath.Ext(p) != ".json" {
		t.Errorf("LegacyJSONPath() = %q, want .json extension", p)
	}
}

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Targets.Claude.Path == "" || cfg.Targets.Codex.Path == "" {
		t.Fatal("Default() should have non-empty target paths")
	}
	if !cfg.RepoActions.ImportAutoCommit || !cfg.RepoActions.DeleteAutoCommit {
		t.Fatalf("RepoActions = %+v, want both true", cfg.RepoActions)
	}
}

func TestLoad_LegacyStringTargets(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	jsonPath := filepath.Join(dir, "config.json")
	data := []byte(`{
  "repo_path": "/tmp/repo",
  "targets": {
    "claude": "/tmp/claude",
    "codex": "/tmp/codex"
  }
}`)
	if err := os.WriteFile(jsonPath, data, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cfg, err := Load(tomlPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Targets.Claude.Enabled || cfg.Targets.Claude.Path != "/tmp/claude" {
		t.Fatalf("claude target = %+v", cfg.Targets.Claude)
	}
	if !cfg.Targets.Codex.Enabled || cfg.Targets.Codex.Path != "/tmp/codex" {
		t.Fatalf("codex target = %+v", cfg.Targets.Codex)
	}
	if !cfg.RepoActions.ImportAutoCommit || !cfg.RepoActions.DeleteAutoCommit {
		t.Fatalf("repo actions = %+v, want both true", cfg.RepoActions)
	}

	// Migration side effects: TOML written, original JSON untouched.
	if _, err := os.Stat(tomlPath); err != nil {
		t.Fatalf("expected TOML file at %s after migration: %v", tomlPath, err)
	}
	got, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read legacy JSON after migration: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("legacy JSON modified; want untouched")
	}
}

func TestLoad_LegacyMissingRepoActionsDefaultsTrue(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	jsonPath := filepath.Join(dir, "config.json")
	data := []byte(`{
  "repo_path": "/tmp/repo",
  "targets": {
    "claude": {"enabled": true, "path": "/tmp/claude"},
    "codex": {"enabled": true, "path": "/tmp/codex"}
  }
}`)
	if err := os.WriteFile(jsonPath, data, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cfg, err := Load(tomlPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.RepoActions.ImportAutoCommit || !cfg.RepoActions.DeleteAutoCommit {
		t.Fatalf("repo actions = %+v, want both true", cfg.RepoActions)
	}
	if _, err := os.Stat(tomlPath); err != nil {
		t.Fatalf("expected TOML file at %s after migration: %v", tomlPath, err)
	}
}

func TestLoad_AutoMigratesJSON(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	jsonPath := filepath.Join(dir, "config.json")

	original := Config{
		RepoPath: filepath.Join(dir, "repo"),
		Targets: TargetPaths{
			Claude: TargetConfig{Enabled: true, Path: filepath.Join(dir, "claude")},
			Codex:  TargetConfig{Enabled: false, Path: ""},
		},
		RepoActions: RepoActions{
			ImportAutoCommit: false,
			DeleteAutoCommit: true,
		},
	}

	// Write a JSON config the way the old Save would have.
	if err := writeJSON(jsonPath, original); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cfg, err := Load(tomlPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.RepoPath != original.RepoPath {
		t.Errorf("RepoPath = %q, want %q", cfg.RepoPath, original.RepoPath)
	}
	if cfg.Targets.Claude != original.Targets.Claude {
		t.Errorf("Targets.Claude = %+v, want %+v", cfg.Targets.Claude, original.Targets.Claude)
	}
	if cfg.Targets.Codex != original.Targets.Codex {
		t.Errorf("Targets.Codex = %+v, want %+v", cfg.Targets.Codex, original.Targets.Codex)
	}
	if cfg.RepoActions != original.RepoActions {
		t.Errorf("RepoActions = %+v, want %+v", cfg.RepoActions, original.RepoActions)
	}

	// Re-loading the now-existing TOML returns equivalent values without re-migrating.
	again, err := Load(tomlPath)
	if err != nil {
		t.Fatalf("second Load() error = %v", err)
	}
	if again != cfg {
		t.Errorf("second load = %+v, want %+v", again, cfg)
	}
}

func TestLoad_OmittedTargetTableDisablesTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	data := []byte(`repo_path = "/tmp/repo"

[targets.claude]
enabled = true
path = "/tmp/claude"
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Targets.Codex.Enabled || cfg.Targets.Codex.Path != "" {
		t.Errorf("Targets.Codex = %+v, want zero value (disabled, empty path)", cfg.Targets.Codex)
	}
}

func TestLoad_OmittedRepoActionsDefaultsTrue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	data := []byte(`repo_path = "/tmp/repo"

[targets.claude]
enabled = true
path = "/tmp/claude"
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.RepoActions.ImportAutoCommit || !cfg.RepoActions.DeleteAutoCommit {
		t.Errorf("RepoActions = %+v, want both true", cfg.RepoActions)
	}
}

func TestLoad_PartialRepoActionsPreservesDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	data := []byte(`repo_path = "/tmp/repo"

[repo_actions]
import_auto_commit = false
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.RepoActions.ImportAutoCommit {
		t.Errorf("ImportAutoCommit = true, want false (explicit)")
	}
	if !cfg.RepoActions.DeleteAutoCommit {
		t.Errorf("DeleteAutoCommit = false, want true (default preserved)")
	}
}

func TestHasExistingConfig(t *testing.T) {
	tests := []struct {
		name      string
		writeTOML bool
		writeJSON bool
		want      bool
	}{
		{name: "neither", writeTOML: false, writeJSON: false, want: false},
		{name: "only_toml", writeTOML: true, writeJSON: false, want: true},
		{name: "only_json", writeTOML: false, writeJSON: true, want: true},
		{name: "both", writeTOML: true, writeJSON: true, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)

			cfgDir := filepath.Join(home, ".config", "loadout")
			if err := os.MkdirAll(cfgDir, 0o755); err != nil {
				t.Fatalf("setup: %v", err)
			}
			if tt.writeTOML {
				if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte("repo_path = \"\"\n"), 0o644); err != nil {
					t.Fatalf("setup toml: %v", err)
				}
			}
			if tt.writeJSON {
				if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte("{}"), 0o644); err != nil {
					t.Fatalf("setup json: %v", err)
				}
			}

			got, err := HasExistingConfig()
			if err != nil {
				t.Fatalf("HasExistingConfig() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("HasExistingConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasExistingConfig_PropagatesStatError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission model differs on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)

	// Create the config directory but make it unreadable so that os.Stat on
	// children fails with EACCES rather than ENOENT.
	cfgDir := filepath.Join(home, ".config", "loadout")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.Chmod(cfgDir, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(cfgDir, 0o755) })

	_, err := HasExistingConfig()
	if err == nil {
		t.Fatal("expected error when stat fails with non-IsNotExist")
	}
	if errors.Is(err, os.ErrNotExist) {
		t.Errorf("error = %v, want non-IsNotExist", err)
	}
}
