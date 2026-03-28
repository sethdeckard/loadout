package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/sethdeckard/loadout/internal/domain"
	"github.com/sethdeckard/loadout/internal/fsx"
)

const skillsDir = "skills"

// Load scans the skills/ directory in repoPath and returns all valid skills.
func Load(repoPath string) ([]domain.Skill, error) {
	root := filepath.Join(repoPath, skillsDir)
	if !fsx.DirExists(root) {
		return nil, fmt.Errorf("%w: no skills/ directory in %q", domain.ErrRepoNotFound, repoPath)
	}

	dirs, err := fsx.ListDirs(root)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}

	seen := make(map[domain.SkillName]string)
	var skills []domain.Skill

	for _, dir := range dirs {
		skill, err := loadSkill(root, dir)
		if err != nil {
			if errors.Is(err, errSkipSkillDir) {
				continue
			}
			return nil, fmt.Errorf("skill %q: %w", dir, err)
		}

		if prev, ok := seen[skill.Name]; ok {
			return nil, fmt.Errorf("%w: duplicate name %q in %q and %q", domain.ErrInvalidSkill, skill.Name, prev, dir)
		}
		seen[skill.Name] = dir
		skills = append(skills, skill)
	}

	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})
	return skills, nil
}

var errSkipSkillDir = errors.New("skip skill dir")

// LoadOne loads a single skill by name from the repo.
func LoadOne(repoPath string, name domain.SkillName) (domain.Skill, error) {
	skills, err := Load(repoPath)
	if err != nil {
		return domain.Skill{}, err
	}
	for _, s := range skills {
		if s.Name == name {
			return s, nil
		}
	}
	return domain.Skill{}, fmt.Errorf("%w: %q", domain.ErrSkillNotFound, name)
}

// ReadSkillMarkdown returns the contents of SKILL.md for the given skill.
func ReadSkillMarkdown(repoPath string, name domain.SkillName) (string, error) {
	skill, err := LoadOne(repoPath, name)
	if err != nil {
		return "", err
	}
	mdPath := filepath.Join(repoPath, skill.Path, "SKILL.md")
	data, err := os.ReadFile(mdPath)
	if err != nil {
		return "", fmt.Errorf("read SKILL.md: %w", err)
	}
	return string(data), nil
}

func loadSkill(root, dir string) (domain.Skill, error) {
	skillDir := filepath.Join(root, dir)

	mdPath := filepath.Join(skillDir, "SKILL.md")
	jsonPath := filepath.Join(skillDir, "skill.json")

	mdExists := fileExists(mdPath)
	jsonExists := fileExists(jsonPath)
	if !mdExists && !jsonExists {
		return domain.Skill{}, errSkipSkillDir
	}

	if !mdExists {
		return domain.Skill{}, fmt.Errorf("%w: missing SKILL.md", domain.ErrInvalidSkill)
	}

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.Skill{}, fmt.Errorf("%w: missing skill.json", domain.ErrInvalidSkill)
		}
		return domain.Skill{}, fmt.Errorf("read skill.json: %w", err)
	}

	var skill domain.Skill
	if err := json.Unmarshal(data, &skill); err != nil {
		return domain.Skill{}, fmt.Errorf("%w: invalid skill.json: %w", domain.ErrInvalidSkill, err)
	}

	if skill.Name == "" {
		skill.Name = domain.SkillName(dir)
	}
	skill.Path = filepath.Join(skillsDir, dir)

	if err := domain.ValidateSkill(skill); err != nil {
		return domain.Skill{}, err
	}

	return skill, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
