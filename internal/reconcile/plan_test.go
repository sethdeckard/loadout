package reconcile

import (
	"testing"

	"github.com/sethdeckard/loadout/internal/domain"
)

func skill(name string, targets ...domain.Target) domain.Skill { //nolint:unparam // name varies across tests
	return domain.Skill{Name: domain.SkillName(name), Targets: targets, Path: "skills/" + name}
}

func TestPlan_InstalledAndManaged(t *testing.T) {
	skills := []domain.Skill{skill("s1", domain.TargetClaude)}
	actual := NewActualState()
	actual.Claude["s1"] = true
	actual.SetManaged("s1", domain.TargetClaude)

	statuses := Plan(skills, actual)
	if len(statuses) != 1 || statuses[0].Flags[0] != StatusCurrent {
		t.Errorf("expected current status, got %+v", statuses)
	}
}

func TestPlan_NotInstalled(t *testing.T) {
	skills := []domain.Skill{skill("s1", domain.TargetClaude)}
	actual := NewActualState()

	statuses := Plan(skills, actual)
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	assertHasFlag(t, statuses, "s1", StatusInactive)
}

func TestPlan_MissingFromRepo_Managed(t *testing.T) {
	skills := []domain.Skill{} // empty registry
	actual := NewActualState()
	actual.Claude["gone"] = true
	actual.SetManaged("gone", domain.TargetClaude)

	statuses := Plan(skills, actual)
	assertHasFlag(t, statuses, "gone", StatusMissingFromRepo)
}

func TestPlan_Unmanaged(t *testing.T) {
	skills := []domain.Skill{}
	actual := NewActualState()
	actual.Claude["mystery"] = true
	// No marker → unmanaged

	statuses := Plan(skills, actual)
	assertHasFlag(t, statuses, "mystery", StatusUnmanaged)
}

func TestPlan_MultiTarget(t *testing.T) {
	skills := []domain.Skill{skill("s1", domain.TargetClaude, domain.TargetCodex)}
	actual := NewActualState()
	actual.Claude["s1"] = true
	actual.SetManaged("s1", domain.TargetClaude)
	// codex not installed

	statuses := Plan(skills, actual)
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	assertHasFlag(t, statuses, "s1", StatusCurrent)
	if !statuses[0].ManagedClaude {
		t.Error("expected ManagedClaude=true")
	}
	if statuses[0].ManagedCodex {
		t.Error("expected ManagedCodex=false")
	}
}

func TestPlan_Idempotent(t *testing.T) {
	skills := []domain.Skill{skill("s1", domain.TargetClaude)}
	actual := NewActualState()
	actual.Claude["s1"] = true
	actual.SetManaged("s1", domain.TargetClaude)

	statuses1 := Plan(skills, actual)
	statuses2 := Plan(skills, actual)

	if len(statuses1) != len(statuses2) {
		t.Error("expected same number of statuses")
	}
}

func TestPlan_UnmanagedCodexAlsoInClaude(t *testing.T) {
	skills := []domain.Skill{}
	actual := NewActualState()
	actual.Claude["mystery"] = true
	actual.Codex["mystery"] = true

	statuses := Plan(skills, actual)

	count := 0
	for _, s := range statuses {
		if s.Skill.Name == "mystery" {
			count++
			if !s.InstalledClaude {
				t.Error("expected InstalledClaude=true")
			}
			if !s.InstalledCodex {
				t.Error("expected InstalledCodex=true")
			}
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 status for mystery, got %d", count)
	}
	assertHasFlag(t, statuses, "mystery", StatusUnmanaged)
}

func TestPlan_UnmanagedCodexOnly(t *testing.T) {
	skills := []domain.Skill{}
	actual := NewActualState()
	actual.Codex["codex-only"] = true

	statuses := Plan(skills, actual)
	assertHasFlag(t, statuses, "codex-only", StatusUnmanaged)

	for _, s := range statuses {
		if s.Skill.Name == "codex-only" {
			if s.InstalledClaude {
				t.Error("expected InstalledClaude=false")
			}
			if !s.InstalledCodex {
				t.Error("expected InstalledCodex=true")
			}
		}
	}
}

func TestActualState_IsInstalled_UnknownTarget(t *testing.T) {
	a := NewActualState()
	a.Claude["s1"] = true
	if a.IsInstalled("s1", domain.Target("unknown")) {
		t.Error("expected false for unknown target")
	}
}

func TestActualState_IsInstalled_NilMaps(t *testing.T) {
	var a ActualState
	if a.IsInstalled("s1", domain.TargetClaude) {
		t.Error("expected false for nil Claude map")
	}
	if a.IsInstalled("s1", domain.TargetCodex) {
		t.Error("expected false for nil Codex map")
	}
}

func TestActualState_IsManaged_NilMaps(t *testing.T) {
	var a ActualState
	if a.IsManaged("s1", domain.TargetClaude) {
		t.Error("expected false for nil Managed map")
	}
}

func TestPlan_EmptyEverything(t *testing.T) {
	statuses := Plan(nil, NewActualState())
	if len(statuses) != 0 {
		t.Errorf("expected 0 statuses, got %d", len(statuses))
	}
}

func TestPlan_ManagedCodexMissingFromRepo(t *testing.T) {
	skills := []domain.Skill{}
	actual := NewActualState()
	actual.Codex["gone"] = true
	actual.SetManaged("gone", domain.TargetCodex)

	statuses := Plan(skills, actual)
	assertHasFlag(t, statuses, "gone", StatusMissingFromRepo)
}

func assertHasFlag(t *testing.T, statuses []SkillStatus, id string, flag StatusFlag) {
	t.Helper()
	for _, s := range statuses {
		if s.Skill.Name == domain.SkillName(id) {
			for _, f := range s.Flags {
				if f == flag {
					return
				}
			}
			t.Errorf("skill %q missing flag %q, has %v", id, flag, s.Flags)
			return
		}
	}
	t.Errorf("skill %q not found in statuses", id)
}
