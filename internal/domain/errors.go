package domain

import "errors"

var (
	ErrSkillNotFound     = errors.New("skill not found")
	ErrSkillInstalled    = errors.New("skill installed")
	ErrInvalidSkill      = errors.New("invalid skill")
	ErrSkillExists       = errors.New("skill already exists")
	ErrImportConflict    = errors.New("import conflict")
	ErrUnsupportedTarget = errors.New("unsupported target")
	ErrTargetDisabled    = errors.New("target disabled")
	ErrRepoNotFound      = errors.New("repo not found")
	ErrConfigNotFound    = errors.New("config not found")
)
