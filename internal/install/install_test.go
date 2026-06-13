package install

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sethdeckard/loadout/internal/codex"
	"github.com/sethdeckard/loadout/internal/domain"
	"github.com/sethdeckard/loadout/internal/skillmd"

	yaml "go.yaml.in/yaml/v3"
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

func TestInstall_RejectsUnmanagedDir(t *testing.T) {
	repo := setupTestRepo(t)
	targetRoot := filepath.Join(t.TempDir(), "skills")

	// Create an existing directory WITHOUT a .loadout marker
	unmanaged := filepath.Join(targetRoot, "test-skill")
	if err := os.MkdirAll(unmanaged, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(unmanaged, "user-data.txt"), []byte("important"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	err := Install(repo, testSkill(), domain.TargetClaude, targetRoot, "abc123")
	if !errors.Is(err, domain.ErrUnmanagedDir) {
		t.Fatalf("Install() error = %v, want ErrUnmanagedDir", err)
	}

	// Verify the directory was NOT deleted
	data, err := os.ReadFile(filepath.Join(unmanaged, "user-data.txt"))
	if err != nil {
		t.Fatal("unmanaged directory contents should be preserved")
	}
	if string(data) != "important" {
		t.Errorf("file content = %q, want %q", data, "important")
	}
}

func TestRemove_RejectsUnmanagedDir(t *testing.T) {
	targetRoot := t.TempDir()

	// Create directory without marker
	unmanaged := filepath.Join(targetRoot, "test-skill")
	if err := os.MkdirAll(unmanaged, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(unmanaged, "important.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	err := Remove("test-skill", targetRoot)
	if !errors.Is(err, domain.ErrUnmanagedDir) {
		t.Fatalf("Remove() error = %v, want ErrUnmanagedDir", err)
	}

	// Directory must still exist
	if _, err := os.Stat(filepath.Join(unmanaged, "important.txt")); err != nil {
		t.Error("unmanaged directory contents should be preserved")
	}
}

func TestInstall_MarkerIsAtomicWithRename(t *testing.T) {
	repo := setupTestRepo(t)
	targetRoot := filepath.Join(t.TempDir(), "skills")

	// First install — marker should arrive atomically with content
	if err := Install(repo, testSkill(), domain.TargetClaude, targetRoot, "abc123"); err != nil {
		t.Fatalf("first Install() error = %v", err)
	}
	if !HasMarker("test-skill", targetRoot) {
		t.Fatal("marker must exist after install")
	}

	// Simulate the old bug: delete marker as if the post-rename write had failed
	markerPath := filepath.Join(targetRoot, "test-skill", ".loadout")
	if err := os.Remove(markerPath); err != nil {
		t.Fatalf("remove marker: %v", err)
	}

	// Reinstall must now fail — the directory exists without a marker
	err := Install(repo, testSkill(), domain.TargetClaude, targetRoot, "def456")
	if !errors.Is(err, domain.ErrUnmanagedDir) {
		t.Fatalf("Install() over markerless dir: error = %v, want ErrUnmanagedDir", err)
	}

	// Content from the first install must still be intact
	if _, err := os.Stat(filepath.Join(targetRoot, "test-skill", "SKILL.md")); err != nil {
		t.Error("existing content should be preserved when marker is missing")
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

func TestInstall_StripsExistingFrontmatter(t *testing.T) {
	repo := setupTestRepo(t)
	// Write SKILL.md with its own frontmatter block
	mdPath := filepath.Join(repo, "skills", "test-skill", "SKILL.md")
	src := "---\nname: test-skill\ndescription: \"original\"\n---\n\n# Test\n\nBody content.\n"
	if err := os.WriteFile(mdPath, []byte(src), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	targetRoot := filepath.Join(t.TempDir(), "skills")
	if err := Install(repo, testSkill(), domain.TargetClaude, targetRoot, "abc123"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	md, _ := os.ReadFile(filepath.Join(targetRoot, "test-skill", "SKILL.md"))
	content := string(md)

	// Count lines that are exactly "---" — should be exactly 2 (one open, one close)
	var delimCount int
	for _, line := range strings.Split(content, "\n") {
		if line == "---" {
			delimCount++
		}
	}
	if delimCount != 2 {
		t.Errorf("expected exactly 2 frontmatter delimiters, got %d in:\n%s", delimCount, content)
	}

	if !strings.Contains(content, "# Test") {
		t.Error("body should be preserved")
	}
	if !strings.Contains(content, "Body content.") {
		t.Error("body content should be preserved")
	}
}

func TestStage_NoMarker(t *testing.T) {
	repo := setupTestRepo(t)
	dest := filepath.Join(t.TempDir(), "staged")

	if err := Stage(repo, testSkill(), domain.TargetClaude, dest); err != nil {
		t.Fatalf("Stage() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(dest, ".loadout")); !os.IsNotExist(err) {
		t.Errorf("Stage() output should not contain .loadout marker, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "SKILL.md")); err != nil {
		t.Errorf("Stage() should produce SKILL.md, got err=%v", err)
	}
}

func TestStage_RewritesFrontmatter(t *testing.T) {
	repo := setupTestRepo(t)
	dest := filepath.Join(t.TempDir(), "staged")

	skill := testSkill()
	skill.Claude = map[string]any{"allowed-tools": "Read"}

	if err := Stage(repo, skill, domain.TargetClaude, dest); err != nil {
		t.Fatalf("Stage() error = %v", err)
	}

	md, err := os.ReadFile(filepath.Join(dest, "SKILL.md"))
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	content := string(md)
	if !strings.Contains(content, "name: test-skill") {
		t.Error("Stage() output should include frontmatter")
	}
	if !strings.Contains(content, "allowed-tools: Read") {
		t.Error("Stage() output should include claude metadata")
	}
}

func TestStage_UnsupportedTarget(t *testing.T) {
	repo := setupTestRepo(t)
	skill := domain.Skill{
		Name:    "test-skill",
		Targets: []domain.Target{domain.TargetClaude},
		Path:    "skills/test-skill",
	}

	err := Stage(repo, skill, domain.TargetCodex, filepath.Join(t.TempDir(), "staged"))
	if !errors.Is(err, domain.ErrUnsupportedTarget) {
		t.Errorf("Stage() error = %v, want ErrUnsupportedTarget", err)
	}
}

func TestInstall_DescriptionWithColon(t *testing.T) {
	repo := setupTestRepo(t)
	targetRoot := filepath.Join(t.TempDir(), "skills")

	skill := testSkill()
	skill.Description = "Does stuff. Conversational: analyzes things"

	if err := Install(repo, skill, domain.TargetClaude, targetRoot, "abc123"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	md, _ := os.ReadFile(filepath.Join(targetRoot, "test-skill", "SKILL.md"))
	content := string(md)

	// The colon-space forces quoting; yaml.v3 emits a single-quoted scalar. The
	// key assertion is that the value round-trips losslessly through YAML.
	want := `description: 'Does stuff. Conversational: analyzes things'`
	if !strings.Contains(content, want) {
		t.Errorf("expected line %q in:\n%s", want, content)
	}
	parsed := skillmd.Parse(content)
	if got := parsed.Fields["description"]; got != skill.Description {
		t.Errorf("description round-trip = %q, want %q", got, skill.Description)
	}
}

// readInstalledPolicy reads agents/openai.yaml from the installed test-skill and
// returns its allow_implicit_invocation value (present=false if no file).
func readInstalledPolicy(t *testing.T, targetRoot string) (val, present bool) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(targetRoot, "test-skill", filepath.FromSlash(codex.OpenAIYAMLPath)))
	if err != nil {
		if os.IsNotExist(err) {
			return false, false
		}
		t.Fatalf("read openai.yaml: %v", err)
	}
	return codex.InferAllowImplicit(data)
}

func TestInstall_CodexNativePolicy(t *testing.T) {
	repo := setupTestRepo(t)
	targetRoot := filepath.Join(t.TempDir(), "skills")

	skill := testSkill()
	skill.Codex = map[string]any{"policy": map[string]any{"allow_implicit_invocation": false}}

	if err := Install(repo, skill, domain.TargetCodex, targetRoot, "abc123"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	val, present := readInstalledPolicy(t, targetRoot)
	if !present || val != false {
		t.Errorf("policy = (%v, %v), want (false, true)", val, present)
	}
}

func TestInstall_CodexAliasPolicy(t *testing.T) {
	repo := setupTestRepo(t)
	targetRoot := filepath.Join(t.TempDir(), "skills")

	skill := testSkill()
	skill.Codex = map[string]any{"disable-model-invocation": true}

	if err := Install(repo, skill, domain.TargetCodex, targetRoot, "abc123"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	val, present := readInstalledPolicy(t, targetRoot)
	if !present || val != false {
		t.Errorf("alias disable-model-invocation:true should yield allow_implicit_invocation:false; got (%v, %v)", val, present)
	}
}

func TestInstall_CodexNativeWinsOverAlias(t *testing.T) {
	repo := setupTestRepo(t)
	targetRoot := filepath.Join(t.TempDir(), "skills")

	skill := testSkill()
	skill.Codex = map[string]any{
		"policy":                   map[string]any{"allow_implicit_invocation": true},
		"disable-model-invocation": true, // would imply false if it won
	}

	if err := Install(repo, skill, domain.TargetCodex, targetRoot, "abc123"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	val, present := readInstalledPolicy(t, targetRoot)
	if !present || val != true {
		t.Errorf("native policy should win; got (%v, %v), want (true, true)", val, present)
	}
}

func TestInstall_CodexPreservesExistingOpenAIYAML(t *testing.T) {
	repo := setupTestRepo(t)
	targetRoot := filepath.Join(t.TempDir(), "skills")

	// Author-written openai.yaml in the repo source with fields Loadout must keep.
	agentsDir := filepath.Join(repo, "skills", "test-skill", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("mkdir agents: %v", err)
	}
	existing := "interface:\n  command: do-thing\ndependencies:\n  - foo\npolicy:\n  allow_implicit_invocation: true\n  some_other_flag: keep-me\n"
	if err := os.WriteFile(filepath.Join(agentsDir, "openai.yaml"), []byte(existing), 0o644); err != nil {
		t.Fatalf("write openai.yaml: %v", err)
	}

	skill := testSkill()
	skill.Codex = map[string]any{"policy": map[string]any{"allow_implicit_invocation": false}}

	if err := Install(repo, skill, domain.TargetCodex, targetRoot, "abc123"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(targetRoot, "test-skill", "agents", "openai.yaml"))
	if err != nil {
		t.Fatalf("read installed openai.yaml: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if iface, ok := doc["interface"].(map[string]any); !ok || iface["command"] != "do-thing" {
		t.Errorf("interface not preserved: %v", doc["interface"])
	}
	if _, ok := doc["dependencies"].([]any); !ok {
		t.Errorf("dependencies not preserved: %v", doc["dependencies"])
	}
	policy, _ := doc["policy"].(map[string]any)
	if policy["some_other_flag"] != "keep-me" {
		t.Errorf("unrelated policy field not preserved: %v", policy)
	}
	if policy["allow_implicit_invocation"] != false {
		t.Errorf("allow_implicit_invocation = %v, want false", policy["allow_implicit_invocation"])
	}
}

func TestInstall_CodexNoPolicyNoFile(t *testing.T) {
	repo := setupTestRepo(t)
	targetRoot := filepath.Join(t.TempDir(), "skills")

	skill := testSkill()
	skill.Codex = map[string]any{} // no policy declared

	if err := Install(repo, skill, domain.TargetCodex, targetRoot, "abc123"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	if _, present := readInstalledPolicy(t, targetRoot); present {
		t.Error("no policy declared should not synthesize agents/openai.yaml")
	}
}

func TestInstall_ClaudePolicyUnchanged(t *testing.T) {
	repo := setupTestRepo(t)
	targetRoot := filepath.Join(t.TempDir(), "skills")

	skill := testSkill()
	skill.Claude = map[string]any{"disable-model-invocation": true}

	if err := Install(repo, skill, domain.TargetClaude, targetRoot, "abc123"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	md, _ := os.ReadFile(filepath.Join(targetRoot, "test-skill", "SKILL.md"))
	if !strings.Contains(string(md), "disable-model-invocation: true") {
		t.Errorf("claude frontmatter should retain disable-model-invocation; got:\n%s", md)
	}
	if _, present := readInstalledPolicy(t, targetRoot); present {
		t.Error("claude install should not write agents/openai.yaml")
	}
}
