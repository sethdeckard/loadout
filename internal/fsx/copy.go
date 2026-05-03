package fsx

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sethdeckard/loadout/internal/domain"
)

// CopyDir recursively copies src directory to dst.
// dst must not already exist.
func CopyDir(src, dst string) error {
	return CopyDirFiltered(src, dst, nil)
}

// CopyDirFiltered recursively copies src directory to dst, omitting any path
// for which skip returns true. The skip predicate receives the path relative
// to src, with forward-slash separators. If skip is nil, every entry is
// copied. dst must not already exist.
func CopyDirFiltered(src, dst string, skip func(relPath string) bool) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("copy dir: stat source: %w", err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("copy dir: source %q is not a directory", src)
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("copy dir: create dest: %w", err)
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		if skip != nil && rel != "." && skip(filepath.ToSlash(rel)) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		target := filepath.Join(dst, rel)

		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("copy dir: %w: %s", domain.ErrSymlinkInTree, path)
		}

		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(target, info.Mode())
		}

		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
