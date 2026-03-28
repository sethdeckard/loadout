package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShortenHomePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Fatalf("UserHomeDir() error = %v, home = %q", err, home)
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{"empty", "", ""},
		{"exact home", home, "~"},
		{"under home", filepath.Join(home, "projects"), "~/projects"},
		{"not under home", filepath.Join(os.TempDir(), "other"), filepath.Join(os.TempDir(), "other")},
		{"home prefix but no separator", home + "extra", home + "extra"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortenHomePath(tt.path)
			if got != tt.want {
				t.Errorf("shortenHomePath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
