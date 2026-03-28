package domain

import (
	"errors"
	"testing"
)

func TestValidateSkillName(t *testing.T) {
	tests := []struct {
		name    string
		skill   SkillName
		wantErr bool
	}{
		{"valid simple", "swift-refactor", false},
		{"valid single char", "a", false},
		{"valid single digit", "3", false},
		{"valid with numbers", "api-v2-docs", false},
		{"valid two chars", "ab", false},
		{"valid starts with number", "3d-printing", false},
		{"valid numeric prefix", "7zip-helper", false},
		{"empty", "", true},
		{"starts with hyphen", "-bad", true},
		{"ends with hyphen", "bad-", true},
		{"uppercase", "Bad", true},
		{"spaces", "bad name", true},
		{"underscores", "bad_name", true},
		{"dots", "bad.name", true},
		{"exactly 64 chars", SkillName("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), false},
		{"65 chars", SkillName("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSkillName(tt.skill)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSkillName(%q) error = %v, wantErr %v", tt.skill, err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, ErrInvalidSkill) {
				t.Errorf("ValidateSkillName(%q) error should wrap ErrInvalidSkill", tt.skill)
			}
		})
	}
}

func TestValidateSkill(t *testing.T) {
	valid := Skill{
		Name:    "swift-refactor",
		Targets: []Target{TargetClaude},
	}

	tests := []struct {
		name    string
		modify  func(Skill) Skill
		wantErr bool
	}{
		{"valid", func(s Skill) Skill { return s }, false},
		{"both targets", func(s Skill) Skill { s.Targets = []Target{TargetClaude, TargetCodex}; return s }, false},
		{"empty name", func(s Skill) Skill { s.Name = ""; return s }, true},
		{"invalid name", func(s Skill) Skill { s.Name = "Bad"; return s }, true},
		{"no targets", func(s Skill) Skill { s.Targets = nil; return s }, true},
		{"bad target", func(s Skill) Skill { s.Targets = []Target{"vim"}; return s }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSkill(tt.modify(valid))
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSkill() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseTarget(t *testing.T) {
	tests := []struct {
		input   string
		want    Target
		wantErr bool
	}{
		{"claude", TargetClaude, false},
		{"codex", TargetCodex, false},
		{"vim", "", true},
		{"", "", true},
		{"Claude", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTarget(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTarget(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseTarget(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestAllTargets(t *testing.T) {
	targets := AllTargets()
	if len(targets) != 2 {
		t.Fatalf("AllTargets() returned %d targets, want 2", len(targets))
	}
	if targets[0] != TargetClaude || targets[1] != TargetCodex {
		t.Errorf("AllTargets() = %v, want [claude codex]", targets)
	}
}

func TestSkillSupportsTarget(t *testing.T) {
	s := Skill{Targets: []Target{TargetClaude}}
	if !s.SupportsTarget(TargetClaude) {
		t.Error("expected SupportsTarget(claude) = true")
	}
	if s.SupportsTarget(TargetCodex) {
		t.Error("expected SupportsTarget(codex) = false")
	}
}
