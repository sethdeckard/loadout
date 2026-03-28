package registry

import (
	"errors"
	"testing"

	"github.com/sethdeckard/loadout/internal/domain"
)

func TestLoadValid(t *testing.T) {
	skills, err := Load("../../testdata/registry-valid")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("got %d skills, want 2", len(skills))
	}

	// Sorted by name
	if skills[0].Name != "rails-review" {
		t.Errorf("skills[0].Name = %q, want rails-review", skills[0].Name)
	}
	if skills[1].Name != "swift-refactor" {
		t.Errorf("skills[1].Name = %q, want swift-refactor", skills[1].Name)
	}

	// Check swift-refactor details
	sr := skills[1]
	if len(sr.Targets) != 2 {
		t.Errorf("Targets = %v, want 2", sr.Targets)
	}
	if sr.Path != "skills/swift-refactor" {
		t.Errorf("Path = %q", sr.Path)
	}
}

func TestLoadMissingSkillMD(t *testing.T) {
	_, err := Load("../../testdata/registry-missing-skill-md")
	if err == nil {
		t.Fatal("expected error for missing SKILL.md")
	}
	if !errors.Is(err, domain.ErrInvalidSkill) {
		t.Errorf("error should wrap ErrInvalidSkill, got: %v", err)
	}
}

func TestLoadMissingSkillJSON(t *testing.T) {
	_, err := Load("../../testdata/registry-missing-skill-json")
	if err == nil {
		t.Fatal("expected error for missing skill.json")
	}
	if !errors.Is(err, domain.ErrInvalidSkill) {
		t.Errorf("error should wrap ErrInvalidSkill, got: %v", err)
	}
}

func TestLoadIgnoresEmptySkillDir(t *testing.T) {
	skills, err := Load("../../testdata/registry-empty-dir")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	if skills[0].Name != "swift-refactor" {
		t.Fatalf("skills[0].Name = %q, want swift-refactor", skills[0].Name)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	_, err := Load("../../testdata/registry-invalid-json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !errors.Is(err, domain.ErrInvalidSkill) {
		t.Errorf("error should wrap ErrInvalidSkill, got: %v", err)
	}
}

func TestLoadDuplicateNames(t *testing.T) {
	_, err := Load("../../testdata/registry-duplicate-ids")
	if err == nil {
		t.Fatal("expected error for duplicate names")
	}
	if !errors.Is(err, domain.ErrInvalidSkill) {
		t.Errorf("error should wrap ErrInvalidSkill, got: %v", err)
	}
}

func TestLoadMissingRepo(t *testing.T) {
	_, err := Load("/nonexistent/repo")
	if err == nil {
		t.Fatal("expected error for missing repo")
	}
	if !errors.Is(err, domain.ErrRepoNotFound) {
		t.Errorf("error should wrap ErrRepoNotFound, got: %v", err)
	}
}

func TestLoadOne(t *testing.T) {
	skill, err := LoadOne("../../testdata/registry-valid", "swift-refactor")
	if err != nil {
		t.Fatalf("LoadOne() error = %v", err)
	}
	if skill.Name != "swift-refactor" {
		t.Errorf("Name = %q", skill.Name)
	}
}

func TestLoadOneNotFound(t *testing.T) {
	_, err := LoadOne("../../testdata/registry-valid", "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, domain.ErrSkillNotFound) {
		t.Errorf("error should wrap ErrSkillNotFound, got: %v", err)
	}
}

func TestReadSkillMarkdown(t *testing.T) {
	md, err := ReadSkillMarkdown("../../testdata/registry-valid", "swift-refactor")
	if err != nil {
		t.Fatalf("ReadSkillMarkdown() error = %v", err)
	}
	if md == "" {
		t.Error("expected non-empty markdown")
	}
}

func TestLoadOne_DirNameDiffersFromDeclaredName(t *testing.T) {
	skill, err := LoadOne("../../testdata/registry-name-mismatch", "declared-name")
	if err != nil {
		t.Fatalf("LoadOne() error = %v", err)
	}
	if skill.Name != "declared-name" {
		t.Errorf("Name = %q, want declared-name", skill.Name)
	}
	if skill.Path != "skills/dir-name" {
		t.Errorf("Path = %q, want skills/dir-name", skill.Path)
	}
}

func TestReadSkillMarkdown_DirNameDiffersFromDeclaredName(t *testing.T) {
	md, err := ReadSkillMarkdown("../../testdata/registry-name-mismatch", "declared-name")
	if err != nil {
		t.Fatalf("ReadSkillMarkdown() error = %v", err)
	}
	if md == "" {
		t.Error("expected non-empty markdown")
	}
}

func TestLoadDefaultsNameFromDir(t *testing.T) {
	skills, err := Load("../../testdata/registry-no-name")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	if skills[0].Name != "auto-named" {
		t.Errorf("Name = %q, want auto-named", skills[0].Name)
	}
}
