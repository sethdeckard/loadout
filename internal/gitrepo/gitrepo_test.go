package gitrepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsRepo(t *testing.T) {
	// Current project is a git repo
	if !IsRepo("../..") {
		t.Error("expected project root to be a git repo")
	}
	if IsRepo(t.TempDir()) {
		t.Error("expected temp dir to not be a git repo")
	}
}

func TestIsRepo_WorktreeGitFile(t *testing.T) {
	dir := t.TempDir()
	// Simulate a git worktree where .git is a file, not a directory
	gitFile := filepath.Join(dir, ".git")
	if err := os.WriteFile(gitFile, []byte("gitdir: /some/main/repo/.git/worktrees/wt"), 0o644); err != nil {
		t.Fatalf("write .git file: %v", err)
	}
	if !IsRepo(dir) {
		t.Error("expected worktree with .git file to be detected as a repo")
	}
}

func TestHeadCommit(t *testing.T) {
	dir := initTestRepo(t)
	hash, err := HeadCommit(dir)
	if err != nil {
		t.Fatalf("HeadCommit() error = %v", err)
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestIsDirty(t *testing.T) {
	dir := initTestRepo(t)

	dirty, err := IsDirty(dir)
	if err != nil {
		t.Fatalf("IsDirty() error = %v", err)
	}
	if dirty {
		t.Error("expected clean repo")
	}

	// Make it dirty
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("change"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	dirty, err = IsDirty(dir)
	if err != nil {
		t.Fatalf("IsDirty() error = %v", err)
	}
	if !dirty {
		t.Error("expected dirty repo")
	}
}

func TestCheckSyncReadiness_ValidUpstream(t *testing.T) {
	dir := initTrackedRepo(t)

	readiness, err := CheckSyncReadiness(dir)
	if err != nil {
		t.Fatalf("CheckSyncReadiness() error = %v", err)
	}
	if readiness.Branch != "main" {
		t.Fatalf("Branch = %q, want main", readiness.Branch)
	}
	if readiness.Upstream != "origin/main" {
		t.Fatalf("Upstream = %q, want origin/main", readiness.Upstream)
	}
}

func TestCheckSyncReadiness_NoUpstream(t *testing.T) {
	dir := initTestRepo(t)

	readiness, err := CheckSyncReadiness(dir)
	if err == nil {
		t.Fatal("CheckSyncReadiness() expected error, got nil")
	}
	if readiness.Branch == "" {
		t.Fatal("expected branch name")
	}
	if !strings.Contains(err.Error(), "no upstream") {
		t.Fatalf("error = %q, want no upstream", err.Error())
	}
}

func TestCheckSyncReadiness_StaleUpstream(t *testing.T) {
	dir := initTrackedRepo(t)
	setBranchUpstream(t, dir, "main", "origin", "refs/heads/master")

	readiness, err := CheckSyncReadiness(dir)
	if err == nil {
		t.Fatal("CheckSyncReadiness() expected error, got nil")
	}
	if readiness.Branch != "main" {
		t.Fatalf("Branch = %q, want main", readiness.Branch)
	}
	if readiness.Upstream != "origin/master" {
		t.Fatalf("Upstream = %q, want origin/master", readiness.Upstream)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %q, want not found", err.Error())
	}
}

func TestCheckSyncReadiness_DetachedHEAD(t *testing.T) {
	dir := initTestRepo(t)
	run(t, dir, "git", "checkout", "--detach")

	_, err := CheckSyncReadiness(dir)
	if err == nil {
		t.Fatal("CheckSyncReadiness() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "detached HEAD") {
		t.Fatalf("error = %q, want detached HEAD", err.Error())
	}
}

func TestAssessSyncState_ValidUpstreamUpToDate(t *testing.T) {
	dir := initTrackedRepo(t)

	assessment, err := AssessSyncState(dir)
	if err != nil {
		t.Fatalf("AssessSyncState() error = %v", err)
	}
	if assessment.State != SyncStateUpToDate {
		t.Fatalf("State = %q, want %q", assessment.State, SyncStateUpToDate)
	}
}

func TestAssessSyncState_BootstrapEmptyRemote(t *testing.T) {
	remote := t.TempDir()
	run(t, remote, "git", "init", "--bare", "--initial-branch=master")

	dir := t.TempDir()
	run(t, dir, "git", "init", "--initial-branch=master")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	run(t, dir, "git", "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("test"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	run(t, dir, "git", "remote", "add", "origin", remote)
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "init")

	assessment, err := AssessSyncState(dir)
	if err != nil {
		t.Fatalf("AssessSyncState() error = %v", err)
	}
	if assessment.State != SyncStateBootstrapEmptyRemote {
		t.Fatalf("State = %q, want %q", assessment.State, SyncStateBootstrapEmptyRemote)
	}
}

func TestAssessSyncState_LocalAhead(t *testing.T) {
	dir := initTrackedRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "LOCAL.md"), []byte("local"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "local")

	assessment, err := AssessSyncState(dir)
	if err != nil {
		t.Fatalf("AssessSyncState() error = %v", err)
	}
	if assessment.State != SyncStateLocalAhead {
		t.Fatalf("State = %q, want %q", assessment.State, SyncStateLocalAhead)
	}
}

func TestAssessSyncState_RemoteAhead(t *testing.T) {
	dir := initTrackedRepo(t)
	remote := strings.TrimSpace(runOutput(t, dir, "git", "remote", "get-url", "origin"))

	clone := filepath.Join(t.TempDir(), "clone")
	run(t, ".", "git", "clone", remote, clone)
	run(t, clone, "git", "config", "user.email", "test@test.com")
	run(t, clone, "git", "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(clone, "REMOTE.md"), []byte("remote"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	run(t, clone, "git", "add", ".")
	run(t, clone, "git", "commit", "-m", "remote")
	run(t, clone, "git", "push", "origin", "main")
	if err := Fetch(dir); err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	assessment, err := AssessSyncState(dir)
	if err != nil {
		t.Fatalf("AssessSyncState() error = %v", err)
	}
	if assessment.State != SyncStateRemoteAhead {
		t.Fatalf("State = %q, want %q", assessment.State, SyncStateRemoteAhead)
	}
}

func TestAssessSyncState_Diverged(t *testing.T) {
	dir := initTrackedRepo(t)
	remote := strings.TrimSpace(runOutput(t, dir, "git", "remote", "get-url", "origin"))

	clone := filepath.Join(t.TempDir(), "clone")
	run(t, ".", "git", "clone", remote, clone)
	run(t, clone, "git", "config", "user.email", "test@test.com")
	run(t, clone, "git", "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(clone, "REMOTE.md"), []byte("remote"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	run(t, clone, "git", "add", ".")
	run(t, clone, "git", "commit", "-m", "remote")
	run(t, clone, "git", "push", "origin", "main")

	if err := os.WriteFile(filepath.Join(dir, "LOCAL.md"), []byte("local"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "local")
	if err := Fetch(dir); err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	assessment, err := AssessSyncState(dir)
	if err != nil {
		t.Fatalf("AssessSyncState() error = %v", err)
	}
	if assessment.State != SyncStateDiverged {
		t.Fatalf("State = %q, want %q", assessment.State, SyncStateDiverged)
	}
}

func TestAddPathsAndCommit(t *testing.T) {
	dir := initTestRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, "skills", "alpha"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skills", "alpha", "SKILL.md"), []byte("# Alpha"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatalf("write notes: %v", err)
	}

	if err := AddPathsAndCommit(dir, []string{"skills/alpha"}, "add alpha"); err != nil {
		t.Fatalf("AddPathsAndCommit() error = %v", err)
	}

	out := runOutput(t, dir, "git", "show", "--name-only", "--format=", "HEAD")
	if !strings.Contains(out, "skills/alpha/SKILL.md") {
		t.Fatalf("commit files = %q, want skills/alpha/SKILL.md", out)
	}
	if strings.Contains(out, "notes.txt") {
		t.Fatalf("commit files = %q, should not include notes.txt", out)
	}
}

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	run(t, dir, "git", "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("test"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "init")
	return dir
}

func initTrackedRepo(t *testing.T) string {
	t.Helper()

	remote := t.TempDir()
	run(t, remote, "git", "init", "--bare", "--initial-branch=main")

	dir := t.TempDir()
	run(t, dir, "git", "init", "--initial-branch=main")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	run(t, dir, "git", "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("test"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "init")
	run(t, dir, "git", "remote", "add", "origin", remote)
	run(t, dir, "git", "push", "-u", "origin", "main")
	return dir
}

func setBranchUpstream(t *testing.T, dir, branch, remote, mergeRef string) {
	t.Helper()
	run(t, dir, "git", "config", "branch."+branch+".remote", remote)
	run(t, dir, "git", "config", "branch."+branch+".merge", mergeRef)
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	if len(args) >= 3 && args[0] == "git" && args[1] == "commit" {
		args = []string{"git", "-c", "commit.gpgsign=false", "commit", args[2], args[3]}
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s: %v", args, out, err)
	}
}

func runOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s: %v", args, out, err)
	}
	return string(out)
}
