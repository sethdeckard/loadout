package scope

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sethdeckard/loadout/internal/config"
	"github.com/sethdeckard/loadout/internal/domain"
)

func TestResolve_Empty(t *testing.T) {
	sc, err := Resolve("")
	if err != nil {
		t.Fatalf("Resolve('') error = %v", err)
	}
	if !sc.IsUser() {
		t.Error("expected user scope")
	}
	if sc.IsProject() {
		t.Error("expected not project scope")
	}
	if sc.Project != "" {
		t.Errorf("Project = %q, want empty", sc.Project)
	}
}

func TestResolve_Dot_InProject(t *testing.T) {
	dir := resolveSymlinks(t, t.TempDir())
	gitDir := filepath.Join(dir, ".git")
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("setup .git: %v", err)
	}
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("setup .claude: %v", err)
	}

	// Change to a subdirectory
	sub := filepath.Join(dir, "sub", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("setup sub: %v", err)
	}

	chdir(t, sub)

	sc, err := Resolve(".")
	if err != nil {
		t.Fatalf("Resolve('.') error = %v", err)
	}
	if !sc.IsProject() {
		t.Error("expected project scope")
	}
	if sc.Project != dir {
		t.Errorf("Project = %q, want %q", sc.Project, dir)
	}
}

func resolveSymlinks(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", path, err)
	}
	return resolved
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(origWd); err != nil {
			t.Logf("restore wd: %v", err)
		}
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%q): %v", dir, err)
	}
}

func TestResolve_Dot_NotGitRepo(t *testing.T) {
	dir := resolveSymlinks(t, t.TempDir())
	// No .git directory
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("setup .claude: %v", err)
	}

	chdir(t, dir)

	_, err := Resolve(".")
	if err == nil {
		t.Fatal("expected error for non-git repo, got nil")
	}
}

func TestResolve_Dot_NoAgentDirs(t *testing.T) {
	dir := resolveSymlinks(t, t.TempDir())
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("setup .git: %v", err)
	}
	// No .claude/ or .codex/ directory

	chdir(t, dir)

	_, err := Resolve(".")
	if err == nil {
		t.Fatal("expected error for git repo without .claude/ or .codex/, got nil")
	}
}

func TestResolve_ExplicitPath(t *testing.T) {
	dir := resolveSymlinks(t, t.TempDir())
	gitDir := filepath.Join(dir, ".git")
	codexDir := filepath.Join(dir, ".codex")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("setup .git: %v", err)
	}
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatalf("setup .codex: %v", err)
	}

	sc, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve(%q) error = %v", dir, err)
	}
	if !sc.IsProject() {
		t.Error("expected project scope")
	}
	if sc.Project != dir {
		t.Errorf("Project = %q, want %q", sc.Project, dir)
	}
}

func TestResolve_ExplicitPath_NoSymlinks(t *testing.T) {
	// A real directory that is not a symlink should resolve without error.
	dir := resolveSymlinks(t, t.TempDir())
	gitDir := filepath.Join(dir, ".git")
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("setup .git: %v", err)
	}
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("setup .claude: %v", err)
	}

	sc, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve(%q) error = %v", dir, err)
	}
	if sc.Project != dir {
		t.Errorf("Project = %q, want %q", sc.Project, dir)
	}
}

func TestResolve_ExplicitPath_NonexistentFallsBackToAbs(t *testing.T) {
	// When the explicit path does not exist, EvalSymlinks returns
	// os.ErrNotExist and Resolve should fall back to the absolute path
	// rather than returning an error.
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	_, err := Resolve(missing)
	// The path doesn't contain .git so DetectProjectRoot will fail,
	// but the important thing is we get past the EvalSymlinks step.
	if err == nil {
		t.Fatal("expected error from DetectProjectRoot, got nil")
	}
	// Should NOT be a "resolve symlinks" error.
	if strings.Contains(err.Error(), "resolve symlinks") {
		t.Errorf("error = %q, should not fail at symlink resolution for missing path", err)
	}
}

func TestResolve_ExplicitPath_Invalid(t *testing.T) {
	dir := t.TempDir()
	// No .git
	_, err := Resolve(dir)
	if err == nil {
		t.Fatal("expected error for non-git directory")
	}
}

func TestTargetRoot_User(t *testing.T) {
	claudePath := filepath.Join(os.TempDir(), "user", ".claude", "skills")
	codexPath := filepath.Join(os.TempDir(), "user", ".codex", "skills")
	sc := Scope{Project: ""}
	paths := config.TargetPaths{
		Claude: config.TargetConfig{Enabled: true, Path: claudePath},
		Codex:  config.TargetConfig{Enabled: true, Path: codexPath},
	}

	got := sc.TargetRoot(domain.TargetClaude, paths)
	if got != claudePath {
		t.Errorf("TargetRoot(claude) = %q, want user path", got)
	}
	got = sc.TargetRoot(domain.TargetCodex, paths)
	if got != codexPath {
		t.Errorf("TargetRoot(codex) = %q, want user path", got)
	}
}

func TestTargetRoot_Project(t *testing.T) {
	projectDir := filepath.Join(os.TempDir(), "projects", "my-app")
	sc := Scope{Project: projectDir}
	paths := config.TargetPaths{
		Claude: config.TargetConfig{Enabled: true, Path: filepath.Join(os.TempDir(), "user", ".claude", "skills")},
		Codex:  config.TargetConfig{Enabled: true, Path: filepath.Join(os.TempDir(), "user", ".codex", "skills")},
	}

	got := sc.TargetRoot(domain.TargetClaude, paths)
	want := filepath.Join(projectDir, ".claude", "skills")
	if got != want {
		t.Errorf("TargetRoot(claude) = %q, want %q", got, want)
	}

	got = sc.TargetRoot(domain.TargetCodex, paths)
	want = filepath.Join(projectDir, ".codex", "skills")
	if got != want {
		t.Errorf("TargetRoot(codex) = %q, want %q", got, want)
	}
}

func TestDetectProjectRoot(t *testing.T) {
	dir := resolveSymlinks(t, t.TempDir())
	gitDir := filepath.Join(dir, ".git")
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("setup .git: %v", err)
	}
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("setup .claude: %v", err)
	}

	// Start from a nested subdirectory
	deep := filepath.Join(dir, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("setup deep: %v", err)
	}

	root, err := DetectProjectRoot(deep)
	if err != nil {
		t.Fatalf("DetectProjectRoot(%q) error = %v", deep, err)
	}
	if root != dir {
		t.Errorf("root = %q, want %q", root, dir)
	}
}

func TestDetectProjectRoot_WorktreeGitFile(t *testing.T) {
	dir := resolveSymlinks(t, t.TempDir())
	// Simulate a git worktree where .git is a file
	gitFile := filepath.Join(dir, ".git")
	if err := os.WriteFile(gitFile, []byte("gitdir: /some/main/repo/.git/worktrees/wt"), 0o644); err != nil {
		t.Fatalf("write .git file: %v", err)
	}
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("setup .claude: %v", err)
	}

	root, err := DetectProjectRoot(dir)
	if err != nil {
		t.Fatalf("DetectProjectRoot() error = %v", err)
	}
	if root != dir {
		t.Errorf("root = %q, want %q", root, dir)
	}
}

func TestDetectProjectRoot_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := DetectProjectRoot(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDetectProjectRoot_GitButNoAgentDirs(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("setup .git: %v", err)
	}

	_, err := DetectProjectRoot(dir)
	if err == nil {
		t.Fatal("expected error for git repo without agent dirs, got nil")
	}
}
