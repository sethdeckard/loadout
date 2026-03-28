package cmd

import (
	"testing"

	"github.com/sethdeckard/loadout/internal/domain"
)

func TestParseImportTargets(t *testing.T) {
	targets, err := parseImportTargets("claude,codex,claude")
	if err != nil {
		t.Fatalf("parseImportTargets() error = %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("len(targets) = %d, want 2", len(targets))
	}
	if targets[0] != domain.TargetClaude || targets[1] != domain.TargetCodex {
		t.Fatalf("targets = %v", targets)
	}
}

func TestParseImportTargets_Empty(t *testing.T) {
	targets, err := parseImportTargets("")
	if err != nil {
		t.Fatalf("parseImportTargets() error = %v", err)
	}
	if targets != nil {
		t.Fatalf("targets = %v, want nil", targets)
	}
}
