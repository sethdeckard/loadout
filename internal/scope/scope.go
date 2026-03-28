package scope

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sethdeckard/loadout/internal/config"
	"github.com/sethdeckard/loadout/internal/domain"
)

type Scope struct {
	Project string // empty = user, otherwise project root path
}

func (s Scope) IsProject() bool {
	return s.Project != ""
}

func (s Scope) IsUser() bool {
	return s.Project == ""
}

func (s Scope) TargetRoot(target domain.Target, userCfg config.TargetPaths) string {
	if s.IsUser() {
		return userCfg.Path(target)
	}
	switch target {
	case domain.TargetClaude:
		return filepath.Join(s.Project, ".claude", "skills")
	case domain.TargetCodex:
		return filepath.Join(s.Project, ".codex", "skills")
	default:
		return ""
	}
}

func Resolve(projectFlag string) (Scope, error) {
	if projectFlag == "" {
		return Scope{}, nil
	}

	var startDir string
	if projectFlag == "." {
		wd, err := os.Getwd()
		if err != nil {
			return Scope{}, fmt.Errorf("get working directory: %w", err)
		}
		startDir = wd
	} else {
		abs, err := filepath.Abs(projectFlag)
		if err != nil {
			return Scope{}, fmt.Errorf("resolve path: %w", err)
		}
		// Resolve symlinks for consistent path comparison.
		// Fall back to the absolute path if it does not exist yet
		// (e.g. Windows without elevated symlink privileges).
		resolved, err := filepath.EvalSymlinks(abs)
		switch {
		case err == nil:
			startDir = resolved
		case errors.Is(err, os.ErrNotExist):
			startDir = abs
		default:
			return Scope{}, fmt.Errorf("resolve symlinks: %w", err)
		}
	}

	root, err := DetectProjectRoot(startDir)
	if err != nil {
		return Scope{}, err
	}
	return Scope{Project: root}, nil
}

func DetectProjectRoot(startDir string) (string, error) {
	dir := startDir
	for {
		gitPath := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			if hasAgentDir(dir) {
				return dir, nil
			}
			return "", fmt.Errorf("git repo at %q has no .claude/ or .codex/ directory", dir)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("no git repository found from %q", startDir)
}

func hasAgentDir(dir string) bool {
	for _, name := range []string{".claude", ".codex"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err == nil && info.IsDir() {
			return true
		}
	}
	return false
}
