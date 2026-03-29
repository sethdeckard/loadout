package importer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/sethdeckard/loadout/internal/config"
	"github.com/sethdeckard/loadout/internal/domain"
	"github.com/sethdeckard/loadout/internal/fsx"
	"github.com/sethdeckard/loadout/internal/skillmd"
)

type ImportParams struct {
	SourceDir string
	RepoPath  string
	Targets   []domain.Target
}

type ImportResult struct {
	Skill   domain.Skill
	DestDir string
}

type Preview struct {
	Skill     domain.Skill
	Markdown  string
	SourceDir string
	Ready     bool
	Problem   string
}

type Candidate struct {
	SkillName  domain.SkillName
	SourceDir  string
	Targets    []domain.Target
	Ready      bool
	Problem    string
	Duplicate  bool
	FromRoots  []domain.Target
	Orphan     bool // source directory contains a .loadout marker (recovery candidate)
	sourceDirs []string
}

type preparedImport struct {
	skill        domain.Skill
	markdown     string
	normalized   []byte
	skillJSON    []byte
	sourceTarget *domain.Target
	sourceDir    string
	existingJSON bool
	managed      bool  // source directory has a .loadout marker
	prepareErr   error // non-nil if prepareImport failed for this entry
}

func Import(params ImportParams) (ImportResult, error) {
	prepared, err := prepareImport(params.SourceDir, params.Targets)
	if err != nil {
		return ImportResult{}, err
	}
	prepared, err = applyTargets(prepared, params.Targets)
	if err != nil {
		return ImportResult{}, err
	}

	destDir := filepath.Join(params.RepoPath, "skills", string(prepared.skill.Name))
	if fsx.DirExists(destDir) {
		return ImportResult{}, fmt.Errorf("%w: %q", domain.ErrSkillExists, prepared.skill.Name)
	}

	if err := fsx.EnsureDir(filepath.Dir(destDir)); err != nil {
		return ImportResult{}, fmt.Errorf("ensure skills dir: %w", err)
	}
	if err := fsx.CopyDir(params.SourceDir, destDir); err != nil {
		return ImportResult{}, fmt.Errorf("copy skill: %w", err)
	}
	if err := os.Remove(filepath.Join(destDir, fsx.MarkerFile)); err != nil && !os.IsNotExist(err) {
		return ImportResult{}, fmt.Errorf("remove marker: %w", err)
	}
	if err := os.WriteFile(filepath.Join(destDir, "SKILL.md"), []byte(prepared.markdown), 0o644); err != nil {
		return ImportResult{}, fmt.Errorf("write SKILL.md: %w", err)
	}
	if err := os.WriteFile(filepath.Join(destDir, "skill.json"), prepared.skillJSON, 0o644); err != nil {
		return ImportResult{}, fmt.Errorf("write skill.json: %w", err)
	}

	return ImportResult{Skill: prepared.skill, DestDir: destDir}, nil
}

func PreviewSourceDir(sourceDir string, targets []domain.Target) (Preview, error) {
	prepared, err := prepareImport(sourceDir, targets)
	if err != nil {
		return Preview{}, err
	}
	prepared, err = applyTargets(prepared, targets)
	if err != nil {
		return Preview{}, err
	}
	return Preview{
		Skill:     prepared.skill,
		Markdown:  prepared.markdown,
		SourceDir: prepared.sourceDir,
		Ready:     true,
	}, nil
}

func DiscoverCandidates(targets config.TargetPaths) ([]Candidate, error) {
	byName := make(map[domain.SkillName][]preparedImport)
	for _, target := range targets.ConfiguredTargets() {
		root := targets.Path(target)
		scanned, err := scanRoot(root, target)
		if err != nil {
			return nil, err
		}
		for name, entries := range scanned {
			byName[name] = append(byName[name], entries...)
		}
	}
	return mergeCandidates(byName, false), nil
}

func DiscoverCandidatesInDir(dir string, configuredTargets []domain.Target) ([]Candidate, error) {
	byName := make(map[domain.SkillName][]preparedImport)

	claudeSkills := filepath.Join(dir, ".claude", "skills")
	codexSkills := filepath.Join(dir, ".codex", "skills")
	repoSkills := filepath.Join(dir, "skills")
	claudeExists := fsx.DirExists(claudeSkills)
	codexExists := fsx.DirExists(codexSkills)
	repoSkillsExists := fsx.DirExists(repoSkills)

	switch {
	case claudeExists || codexExists:
		if claudeExists {
			scanned, err := scanRoot(claudeSkills, domain.TargetClaude)
			if err != nil {
				return nil, err
			}
			for name, entries := range scanned {
				byName[name] = append(byName[name], entries...)
			}
		}
		if codexExists {
			scanned, err := scanRoot(codexSkills, domain.TargetCodex)
			if err != nil {
				return nil, err
			}
			for name, entries := range scanned {
				byName[name] = append(byName[name], entries...)
			}
		}
	case repoSkillsExists:
		scanned, err := scanRootMultiTarget(repoSkills, configuredTargets)
		if err != nil {
			return nil, err
		}
		for name, entries := range scanned {
			byName[name] = append(byName[name], entries...)
		}
	default:
		scanned, err := scanRootMultiTarget(dir, configuredTargets)
		if err != nil {
			return nil, err
		}
		for name, entries := range scanned {
			byName[name] = append(byName[name], entries...)
		}
	}

	return mergeCandidates(byName, true), nil
}

func scanRoot(root string, target domain.Target) (map[domain.SkillName][]preparedImport, error) {
	byName := make(map[domain.SkillName][]preparedImport)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return byName, nil
		}
		return nil, fmt.Errorf("scan %s root: %w", target, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
			continue
		}
		prepared, err := prepareImport(dir, []domain.Target{target})
		if err != nil {
			name := domain.SkillName(slugify(entry.Name()))
			byName[name] = append(byName[name], preparedImport{
				skill:        domain.Skill{Name: name, Targets: []domain.Target{target}},
				sourceDir:    dir,
				sourceTarget: &target,
				normalized:   []byte(err.Error()),
				managed:      hasMarker(dir),
				prepareErr:   err,
			})
			continue
		}
		prepared.sourceTarget = &target
		prepared.managed = hasMarker(dir)
		byName[prepared.skill.Name] = append(byName[prepared.skill.Name], prepared)
	}
	return byName, nil
}

func scanRootMultiTarget(root string, targets []domain.Target) (map[domain.SkillName][]preparedImport, error) {
	byName := make(map[domain.SkillName][]preparedImport)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return byName, nil
		}
		return nil, fmt.Errorf("scan root: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
			continue
		}
		prepared, err := prepareImport(dir, targets)
		if err != nil {
			name := domain.SkillName(slugify(entry.Name()))
			byName[name] = append(byName[name], preparedImport{
				skill:      domain.Skill{Name: name, Targets: targets},
				sourceDir:  dir,
				normalized: []byte(err.Error()),
				managed:    hasMarker(dir),
				prepareErr: err,
			})
			continue
		}
		prepared.managed = hasMarker(dir)
		byName[prepared.skill.Name] = append(byName[prepared.skill.Name], prepared)
	}
	return byName, nil
}

func mergeCandidates(byName map[domain.SkillName][]preparedImport, allowTargetMetadataDiff bool) []Candidate {
	var candidates []Candidate
	for name, entries := range byName {
		candidate := Candidate{
			SkillName: name,
			Ready:     true,
		}
		targetSet := make(map[domain.Target]bool)
		rootSet := make(map[domain.Target]bool)
		for _, entry := range entries {
			candidate.sourceDirs = append(candidate.sourceDirs, entry.sourceDir)
			if entry.sourceTarget != nil {
				rootSet[*entry.sourceTarget] = true
			}
			for _, target := range entry.skill.Targets {
				targetSet[target] = true
			}
		}
		for target := range rootSet {
			candidate.FromRoots = append(candidate.FromRoots, target)
		}
		for target := range targetSet {
			candidate.Targets = append(candidate.Targets, target)
		}
		sort.Slice(candidate.FromRoots, func(i, j int) bool { return candidate.FromRoots[i] < candidate.FromRoots[j] })
		sort.Slice(candidate.Targets, func(i, j int) bool { return candidate.Targets[i] < candidate.Targets[j] })
		candidate.Duplicate = len(entries) > 1
		candidate.SourceDir = entries[0].sourceDir
		for _, entry := range entries {
			if entry.managed {
				candidate.Orphan = true
				break
			}
		}

		for _, entry := range entries {
			if entry.prepareErr != nil {
				candidate.Ready = false
				candidate.Problem = entry.prepareErr.Error()
				break
			}
		}

		if candidate.Ready && len(entries) > 1 {
			for i := 1; i < len(entries); i++ {
				if !preparedEqual(entries[0], entries[i], allowTargetMetadataDiff) {
					candidate.Ready = false
					candidate.Problem = "conflicting local copies in enabled target roots"
					break
				}
			}
		}

		candidates = append(candidates, candidate)
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].SkillName < candidates[j].SkillName
	})
	return candidates
}

func preparedEqual(a, b preparedImport, allowTargetMetadataDiff bool) bool {
	if allowTargetMetadataDiff {
		return bytes.Equal(comparableNormalized(a), comparableNormalized(b))
	}
	return bytes.Equal(a.normalized, b.normalized)
}

func comparableNormalized(entry preparedImport) []byte {
	skill := entry.skill
	skill.Targets = nil
	skill.Claude = nil
	skill.Codex = nil
	skillJSON, err := json.MarshalIndent(skill, "", "  ")
	if err != nil {
		return entry.normalized
	}
	skillJSON = append(skillJSON, '\n')
	normalized, err := snapshotNormalized(entry.sourceDir, entry.markdown, skillJSON)
	if err != nil {
		return entry.normalized
	}
	return normalized
}

func applyTargets(prepared preparedImport, targets []domain.Target) (preparedImport, error) {
	if len(targets) == 0 {
		return prepared, nil
	}
	prepared.skill.Targets = normalizeTargets(targets)
	skillJSON, err := json.MarshalIndent(prepared.skill, "", "  ")
	if err != nil {
		return preparedImport{}, fmt.Errorf("marshal skill.json: %w", err)
	}
	skillJSON = append(skillJSON, '\n')
	prepared.skillJSON = skillJSON
	return prepared, nil
}

func normalizeTargets(targets []domain.Target) []domain.Target {
	seen := make(map[domain.Target]bool)
	normalized := make([]domain.Target, 0, len(targets))
	for _, target := range targets {
		if seen[target] {
			continue
		}
		seen[target] = true
		normalized = append(normalized, target)
	}
	sort.Slice(normalized, func(i, j int) bool { return normalized[i] < normalized[j] })
	return normalized
}

func hasMarker(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, fsx.MarkerFile))
	return err == nil
}

func rejectSymlinks(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%w: %s", domain.ErrSymlinkInTree, path)
		}
		return nil
	})
}

func prepareImport(sourceDir string, targets []domain.Target) (preparedImport, error) {
	if err := rejectSymlinks(sourceDir); err != nil {
		return preparedImport{}, err
	}

	mdPath := filepath.Join(sourceDir, "SKILL.md")
	mdBytes, err := os.ReadFile(mdPath)
	if err != nil {
		if os.IsNotExist(err) {
			return preparedImport{}, fmt.Errorf("%w: missing SKILL.md", domain.ErrInvalidSkill)
		}
		return preparedImport{}, fmt.Errorf("read SKILL.md: %w", err)
	}
	content := string(mdBytes)
	parsed := skillmd.Parse(content)
	body := parsed.Body

	jsonPath := filepath.Join(sourceDir, "skill.json")
	var skill domain.Skill
	existingJSON := false
	if data, err := os.ReadFile(jsonPath); err == nil {
		existingJSON = true
		if err := json.Unmarshal(data, &skill); err != nil {
			return preparedImport{}, fmt.Errorf("%w: invalid skill.json: %w", domain.ErrInvalidSkill, err)
		}
		if skill.Name == "" {
			skill.Name = domain.SkillName(slugify(filepath.Base(sourceDir)))
		}
	} else if !os.IsNotExist(err) {
		return preparedImport{}, fmt.Errorf("read skill.json: %w", err)
	} else {
		skill, err = inferSkill(sourceDir, parsed, targets)
		if err != nil {
			return preparedImport{}, err
		}
	}

	if err := domain.ValidateSkill(skill); err != nil {
		return preparedImport{}, err
	}

	skillJSON, err := json.MarshalIndent(skill, "", "  ")
	if err != nil {
		return preparedImport{}, fmt.Errorf("marshal skill.json: %w", err)
	}
	skillJSON = append(skillJSON, '\n')

	normalized, err := snapshotNormalized(sourceDir, body, skillJSON)
	if err != nil {
		return preparedImport{}, err
	}

	return preparedImport{
		skill:        skill,
		markdown:     body,
		normalized:   normalized,
		skillJSON:    skillJSON,
		sourceDir:    sourceDir,
		existingJSON: existingJSON,
	}, nil
}

func inferSkill(sourceDir string, parsed skillmd.Parsed, targets []domain.Target) (domain.Skill, error) {
	name := domain.SkillName(slugify(filepath.Base(sourceDir)))
	if err := domain.ValidateSkillName(name); err != nil {
		return domain.Skill{}, err
	}
	inferredTargets := append([]domain.Target(nil), targets...)
	if len(inferredTargets) == 0 {
		return domain.Skill{}, fmt.Errorf("%w: explicit targets required", domain.ErrInvalidSkill)
	}

	description := parsed.Fields["description"]
	meta := map[string]any{}
	for key, value := range parsed.Fields {
		if key == "name" || key == "description" {
			continue
		}
		meta[key] = parseScalar(value)
	}

	skill := domain.Skill{
		Name:        name,
		Description: description,
		Targets:     inferredTargets,
	}
	if len(meta) > 0 && len(inferredTargets) == 1 {
		if inferredTargets[0] == domain.TargetClaude {
			skill.Claude = meta
		}
		if inferredTargets[0] == domain.TargetCodex {
			skill.Codex = meta
		}
	}
	return skill, nil
}

func snapshotNormalized(sourceDir, markdown string, skillJSON []byte) ([]byte, error) {
	files := make(map[string][]byte)
	err := filepath.WalkDir(sourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("snapshot: %w: %s", domain.ErrSymlinkInTree, path)
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		switch rel {
		case fsx.MarkerFile:
			return nil
		case "SKILL.md":
			files[rel] = []byte(markdown)
			return nil
		case "skill.json":
			files[rel] = skillJSON
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[rel] = data
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("snapshot source: %w", err)
	}

	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b bytes.Buffer
	for _, key := range keys {
		b.WriteString("== " + key + " ==\n")
		b.Write(files[key])
		b.WriteString("\n")
	}
	return b.Bytes(), nil
}

var invalidNameChars = regexp.MustCompile(`[^a-z0-9-]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	s = invalidNameChars.ReplaceAllString(s, "")
	s = strings.Trim(s, "-")
	if s == "" {
		return "imported-skill"
	}
	if len(s) > 64 {
		s = strings.TrimRight(s[:64], "-")
	}
	return s
}

func parseScalar(value string) any {
	if b, err := strconv.ParseBool(value); err == nil {
		return b
	}
	if i, err := strconv.Atoi(value); err == nil {
		return i
	}
	return value
}
