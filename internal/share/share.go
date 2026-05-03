// Package share builds portable archives of skills for handoff to other people.
//
// An archive is a .tar.gz containing claude-build/, codex-build/,
// loadout-source/, and README.md at the top level. *-build subdirs are
// drop-in installs for ~/.claude/skills/ and ~/.codex/skills/. loadout-source
// is the verbatim repo skill directory, suitable for `loadout import`.
package share

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/sethdeckard/loadout/internal/domain"
	"github.com/sethdeckard/loadout/internal/fsx"
	"github.com/sethdeckard/loadout/internal/install"
)

//go:embed README.tmpl
var readmeTemplate string

// Build packages skill into a tar.gz archive at outPath.
//
// repoPath is the registry root (containing skills/). skill is an
// already-resolved domain.Skill loaded from that repo. outPath is the final
// archive path; its parent directory must exist, and outPath itself must
// not. Build is responsible for archive-write safety only — path policy
// (defaults, --out semantics) lives in the caller.
func Build(repoPath string, skill domain.Skill, outPath string) error {
	if fsx.Exists(outPath) {
		return fmt.Errorf("output already exists: %s", outPath)
	}
	parent := filepath.Dir(outPath)
	if !fsx.DirExists(parent) {
		return fmt.Errorf("output parent directory does not exist: %s", parent)
	}

	staging, err := os.MkdirTemp("", "loadout-share-*")
	if err != nil {
		return fmt.Errorf("create staging dir: %w", err)
	}
	defer os.RemoveAll(staging)

	for _, target := range skill.Targets {
		buildDir := filepath.Join(staging, string(target)+"-build")
		if err := install.Stage(repoPath, skill, target, buildDir); err != nil {
			return fmt.Errorf("stage %s: %w", target, err)
		}
	}

	srcDir := filepath.Join(repoPath, skill.Path)
	loadoutDir := filepath.Join(staging, "loadout-source")
	if err := fsx.CopyDirFiltered(srcDir, loadoutDir, isJunk); err != nil {
		return fmt.Errorf("copy loadout-source: %w", err)
	}

	readme, err := renderReadme(skill)
	if err != nil {
		return fmt.Errorf("render README: %w", err)
	}
	if err := os.WriteFile(filepath.Join(staging, "README.md"), readme, 0o644); err != nil {
		return fmt.Errorf("write README: %w", err)
	}

	tempArchive, err := os.CreateTemp(parent, filepath.Base(outPath)+".tmp*")
	if err != nil {
		return fmt.Errorf("create temp archive: %w", err)
	}
	tempPath := tempArchive.Name()
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			os.Remove(tempPath)
		}
	}()

	if err := writeArchive(tempArchive, staging); err != nil {
		tempArchive.Close()
		return fmt.Errorf("write archive: %w", err)
	}
	if err := tempArchive.Close(); err != nil {
		return fmt.Errorf("close temp archive: %w", err)
	}

	if err := promoteNoReplace(tempPath, outPath); err != nil {
		return fmt.Errorf("promote archive: %w", err)
	}
	cleanupTemp = false
	return nil
}

// promoteNoReplace links src to dst and removes src. It returns an error if
// dst already exists. The implementation may use os.Link, renameat2, or any
// other primitive that provides no-overwrite semantics on its host platform;
// callers must not rely on a specific syscall.
func promoteNoReplace(src, dst string) error {
	if err := os.Link(src, dst); err != nil {
		return err
	}
	if err := os.Remove(src); err != nil {
		// dst is in place; leaving the temp file behind is harmless but
		// surface the error so the caller can decide.
		return fmt.Errorf("remove temp after link: %w", err)
	}
	return nil
}

// isJunk returns true for paths that should never appear in a shared archive,
// regardless of where they came from. rel is forward-slash relative.
func isJunk(rel string) bool {
	switch path.Base(rel) {
	case fsx.MarkerFile, ".DS_Store", "Thumbs.db":
		return true
	}
	for _, segment := range strings.Split(rel, "/") {
		if segment == ".git" {
			return true
		}
	}
	return false
}

type readmeData struct {
	Name        domain.SkillName
	Description string
	HasClaude   bool
	HasCodex    bool
}

func renderReadme(skill domain.Skill) ([]byte, error) {
	tmpl, err := template.New("readme").Parse(readmeTemplate)
	if err != nil {
		return nil, err
	}
	data := readmeData{
		Name:        skill.Name,
		Description: skill.Description,
		HasClaude:   skill.SupportsTarget(domain.TargetClaude),
		HasCodex:    skill.SupportsTarget(domain.TargetCodex),
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeArchive walks staging and writes its contents into a deterministic
// tar.gz on w. Lexical walk order, normalized timestamps, slash-separated
// header names, normalized owner metadata, preserved mode bits.
func writeArchive(w *os.File, staging string) error {
	gzw := gzip.NewWriter(w)
	gzw.ModTime = time.Time{}
	gzw.Name = ""

	tw := tar.NewWriter(gzw)

	walkErr := filepath.WalkDir(staging, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == staging {
			return nil
		}

		rel, err := filepath.Rel(staging, p)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)

		if isJunk(relSlash) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%w: %s", domain.ErrSymlinkInTree, p)
		}

		header := &tar.Header{
			Name:    relSlash,
			Mode:    int64(info.Mode().Perm()),
			ModTime: time.Time{},
			Format:  tar.FormatPAX,
		}
		if d.IsDir() {
			header.Typeflag = tar.TypeDir
			header.Name = relSlash + "/"
		} else {
			header.Typeflag = tar.TypeReg
			header.Size = info.Size()
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		f, err := os.Open(p)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tw, f)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if walkErr != nil {
		tw.Close()
		gzw.Close()
		return walkErr
	}

	if err := tw.Close(); err != nil {
		gzw.Close()
		return err
	}
	if err := gzw.Close(); err != nil {
		return err
	}
	return nil
}
