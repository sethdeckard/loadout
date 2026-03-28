package cmd

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sethdeckard/loadout/internal/config"
	"github.com/sethdeckard/loadout/internal/fsx"
	"github.com/sethdeckard/loadout/internal/gitrepo"
)

func TestSetupRepo_NewPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new-repo")
	var out bytes.Buffer

	if err := setupRepo(&out, dir, false); err != nil {
		t.Fatalf("setupRepo() error = %v", err)
	}

	if !gitrepo.IsRepo(dir) {
		t.Error("should be a git repo")
	}
	if !fsx.Exists(filepath.Join(dir, "skills", ".gitkeep")) {
		t.Error("should have skills/.gitkeep")
	}
	if fsx.DirExists(filepath.Join(dir, "skills", "example-skill")) {
		t.Error("should not scaffold example-skill")
	}
	if _, err := os.Stat(filepath.Join(dir, "README.md")); err != nil {
		t.Error("README.md should exist")
	}
	head, err := gitrepo.HeadCommit(dir)
	if err != nil {
		t.Fatalf("HeadCommit() error = %v", err)
	}
	if head == "" {
		t.Error("expected initial commit")
	}
	dirty, err := gitrepo.IsDirty(dir)
	if err != nil {
		t.Fatalf("IsDirty() error = %v", err)
	}
	if dirty {
		t.Error("repo should be clean after setupRepo")
	}
}

func TestSetupRepo_NewPath_NoGitIdentityRequired(t *testing.T) {
	t.Setenv("GIT_AUTHOR_NAME", "")
	t.Setenv("GIT_AUTHOR_EMAIL", "")
	t.Setenv("GIT_COMMITTER_NAME", "")
	t.Setenv("GIT_COMMITTER_EMAIL", "")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "empty"))
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(t.TempDir(), "empty"))

	dir := filepath.Join(t.TempDir(), "new-repo")
	var out bytes.Buffer

	if err := setupRepo(&out, dir, false); err != nil {
		t.Fatalf("setupRepo() should not require git identity, error = %v", err)
	}

	head, err := gitrepo.HeadCommit(dir)
	if err != nil {
		t.Fatalf("HeadCommit() error = %v", err)
	}
	if head == "" {
		t.Error("expected initial commit even without git identity")
	}
}

func TestSetupRepo_GitRepoWithSkills(t *testing.T) {
	dir := t.TempDir()
	if err := gitrepo.Init(dir); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "skills", "my-skill"), 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skills", "my-skill", "SKILL.md"), []byte("# Mine"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	var out bytes.Buffer
	if err := setupRepo(&out, dir, false); err != nil {
		t.Fatalf("setupRepo() error = %v", err)
	}

	if fsx.DirExists(filepath.Join(dir, "skills", "example-skill")) {
		t.Error("should not scaffold when skills/ already exists")
	}
	if got := out.String(); got == "" {
		t.Error("should print message about using existing repo")
	}
}

func TestSetupRepo_GitRepoWithoutSkills(t *testing.T) {
	dir := t.TempDir()
	if err := gitrepo.Init(dir); err != nil {
		t.Fatalf("setup: %v", err)
	}

	var out bytes.Buffer
	if err := setupRepo(&out, dir, false); err != nil {
		t.Fatalf("setupRepo() error = %v", err)
	}

	if !fsx.Exists(filepath.Join(dir, "skills", ".gitkeep")) {
		t.Error("should have skills/.gitkeep")
	}
	head, err := gitrepo.HeadCommit(dir)
	if err != nil {
		t.Fatalf("HeadCommit() error = %v", err)
	}
	if head == "" {
		t.Error("expected initial commit")
	}
}

func TestSetupRepo_EmptyClonedRepo(t *testing.T) {
	bare := filepath.Join(t.TempDir(), "bare.git")
	run(t, ".", "git", "init", "--bare", bare)

	clone := filepath.Join(t.TempDir(), "clone")
	run(t, ".", "git", "clone", bare, clone)

	var out bytes.Buffer
	if err := setupRepo(&out, clone, true); err != nil {
		t.Fatalf("setupRepo() error = %v", err)
	}

	if !fsx.Exists(filepath.Join(clone, "skills", ".gitkeep")) {
		t.Error("should have skills/.gitkeep")
	}
	// HEAD should NOT exist — clones keep unborn HEAD
	if _, err := gitrepo.HeadCommit(clone); err == nil {
		t.Error("expected unborn HEAD for cloned empty repo")
	}
}

func TestSetupRepo_NonGitDir(t *testing.T) {
	dir := t.TempDir()

	var out bytes.Buffer
	err := setupRepo(&out, dir, false)
	if err == nil {
		t.Fatal("expected error for non-git directory")
	}
}

func TestScaffoldEmptySkillsDir(t *testing.T) {
	dir := t.TempDir()
	if err := scaffoldEmptySkillsDir(dir); err != nil {
		t.Fatalf("scaffoldEmptySkillsDir() error = %v", err)
	}

	if !fsx.DirExists(filepath.Join(dir, "skills")) {
		t.Error("skills/ should exist")
	}
	if !fsx.Exists(filepath.Join(dir, "skills", ".gitkeep")) {
		t.Error("skills/.gitkeep should exist")
	}
}

func TestWriteRepoReadme_Idempotent(t *testing.T) {
	dir := t.TempDir()
	existing := "# My Existing README\n"
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(existing), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := writeRepoReadme(dir); err != nil {
		t.Fatalf("writeRepoReadme() error = %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != existing {
		t.Error("writeRepoReadme should not overwrite existing README.md")
	}
}

func TestPromptChoice_Default(t *testing.T) {
	var out bytes.Buffer
	scanner := newScanner("")
	got := promptChoice(scanner, &out, "Pick", []string{"a", "b", "c"}, 1)
	if got != 1 {
		t.Errorf("promptChoice() = %d, want 1 (default)", got)
	}
}

func TestPromptChoice_Selection(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"1\n", 0},
		{"2\n", 1},
		{"3\n", 2},
	}
	for _, tt := range tests {
		var out bytes.Buffer
		scanner := newScanner(tt.input)
		got := promptChoice(scanner, &out, "Pick", []string{"a", "b", "c"}, 0)
		if got != tt.want {
			t.Errorf("promptChoice(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestPromptChoice_InvalidFallsBackToDefault(t *testing.T) {
	tests := []string{"0\n", "4\n", "abc\n"}
	for _, input := range tests {
		var out bytes.Buffer
		scanner := newScanner(input)
		got := promptChoice(scanner, &out, "Pick", []string{"a", "b", "c"}, 2)
		if got != 2 {
			t.Errorf("promptChoice(%q) = %d, want 2 (default)", input, got)
		}
	}
}

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	tests := []struct {
		in   string
		want string
	}{
		{"~", home},
		{"~/foo", filepath.Join(home, "foo")},
		{"~" + string(filepath.Separator) + "bar", filepath.Join(home, "bar")},
		{"/abs/path", "/abs/path"},
		{"relative", "relative"},
		{"~user", "~user"},
	}
	for _, tt := range tests {
		got := expandHome(tt.in)
		if got != tt.want {
			t.Errorf("expandHome(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func nonEmptyDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "placeholder"), []byte("x"), 0o644); err != nil {
		t.Fatalf("setup non-empty dir: %v", err)
	}
	return dir
}

func TestPromptCloneDestination_RepromptsOnExistingPath(t *testing.T) {
	existingDir := nonEmptyDir(t)
	newDir := filepath.Join(t.TempDir(), "new-clone")

	// First prompt returns existing path (default), second provides a new path
	input := "\n" + newDir + "\n"
	scanner := newScanner(input)
	var out bytes.Buffer

	got := promptCloneDestination(scanner, &out, existingDir)
	if got != newDir {
		t.Fatalf("promptCloneDestination() = %q, want %q", got, newDir)
	}
	if !strings.Contains(out.String(), "already exists and is not empty") {
		t.Errorf("output = %q, want rejection message", out.String())
	}
}

func TestPromptCloneDestination_AcceptsNewPathImmediately(t *testing.T) {
	newDir := filepath.Join(t.TempDir(), "fresh-clone")

	scanner := newScanner("\n")
	var out bytes.Buffer

	got := promptCloneDestination(scanner, &out, newDir)
	if got != newDir {
		t.Fatalf("promptCloneDestination() = %q, want %q", got, newDir)
	}
	if strings.Contains(out.String(), "already exists") {
		t.Error("should not print rejection for non-existing path")
	}
}

func TestPromptCloneDestination_AcceptsEmptyDir(t *testing.T) {
	emptyDir := t.TempDir()

	scanner := newScanner("\n")
	var out bytes.Buffer

	got := promptCloneDestination(scanner, &out, emptyDir)
	if got != emptyDir {
		t.Fatalf("promptCloneDestination() = %q, want %q", got, emptyDir)
	}
	if strings.Contains(out.String(), "already exists") {
		t.Error("should not reject an empty directory")
	}
}

func TestPromptCloneDestination_EOFBreaksLoop(t *testing.T) {
	existingDir := nonEmptyDir(t)

	// Empty input = EOF after first prompt
	scanner := newScanner("")
	var out bytes.Buffer

	got := promptCloneDestination(scanner, &out, existingDir)
	// Should return the existing path (loop breaks on EOF), caller handles the error
	if got != existingDir {
		t.Fatalf("promptCloneDestination() = %q, want %q (EOF returns default)", got, existingDir)
	}
}

func TestRunInit_CloneFlagExistingDestination(t *testing.T) {
	existingDir := nonEmptyDir(t)

	// Set flag state
	initCloneURL = "git@example.com:repo.git"
	initRepoPath = existingDir
	initTargets = "claude"
	defer func() {
		initCloneURL = ""
		initRepoPath = ""
		initTargets = ""
	}()

	var out bytes.Buffer
	err := runInitWith(&out, strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error for existing clone destination")
	}
	if !strings.Contains(err.Error(), "clone destination already exists") {
		t.Fatalf("error = %q, want clone destination already exists message", err.Error())
	}
}

func TestRunInit_NonInteractiveWithAllFlags(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	repoDir := filepath.Join(tmp, "repo")
	claudeDir := filepath.Join(tmp, "claude-skills")
	codexDir := filepath.Join(tmp, "codex-skills")

	initRepoPath = repoDir
	initTargets = "claude,codex"
	initClaudeSkills = claudeDir
	initCodexSkills = codexDir
	defer func() {
		initRepoPath = ""
		initTargets = ""
		initClaudeSkills = ""
		initCodexSkills = ""
	}()

	var out bytes.Buffer
	if err := runInitWith(&out, strings.NewReader("")); err != nil {
		t.Fatalf("runInitWith() error = %v", err)
	}

	cfgPath := filepath.Join(tmp, ".config", "loadout", "config.json")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if cfg.Targets.Claude.Path != claudeDir {
		t.Errorf("Claude path = %q, want %q", cfg.Targets.Claude.Path, claudeDir)
	}
	if cfg.Targets.Codex.Path != codexDir {
		t.Errorf("Codex path = %q, want %q", cfg.Targets.Codex.Path, codexDir)
	}
	if !cfg.Targets.Claude.Enabled {
		t.Error("Claude should be enabled")
	}
	if !cfg.Targets.Codex.Enabled {
		t.Error("Codex should be enabled")
	}
}

func newScanner(input string) *bufio.Scanner {
	return bufio.NewScanner(strings.NewReader(input))
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s: %v", args, out, err)
	}
}
