package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sethdeckard/loadout/internal/domain"
	"github.com/sethdeckard/loadout/internal/fsx"
	"github.com/sethdeckard/loadout/internal/skillmd"
)

// Marker is the metadata written to .loadout inside each installed skill directory.
type Marker struct {
	RepoCommit  string    `json:"repo_commit"`
	InstalledAt time.Time `json:"installed_at"`
}

// Stage copies a skill from <repoPath>/<skill.Path> into destDir and rewrites
// destDir/SKILL.md with target-specific YAML frontmatter. It does not write
// the .loadout marker, does not perform atomic rename, and does not interact
// with any target root. The caller decides where destDir is and whether/how
// it becomes a final install or an archive subdirectory.
func Stage(repoPath string, skill domain.Skill, target domain.Target, destDir string) error {
	if !skill.SupportsTarget(target) {
		return fmt.Errorf("%w: skill %q does not support target %q", domain.ErrUnsupportedTarget, skill.Name, target)
	}

	srcDir := filepath.Join(repoPath, skill.Path)
	if !fsx.DirExists(srcDir) {
		return fmt.Errorf("%w: source directory %q does not exist", domain.ErrSkillNotFound, srcDir)
	}

	if err := fsx.CopyDir(srcDir, destDir); err != nil {
		return fmt.Errorf("copy skill: %w", err)
	}

	if err := transformSkillMD(destDir, skill, target); err != nil {
		return fmt.Errorf("transform SKILL.md: %w", err)
	}

	return nil
}

// Install copies a skill from the repo into the target root directory.
// Uses atomic rename: stages to a temp dir on the same filesystem first,
// writes the .loadout marker there, then renames into place.
func Install(repoPath string, skill domain.Skill, target domain.Target, targetRoot string, commit string) error {
	if err := fsx.EnsureDir(targetRoot); err != nil {
		return fmt.Errorf("ensure target root: %w", err)
	}

	finalDir := filepath.Join(targetRoot, string(skill.Name))

	tmpDir, err := os.MkdirTemp(targetRoot, ".tmp-install-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	stagingDir := filepath.Join(tmpDir, string(skill.Name))
	if err := Stage(repoPath, skill, target, stagingDir); err != nil {
		return err
	}

	marker := Marker{
		RepoCommit:  commit,
		InstalledAt: time.Now().UTC().Truncate(time.Second),
	}
	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal marker: %w", err)
	}
	if err := os.WriteFile(filepath.Join(stagingDir, fsx.MarkerFile), data, 0o644); err != nil {
		return fmt.Errorf("write marker: %w", err)
	}

	if fsx.DirExists(finalDir) {
		if !HasMarker(skill.Name, targetRoot) {
			return fmt.Errorf("%w: %s", domain.ErrUnmanagedDir, finalDir)
		}
		if err := os.RemoveAll(finalDir); err != nil {
			return fmt.Errorf("remove existing: %w", err)
		}
	}

	if err := os.Rename(stagingDir, finalDir); err != nil {
		return fmt.Errorf("rename into place: %w", err)
	}

	return nil
}

// Remove deletes a skill from the target root directory.
// Succeeds even if the skill is not installed.
func Remove(skillName domain.SkillName, targetRoot string) error {
	dir := filepath.Join(targetRoot, string(skillName))
	if !fsx.DirExists(dir) {
		return nil
	}
	if !HasMarker(skillName, targetRoot) {
		return fmt.Errorf("%w: %s", domain.ErrUnmanagedDir, dir)
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove skill %q: %w", skillName, err)
	}
	return nil
}

// IsInstalled returns true if the skill directory exists in the target root.
func IsInstalled(skillName domain.SkillName, targetRoot string) bool {
	return fsx.DirExists(filepath.Join(targetRoot, string(skillName)))
}

// transformSkillMD reads the SKILL.md in stagingDir, prepends target-specific
// YAML frontmatter, and writes it back.
func transformSkillMD(stagingDir string, skill domain.Skill, target domain.Target) error {
	mdPath := filepath.Join(stagingDir, "SKILL.md")
	body, err := os.ReadFile(mdPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no SKILL.md to transform
		}
		return err
	}

	fm := skillmd.BuildFrontmatter(skill, target)
	stripped := skillmd.StripFrontmatter(string(body))
	content := fmt.Sprintf("---\n%s---\n\n%s", fm, stripped)

	return os.WriteFile(mdPath, []byte(content), 0o644)
}

// HasMarker returns true if the skill directory contains a .loadout marker.
func HasMarker(skillName domain.SkillName, targetRoot string) bool {
	p := filepath.Join(targetRoot, string(skillName), fsx.MarkerFile)
	_, err := os.Stat(p)
	return err == nil
}

// ReadMarker reads and parses the .loadout marker from a skill directory.
func ReadMarker(skillName domain.SkillName, targetRoot string) (Marker, error) {
	p := filepath.Join(targetRoot, string(skillName), fsx.MarkerFile)
	data, err := os.ReadFile(p)
	if err != nil {
		return Marker{}, err
	}
	var m Marker
	if err := json.Unmarshal(data, &m); err != nil {
		return Marker{}, fmt.Errorf("parse marker: %w", err)
	}
	return m, nil
}

// ScanManaged returns the names of subdirectories that contain a .loadout marker.
func ScanManaged(targetRoot string) []domain.SkillName {
	entries, err := os.ReadDir(targetRoot)
	if err != nil {
		return nil
	}
	var names []domain.SkillName
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		markerPath := filepath.Join(targetRoot, e.Name(), fsx.MarkerFile)
		if _, err := os.Stat(markerPath); err == nil {
			names = append(names, domain.SkillName(e.Name()))
		}
	}
	return names
}
