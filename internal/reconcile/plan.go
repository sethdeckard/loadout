package reconcile

import "github.com/sethdeckard/loadout/internal/domain"

// Plan compares the registry and actual installed state (with marker info),
// returning the status of each skill. No operations are returned — the caller
// decides what to do based on statuses.
func Plan(skills []domain.Skill, actual ActualState) []SkillStatus {
	var statuses []SkillStatus

	// Index registry skills by name
	registry := make(map[domain.SkillName]bool)
	for _, s := range skills {
		registry[s.Name] = true
	}

	// Process each registry skill
	for _, skill := range skills {
		status := SkillStatus{
			Skill:           skill,
			InstalledClaude: actual.IsInstalled(skill.Name, domain.TargetClaude),
			InstalledCodex:  actual.IsInstalled(skill.Name, domain.TargetCodex),
			ManagedClaude:   actual.IsManaged(skill.Name, domain.TargetClaude),
			ManagedCodex:    actual.IsManaged(skill.Name, domain.TargetCodex),
		}

		if status.ManagedClaude || status.ManagedCodex {
			status.Flags = []StatusFlag{StatusCurrent}
		} else {
			status.Flags = []StatusFlag{StatusInactive}
		}

		statuses = append(statuses, status)
	}

	// Check for installed skills not in registry
	seen := make(map[domain.SkillName]bool)
	for id := range actual.Claude {
		if registry[id] {
			continue
		}
		managed := actual.IsManaged(id, domain.TargetClaude)
		flag := StatusUnmanaged
		if managed {
			flag = StatusMissingFromRepo
		}
		s := SkillStatus{
			Skill:           domain.Skill{Name: id},
			InstalledClaude: true,
			InstalledCodex:  actual.IsInstalled(id, domain.TargetCodex),
			ManagedClaude:   managed,
			ManagedCodex:    actual.IsManaged(id, domain.TargetCodex),
			Flags:           []StatusFlag{flag},
		}
		statuses = append(statuses, s)
		seen[id] = true
	}
	for id := range actual.Codex {
		if registry[id] {
			continue
		}
		if seen[id] {
			// Already added from claude loop — update codex fields
			for i := range statuses {
				if statuses[i].Skill.Name == id && !registry[id] {
					statuses[i].InstalledCodex = true
					statuses[i].ManagedCodex = actual.IsManaged(id, domain.TargetCodex)
					break
				}
			}
			continue
		}
		managed := actual.IsManaged(id, domain.TargetCodex)
		flag := StatusUnmanaged
		if managed {
			flag = StatusMissingFromRepo
		}
		statuses = append(statuses, SkillStatus{
			Skill:          domain.Skill{Name: id},
			InstalledCodex: true,
			ManagedCodex:   managed,
			Flags:          []StatusFlag{flag},
		})
	}

	return statuses
}
