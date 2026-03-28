package domain

type SkillName string

type Skill struct {
	Name        SkillName      `json:"name"`
	Description string         `json:"description"`
	Tags        []string       `json:"tags"`
	Targets     []Target       `json:"targets"`
	Claude      map[string]any `json:"claude,omitempty"`
	Codex       map[string]any `json:"codex,omitempty"`
	Path        string         `json:"-"`
}

func (s Skill) TargetMeta(t Target) map[string]any {
	switch t {
	case TargetClaude:
		return s.Claude
	case TargetCodex:
		return s.Codex
	default:
		return nil
	}
}

func (s Skill) SupportsTarget(t Target) bool {
	for _, st := range s.Targets {
		if st == t {
			return true
		}
	}
	return false
}
