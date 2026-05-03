package fsx

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileAtomic writes data to path atomically by writing to a temp file in
// the same directory and renaming. The caller is responsible for any
// format-specific marshaling.
func WriteFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}

// MarkerFile is the name of the marker file written by install into each
// managed skill directory.
const MarkerFile = ".loadout"

// DirExists returns true if path exists and is a directory.
func DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// Exists returns true if path exists (file or directory).
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// EnsureDir creates the directory at path if it doesn't exist.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

// HomeOrRoot returns the user's home directory, or the filesystem root
// if the home directory cannot be determined.
func HomeOrRoot() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return string(filepath.Separator)
}

// ListDirs returns the names of immediate subdirectories in path.
func ListDirs(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && e.Name()[0] != '.' {
			dirs = append(dirs, e.Name())
		}
	}
	return dirs, nil
}
