package install

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sethdeckard/loadout/internal/domain"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills", "test-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Test\n\nSome content."), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	return dir
}

func testSkill() domain.Skill {
	return domain.Skill{
		Name:        "test-skill",
		Description: "A test skill.",
		Targets:     []domain.Target{domain.TargetClaude, domain.TargetCodex},
		Path:        "skills/test-skill",
	}
}

func TestInstallNew(t *testing.T) {
	repo := setupTestRepo(t)
	targetRoot := filepath.Join(t.TempDir(), "skills")

	err := Install(repo, testSkill(), domain.TargetClaude, targetRoot, "abc123")
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	if !IsInstalled("test-skill", targetRoot) {
		t.Error("skill should be installed")
	}

	md, err := os.ReadFile(filepath.Join(targetRoot, "test-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	content := string(md)
	if !strings.HasPrefix(content, "---\n") {
		t.Error("SKILL.md should start with frontmatter delimiters")
	}
	if !strings.Contains(content, "name: test-skill") {
		t.Error("frontmatter should contain name")
	}
	if !strings.Contains(content, "description: A test skill.") {
		t.Error("frontmatter should contain description")
	}
	if !strings.Contains(content, "# Test") {
		t.Error("body content should be preserved")
	}
}

func TestInstallReplace(t *testing.T) {
	repo := setupTestRepo(t)
	targetRoot := filepath.Join(t.TempDir(), "skills")

	if err := Install(repo, testSkill(), domain.TargetClaude, targetRoot, "abc123"); err != nil {
		t.Fatalf("first Install() error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(repo, "skills", "test-skill", "SKILL.md"), []byte("# Updated"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	err := Install(repo, testSkill(), domain.TargetClaude, targetRoot, "abc123")
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	md, _ := os.ReadFile(filepath.Join(targetRoot, "test-skill", "SKILL.md"))
	if !strings.Contains(string(md), "# Updated") {
		t.Errorf("SKILL.md should contain updated body, got %q", md)
	}
}

func TestInstallUnsupportedTarget(t *testing.T) {
	repo := setupTestRepo(t)
	skill := domain.Skill{
		Name:    "test-skill",
		Targets: []domain.Target{domain.TargetClaude},
		Path:    "skills/test-skill",
	}

	err := Install(repo, skill, domain.TargetCodex, t.TempDir(), "")
	if err == nil {
		t.Fatal("expected error for unsupported target")
	}
	if !errors.Is(err, domain.ErrUnsupportedTarget) {
		t.Errorf("error should wrap ErrUnsupportedTarget, got: %v", err)
	}
}

func TestInstallClaudeFrontmatter(t *testing.T) {
	repo := setupTestRepo(t)
	targetRoot := filepath.Join(t.TempDir(), "skills")

	skill := testSkill()
	skill.Claude = map[string]any{
		"allowed-tools":            "Read, Grep",
		"disable-model-invocation": true,
	}

	err := Install(repo, skill, domain.TargetClaude, targetRoot, "abc123")
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	md, _ := os.ReadFile(filepath.Join(targetRoot, "test-skill", "SKILL.md"))
	content := string(md)

	if !strings.Contains(content, "allowed-tools: Read, Grep") {
		t.Error("should include allowed-tools from claude config")
	}
	if !strings.Contains(content, "disable-model-invocation: true") {
		t.Error("should include disable-model-invocation from claude config")
	}
	if !strings.Contains(content, "name: test-skill") {
		t.Error("should include name")
	}
}

func TestInstallCodexFrontmatter(t *testing.T) {
	repo := setupTestRepo(t)
	targetRoot := filepath.Join(t.TempDir(), "skills")

	skill := testSkill()
	skill.Claude = map[string]any{
		"allowed-tools": "Read, Grep",
	}
	skill.Codex = map[string]any{}

	err := Install(repo, skill, domain.TargetCodex, targetRoot, "abc123")
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	md, _ := os.ReadFile(filepath.Join(targetRoot, "test-skill", "SKILL.md"))
	content := string(md)

	if !strings.Contains(content, "name: test-skill") {
		t.Error("should include name")
	}
	if !strings.Contains(content, "description: A test skill.") {
		t.Error("should include description")
	}
	// Claude-specific fields should NOT appear in codex install
	if strings.Contains(content, "allowed-tools") {
		t.Error("codex install should not include claude-specific fields")
	}
}

func TestInstallEmptyTargetMap(t *testing.T) {
	repo := setupTestRepo(t)
	targetRoot := filepath.Join(t.TempDir(), "skills")

	skill := testSkill()
	// No Claude or Codex maps set

	err := Install(repo, skill, domain.TargetClaude, targetRoot, "abc123")
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	md, _ := os.ReadFile(filepath.Join(targetRoot, "test-skill", "SKILL.md"))
	content := string(md)

	// Should have name + description only in frontmatter
	lines := strings.Split(content, "\n")
	if lines[0] != "---" {
		t.Fatal("should start with ---")
	}

	fmEnd := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			fmEnd = i
			break
		}
	}
	if fmEnd == -1 {
		t.Fatal("no closing --- found")
	}
	// Frontmatter should be exactly: name + description (2 lines)
	fmLines := lines[1:fmEnd]
	if len(fmLines) != 2 {
		t.Errorf("expected 2 frontmatter lines, got %d: %v", len(fmLines), fmLines)
	}
}

func TestInstallPreservesBody(t *testing.T) {
	repo := setupTestRepo(t)
	targetRoot := filepath.Join(t.TempDir(), "skills")

	err := Install(repo, testSkill(), domain.TargetClaude, targetRoot, "abc123")
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	md, _ := os.ReadFile(filepath.Join(targetRoot, "test-skill", "SKILL.md"))
	content := string(md)

	// Body should appear after frontmatter
	parts := strings.SplitN(content, "---\n", 3)
	if len(parts) < 3 {
		t.Fatal("expected frontmatter delimiters")
	}
	body := parts[2]
	if !strings.Contains(body, "# Test\n\nSome content.") {
		t.Errorf("body not preserved, got: %q", body)
	}
}

func TestRemoveExisting(t *testing.T) {
	repo := setupTestRepo(t)
	targetRoot := filepath.Join(t.TempDir(), "skills")
	if err := Install(repo, testSkill(), domain.TargetClaude, targetRoot, "abc123"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	err := Remove("test-skill", targetRoot)
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if IsInstalled("test-skill", targetRoot) {
		t.Error("skill should not be installed after remove")
	}
}

func TestRemoveAbsent(t *testing.T) {
	err := Remove("nonexistent", t.TempDir())
	if err != nil {
		t.Fatalf("Remove() error = %v, want nil for absent skill", err)
	}
}

func TestIsInstalled(t *testing.T) {
	targetRoot := t.TempDir()
	if IsInstalled("nope", targetRoot) {
		t.Error("expected false for nonexistent")
	}

	if err := os.MkdirAll(filepath.Join(targetRoot, "exists"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if !IsInstalled("exists", targetRoot) {
		t.Error("expected true for existing dir")
	}
}

func TestInstall_WritesMarker(t *testing.T) {
	repo := setupTestRepo(t)
	targetRoot := filepath.Join(t.TempDir(), "skills")

	err := Install(repo, testSkill(), domain.TargetClaude, targetRoot, "def456")
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	if !HasMarker("test-skill", targetRoot) {
		t.Fatal("expected marker to exist after install")
	}

	marker, err := ReadMarker("test-skill", targetRoot)
	if err != nil {
		t.Fatalf("ReadMarker() error = %v", err)
	}
	if marker.RepoCommit != "def456" {
		t.Errorf("RepoCommit = %q, want %q", marker.RepoCommit, "def456")
	}
	if marker.InstalledAt.IsZero() {
		t.Error("InstalledAt should not be zero")
	}
}

func TestHasMarker(t *testing.T) {
	targetRoot := t.TempDir()

	// No dir at all
	if HasMarker("nope", targetRoot) {
		t.Error("expected false for nonexistent")
	}

	// Dir exists but no marker
	if err := os.MkdirAll(filepath.Join(targetRoot, "no-marker"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if HasMarker("no-marker", targetRoot) {
		t.Error("expected false for dir without marker")
	}

	// Dir with marker
	markerDir := filepath.Join(targetRoot, "has-marker")
	if err := os.MkdirAll(markerDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(markerDir, ".loadout"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if !HasMarker("has-marker", targetRoot) {
		t.Error("expected true for dir with marker")
	}
}

func TestReadMarker_NotFound(t *testing.T) {
	_, err := ReadMarker("nope", t.TempDir())
	if err == nil {
		t.Error("expected error for missing marker")
	}
}

func TestScanManaged(t *testing.T) {
	targetRoot := t.TempDir()

	// Create managed skill (with marker)
	managed := filepath.Join(targetRoot, "managed-skill")
	if err := os.MkdirAll(managed, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(managed, ".loadout"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Create unmanaged skill (no marker)
	unmanaged := filepath.Join(targetRoot, "unmanaged-skill")
	if err := os.MkdirAll(unmanaged, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Create a dot-prefixed dir (should be skipped)
	if err := os.MkdirAll(filepath.Join(targetRoot, ".hidden"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetRoot, ".hidden", ".loadout"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	ids := ScanManaged(targetRoot)
	if len(ids) != 1 {
		t.Fatalf("ScanManaged() = %v, want 1 entry", ids)
	}
	if ids[0] != "managed-skill" {
		t.Errorf("ScanManaged()[0] = %q, want %q", ids[0], "managed-skill")
	}
}

func TestInstall_PreservesExtraFiles(t *testing.T) {
	repo := setupTestRepo(t)
	// Add extra file to the repo skill
	refDir := filepath.Join(repo, "skills", "test-skill", "references")
	if err := os.MkdirAll(refDir, 0o755); err != nil {
		t.Fatalf("mkdir references: %v", err)
	}
	if err := os.WriteFile(filepath.Join(refDir, "notes.md"), []byte("# Notes\n"), 0o644); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}

	targetRoot := filepath.Join(t.TempDir(), "skills")
	err := Install(repo, testSkill(), domain.TargetClaude, targetRoot, "abc123")
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	notesPath := filepath.Join(targetRoot, "test-skill", "references", "notes.md")
	if _, err := os.Stat(notesPath); err != nil {
		t.Fatalf("extra file not preserved after install: %v", err)
	}
}

func TestScanManaged_EmptyDir(t *testing.T) {
	ids := ScanManaged(t.TempDir())
	if len(ids) != 0 {
		t.Errorf("ScanManaged() = %v, want empty", ids)
	}
}

func TestScanManaged_NonexistentDir(t *testing.T) {
	ids := ScanManaged("/nonexistent/path")
	if ids != nil {
		t.Errorf("ScanManaged() = %v, want nil", ids)
	}
}
