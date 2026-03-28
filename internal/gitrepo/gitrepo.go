package gitrepo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sethdeckard/loadout/internal/fsx"
)

type SyncReadiness struct {
	Branch   string
	Upstream string
}

type SyncState string

const (
	SyncStateNoRemote             SyncState = "no_remote"
	SyncStateEmptyRemoteNoCommits SyncState = "empty_remote_no_commits"
	SyncStateBootstrapEmptyRemote SyncState = "bootstrap_empty_remote"
	SyncStateNoUpstream           SyncState = "no_upstream"
	SyncStateStaleUpstream        SyncState = "stale_upstream"
	SyncStateUpToDate             SyncState = "up_to_date"
	SyncStateLocalAhead           SyncState = "local_ahead"
	SyncStateRemoteAhead          SyncState = "remote_ahead"
	SyncStateDiverged             SyncState = "diverged"
)

type SyncAssessment struct {
	State    SyncState
	Branch   string
	Upstream string
}

// IsRepo returns true if path is a git repository.
// It accepts both regular repos (.git directory) and worktrees (.git file).
func IsRepo(path string) bool {
	return fsx.Exists(filepath.Join(path, ".git"))
}

// CheckSyncReadiness reports whether the repo has enough local git configuration
// for `git pull --ff-only` to be attempted without mutating state.
func CheckSyncReadiness(path string) (SyncReadiness, error) {
	branch, err := currentBranch(path)
	if err != nil {
		return SyncReadiness{}, err
	}

	upstream, err := currentUpstream(path, branch)
	if err != nil {
		return SyncReadiness{Branch: branch}, err
	}

	if err := verifyRemoteTrackingRef(path, upstream); err != nil {
		return SyncReadiness{Branch: branch, Upstream: upstream}, err
	}

	return SyncReadiness{Branch: branch, Upstream: upstream}, nil
}

// Pull runs git pull in the given repo path.
func Pull(path string) error {
	cmd := exec.Command("git", "-C", path, "pull", "--ff-only")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git pull: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// Fetch updates remote-tracking refs without modifying the working tree.
func Fetch(path string) error {
	cmd := exec.Command("git", "-C", path, "fetch", "--quiet", "--prune")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// Push publishes the current branch to its configured upstream.
func Push(path string) error {
	cmd := exec.Command("git", "-C", path, "push")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// PushSetUpstream pushes the given branch to the named remote and configures upstream tracking.
func PushSetUpstream(path, remote, branch string) error {
	cmd := exec.Command("git", "-C", path, "push", "-u", remote, branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push -u: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// HeadCommit returns the short HEAD commit hash.
func HeadCommit(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// IsDirty returns true if the working tree has uncommitted changes.
func IsDirty(path string) (bool, error) {
	cmd := exec.Command("git", "-C", path, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}

// HasRemote reports whether the repo has at least one configured remote.
func HasRemote(path string) bool {
	cmd := exec.Command("git", "-C", path, "remote")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// HasRemoteHeads reports whether the remote has any branches.
func HasRemoteHeads(path string) (bool, error) {
	cmd := exec.Command("git", "-C", path, "ls-remote", "--heads")
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git ls-remote: %w", err)
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}

// Init creates a new git repository at path.
func Init(path string) error {
	cmd := exec.Command("git", "init", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git init: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// AddAndCommit stages all files and commits in the given repo.
func AddAndCommit(path, message string) error {
	add := exec.Command("git", "-C", path, "add", ".")
	if out, err := add.CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %s: %w", strings.TrimSpace(string(out)), err)
	}
	commit := commitCmd(path, message)
	if out, err := commit.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// AddPathsAndCommit stages only the provided paths and creates a commit.
func AddPathsAndCommit(path string, paths []string, message string) error {
	args := []string{"-C", path, "add", "--"}
	args = append(args, paths...)
	add := exec.Command("git", args...)
	if out, err := add.CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %s: %w", strings.TrimSpace(string(out)), err)
	}

	commit := commitCmd(path, message)
	if out, err := commit.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// HasTrackedPath reports whether the given relative path is tracked by git.
func HasTrackedPath(repoPath, relPath string) bool {
	cmd := exec.Command("git", "-C", repoPath, "ls-files", "--", relPath)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// Clone clones a git repo from url to path.
func Clone(url, path string) error {
	cmd := exec.Command("git", "clone", url, path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// AssessSyncState inspects sync direction after any needed fetch has refreshed remote-tracking refs.
func AssessSyncState(path string) (SyncAssessment, error) {
	branch, err := currentBranch(path)
	if err != nil {
		return SyncAssessment{}, err
	}

	assessment := SyncAssessment{Branch: branch}
	if !HasRemote(path) {
		assessment.State = SyncStateNoRemote
		return assessment, nil
	}

	hasLocalCommits, err := hasLocalCommits(path)
	if err != nil {
		return SyncAssessment{Branch: branch}, err
	}

	upstream, upstreamErr := currentUpstream(path, branch)
	if upstreamErr != nil {
		hasHeads, err := HasRemoteHeads(path)
		if err != nil {
			return SyncAssessment{Branch: branch}, err
		}
		switch {
		case !hasHeads && hasLocalCommits:
			assessment.State = SyncStateBootstrapEmptyRemote
		case !hasHeads:
			assessment.State = SyncStateEmptyRemoteNoCommits
		default:
			assessment.State = SyncStateNoUpstream
		}
		return assessment, nil
	}
	assessment.Upstream = upstream

	if err := verifyRemoteTrackingRef(path, upstream); err != nil {
		hasHeads, headsErr := HasRemoteHeads(path)
		if headsErr != nil {
			return SyncAssessment{Branch: branch, Upstream: upstream}, headsErr
		}
		switch {
		case !hasHeads && hasLocalCommits:
			assessment.State = SyncStateBootstrapEmptyRemote
		case !hasHeads:
			assessment.State = SyncStateEmptyRemoteNoCommits
		default:
			assessment.State = SyncStateStaleUpstream
		}
		return assessment, nil
	}

	if !hasLocalCommits {
		assessment.State = SyncStateUpToDate
		return assessment, nil
	}

	ahead, behind, err := aheadBehindCounts(path)
	if err != nil {
		return SyncAssessment{Branch: branch, Upstream: upstream}, err
	}
	switch {
	case ahead > 0 && behind > 0:
		assessment.State = SyncStateDiverged
	case ahead > 0:
		assessment.State = SyncStateLocalAhead
	case behind > 0:
		assessment.State = SyncStateRemoteAhead
	default:
		assessment.State = SyncStateUpToDate
	}
	return assessment, nil
}

// commitCmd builds a git commit command with gpg signing disabled and
// fallback author/committer identity so commits succeed even when the
// user has not configured git user.name / user.email globally.
func commitCmd(path, message string) *exec.Cmd {
	cmd := exec.Command("git", "-C", path, "-c", "commit.gpgsign=false", "commit", "-m", message)
	if !hasGitIdentity(path) {
		env := cmd.Environ()
		env = append(env,
			"GIT_AUTHOR_NAME=Loadout",
			"GIT_AUTHOR_EMAIL=loadout@localhost",
			"GIT_COMMITTER_NAME=Loadout",
			"GIT_COMMITTER_EMAIL=loadout@localhost",
		)
		cmd.Env = env
	}
	return cmd
}

func hasGitIdentity(path string) bool {
	cmd := exec.Command("git", "-C", path, "config", "user.name")
	if out, err := cmd.Output(); err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return true
	}
	for _, key := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		if v, ok := os.LookupEnv(key); ok && v != "" {
			return true
		}
	}
	return false
}

func currentBranch(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "symbolic-ref", "--quiet", "--short", "HEAD")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("detached HEAD")
	}
	return strings.TrimSpace(string(out)), nil
}

func currentUpstream(path, branch string) (string, error) {
	cmd := exec.Command("git", "-C", path, "config", "--get", "branch."+branch+".remote")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("branch has no upstream")
	}
	remote := strings.TrimSpace(string(out))
	if remote == "" {
		return "", fmt.Errorf("branch has no upstream")
	}

	cmd = exec.Command("git", "-C", path, "config", "--get", "branch."+branch+".merge")
	out, err = cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("branch has no upstream")
	}
	mergeRef := strings.TrimSpace(string(out))
	if mergeRef == "" {
		return "", fmt.Errorf("branch has no upstream")
	}

	upstreamBranch := strings.TrimPrefix(mergeRef, "refs/heads/")
	return remote + "/" + upstreamBranch, nil
}

func verifyRemoteTrackingRef(path, upstream string) error {
	ref := upstream
	if !strings.HasPrefix(ref, "refs/") {
		ref = "refs/remotes/" + upstream
	}

	cmd := exec.Command("git", "-C", path, "show-ref", "--verify", "--quiet", ref)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("upstream %s not found", upstream)
	}
	return nil
}

// IsBehindUpstream reports whether HEAD is behind its configured upstream.
func IsBehindUpstream(path string) (bool, error) {
	branch, err := currentBranch(path)
	if err != nil {
		return false, err
	}
	if _, err := currentUpstream(path, branch); err != nil {
		return false, err
	}

	cmd := exec.Command("git", "-C", path, "rev-list", "--count", "HEAD..@{upstream}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("git rev-list: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)) != "0", nil
}

func hasLocalCommits(path string) (bool, error) {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--verify", "HEAD")
	if out, err := cmd.CombinedOutput(); err != nil {
		text := strings.TrimSpace(string(out))
		if strings.Contains(text, "Needed a single revision") || strings.Contains(text, "unknown revision") || strings.Contains(text, "ambiguous argument") {
			return false, nil
		}
		return false, fmt.Errorf("git rev-parse: %s: %w", text, err)
	}
	return true, nil
}

func aheadBehindCounts(path string) (ahead int, behind int, err error) {
	cmd := exec.Command("git", "-C", path, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, 0, fmt.Errorf("git rev-list: %s: %w", strings.TrimSpace(string(out)), err)
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("git rev-list: unexpected count output %q", strings.TrimSpace(string(out)))
	}
	var left, right int
	if _, err := fmt.Sscanf(parts[0]+" "+parts[1], "%d %d", &left, &right); err != nil {
		return 0, 0, fmt.Errorf("parse ahead/behind: %w", err)
	}
	return left, right, nil
}
