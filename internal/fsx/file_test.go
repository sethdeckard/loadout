package fsx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHomeOrRoot(t *testing.T) {
	got := HomeOrRoot()
	if got == "" {
		t.Fatal("HomeOrRoot() returned empty string")
	}

	// On a normal system, should return the home directory.
	home, err := os.UserHomeDir()
	if err == nil && got != home {
		t.Errorf("HomeOrRoot() = %q, want %q", got, home)
	}

	// Result should always be a valid absolute path or the separator root.
	if !filepath.IsAbs(got) && got != string(filepath.Separator) {
		t.Errorf("HomeOrRoot() = %q, want absolute path or root", got)
	}
}
