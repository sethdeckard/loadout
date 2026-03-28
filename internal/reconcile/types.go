package reconcile

import "github.com/sethdeckard/loadout/internal/domain"

type StatusFlag string

const (
	StatusCurrent         StatusFlag = "current"
	StatusInactive        StatusFlag = "inactive"
	StatusMissingFromRepo StatusFlag = "missing from repo"
	StatusUnmanaged       StatusFlag = "unmanaged"
)

type SkillStatus struct {
	Skill           domain.Skill
	InstalledClaude bool
	InstalledCodex  bool
	ManagedClaude   bool
	ManagedCodex    bool
	Flags           []StatusFlag
}

// ActualState describes what is currently installed on disk.
type ActualState struct {
	Claude  map[domain.SkillName]bool // skill name -> is installed
	Codex   map[domain.SkillName]bool
	Managed map[domain.SkillName]map[domain.Target]bool // skill name -> target -> has marker
}

func NewActualState() ActualState {
	return ActualState{
		Claude:  make(map[domain.SkillName]bool),
		Codex:   make(map[domain.SkillName]bool),
		Managed: make(map[domain.SkillName]map[domain.Target]bool),
	}
}

func (a ActualState) IsInstalled(name domain.SkillName, target domain.Target) bool {
	switch target {
	case domain.TargetClaude:
		return a.Claude[name]
	case domain.TargetCodex:
		return a.Codex[name]
	default:
		return false
	}
}

func (a ActualState) IsManaged(name domain.SkillName, target domain.Target) bool {
	if a.Managed == nil {
		return false
	}
	tm := a.Managed[name]
	if tm == nil {
		return false
	}
	return tm[target]
}

func (a ActualState) SetManaged(name domain.SkillName, target domain.Target) {
	if a.Managed[name] == nil {
		a.Managed[name] = make(map[domain.Target]bool)
	}
	a.Managed[name][target] = true
}
