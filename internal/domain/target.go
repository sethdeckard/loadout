package domain

import "fmt"

type Target string

const (
	TargetClaude Target = "claude"
	TargetCodex  Target = "codex"
)

func AllTargets() []Target {
	return []Target{TargetClaude, TargetCodex}
}

func ParseTarget(s string) (Target, error) {
	switch Target(s) {
	case TargetClaude:
		return TargetClaude, nil
	case TargetCodex:
		return TargetCodex, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrUnsupportedTarget, s)
	}
}
