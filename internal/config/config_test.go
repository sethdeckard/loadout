package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

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

func TestLoadMissing(t *testing.T) {
	_, err := Load("/nonexistent/config.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{bad json"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDefaultPath(t *testing.T) {
	p := DefaultPath()
	if p == "" {
		t.Fatal("DefaultPath() returned empty string")
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
	path := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{
  "repo_path": "/tmp/repo",
  "targets": {
    "claude": "/tmp/claude",
    "codex": "/tmp/codex"
  }
}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cfg, err := Load(path)
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
}

func TestLoad_LegacyMissingRepoActionsDefaultsTrue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{
  "repo_path": "/tmp/repo",
  "targets": {
    "claude": {"enabled": true, "path": "/tmp/claude"},
    "codex": {"enabled": true, "path": "/tmp/codex"}
  }
}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.RepoActions.ImportAutoCommit || !cfg.RepoActions.DeleteAutoCommit {
		t.Fatalf("repo actions = %+v, want both true", cfg.RepoActions)
	}
}
