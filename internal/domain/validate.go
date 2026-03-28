package domain

import (
	"fmt"
	"regexp"
)

const maxSkillNameLen = 64

var validSkillName = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

func ValidateSkillName(name SkillName) error {
	if name == "" {
		return fmt.Errorf("%w: name is empty", ErrInvalidSkill)
	}
	if len(name) > maxSkillNameLen {
		return fmt.Errorf("%w: name %q exceeds %d characters", ErrInvalidSkill, name, maxSkillNameLen)
	}
	if !validSkillName.MatchString(string(name)) {
		return fmt.Errorf("%w: name %q must use lowercase letters, numbers, and hyphens only, must not start or end with a hyphen", ErrInvalidSkill, name)
	}
	return nil
}

func ValidateSkill(s Skill) error {
	if err := ValidateSkillName(s.Name); err != nil {
		return err
	}
	if len(s.Targets) == 0 {
		return fmt.Errorf("%w: no targets for skill %q", ErrInvalidSkill, s.Name)
	}
	for _, t := range s.Targets {
		if _, err := ParseTarget(string(t)); err != nil {
			return fmt.Errorf("%w: skill %q has %w", ErrInvalidSkill, s.Name, err)
		}
	}
	return nil
}
