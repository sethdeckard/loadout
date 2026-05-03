package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sethdeckard/loadout/internal/config"
	"github.com/sethdeckard/loadout/internal/domain"
	"github.com/sethdeckard/loadout/internal/install"
	"github.com/sethdeckard/loadout/internal/reconcile"
)

func testTargetConfig(path string) config.TargetConfig {
	return config.TargetConfig{
		Enabled: true,
		Path:    path,
	}
}

func setupTestEnv(t *testing.T) (*Service, string) {
	t.Helper()

	// Create a test repo
	repoDir := t.TempDir()
	skillDir := filepath.Join(repoDir, "skills", "test-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Test Skill\nA test."), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.json"), []byte(`{
		"name": "test-skill",
		"description": "A test skill",
		"tags": ["test"],
		"targets": ["claude", "codex"]
	}`), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	claudeDir := filepath.Join(t.TempDir(), "claude-skills")
	codexDir := filepath.Join(t.TempDir(), "codex-skills")

	cfg := config.Config{
		RepoPath: repoDir,
		Targets: config.TargetPaths{
			Claude: testTargetConfig(claudeDir),
			Codex:  testTargetConfig(codexDir),
		},
	}

	svc := New(cfg)
	return svc, claudeDir
}

func TestListSkills(t *testing.T) {
	svc, _ := setupTestEnv(t)
	views, err := svc.ListSkills()
	if err != nil {
		t.Fatalf("ListSkills() error = %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("got %d views, want 1", len(views))
	}
	if views[0].Skill.Name != "test-skill" {
		t.Errorf("ID = %q", views[0].Skill.Name)
	}
}

func TestPreviewSkill(t *testing.T) {
	svc, _ := setupTestEnv(t)
	preview, err := svc.PreviewSkill("test-skill")
	if err != nil {
		t.Fatalf("PreviewSkill() error = %v", err)
	}
	if preview.Skill.Name != "test-skill" {
		t.Errorf("Name = %q", preview.Skill.Name)
	}
	if preview.Markdown == "" {
		t.Error("expected non-empty markdown")
	}
}

func TestPreviewImportSource(t *testing.T) {
	svc, _ := setupTestEnv(t)
	source := filepath.Join(t.TempDir(), "import-skill")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("setup source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("# Import Skill\nImported from disk."), 0o644); err != nil {
		t.Fatalf("setup SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "skill.json"), []byte(`{
		"name": "import-skill",
		"description": "From local source",
		"targets": ["claude"]
	}`), 0o644); err != nil {
		t.Fatalf("setup skill.json: %v", err)
	}

	preview, err := svc.PreviewImportSource(source, []domain.Target{domain.TargetClaude, domain.TargetCodex})
	if err != nil {
		t.Fatalf("PreviewImportSource() error = %v", err)
	}
	if preview.Skill.Name != "import-skill" {
		t.Fatalf("ID = %q, want import-skill", preview.Skill.Name)
	}
	if got := preview.Skill.Targets; len(got) != 2 {
		t.Fatalf("Targets len = %d, want 2", len(got))
	}
	if preview.SourceDir != source {
		t.Fatalf("SourceDir = %q, want %q", preview.SourceDir, source)
	}
}

func TestListImportCandidates_ExcludesSkillsAlreadyInRepo(t *testing.T) {
	svc, claudeDir := setupTestEnv(t)
	source := filepath.Join(claudeDir, "test-skill")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("setup source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("# Test Skill\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "skill.json"), []byte(`{
		"name": "test-skill",
		"description": "A test skill",
		"targets": ["claude"]
	}`), 0o644); err != nil {
		t.Fatalf("write skill.json: %v", err)
	}

	views, err := svc.ListImportCandidates()
	if err != nil {
		t.Fatalf("ListImportCandidates() error = %v", err)
	}
	if len(views) != 0 {
		t.Fatalf("views len = %d, want 0", len(views))
	}
}

func TestListImportCandidatesFromDir_MarksSkillsAlreadyInRepoNotReady(t *testing.T) {
	svc, _ := setupTestEnv(t)
	root := t.TempDir()
	source := filepath.Join(root, "skills", "test-skill")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("setup source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("# Test Skill\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "skill.json"), []byte(`{
		"name": "test-skill",
		"description": "A test skill",
		"targets": ["claude"]
	}`), 0o644); err != nil {
		t.Fatalf("write skill.json: %v", err)
	}

	views, err := svc.ListImportCandidatesFromDir(root)
	if err != nil {
		t.Fatalf("ListImportCandidatesFromDir() error = %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("views len = %d, want 1", len(views))
	}
	if views[0].Ready {
		t.Fatal("expected duplicate repo candidate to be blocked")
	}
	if got, want := views[0].Problem, "already in repo"; got != want {
		t.Fatalf("Problem = %q, want %q", got, want)
	}
}

func TestToggleSkillTarget(t *testing.T) {
	svc, claudeDir := setupTestEnv(t)

	// Enable
	err := svc.ToggleSkillTarget("test-skill", domain.TargetClaude)
	if err != nil {
		t.Fatalf("ToggleSkillTarget(enable) error = %v", err)
	}
	if !install.IsInstalled("test-skill", claudeDir) {
		t.Error("expected skill to be installed after enable")
	}
	if !install.HasMarker("test-skill", claudeDir) {
		t.Error("expected marker after enable")
	}

	// Disable (toggle again)
	err = svc.ToggleSkillTarget("test-skill", domain.TargetClaude)
	if err != nil {
		t.Fatalf("ToggleSkillTarget(disable) error = %v", err)
	}
	if install.IsInstalled("test-skill", claudeDir) {
		t.Error("expected skill to be removed after disable")
	}
}

func TestEnableDisable(t *testing.T) {
	svc, claudeDir := setupTestEnv(t)

	if err := svc.EnableSkillTarget("test-skill", domain.TargetClaude); err != nil {
		t.Fatalf("Enable error = %v", err)
	}
	if !install.IsInstalled("test-skill", claudeDir) {
		t.Error("expected installed")
	}
	if !install.HasMarker("test-skill", claudeDir) {
		t.Error("expected marker")
	}

	if err := svc.DisableSkillTarget("test-skill", domain.TargetClaude); err != nil {
		t.Fatalf("Disable error = %v", err)
	}
	if install.IsInstalled("test-skill", claudeDir) {
		t.Error("expected removed")
	}
}

func TestDoctor(t *testing.T) {
	svc, _ := setupTestEnv(t)
	report, err := svc.Doctor()
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	if len(report.Checks) == 0 {
		t.Error("expected checks")
	}
}

func setupSkillOnlyRepo(t *testing.T) *Service {
	t.Helper()
	repoDir := t.TempDir()
	skillDir := filepath.Join(repoDir, "skills", "alpha-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Alpha"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.json"), []byte(`{
		"id": "alpha-skill",
		"name": "Alpha Skill",
		"description": "Alpha test skill",
		"tags": ["alpha"],
		"targets": ["claude"]
	}`), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	claudeDir := filepath.Join(t.TempDir(), "claude")
	codexDir := filepath.Join(t.TempDir(), "codex")

	cfg := config.Config{
		RepoPath: repoDir,
		Targets: config.TargetPaths{
			Claude: testTargetConfig(claudeDir),
			Codex:  testTargetConfig(codexDir),
		},
	}
	return New(cfg)
}

func TestSyncRepo_RepoNotFound(t *testing.T) {
	svc := setupSkillOnlyRepo(t)
	err := svc.SyncRepo()
	if err == nil {
		t.Fatal("SyncRepo() expected error, got nil")
	}
	if !errors.Is(err, domain.ErrRepoNotFound) {
		t.Errorf("SyncRepo() error = %v, want ErrRepoNotFound", err)
	}
}

func TestSyncRepoWithResult_RefreshesOutdatedUserInstall(t *testing.T) {
	svc, claudeDir := setupTrackedTestEnv(t)

	if err := svc.EnableSkillTarget("test-skill", domain.TargetClaude); err != nil {
		t.Fatalf("EnableSkillTarget() error = %v", err)
	}

	repoSkill := filepath.Join(svc.Config.RepoPath, "skills", "test-skill", "SKILL.md")
	if err := os.WriteFile(repoSkill, []byte("# Test Skill\nUpdated content."), 0o644); err != nil {
		t.Fatalf("write repo skill: %v", err)
	}
	runGit(t, svc.Config.RepoPath, "git", "add", ".")
	runGit(t, svc.Config.RepoPath, "git", "commit", "-m", "update test skill")

	result, err := svc.SyncRepoWithResult("")
	if err != nil {
		t.Fatalf("SyncRepoWithResult() error = %v", err)
	}
	if result.RefreshedUser != 1 || result.RefreshedProject != 0 {
		t.Fatalf("refresh counts = user:%d project:%d, want 1/0", result.RefreshedUser, result.RefreshedProject)
	}

	installedMD, err := os.ReadFile(filepath.Join(claudeDir, "test-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("read installed SKILL.md: %v", err)
	}
	if !strings.Contains(string(installedMD), "Updated content.") {
		t.Fatalf("installed SKILL.md = %q, want updated content", string(installedMD))
	}
}

func TestSyncRepoWithResult_RefreshesOutdatedProjectInstall(t *testing.T) {
	svc, _ := setupTrackedTestEnv(t)
	projectDir := t.TempDir()
	projectClaudeDir := filepath.Join(projectDir, ".claude", "skills")

	if err := svc.ProjectInstall("test-skill", domain.TargetClaude, projectDir); err != nil {
		t.Fatalf("ProjectInstall() error = %v", err)
	}

	repoSkill := filepath.Join(svc.Config.RepoPath, "skills", "test-skill", "SKILL.md")
	if err := os.WriteFile(repoSkill, []byte("# Test Skill\nProject updated content."), 0o644); err != nil {
		t.Fatalf("write repo skill: %v", err)
	}
	runGit(t, svc.Config.RepoPath, "git", "add", ".")
	runGit(t, svc.Config.RepoPath, "git", "commit", "-m", "update test skill for project")

	result, err := svc.SyncRepoWithResult(projectDir)
	if err != nil {
		t.Fatalf("SyncRepoWithResult() error = %v", err)
	}
	if result.RefreshedUser != 0 || result.RefreshedProject != 1 {
		t.Fatalf("refresh counts = user:%d project:%d, want 0/1", result.RefreshedUser, result.RefreshedProject)
	}

	installedMD, err := os.ReadFile(filepath.Join(projectClaudeDir, "test-skill", "SKILL.md"))
	if err != nil {
		t.Fatalf("read project installed SKILL.md: %v", err)
	}
	if !strings.Contains(string(installedMD), "Project updated content.") {
		t.Fatalf("project installed SKILL.md = %q, want updated content", string(installedMD))
	}
}

func TestSyncRepoWithResult_SkipsPullNoRemote(t *testing.T) {
	svc, _ := setupGitTestEnv(t)

	result, err := svc.SyncRepoWithResult("")
	if err != nil {
		t.Fatalf("SyncRepoWithResult() error = %v", err)
	}
	if result.RepoChanged {
		t.Error("expected RepoChanged == false for local-only repo")
	}
	if result.Pushed || result.Pulled || result.Bootstrapped {
		t.Fatalf("expected no remote actions, got %+v", result)
	}
}

func TestSyncRepoWithResult_StaleUpstream(t *testing.T) {
	svc, _ := setupTrackedTestEnv(t)

	// Point the branch at a non-existent upstream ref
	setGitBranchUpstream(t, svc.Config.RepoPath, "main", "origin", "refs/heads/deleted-branch")

	_, err := svc.SyncRepoWithResult("")
	if err == nil {
		t.Fatal("expected error for stale upstream, got nil")
	}
}

func TestSyncRepoWithResult_PushesLocalAhead(t *testing.T) {
	svc, _, remote := setupTrackedTestEnvWithRemote(t)

	if err := os.WriteFile(filepath.Join(svc.Config.RepoPath, "LOCAL.md"), []byte("local change"), 0o644); err != nil {
		t.Fatalf("write local change: %v", err)
	}
	runGit(t, svc.Config.RepoPath, "git", "add", ".")
	runGit(t, svc.Config.RepoPath, "git", "commit", "-m", "local update")

	result, err := svc.SyncRepoWithResult("")
	if err != nil {
		t.Fatalf("SyncRepoWithResult() error = %v", err)
	}
	if !result.Pushed || result.Pulled || result.Bootstrapped {
		t.Fatalf("result = %+v, want pushed-only", result)
	}

	out := runGitOutput(t, remote, "git", "log", "--pretty=%s", "-1", "main")
	if !strings.Contains(out, "local update") {
		t.Fatalf("remote HEAD = %q, want pushed local commit", out)
	}
}

func TestSyncRepoWithResult_BootstrapsEmptyRemote(t *testing.T) {
	svc, _ := setupTestEnv(t)
	remote := t.TempDir()
	runGit(t, remote, "git", "init", "--bare", "--initial-branch=master")
	runGit(t, svc.Config.RepoPath, "git", "init", "--initial-branch=master")
	runGit(t, svc.Config.RepoPath, "git", "config", "user.email", "test@test.com")
	runGit(t, svc.Config.RepoPath, "git", "config", "user.name", "Test")
	runGit(t, svc.Config.RepoPath, "git", "remote", "add", "origin", remote)
	runGit(t, svc.Config.RepoPath, "git", "add", ".")
	runGit(t, svc.Config.RepoPath, "git", "commit", "-m", "first local commit")

	result, err := svc.SyncRepoWithResult("")
	if err != nil {
		t.Fatalf("SyncRepoWithResult() error = %v", err)
	}
	if !result.Pushed || !result.Bootstrapped || result.Pulled {
		t.Fatalf("result = %+v, want bootstrap push", result)
	}

	upstream := strings.TrimSpace(runGitOutput(t, svc.Config.RepoPath, "git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"))
	if upstream != "origin/master" {
		t.Fatalf("upstream = %q, want origin/master", upstream)
	}
	remoteHead := runGitOutput(t, remote, "git", "log", "--pretty=%s", "-1", "master")
	if !strings.Contains(remoteHead, "first local commit") {
		t.Fatalf("remote HEAD = %q, want first local commit", remoteHead)
	}
}

func TestSyncRepoWithResult_ErrorsOnDivergedHistory(t *testing.T) {
	svc, _, remote := setupTrackedTestEnvWithRemote(t)

	parent := t.TempDir()
	clone := filepath.Join(parent, "clone")
	runGit(t, parent, "git", "clone", remote, clone)
	runGit(t, clone, "git", "config", "user.email", "test@test.com")
	runGit(t, clone, "git", "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(clone, "REMOTE.md"), []byte("remote change"), 0o644); err != nil {
		t.Fatalf("write remote change: %v", err)
	}
	runGit(t, clone, "git", "add", ".")
	runGit(t, clone, "git", "commit", "-m", "remote update")
	runGit(t, clone, "git", "push", "origin", "main")

	if err := os.WriteFile(filepath.Join(svc.Config.RepoPath, "LOCAL.md"), []byte("local change"), 0o644); err != nil {
		t.Fatalf("write local change: %v", err)
	}
	runGit(t, svc.Config.RepoPath, "git", "add", ".")
	runGit(t, svc.Config.RepoPath, "git", "commit", "-m", "local update")

	_, err := svc.SyncRepoWithResult("")
	if err == nil {
		t.Fatal("expected divergence error, got nil")
	}
	if !strings.Contains(err.Error(), "have diverged") {
		t.Fatalf("error = %q, want divergence detail", err.Error())
	}
}

func TestSyncStatus_NoCommits(t *testing.T) {
	repoDir := t.TempDir()
	runGit(t, repoDir, "git", "init", "--initial-branch=main")

	claudeDir := filepath.Join(t.TempDir(), "claude")
	codexDir := filepath.Join(t.TempDir(), "codex")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	svc := New(config.Config{
		RepoPath: repoDir,
		Targets: config.TargetPaths{
			Claude: testTargetConfig(claudeDir),
			Codex:  testTargetConfig(codexDir),
		},
	})

	status, err := svc.SyncStatus("")
	if err != nil {
		t.Fatalf("SyncStatus() error = %v", err)
	}
	if status.NeedsSync {
		t.Error("expected NeedsSync == false for empty repo")
	}
}

func TestSyncStatus_DetectsOutdatedManagedInstall(t *testing.T) {
	svc, _ := setupTrackedTestEnv(t)

	if err := svc.EnableSkillTarget("test-skill", domain.TargetClaude); err != nil {
		t.Fatalf("EnableSkillTarget() error = %v", err)
	}

	repoSkill := filepath.Join(svc.Config.RepoPath, "skills", "test-skill", "SKILL.md")
	if err := os.WriteFile(repoSkill, []byte("# Test Skill\nLocally changed."), 0o644); err != nil {
		t.Fatalf("write repo skill: %v", err)
	}
	runGit(t, svc.Config.RepoPath, "git", "add", ".")
	runGit(t, svc.Config.RepoPath, "git", "commit", "-m", "local update")

	status, err := svc.SyncStatus("")
	if err != nil {
		t.Fatalf("SyncStatus() error = %v", err)
	}
	if !status.NeedsSync {
		t.Fatal("expected sync attention for outdated managed install")
	}
	if status.OutdatedUser != 1 {
		t.Fatalf("OutdatedUser = %d, want 1", status.OutdatedUser)
	}
}

func TestSyncStatus_DetectsRemoteAheadAfterFetch(t *testing.T) {
	svc, _, remote := setupTrackedTestEnvWithRemote(t)

	parent := t.TempDir()
	clone := filepath.Join(parent, "clone")
	runGit(t, parent, "git", "clone", remote, clone)
	runGit(t, clone, "git", "config", "user.email", "test@test.com")
	runGit(t, clone, "git", "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(clone, "REMOTE.md"), []byte("remote change"), 0o644); err != nil {
		t.Fatalf("write remote change: %v", err)
	}
	runGit(t, clone, "git", "add", ".")
	runGit(t, clone, "git", "commit", "-m", "remote update")
	runGit(t, clone, "git", "push", "origin", "main")

	status, err := svc.SyncStatus("")
	if err != nil {
		t.Fatalf("SyncStatus() error = %v", err)
	}
	if !status.RemoteAhead || !status.NeedsSync {
		t.Fatalf("status = %+v, want remote behind sync attention", status)
	}
}

func TestSyncStatus_DetectsLocalAhead(t *testing.T) {
	svc, _ := setupTrackedTestEnv(t)

	if err := os.WriteFile(filepath.Join(svc.Config.RepoPath, "LOCAL.md"), []byte("local change"), 0o644); err != nil {
		t.Fatalf("write local change: %v", err)
	}
	runGit(t, svc.Config.RepoPath, "git", "add", ".")
	runGit(t, svc.Config.RepoPath, "git", "commit", "-m", "local update")

	status, err := svc.SyncStatus("")
	if err != nil {
		t.Fatalf("SyncStatus() error = %v", err)
	}
	if !status.LocalAhead || !status.NeedsSync {
		t.Fatalf("status = %+v, want local ahead sync attention", status)
	}
}

func TestDoctor_MissingRepo(t *testing.T) {
	svc := setupSkillOnlyRepo(t)
	report, err := svc.Doctor()
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}

	var repoCheck *DoctorCheck
	for i := range report.Checks {
		if report.Checks[i].Name == "Repository" {
			repoCheck = &report.Checks[i]
		}
	}
	if repoCheck == nil {
		t.Fatal("no Repository check found")
	}
	if repoCheck.OK {
		t.Error("Repository check should be not OK for non-git dir")
	}
	if report.AllOK {
		t.Error("AllOK should be false when repo check fails")
	}
}

func TestDoctor_Converged(t *testing.T) {
	svc, _ := setupTrackedTestEnv(t)

	// Enable and install a skill → converged
	if err := svc.EnableSkillTarget("test-skill", domain.TargetClaude); err != nil {
		t.Fatalf("EnableSkillTarget: %v", err)
	}

	report, err := svc.Doctor()
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}

	var convergenceCheck *DoctorCheck
	for i := range report.Checks {
		if report.Checks[i].Name == "Convergence" {
			convergenceCheck = &report.Checks[i]
		}
	}
	if convergenceCheck == nil {
		t.Fatal("no Convergence check found")
	}
	if !convergenceCheck.OK {
		t.Errorf("Convergence check should be OK; detail: %s", convergenceCheck.Detail)
	}
	if !report.AllOK {
		t.Error("AllOK should be true when fully converged")
	}
}

func TestDoctor_SyncReadinessMissingUpstream(t *testing.T) {
	svc, _ := setupGitTestEnv(t)

	report, err := svc.Doctor()
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}

	check := findDoctorCheck(t, report, "Sync Readiness")
	if !check.OK {
		t.Fatalf("Sync Readiness should pass for local-only repo, detail: %s", check.Detail)
	}
	if !strings.Contains(check.Detail, "no remote configured") {
		t.Fatalf("detail = %q, want local-only detail", check.Detail)
	}
	if !report.AllOK {
		t.Error("AllOK should be true for local-only repo")
	}
}

func TestDoctor_SyncReadinessBootstrapEmptyRemote(t *testing.T) {
	svc, _ := setupTestEnv(t)
	remote := t.TempDir()
	runGit(t, remote, "git", "init", "--bare", "--initial-branch=master")
	runGit(t, svc.Config.RepoPath, "git", "init", "--initial-branch=master")
	runGit(t, svc.Config.RepoPath, "git", "config", "user.email", "test@test.com")
	runGit(t, svc.Config.RepoPath, "git", "config", "user.name", "Test")
	runGit(t, svc.Config.RepoPath, "git", "remote", "add", "origin", remote)
	runGit(t, svc.Config.RepoPath, "git", "add", ".")
	runGit(t, svc.Config.RepoPath, "git", "commit", "-m", "first local commit")

	report, err := svc.Doctor()
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}

	check := findDoctorCheck(t, report, "Sync Readiness")
	if !check.OK {
		t.Fatalf("Sync Readiness should pass for bootstrapable empty remote, detail: %s", check.Detail)
	}
	if !strings.Contains(check.Detail, "sync will publish first local commit") {
		t.Fatalf("detail = %q, want bootstrap detail", check.Detail)
	}
}

func TestDoctor_SyncReadinessStaleUpstream(t *testing.T) {
	svc, _ := setupTrackedTestEnv(t)
	setGitBranchUpstream(t, svc.Config.RepoPath, "main", "origin", "refs/heads/master")

	report, err := svc.Doctor()
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}

	check := findDoctorCheck(t, report, "Sync Readiness")
	if check.OK {
		t.Fatalf("Sync Readiness should fail, detail: %s", check.Detail)
	}
	if !strings.Contains(check.Detail, "origin/master") || !strings.Contains(check.Detail, "not found") {
		t.Fatalf("detail = %q, want stale upstream detail", check.Detail)
	}
	if report.AllOK {
		t.Error("AllOK should be false when sync readiness fails")
	}
}

func TestDoctor_SyncReadinessHealthyTrackedBranch(t *testing.T) {
	svc, _ := setupTrackedTestEnv(t)

	report, err := svc.Doctor()
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}

	check := findDoctorCheck(t, report, "Sync Readiness")
	if !check.OK {
		t.Fatalf("Sync Readiness should pass, detail: %s", check.Detail)
	}
	if !strings.Contains(check.Detail, "main -> origin/main") {
		t.Fatalf("detail = %q, want main -> origin/main", check.Detail)
	}
}

func TestListSkills_InvalidRepo(t *testing.T) {
	emptyDir := t.TempDir()
	cfg := config.Config{
		RepoPath: emptyDir,
		Targets:  config.TargetPaths{},
	}
	svc := New(cfg)
	_, err := svc.ListSkills()
	if err == nil {
		t.Fatal("ListSkills() expected error for missing skills dir, got nil")
	}
}

func TestPreviewSkill_NotFound(t *testing.T) {
	svc, _ := setupTestEnv(t)
	_, err := svc.PreviewSkill("no-such-skill")
	if err == nil {
		t.Fatal("PreviewSkill() expected error, got nil")
	}
	if !errors.Is(err, domain.ErrSkillNotFound) {
		t.Errorf("PreviewSkill() error = %v, want ErrSkillNotFound", err)
	}
}

func TestPreviewSkill_InvalidRepo(t *testing.T) {
	emptyDir := t.TempDir()
	cfg := config.Config{
		RepoPath: emptyDir,
		Targets:  config.TargetPaths{},
	}
	svc := New(cfg)
	_, err := svc.PreviewSkill("test-skill")
	if err == nil {
		t.Fatal("PreviewSkill() expected error for missing skills dir, got nil")
	}
}

func setupClaudeOnlyRepo(t *testing.T) *Service {
	t.Helper()
	repoDir := t.TempDir()
	skillDir := filepath.Join(repoDir, "skills", "claude-only")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Claude Only"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.json"), []byte(`{
		"name": "claude-only",
		"description": "Only for claude",
		"tags": ["test"],
		"targets": ["claude"]
	}`), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cfg := config.Config{
		RepoPath: repoDir,
		Targets: config.TargetPaths{
			Claude: testTargetConfig(filepath.Join(t.TempDir(), "claude")),
			Codex:  testTargetConfig(filepath.Join(t.TempDir(), "codex")),
		},
	}
	return New(cfg)
}

func TestEnableSkillTarget_DisabledTarget(t *testing.T) {
	svc, _ := setupTestEnv(t)
	svc.Config.Targets.Codex.Enabled = false

	err := svc.EnableSkillTarget("test-skill", domain.TargetCodex)
	if err == nil {
		t.Fatal("EnableSkillTarget() expected error for disabled target, got nil")
	}
	if !errors.Is(err, domain.ErrTargetDisabled) {
		t.Fatalf("EnableSkillTarget() error = %v, want ErrTargetDisabled", err)
	}
}

func TestEnableSkillTarget_UnsupportedTarget(t *testing.T) {
	svc := setupClaudeOnlyRepo(t)

	err := svc.EnableSkillTarget("claude-only", domain.TargetCodex)
	if err == nil {
		t.Fatal("EnableSkillTarget() expected error for unsupported target, got nil")
	}
	if !errors.Is(err, domain.ErrUnsupportedTarget) {
		t.Errorf("EnableSkillTarget() error = %v, want ErrUnsupportedTarget", err)
	}
}

func TestToggleSkillTarget_UnsupportedTarget(t *testing.T) {
	svc := setupClaudeOnlyRepo(t)

	err := svc.ToggleSkillTarget("claude-only", domain.TargetCodex)
	if err == nil {
		t.Fatal("ToggleSkillTarget() expected error for unsupported target, got nil")
	}
	if !errors.Is(err, domain.ErrUnsupportedTarget) {
		t.Errorf("ToggleSkillTarget() error = %v, want ErrUnsupportedTarget", err)
	}
}

func TestToggleSkillTarget_RejectsUnmanagedDir(t *testing.T) {
	svc, claudeDir := setupTestEnv(t)

	// Pre-create an unmanaged directory with the same skill name
	unmanaged := filepath.Join(claudeDir, "test-skill")
	if err := os.MkdirAll(unmanaged, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(unmanaged, "user-data.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	err := svc.ToggleSkillTarget("test-skill", domain.TargetClaude)
	if !errors.Is(err, domain.ErrUnmanagedDir) {
		t.Fatalf("ToggleSkillTarget() error = %v, want ErrUnmanagedDir", err)
	}

	// Verify unmanaged directory was preserved
	if _, err := os.Stat(filepath.Join(unmanaged, "user-data.txt")); err != nil {
		t.Error("unmanaged directory contents should be preserved")
	}
}

func TestDisableSkillTarget_NotInstalled(t *testing.T) {
	svc, _ := setupTestEnv(t)
	err := svc.DisableSkillTarget("test-skill", domain.TargetClaude)
	if err != nil {
		t.Errorf("DisableSkillTarget() on not-installed skill: %v", err)
	}
}

func TestProjectInstall(t *testing.T) {
	svc, _ := setupTestEnv(t)
	projectDir := t.TempDir()
	claudeSkills := filepath.Join(projectDir, ".claude", "skills")

	if err := svc.ProjectInstall("test-skill", domain.TargetClaude, projectDir); err != nil {
		t.Fatalf("ProjectInstall() error = %v", err)
	}

	if !install.IsInstalled("test-skill", claudeSkills) {
		t.Error("expected skill installed in project .claude/skills/")
	}
	if !install.HasMarker("test-skill", claudeSkills) {
		t.Error("expected marker in project install")
	}
}

func TestProjectRemove(t *testing.T) {
	svc, _ := setupTestEnv(t)
	projectDir := t.TempDir()

	if err := svc.ProjectInstall("test-skill", domain.TargetClaude, projectDir); err != nil {
		t.Fatalf("ProjectInstall() error = %v", err)
	}

	if err := svc.ProjectRemove("test-skill", domain.TargetClaude, projectDir); err != nil {
		t.Fatalf("ProjectRemove() error = %v", err)
	}

	claudeSkills := filepath.Join(projectDir, ".claude", "skills")
	if install.IsInstalled("test-skill", claudeSkills) {
		t.Error("expected skill to be removed from project")
	}
}

func TestProjectList(t *testing.T) {
	svc, _ := setupTestEnv(t)
	projectDir := t.TempDir()

	if err := svc.ProjectInstall("test-skill", domain.TargetClaude, projectDir); err != nil {
		t.Fatalf("ProjectInstall() error = %v", err)
	}

	views, err := svc.ProjectList(projectDir)
	if err != nil {
		t.Fatalf("ProjectList() error = %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("got %d views, want 1", len(views))
	}
	if views[0].Skill.Name != "test-skill" {
		t.Errorf("ID = %q, want test-skill", views[0].Skill.Name)
	}
	if !views[0].ProjectClaude {
		t.Error("expected ProjectClaude = true")
	}
	if views[0].ProjectCodex {
		t.Error("expected ProjectCodex = false")
	}
}

func TestProjectList_CrossReferencesRegistry(t *testing.T) {
	svc, _ := setupTestEnv(t)
	projectDir := t.TempDir()

	if err := svc.ProjectInstall("test-skill", domain.TargetClaude, projectDir); err != nil {
		t.Fatalf("ProjectInstall() error = %v", err)
	}

	views, err := svc.ProjectList(projectDir)
	if err != nil {
		t.Fatalf("ProjectList() error = %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("got %d views, want 1", len(views))
	}
	if views[0].Skill.Name != "test-skill" {
		t.Errorf("Name = %q, want 'test-skill' (from registry)", views[0].Skill.Name)
	}
	if views[0].Skill.Description != "A test skill" {
		t.Errorf("Description = %q, want 'A test skill'", views[0].Skill.Description)
	}
}

func TestProjectList_UnknownSkill(t *testing.T) {
	svc, _ := setupTestEnv(t)
	projectDir := t.TempDir()

	unknownDir := filepath.Join(projectDir, ".claude", "skills", "unknown-skill")
	if err := os.MkdirAll(unknownDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(unknownDir, "SKILL.md"), []byte("# Unknown"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	views, err := svc.ProjectList(projectDir)
	if err != nil {
		t.Fatalf("ProjectList() error = %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("got %d views, want 1", len(views))
	}
	if views[0].Skill.Name != "unknown-skill" {
		t.Errorf("ID = %q, want unknown-skill", views[0].Skill.Name)
	}
	if !views[0].ProjectClaude {
		t.Error("expected ProjectClaude = true")
	}
}

func TestListSkillsForProject_IncludesUserAndProjectState(t *testing.T) {
	svc, claudeDir := setupTestEnv(t)
	projectDir := t.TempDir()

	if err := svc.EnableSkillTarget("test-skill", domain.TargetClaude); err != nil {
		t.Fatalf("EnableSkillTarget() error = %v", err)
	}
	if err := svc.ProjectInstall("test-skill", domain.TargetCodex, projectDir); err != nil {
		t.Fatalf("ProjectInstall() error = %v", err)
	}
	if !install.IsInstalled("test-skill", claudeDir) {
		t.Fatal("expected user install for claude")
	}

	views, err := svc.ListSkillsForProject(projectDir)
	if err != nil {
		t.Fatalf("ListSkillsForProject() error = %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("got %d views, want 1", len(views))
	}
	view := views[0]
	if !view.InstalledClaude {
		t.Fatal("expected user Claude install")
	}
	if view.ProjectClaude {
		t.Fatal("expected no project Claude install")
	}
	if !view.ProjectCodex {
		t.Fatal("expected project Codex install")
	}
}

func TestListSkillsForProject_AppendsReadyProjectImportCandidates(t *testing.T) {
	svc, _ := setupTestEnv(t)
	projectDir := t.TempDir()
	source := filepath.Join(projectDir, "local-only")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("setup source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("# Local Only\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	views, err := svc.ListSkillsForProject(projectDir)
	if err != nil {
		t.Fatalf("ListSkillsForProject() error = %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("got %d views, want 2", len(views))
	}
	var found *SkillView
	for i := range views {
		if views[i].Skill.Name == "local-only" {
			found = &views[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected ready project-local candidate in views")
	}
	if got, want := found.Flags, []reconcile.StatusFlag{reconcile.StatusUnmanaged}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("Flags = %v, want %v", got, want)
	}
	if got, want := found.LocalSourceDir, source; got != want {
		t.Fatalf("LocalSourceDir = %q, want %q", got, want)
	}
	if got, want := found.Skill.Targets, []domain.Target{domain.TargetClaude, domain.TargetCodex}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Targets = %v, want %v", got, want)
	}
}

func TestListSkillsForProject_SkipsBlockedProjectImportCandidates(t *testing.T) {
	svc, _ := setupTestEnv(t)
	projectDir := t.TempDir()
	source := filepath.Join(projectDir, "skills", "test-skill")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("setup source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("# Test Skill\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	views, err := svc.ListSkillsForProject(projectDir)
	if err != nil {
		t.Fatalf("ListSkillsForProject() error = %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("got %d views, want 1", len(views))
	}
	if views[0].Skill.Name != "test-skill" {
		t.Fatalf("only repo skill should remain, got %q", views[0].Skill.Name)
	}
}

func TestDeleteSkillEligibility_Deletable(t *testing.T) {
	svc, _ := setupTestEnv(t)
	projectDir := t.TempDir()

	eligibility, err := svc.DeleteSkillEligibility("test-skill", projectDir)
	if err != nil {
		t.Fatalf("DeleteSkillEligibility() error = %v", err)
	}
	if !eligibility.Deletable {
		t.Fatalf("Deletable = false, blockers = %v", eligibility.Blockers)
	}
	if got, want := eligibility.DeletedPath, filepath.Join("skills", "test-skill"); got != want {
		t.Fatalf("DeletedPath = %q, want %q", got, want)
	}
}

func TestDeleteSkillEligibility_BlockedByManagedUserInstall(t *testing.T) {
	svc, claudeDir := setupTestEnv(t)
	if err := svc.EnableSkillTarget("test-skill", domain.TargetClaude); err != nil {
		t.Fatalf("EnableSkillTarget() error = %v", err)
	}
	if !install.HasMarker("test-skill", claudeDir) {
		t.Fatal("expected managed marker")
	}

	eligibility, err := svc.DeleteSkillEligibility("test-skill", "")
	if err != nil {
		t.Fatalf("DeleteSkillEligibility() error = %v", err)
	}
	if eligibility.Deletable {
		t.Fatal("expected deletion to be blocked")
	}
	if len(eligibility.Blockers) != 1 || !strings.Contains(eligibility.Blockers[0], "user claude") {
		t.Fatalf("Blockers = %v, want user claude blocker", eligibility.Blockers)
	}
}

func TestDeleteSkillEligibility_BlockedByManagedProjectInstall(t *testing.T) {
	svc, _ := setupTestEnv(t)
	projectDir := t.TempDir()
	if err := svc.ProjectInstall("test-skill", domain.TargetClaude, projectDir); err != nil {
		t.Fatalf("ProjectInstall() error = %v", err)
	}

	eligibility, err := svc.DeleteSkillEligibility("test-skill", projectDir)
	if err != nil {
		t.Fatalf("DeleteSkillEligibility() error = %v", err)
	}
	if eligibility.Deletable {
		t.Fatal("expected deletion to be blocked")
	}
	if len(eligibility.Blockers) != 1 || !strings.Contains(eligibility.Blockers[0], "project claude") {
		t.Fatalf("Blockers = %v, want project claude blocker", eligibility.Blockers)
	}
}

func TestDeleteSkillEligibility_IgnoresUnmanagedCopies(t *testing.T) {
	svc, claudeDir := setupTestEnv(t)
	unmanagedDir := filepath.Join(claudeDir, "test-skill")
	if err := os.MkdirAll(unmanagedDir, 0o755); err != nil {
		t.Fatalf("mkdir unmanaged dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(unmanagedDir, "SKILL.md"), []byte("# Unmanaged"), 0o644); err != nil {
		t.Fatalf("write unmanaged SKILL.md: %v", err)
	}

	eligibility, err := svc.DeleteSkillEligibility("test-skill", "")
	if err != nil {
		t.Fatalf("DeleteSkillEligibility() error = %v", err)
	}
	if !eligibility.Deletable {
		t.Fatalf("Deletable = false, blockers = %v", eligibility.Blockers)
	}
}

func TestDeleteSkillEligibility_BlockedByDisabledTargetInstall(t *testing.T) {
	svc, claudeDir := setupTestEnv(t)

	// Install the skill while target is enabled
	if err := svc.EnableSkillTarget("test-skill", domain.TargetClaude); err != nil {
		t.Fatalf("EnableSkillTarget() error = %v", err)
	}
	if !install.HasMarker("test-skill", claudeDir) {
		t.Fatal("expected managed marker")
	}

	// Disable the target — managed install remains on disk
	svc.Config.Targets.Claude.Enabled = false

	eligibility, err := svc.DeleteSkillEligibility("test-skill", "")
	if err != nil {
		t.Fatalf("DeleteSkillEligibility() error = %v", err)
	}
	if eligibility.Deletable {
		t.Fatal("expected deletion to be blocked by managed install on disabled target")
	}
	if len(eligibility.Blockers) != 1 || !strings.Contains(eligibility.Blockers[0], "user claude") {
		t.Fatalf("Blockers = %v, want user claude blocker", eligibility.Blockers)
	}
}

func TestDeleteSkill_RemovesRepoDirectory(t *testing.T) {
	svc, _ := setupTestEnv(t)

	result, err := svc.DeleteSkill("test-skill", "", false)
	if err != nil {
		t.Fatalf("DeleteSkill() error = %v", err)
	}
	if result.CommitCreated {
		t.Fatal("CommitCreated = true, want false")
	}
	if got, want := result.DeletedPath, filepath.Join("skills", "test-skill"); got != want {
		t.Fatalf("DeletedPath = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(svc.Config.RepoPath, result.DeletedPath)); !os.IsNotExist(err) {
		t.Fatalf("repo path still exists or stat error = %v", err)
	}
}

func TestDeleteSkill_BlockedWhenManagedInstallExists(t *testing.T) {
	svc, _ := setupTestEnv(t)
	if err := svc.EnableSkillTarget("test-skill", domain.TargetClaude); err != nil {
		t.Fatalf("EnableSkillTarget() error = %v", err)
	}

	_, err := svc.DeleteSkill("test-skill", "", false)
	if err == nil {
		t.Fatal("DeleteSkill() expected error, got nil")
	}
	if !errors.Is(err, domain.ErrSkillInstalled) {
		t.Fatalf("DeleteSkill() error = %v, want ErrSkillInstalled", err)
	}
}

func TestDeleteSkill_AutoCommit(t *testing.T) {
	svc, _ := setupTrackedTestEnv(t)

	result, err := svc.DeleteSkill("test-skill", "", true)
	if err != nil {
		t.Fatalf("DeleteSkill() error = %v", err)
	}
	if !result.CommitCreated {
		t.Fatal("CommitCreated = false, want true")
	}

	files := runGitOutput(t, svc.Config.RepoPath, "git", "show", "--name-only", "--format=", "HEAD")
	if !strings.Contains(files, "skills/test-skill/SKILL.md") || !strings.Contains(files, "skills/test-skill/skill.json") {
		t.Fatalf("commit files = %q, want deleted skill files", files)
	}
	message := runGitOutput(t, svc.Config.RepoPath, "git", "log", "-1", "--pretty=%s")
	if !strings.Contains(message, "Delete skill: test-skill") {
		t.Fatalf("commit message = %q, want delete message", message)
	}
}

func TestCommitRepoPath_StagesOnlyRequestedPath(t *testing.T) {
	svc, _ := setupTrackedTestEnv(t)
	if err := os.WriteFile(filepath.Join(svc.Config.RepoPath, "skills", "test-skill", "SKILL.md"), []byte("# Test Skill\nUpdated"), 0o644); err != nil {
		t.Fatalf("update skill file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(svc.Config.RepoPath, "notes.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatalf("write notes: %v", err)
	}

	if err := svc.CommitRepoPath(filepath.Join("skills", "test-skill"), "Add skill: test-skill"); err != nil {
		t.Fatalf("CommitRepoPath() error = %v", err)
	}

	files := runGitOutput(t, svc.Config.RepoPath, "git", "show", "--name-only", "--format=", "HEAD")
	if !strings.Contains(files, "skills/test-skill/SKILL.md") {
		t.Fatalf("commit files = %q, want modified test-skill file", files)
	}
	if strings.Contains(files, "notes.txt") {
		t.Fatalf("commit files = %q, should not include notes.txt", files)
	}
}

func TestDeleteSkill_NotFound(t *testing.T) {
	svc, _ := setupTestEnv(t)

	_, err := svc.DeleteSkill("missing-skill", "", false)
	if err == nil {
		t.Fatal("DeleteSkill() expected error, got nil")
	}
	if !errors.Is(err, domain.ErrSkillNotFound) {
		t.Fatalf("DeleteSkill() error = %v, want ErrSkillNotFound", err)
	}
}

func TestDetailOrErr(t *testing.T) {
	if got := detailOrErr("hello", nil); got != "hello" {
		t.Errorf("detailOrErr(nil) = %q, want %q", got, "hello")
	}
	sentinel := errors.New("boom")
	if got := detailOrErr("hello", sentinel); got != "boom" {
		t.Errorf("detailOrErr(err) = %q, want %q", got, "boom")
	}
}

func setupGitTestEnv(t *testing.T) (*Service, string) {
	t.Helper()

	svc, claudeDir := setupTestEnv(t)
	initGitRepo(t, svc.Config.RepoPath, "master")
	return svc, claudeDir
}

func setupTrackedTestEnv(t *testing.T) (*Service, string) {
	t.Helper()

	svc, claudeDir, _ := setupTrackedTestEnvWithRemote(t)
	return svc, claudeDir
}

func setupTrackedTestEnvWithRemote(t *testing.T) (*Service, string, string) {
	t.Helper()

	svc, claudeDir := setupTestEnv(t)
	remote := initTrackedGitRepo(t, svc.Config.RepoPath, "main")
	return svc, claudeDir, remote
}

func initGitRepo(t *testing.T, dir, branch string) {
	t.Helper()

	runGit(t, dir, "git", "init", "--initial-branch="+branch)
	runGit(t, dir, "git", "config", "user.email", "test@test.com")
	runGit(t, dir, "git", "config", "user.name", "Test")
	runGit(t, dir, "git", "add", ".")
	runGit(t, dir, "git", "commit", "-m", "init")
}

func initTrackedGitRepo(t *testing.T, dir, branch string) string {
	t.Helper()

	remote := t.TempDir()
	runGit(t, remote, "git", "init", "--bare", "--initial-branch="+branch)
	initGitRepo(t, dir, branch)
	runGit(t, dir, "git", "remote", "add", "origin", remote)
	runGit(t, dir, "git", "push", "-u", "origin", branch)
	return remote
}

func setGitBranchUpstream(t *testing.T, dir, branch, remote, mergeRef string) {
	t.Helper()
	runGit(t, dir, "git", "config", "branch."+branch+".remote", remote)
	runGit(t, dir, "git", "config", "branch."+branch+".merge", mergeRef)
}

func runGit(t *testing.T, dir string, args ...string) {
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

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s: %v", args, out, err)
	}
	return string(out)
}

func TestListSkills_OrphanedEnrichment(t *testing.T) {
	// Set up an empty repo (no skills)
	repoDir := t.TempDir()
	skillsDir := filepath.Join(repoDir, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	claudeDir := filepath.Join(t.TempDir(), "claude-skills")
	codexDir := filepath.Join(t.TempDir(), "codex-skills")

	// Create an orphaned managed install in claude root
	orphanDir := filepath.Join(claudeDir, "orphan-skill")
	if err := os.MkdirAll(orphanDir, 0o755); err != nil {
		t.Fatalf("setup orphan dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orphanDir, "skill.json"), []byte(`{
		"name": "orphan-skill",
		"description": "An orphaned skill",
		"tags": ["test"],
		"targets": ["claude", "codex"]
	}`), 0o644); err != nil {
		t.Fatalf("write skill.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orphanDir, "SKILL.md"), []byte("# Orphan Skill\nBody."), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	// Write .loadout marker to make it managed
	if err := os.WriteFile(filepath.Join(orphanDir, ".loadout"), []byte(`{"repo_commit":"abc123","installed_at":"2025-01-01T00:00:00Z"}`), 0o644); err != nil {
		t.Fatalf("write .loadout: %v", err)
	}

	cfg := config.Config{
		RepoPath: repoDir,
		Targets: config.TargetPaths{
			Claude: testTargetConfig(claudeDir),
			Codex:  testTargetConfig(codexDir),
		},
	}
	svc := New(cfg)

	views, err := svc.ListSkills()
	if err != nil {
		t.Fatalf("ListSkills() error = %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("got %d views, want 1", len(views))
	}
	view := views[0]
	if view.Skill.Name != "orphan-skill" {
		t.Errorf("Name = %q, want orphan-skill", view.Skill.Name)
	}
	if view.Skill.Description != "An orphaned skill" {
		t.Errorf("Description = %q, want enriched from skill.json", view.Skill.Description)
	}
	if len(view.Skill.Targets) != 2 {
		t.Errorf("Targets len = %d, want 2", len(view.Skill.Targets))
	}
	if !view.Orphaned {
		t.Error("expected Orphaned = true")
	}
	if view.OrphanRoot != claudeDir {
		t.Errorf("OrphanRoot = %q, want %q", view.OrphanRoot, claudeDir)
	}
	if view.LocalRoot != claudeDir {
		t.Errorf("LocalRoot = %q, want %q", view.LocalRoot, claudeDir)
	}
}

func TestListSkills_UnmanagedLocalRoot(t *testing.T) {
	repoDir := t.TempDir()
	skillsDir := filepath.Join(repoDir, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	claudeDir := filepath.Join(t.TempDir(), "claude-skills")
	codexDir := filepath.Join(t.TempDir(), "codex-skills")
	unmanagedDir := filepath.Join(claudeDir, "local-only")
	if err := os.MkdirAll(unmanagedDir, 0o755); err != nil {
		t.Fatalf("setup unmanaged dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(unmanagedDir, "SKILL.md"), []byte("# Local Only\nBody."), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	cfg := config.Config{
		RepoPath: repoDir,
		Targets: config.TargetPaths{
			Claude: testTargetConfig(claudeDir),
			Codex:  testTargetConfig(codexDir),
		},
	}
	svc := New(cfg)

	views, err := svc.ListSkills()
	if err != nil {
		t.Fatalf("ListSkills() error = %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("got %d views, want 1", len(views))
	}
	if got, want := views[0].LocalRoot, claudeDir; got != want {
		t.Fatalf("LocalRoot = %q, want %q", got, want)
	}
}

func TestPreviewLocalSkill(t *testing.T) {
	targetRoot := t.TempDir()
	skillDir := filepath.Join(targetRoot, "local-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.json"), []byte(`{
		"name": "local-skill",
		"description": "From local disk",
		"targets": ["claude"]
	}`), 0o644); err != nil {
		t.Fatalf("write skill.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Local Skill\nPreview body."), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "extra.txt"), []byte("extra"), 0o644); err != nil {
		t.Fatalf("write extra: %v", err)
	}

	svc := New(config.Config{RepoPath: t.TempDir()})
	preview, err := svc.PreviewLocalSkill("local-skill", targetRoot)
	if err != nil {
		t.Fatalf("PreviewLocalSkill() error = %v", err)
	}
	if preview.Skill.Name != "local-skill" {
		t.Errorf("Name = %q", preview.Skill.Name)
	}
	if preview.Skill.Description != "From local disk" {
		t.Errorf("Description = %q", preview.Skill.Description)
	}
	if !strings.Contains(preview.Markdown, "Preview body.") {
		t.Errorf("Markdown = %q, want body content", preview.Markdown)
	}
	if len(preview.Files) != 1 || preview.Files[0] != "extra.txt" {
		t.Errorf("Files = %v, want [extra.txt]", preview.Files)
	}
}

func TestPreviewLocalSkill_DirectoryNameOverridesJSON(t *testing.T) {
	targetRoot := t.TempDir()
	skillDir := filepath.Join(targetRoot, "dir-name")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	// skill.json has a different name than the directory
	if err := os.WriteFile(filepath.Join(skillDir, "skill.json"), []byte(`{
		"name": "json-name",
		"description": "Mismatched name",
		"targets": ["claude"]
	}`), 0o644); err != nil {
		t.Fatalf("write skill.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Test\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	svc := New(config.Config{RepoPath: t.TempDir()})
	preview, err := svc.PreviewLocalSkill("dir-name", targetRoot)
	if err != nil {
		t.Fatalf("PreviewLocalSkill() error = %v", err)
	}
	if preview.Skill.Name != "dir-name" {
		t.Errorf("Name = %q, want dir-name (directory name must win)", preview.Skill.Name)
	}
	if preview.Skill.Description != "Mismatched name" {
		t.Errorf("Description = %q, want metadata preserved from skill.json", preview.Skill.Description)
	}
}

func TestPreviewLocalSkill_FallbackToSKILLmd(t *testing.T) {
	targetRoot := t.TempDir()
	skillDir := filepath.Join(targetRoot, "md-only")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\ndescription: From frontmatter\n---\n\n# MD Only\nBody."), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	svc := New(config.Config{RepoPath: t.TempDir()})
	preview, err := svc.PreviewLocalSkill("md-only", targetRoot)
	if err != nil {
		t.Fatalf("PreviewLocalSkill() error = %v", err)
	}
	if preview.Skill.Name != "md-only" {
		t.Errorf("Name = %q", preview.Skill.Name)
	}
	if preview.Skill.Description != "From frontmatter" {
		t.Errorf("Description = %q, want From frontmatter", preview.Skill.Description)
	}
}

func findDoctorCheck(t *testing.T, report DoctorReport, name string) DoctorCheck { //nolint:unparam // name varies as doctor checks grow
	t.Helper()

	for _, check := range report.Checks {
		if check.Name == name {
			return check
		}
	}
	t.Fatalf("no %s check found", name)
	return DoctorCheck{}
}

func TestShare_UnknownSkill(t *testing.T) {
	svc, _ := setupTestEnv(t)
	out := filepath.Join(t.TempDir(), "missing.tar.gz")

	_, err := svc.Share("does-not-exist", out)
	if !errors.Is(err, domain.ErrSkillNotFound) {
		t.Fatalf("Share() error = %v, want ErrSkillNotFound", err)
	}
	if _, statErr := os.Stat(out); !os.IsNotExist(statErr) {
		t.Errorf("output file should not exist on error, stat err=%v", statErr)
	}
}

func TestShare_OutPathSemantics(t *testing.T) {
	svc, _ := setupTestEnv(t)

	tests := []struct {
		name      string
		outArg    func(t *testing.T) string
		wantBase  string
		wantInDir bool
	}{
		{
			name:      "empty defaults to cwd",
			outArg:    func(t *testing.T) string { return "" },
			wantBase:  "test-skill.tar.gz",
			wantInDir: false, // checked separately by chdir
		},
		{
			name: "trailing slash treated as directory",
			outArg: func(t *testing.T) string {
				dir := t.TempDir()
				return dir + string(filepath.Separator)
			},
			wantBase:  "test-skill.tar.gz",
			wantInDir: true,
		},
		{
			name: "existing directory writes inside",
			outArg: func(t *testing.T) string {
				return t.TempDir()
			},
			wantBase:  "test-skill.tar.gz",
			wantInDir: true,
		},
		{
			name: "verbatim file path",
			outArg: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "custom-name.tgz")
			},
			wantBase:  "custom-name.tgz",
			wantInDir: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outArg := tt.outArg(t)

			if tt.name == "empty defaults to cwd" {
				cwd := t.TempDir()
				prev, err := os.Getwd()
				if err != nil {
					t.Fatalf("getwd: %v", err)
				}
				if err := os.Chdir(cwd); err != nil {
					t.Fatalf("chdir: %v", err)
				}
				t.Cleanup(func() { _ = os.Chdir(prev) })
			}

			path, err := svc.Share("test-skill", outArg)
			if err != nil {
				t.Fatalf("Share() error = %v", err)
			}
			if filepath.Base(path) != tt.wantBase {
				t.Errorf("base(path) = %q, want %q", filepath.Base(path), tt.wantBase)
			}
			if tt.wantInDir {
				if filepath.Dir(path) != strings.TrimRight(outArg, string(filepath.Separator)) {
					t.Errorf("dir(path) = %q, want %q", filepath.Dir(path), strings.TrimRight(outArg, string(filepath.Separator)))
				}
			}
			if _, err := os.Stat(path); err != nil {
				t.Errorf("archive missing at returned path: %v", err)
			}
		})
	}
}

func TestShare_MissingParentDir(t *testing.T) {
	svc, _ := setupTestEnv(t)
	out := filepath.Join(t.TempDir(), "no-such-subdir", "test-skill.tar.gz")

	_, err := svc.Share("test-skill", out)
	if err == nil {
		t.Fatal("Share() with missing parent: want error, got nil")
	}
}
