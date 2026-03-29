package importer

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sethdeckard/loadout/internal/config"
	"github.com/sethdeckard/loadout/internal/domain"
)

func writeSkillSource(t *testing.T, dir, md string, skillJSON string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if skillJSON != "" {
		if err := os.WriteFile(filepath.Join(dir, "skill.json"), []byte(skillJSON), 0o644); err != nil {
			t.Fatalf("write skill.json: %v", err)
		}
	}
}

func TestImport_InferSkillJSON(t *testing.T) {
	repo := t.TempDir()
	source := filepath.Join(t.TempDir(), "my-skill")
	writeSkillSource(t, source, "---\nname: My Skill\ndescription: imported\nallowed-tools: Read\n---\n\n# My Skill\nBody\n", "")

	result, err := Import(ImportParams{
		SourceDir: source,
		RepoPath:  repo,
		Targets:   []domain.Target{domain.TargetClaude},
	})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if got, want := result.Skill.Name, domain.SkillName("my-skill"); got != want {
		t.Fatalf("Name = %q, want %q", got, want)
	}
	if result.Skill.SupportsTarget(domain.TargetCodex) {
		t.Fatal("expected inferred skill to remain single-target")
	}
	data, err := os.ReadFile(filepath.Join(result.DestDir, "skill.json"))
	if err != nil {
		t.Fatalf("read skill.json: %v", err)
	}
	if !strings.Contains(string(data), `"targets": [`+"\n"+`    "claude"`) {
		t.Fatalf("skill.json = %s", data)
	}
	md, err := os.ReadFile(filepath.Join(result.DestDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	if strings.HasPrefix(string(md), "---\n") {
		t.Fatalf("expected stripped frontmatter, got:\n%s", md)
	}
}

func TestImport_InfersNameFromDirName(t *testing.T) {
	repo := t.TempDir()
	source := filepath.Join(t.TempDir(), "my-cool-skill")
	writeSkillSource(t, source, "# Cool Skill\nBody\n", `{"name":"my-cool-skill","targets":["claude"]}`)

	result, err := Import(ImportParams{SourceDir: source, RepoPath: repo})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if got, want := result.Skill.Name, domain.SkillName("my-cool-skill"); got != want {
		t.Fatalf("Name = %q, want %q", got, want)
	}
}

func TestImport_PreservesDeclaredTargets(t *testing.T) {
	repo := t.TempDir()
	source := filepath.Join(t.TempDir(), "skill")
	writeSkillSource(t, source, "# Test\n", `{
  "name": "test-skill",
  "description": "desc",
  "targets": ["claude"],
  "claude": {"allowed-tools": "Read"}
}`)

	result, err := Import(ImportParams{SourceDir: source, RepoPath: repo})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if !result.Skill.SupportsTarget(domain.TargetClaude) || result.Skill.SupportsTarget(domain.TargetCodex) {
		t.Fatalf("targets = %v, want [claude]", result.Skill.Targets)
	}
	if _, ok := result.Skill.Claude["allowed-tools"]; !ok {
		t.Fatalf("expected claude block to be preserved")
	}
}

func TestImport_RemovesMarker(t *testing.T) {
	repo := t.TempDir()
	source := filepath.Join(t.TempDir(), "skill")
	writeSkillSource(t, source, "# Test\n", `{"name":"test-skill","targets":["claude"]}`)
	if err := os.WriteFile(filepath.Join(source, ".loadout"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	result, err := Import(ImportParams{SourceDir: source, RepoPath: repo})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(result.DestDir, ".loadout")); !os.IsNotExist(err) {
		t.Fatalf("expected no .loadout marker, err = %v", err)
	}
}

func TestImport_SkillExists(t *testing.T) {
	repo := t.TempDir()
	dest := filepath.Join(repo, "skills", "test-skill")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	source := filepath.Join(t.TempDir(), "skill")
	writeSkillSource(t, source, "# Test\n", `{"name":"test-skill","targets":["claude"]}`)

	_, err := Import(ImportParams{SourceDir: source, RepoPath: repo})
	if !errors.Is(err, domain.ErrSkillExists) {
		t.Fatalf("Import() error = %v, want ErrSkillExists", err)
	}
}

func TestDiscoverCandidates_DuplicateConflict(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, "claude")
	codexDir := filepath.Join(root, "codex")
	writeSkillSource(t, filepath.Join(claudeDir, "shared-skill"), "# Test\n", `{"name":"shared-skill","targets":["claude"]}`)
	writeSkillSource(t, filepath.Join(codexDir, "shared-skill"), "# Test\n", `{"name":"shared-skill","targets":["codex"]}`)

	candidates, err := DiscoverCandidates(config.TargetPaths{
		Claude: config.TargetConfig{Enabled: true, Path: claudeDir},
		Codex:  config.TargetConfig{Enabled: true, Path: codexDir},
	})
	if err != nil {
		t.Fatalf("DiscoverCandidates() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(candidates))
	}
	if candidates[0].Ready {
		t.Fatalf("expected duplicate conflict candidate")
	}
}

func TestDiscoverCandidates_IdenticalDuplicateReady(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, "claude")
	codexDir := filepath.Join(root, "codex")
	skillJSON := `{"name":"shared-skill","targets":["claude","codex"]}`
	writeSkillSource(t, filepath.Join(claudeDir, "shared-skill"), "# Test\n", skillJSON)
	writeSkillSource(t, filepath.Join(codexDir, "shared-skill"), "# Test\n", skillJSON)

	candidates, err := DiscoverCandidates(config.TargetPaths{
		Claude: config.TargetConfig{Enabled: true, Path: claudeDir},
		Codex:  config.TargetConfig{Enabled: true, Path: codexDir},
	})
	if err != nil {
		t.Fatalf("DiscoverCandidates() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(candidates))
	}
	if !candidates[0].Ready || !candidates[0].Duplicate {
		t.Fatalf("candidate = %+v, want ready duplicate", candidates[0])
	}
}

func TestDiscoverCandidatesInDir_ProjectLayout(t *testing.T) {
	root := t.TempDir()
	claudeSkills := filepath.Join(root, ".claude", "skills")
	codexSkills := filepath.Join(root, ".codex", "skills")
	writeSkillSource(t, filepath.Join(claudeSkills, "skill-a"), "# Skill A\n", `{"name":"skill-a","targets":["claude"]}`)
	writeSkillSource(t, filepath.Join(codexSkills, "skill-b"), "# Skill B\n", `{"name":"skill-b","targets":["codex"]}`)

	candidates, err := DiscoverCandidatesInDir(root, []domain.Target{domain.TargetClaude, domain.TargetCodex})
	if err != nil {
		t.Fatalf("DiscoverCandidatesInDir() error = %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("len(candidates) = %d, want 2", len(candidates))
	}

	byName := map[domain.SkillName]Candidate{}
	for _, c := range candidates {
		byName[c.SkillName] = c
	}

	a, ok := byName["skill-a"]
	if !ok {
		t.Fatal("missing skill-a")
	}
	if len(a.FromRoots) != 1 || a.FromRoots[0] != domain.TargetClaude {
		t.Errorf("skill-a FromRoots = %v, want [claude]", a.FromRoots)
	}

	b, ok := byName["skill-b"]
	if !ok {
		t.Fatal("missing skill-b")
	}
	if len(b.FromRoots) != 1 || b.FromRoots[0] != domain.TargetCodex {
		t.Errorf("skill-b FromRoots = %v, want [codex]", b.FromRoots)
	}
}

func TestDiscoverCandidatesInDir_ProjectLayout_MergesTargets(t *testing.T) {
	root := t.TempDir()
	claudeSkills := filepath.Join(root, ".claude", "skills")
	codexSkills := filepath.Join(root, ".codex", "skills")
	writeSkillSource(t, filepath.Join(claudeSkills, "shared-skill"), "# Shared Skill\n", `{"name":"shared-skill","targets":["claude"]}`)
	writeSkillSource(t, filepath.Join(codexSkills, "shared-skill"), "# Shared Skill\n", `{"name":"shared-skill","targets":["codex"]}`)

	candidates, err := DiscoverCandidatesInDir(root, []domain.Target{domain.TargetClaude, domain.TargetCodex})
	if err != nil {
		t.Fatalf("DiscoverCandidatesInDir() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(candidates))
	}

	candidate := candidates[0]
	if !candidate.Ready {
		t.Fatalf("candidate should be ready, got problem %q", candidate.Problem)
	}
	if !candidate.Duplicate {
		t.Fatal("expected duplicate candidate from per-target copies")
	}
	if got, want := candidate.Targets, []domain.Target{domain.TargetClaude, domain.TargetCodex}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Targets = %v, want %v", got, want)
	}
	if got, want := candidate.FromRoots, []domain.Target{domain.TargetClaude, domain.TargetCodex}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("FromRoots = %v, want %v", got, want)
	}
}

func TestDiscoverCandidatesInDir_FlatRoot(t *testing.T) {
	root := t.TempDir()
	writeSkillSource(t, filepath.Join(root, "my-skill"), "# My Skill\n", `{"name":"my-skill","targets":["claude"]}`)

	candidates, err := DiscoverCandidatesInDir(root, []domain.Target{domain.TargetClaude, domain.TargetCodex})
	if err != nil {
		t.Fatalf("DiscoverCandidatesInDir() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(candidates))
	}
	if candidates[0].SkillName != "my-skill" {
		t.Errorf("SkillName = %q, want my-skill", candidates[0].SkillName)
	}
	if !candidates[0].Ready {
		t.Errorf("expected candidate to be ready")
	}
}

func TestDiscoverCandidatesInDir_FlatRoot_NoFalseConflict(t *testing.T) {
	root := t.TempDir()
	// Skill with no skill.json — relies on inference
	writeSkillSource(t, filepath.Join(root, "inferred-skill"), "# Inferred\n", "")

	candidates, err := DiscoverCandidatesInDir(root, []domain.Target{domain.TargetClaude, domain.TargetCodex})
	if err != nil {
		t.Fatalf("DiscoverCandidatesInDir() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(candidates))
	}
	if !candidates[0].Ready {
		t.Fatalf("candidate not ready: %s", candidates[0].Problem)
	}
	if candidates[0].Duplicate {
		t.Fatal("expected non-duplicate for single source directory")
	}
}

func TestDiscoverCandidatesInDir_LoadoutRepoLayout(t *testing.T) {
	root := t.TempDir()
	writeSkillSource(t, filepath.Join(root, "skills", "repo-skill"), "# Repo Skill\n", `{"name":"repo-skill","targets":["claude"]}`)

	candidates, err := DiscoverCandidatesInDir(root, []domain.Target{domain.TargetClaude, domain.TargetCodex})
	if err != nil {
		t.Fatalf("DiscoverCandidatesInDir() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(candidates))
	}
	if got, want := candidates[0].SkillName, domain.SkillName("repo-skill"); got != want {
		t.Fatalf("SkillName = %q, want %q", got, want)
	}
	if !candidates[0].Ready {
		t.Fatalf("candidate not ready: %s", candidates[0].Problem)
	}
}

func TestPreviewSourceDir_OverridesTargets(t *testing.T) {
	root := filepath.Join(t.TempDir(), "shared-skill")
	writeSkillSource(t, root, "# Shared Skill\n", `{"name":"shared-skill","targets":["claude"]}`)

	preview, err := PreviewSourceDir(root, []domain.Target{domain.TargetClaude, domain.TargetCodex})
	if err != nil {
		t.Fatalf("PreviewSourceDir() error = %v", err)
	}
	if got, want := preview.Skill.Targets, []domain.Target{domain.TargetClaude, domain.TargetCodex}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Targets = %v, want %v", got, want)
	}
}

func TestImport_PreservesExtraFiles(t *testing.T) {
	repo := t.TempDir()
	source := filepath.Join(t.TempDir(), "my-skill")
	writeSkillSource(t, source, "# My Skill\n", `{"name":"my-skill","targets":["claude"]}`)

	// Add extra files
	refDir := filepath.Join(source, "references")
	if err := os.MkdirAll(refDir, 0o755); err != nil {
		t.Fatalf("mkdir references: %v", err)
	}
	if err := os.WriteFile(filepath.Join(refDir, "notes.md"), []byte("# Notes\n"), 0o644); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}

	result, err := Import(ImportParams{
		SourceDir: source,
		RepoPath:  repo,
		Targets:   []domain.Target{domain.TargetClaude},
	})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	notesPath := filepath.Join(result.DestDir, "references", "notes.md")
	if _, err := os.Stat(notesPath); err != nil {
		t.Fatalf("extra file not preserved after import: %v", err)
	}
}

func writeMarker(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".loadout"), []byte(`{"repo_commit":"abc","installed_at":"2025-01-01T00:00:00Z"}`), 0o644); err != nil {
		t.Fatalf("write .loadout: %v", err)
	}
}

func TestDiscoverCandidates_IncludesOrphans(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, "claude")
	codexDir := filepath.Join(root, "codex")

	// Create a managed install (orphan — no repo to exclude it)
	skillDir := filepath.Join(claudeDir, "orphan-skill")
	writeSkillSource(t, skillDir, "# Orphan\n", `{"name":"orphan-skill","targets":["claude"]}`)
	writeMarker(t, skillDir)

	candidates, err := DiscoverCandidates(config.TargetPaths{
		Claude: config.TargetConfig{Enabled: true, Path: claudeDir},
		Codex:  config.TargetConfig{Enabled: true, Path: codexDir},
	})
	if err != nil {
		t.Fatalf("DiscoverCandidates() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(candidates))
	}
	if candidates[0].SkillName != "orphan-skill" {
		t.Errorf("SkillName = %q", candidates[0].SkillName)
	}
	if !candidates[0].Orphan {
		t.Error("expected Orphan = true")
	}
	if !candidates[0].Ready {
		t.Errorf("expected Ready = true, got Problem = %q", candidates[0].Problem)
	}
}

func TestDiscoverCandidates_SymlinkNotReady(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior differs on Windows")
	}

	root := t.TempDir()
	claudeDir := filepath.Join(root, "claude")

	// Create a skill with a symlinked file
	skillDir := filepath.Join(claudeDir, "symlink-skill")
	writeSkillSource(t, skillDir, "# Symlink\n", `{"name":"symlink-skill","targets":["claude"]}`)
	target := filepath.Join(root, "external.txt")
	if err := os.WriteFile(target, []byte("secret"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(skillDir, "link.txt")); err != nil {
		t.Fatalf("setup symlink: %v", err)
	}

	candidates, err := DiscoverCandidates(config.TargetPaths{
		Claude: config.TargetConfig{Enabled: true, Path: claudeDir},
	})
	if err != nil {
		t.Fatalf("DiscoverCandidates() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(candidates))
	}
	if candidates[0].Ready {
		t.Error("expected Ready = false for candidate with symlink")
	}
	if candidates[0].Problem == "" {
		t.Error("expected non-empty Problem for candidate with symlink")
	}
}

func TestDiscoverCandidates_OrphanFlagNotSetForUnmanaged(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, "claude")

	// No .loadout marker — unmanaged
	writeSkillSource(t, filepath.Join(claudeDir, "local-skill"), "# Local\n", `{"name":"local-skill","targets":["claude"]}`)

	candidates, err := DiscoverCandidates(config.TargetPaths{
		Claude: config.TargetConfig{Enabled: true, Path: claudeDir},
	})
	if err != nil {
		t.Fatalf("DiscoverCandidates() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(candidates))
	}
	if candidates[0].Orphan {
		t.Error("expected Orphan = false for unmanaged skill")
	}
}

func TestDiscoverCandidates_ConflictingOrphans(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, "claude")
	codexDir := filepath.Join(root, "codex")

	// Same name, different content in claude vs codex — both managed
	claudeSkill := filepath.Join(claudeDir, "conflict-skill")
	codexSkill := filepath.Join(codexDir, "conflict-skill")
	writeSkillSource(t, claudeSkill, "# Claude version\n", `{"name":"conflict-skill","targets":["claude"]}`)
	writeMarker(t, claudeSkill)
	writeSkillSource(t, codexSkill, "# Codex version\n", `{"name":"conflict-skill","targets":["codex"]}`)
	writeMarker(t, codexSkill)

	candidates, err := DiscoverCandidates(config.TargetPaths{
		Claude: config.TargetConfig{Enabled: true, Path: claudeDir},
		Codex:  config.TargetConfig{Enabled: true, Path: codexDir},
	})
	if err != nil {
		t.Fatalf("DiscoverCandidates() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1", len(candidates))
	}
	c := candidates[0]
	if !c.Orphan {
		t.Error("expected Orphan = true")
	}
	if c.Ready {
		t.Error("expected Ready = false for conflicting orphan copies")
	}
	if !strings.Contains(c.Problem, "conflicting") {
		t.Errorf("Problem = %q, want conflicting reason", c.Problem)
	}
}

func TestPreviewSourceDir_RejectsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink behavior differs on Windows")
	}

	source := filepath.Join(t.TempDir(), "my-skill")
	writeSkillSource(t, source, "# Skill\n", `{"name":"my-skill","targets":["claude"]}`)

	// Add a symlinked file
	target := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(target, []byte("sensitive"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(source, "link.txt")); err != nil {
		t.Fatalf("setup symlink: %v", err)
	}

	_, err := PreviewSourceDir(source, []domain.Target{domain.TargetClaude})
	if err == nil {
		t.Fatal("expected error for symlink in source directory")
	}
	if !errors.Is(err, domain.ErrSymlinkInTree) {
		t.Errorf("error = %v, want ErrSymlinkInTree", err)
	}
}

func TestPrepareImport_NormalizesSkillJSON(t *testing.T) {
	source := filepath.Join(t.TempDir(), "skill")
	writeSkillSource(t, source, "# Test\n", `{"name":"test-skill","targets":["claude"]}`)

	prepared, err := prepareImport(source, nil)
	if err != nil {
		t.Fatalf("prepareImport() error = %v", err)
	}
	var skill domain.Skill
	if err := json.Unmarshal(prepared.skillJSON, &skill); err != nil {
		t.Fatalf("unmarshal normalized skill.json: %v", err)
	}
	if got, want := skill.Name, domain.SkillName("test-skill"); got != want {
		t.Fatalf("Name = %q, want %q", got, want)
	}
}
