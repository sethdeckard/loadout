package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/sethdeckard/loadout/internal/config"
	"github.com/sethdeckard/loadout/internal/domain"
	"github.com/sethdeckard/loadout/internal/fsx"
	"github.com/sethdeckard/loadout/internal/gitrepo"
	"github.com/sethdeckard/loadout/internal/importer"
	"github.com/sethdeckard/loadout/internal/install"
	"github.com/sethdeckard/loadout/internal/reconcile"
	"github.com/sethdeckard/loadout/internal/registry"
	"github.com/sethdeckard/loadout/internal/skillmd"
)

type Service struct {
	Config config.Config
}

func New(cfg config.Config) *Service {
	return &Service{Config: cfg}
}

func (s *Service) ListSkills() ([]SkillView, error) {
	return s.listSkills("")
}

func (s *Service) ListSkillsForProject(projectRoot string) ([]SkillView, error) {
	views, err := s.listSkills(projectRoot)
	if err != nil {
		return nil, err
	}
	return s.appendReadyProjectImportViews(projectRoot, views)
}

func (s *Service) listSkills(projectRoot string) ([]SkillView, error) {
	skills, err := registry.Load(s.Config.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}

	actual := s.scanActual()
	projectActual := s.scanProjectActual(projectRoot)
	statuses := reconcile.Plan(skills, actual)

	var views []SkillView
	for _, status := range statuses {
		view := SkillView{
			Skill:           status.Skill,
			InstalledClaude: status.InstalledClaude,
			InstalledCodex:  status.InstalledCodex,
			ManagedClaude:   status.ManagedClaude,
			ManagedCodex:    status.ManagedCodex,
			ProjectClaude:   projectActual.Claude[status.Skill.Name],
			ProjectCodex:    projectActual.Codex[status.Skill.Name],
			Flags:           status.Flags,
		}
		if slices.Contains(status.Flags, reconcile.StatusMissingFromRepo) {
			enriched, root := s.enrichOrphanedSkill(status.Skill.Name)
			if root != "" {
				view.Skill = enriched
				view.Orphaned = true
				view.OrphanRoot = root
				view.LocalRoot = root
			}
		}
		if slices.Contains(status.Flags, reconcile.StatusUnmanaged) {
			view.LocalRoot = s.localRootForStatus(status)
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *Service) appendReadyProjectImportViews(projectRoot string, views []SkillView) ([]SkillView, error) {
	if projectRoot == "" {
		return views, nil
	}

	candidates, err := s.ListImportCandidatesFromDir(projectRoot)
	if err != nil {
		return nil, err
	}

	existing := make(map[domain.SkillName]bool, len(views))
	for _, view := range views {
		existing[view.Skill.Name] = true
	}

	for _, candidate := range candidates {
		if !candidate.Ready || existing[candidate.SkillName] {
			continue
		}
		views = append(views, SkillView{
			Skill: domain.Skill{
				Name:    candidate.SkillName,
				Targets: append([]domain.Target(nil), candidate.Targets...),
			},
			Flags:          []reconcile.StatusFlag{reconcile.StatusUnmanaged},
			LocalSourceDir: candidate.SourceDir,
		})
		existing[candidate.SkillName] = true
	}

	return views, nil
}

func (s *Service) PreviewSkill(name domain.SkillName) (SkillPreview, error) {
	skill, err := registry.LoadOne(s.Config.RepoPath, name)
	if err != nil {
		return SkillPreview{}, err
	}
	md, err := registry.ReadSkillMarkdown(s.Config.RepoPath, name)
	if err != nil {
		return SkillPreview{}, err
	}
	files := listExtraFiles(filepath.Join(s.Config.RepoPath, skill.Path))
	return SkillPreview{Skill: skill, Markdown: md, Files: files}, nil
}

func (s *Service) PreviewLocalSkill(name domain.SkillName, targetRoot string) (SkillPreview, error) {
	dir := filepath.Join(targetRoot, string(name))
	skill := loadLocalSkill(dir, name)
	md, _ := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	parsed := skillmd.Parse(string(md))
	files := listExtraFiles(dir)
	return SkillPreview{Skill: skill, Markdown: parsed.Body, Files: files}, nil
}

// enrichOrphanedSkill scans enabled roots in stable order (Claude, then Codex)
// and returns the first root containing the installed directory. It reads
// skill.json from that root to populate metadata. Falls back to inferred
// metadata from SKILL.md where recoverable.
func (s *Service) enrichOrphanedSkill(name domain.SkillName) (domain.Skill, string) {
	for _, target := range domain.AllTargets() {
		root := s.targetRoot(target)
		if root == "" {
			continue
		}
		dir := filepath.Join(root, string(name))
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		return loadLocalSkill(dir, name), root
	}
	return domain.Skill{Name: name}, ""
}

func loadLocalSkill(dir string, name domain.SkillName) domain.Skill {
	jsonPath := filepath.Join(dir, "skill.json")
	if data, err := os.ReadFile(jsonPath); err == nil {
		var skill domain.Skill
		if err := json.Unmarshal(data, &skill); err == nil {
			skill.Name = name
			return skill
		}
	}

	// Fall back to SKILL.md for recoverable metadata
	mdPath := filepath.Join(dir, "SKILL.md")
	mdBytes, err := os.ReadFile(mdPath)
	if err != nil {
		return domain.Skill{Name: name}
	}
	parsed := skillmd.Parse(string(mdBytes))
	return domain.Skill{
		Name:        name,
		Description: parsed.Fields["description"],
	}
}

func (s *Service) localRootForStatus(status reconcile.SkillStatus) string {
	if status.InstalledClaude {
		return s.targetRoot(domain.TargetClaude)
	}
	if status.InstalledCodex {
		return s.targetRoot(domain.TargetCodex)
	}
	return ""
}

func listExtraFiles(dir string) []string {
	var files []string
	skip := map[string]bool{"SKILL.md": true, "skill.json": true, fsx.MarkerFile: true}
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		if !skip[rel] {
			files = append(files, rel)
		}
		return nil
	})
	sort.Strings(files)
	return files
}

func (s *Service) PreviewImportSource(sourceDir string, targets []domain.Target) (ImportPreview, error) {
	preview, err := importer.PreviewSourceDir(sourceDir, targets)
	if err != nil {
		return ImportPreview{}, err
	}
	return ImportPreview{
		Skill:     preview.Skill,
		Markdown:  preview.Markdown,
		SourceDir: preview.SourceDir,
		Ready:     preview.Ready,
		Problem:   preview.Problem,
	}, nil
}

func (s *Service) DeleteSkillEligibility(name domain.SkillName, projectRoot string) (DeleteEligibility, error) {
	skill, err := registry.LoadOne(s.Config.RepoPath, name)
	if err != nil {
		return DeleteEligibility{}, err
	}

	eligibility := DeleteEligibility{
		SkillName:   name,
		DeletedPath: skill.Path,
		Deletable:   true,
	}

	for _, target := range domain.AllTargets() {
		root := s.targetRoot(target)
		if root == "" {
			continue
		}
		if install.HasMarker(name, root) {
			eligibility.Deletable = false
			eligibility.Blockers = append(eligibility.Blockers, fmt.Sprintf("managed user %s install exists", target))
		}
		if projectRoot != "" && install.HasMarker(name, projectTargetRoot(target, projectRoot)) {
			eligibility.Deletable = false
			eligibility.Blockers = append(eligibility.Blockers, fmt.Sprintf("managed project %s install exists", target))
		}
	}

	return eligibility, nil
}

func (s *Service) DeleteSkill(name domain.SkillName, projectRoot string, autoCommit bool) (DeleteSkillResult, error) {
	eligibility, err := s.DeleteSkillEligibility(name, projectRoot)
	if err != nil {
		return DeleteSkillResult{}, err
	}
	if !eligibility.Deletable {
		return DeleteSkillResult{}, fmt.Errorf("%w: %s", domain.ErrSkillInstalled, strings.Join(eligibility.Blockers, ", "))
	}
	if autoCommit && !gitrepo.IsRepo(s.Config.RepoPath) {
		return DeleteSkillResult{}, fmt.Errorf("%w: %q", domain.ErrRepoNotFound, s.Config.RepoPath)
	}

	wasTracked := autoCommit && gitrepo.HasTrackedPath(s.Config.RepoPath, eligibility.DeletedPath)

	deletePath := filepath.Join(s.Config.RepoPath, eligibility.DeletedPath)
	if err := os.RemoveAll(deletePath); err != nil {
		return DeleteSkillResult{}, fmt.Errorf("delete skill %q: %w", name, err)
	}

	commitCreated := false
	if autoCommit && wasTracked {
		if err := gitrepo.AddPathsAndCommit(s.Config.RepoPath, []string{eligibility.DeletedPath}, "Delete skill: "+string(name)); err != nil {
			return DeleteSkillResult{}, err
		}
		commitCreated = true
	}

	return DeleteSkillResult{
		SkillName:     name,
		DeletedPath:   eligibility.DeletedPath,
		CommitCreated: commitCreated,
	}, nil
}

func (s *Service) CommitRepoPath(repoPath, message string) error {
	if !gitrepo.IsRepo(s.Config.RepoPath) {
		return fmt.Errorf("%w: %q", domain.ErrRepoNotFound, s.Config.RepoPath)
	}
	return gitrepo.AddPathsAndCommit(s.Config.RepoPath, []string{repoPath}, message)
}

func (s *Service) ToggleSkillTarget(name domain.SkillName, target domain.Target) error {
	skill, err := registry.LoadOne(s.Config.RepoPath, name)
	if err != nil {
		return err
	}
	if err := s.ensureTargetEnabled(target); err != nil {
		return err
	}
	if !skill.SupportsTarget(target) {
		return fmt.Errorf("%w: skill %q does not support %q", domain.ErrUnsupportedTarget, name, target)
	}

	targetRoot := s.targetRoot(target)
	if install.HasMarker(name, targetRoot) {
		return install.Remove(name, targetRoot)
	}
	return s.installWithCommit(skill, target, targetRoot)
}

func (s *Service) EnableSkillTarget(name domain.SkillName, target domain.Target) error {
	skill, err := registry.LoadOne(s.Config.RepoPath, name)
	if err != nil {
		return err
	}
	if err := s.ensureTargetEnabled(target); err != nil {
		return err
	}
	if !skill.SupportsTarget(target) {
		return fmt.Errorf("%w: skill %q does not support %q", domain.ErrUnsupportedTarget, name, target)
	}

	targetRoot := s.targetRoot(target)
	return s.installWithCommit(skill, target, targetRoot)
}

func (s *Service) DisableSkillTarget(name domain.SkillName, target domain.Target) error {
	if err := s.ensureTargetEnabled(target); err != nil {
		return err
	}
	targetRoot := s.targetRoot(target)
	return install.Remove(name, targetRoot)
}

func (s *Service) SyncRepo() error {
	_, err := s.SyncRepoWithResult("")
	return err
}

func (s *Service) SyncRepoWithResult(projectRoot string) (SyncResult, error) {
	if !gitrepo.IsRepo(s.Config.RepoPath) {
		return SyncResult{}, fmt.Errorf("%w: %q", domain.ErrRepoNotFound, s.Config.RepoPath)
	}
	result := SyncResult{}
	beforeHead, _ := gitrepo.HeadCommit(s.Config.RepoPath)
	if gitrepo.HasRemote(s.Config.RepoPath) {
		if err := gitrepo.Fetch(s.Config.RepoPath); err != nil {
			return SyncResult{}, err
		}
		assessment, err := gitrepo.AssessSyncState(s.Config.RepoPath)
		if err != nil {
			return SyncResult{}, err
		}
		switch assessment.State {
		case gitrepo.SyncStateBootstrapEmptyRemote:
			if err := gitrepo.PushSetUpstream(s.Config.RepoPath, "origin", assessment.Branch); err != nil {
				return SyncResult{}, err
			}
			result.Pushed = true
			result.Bootstrapped = true
		case gitrepo.SyncStateLocalAhead:
			if err := gitrepo.Push(s.Config.RepoPath); err != nil {
				return SyncResult{}, err
			}
			result.Pushed = true
		case gitrepo.SyncStateRemoteAhead:
			if err := gitrepo.Pull(s.Config.RepoPath); err != nil {
				return SyncResult{}, err
			}
			result.Pulled = true
		case gitrepo.SyncStateDiverged:
			return SyncResult{}, fmt.Errorf("sync: local branch %s and %s have diverged; resolve with git and retry", assessment.Branch, assessment.Upstream)
		case gitrepo.SyncStateNoUpstream:
			return SyncResult{}, fmt.Errorf("sync: branch %s has no upstream", assessment.Branch)
		case gitrepo.SyncStateStaleUpstream:
			readiness, readinessErr := gitrepo.CheckSyncReadiness(s.Config.RepoPath)
			if readinessErr != nil {
				return SyncResult{}, fmt.Errorf("sync: %w", readinessErr)
			}
			return SyncResult{}, fmt.Errorf("sync: %s", syncReadinessDetail(readiness, nil))
		}
	}
	skills, err := registry.Load(s.Config.RepoPath)
	if err != nil {
		return SyncResult{}, err
	}
	skillMap := make(map[domain.SkillName]domain.Skill)
	for _, sk := range skills {
		skillMap[sk.Name] = sk
	}
	head, _ := gitrepo.HeadCommit(s.Config.RepoPath)
	result.RepoChanged = beforeHead != "" && beforeHead != head

	for _, target := range s.enabledTargets() {
		targetRoot := s.targetRoot(target)
		refreshed, errs := s.refreshManagedRoot(skillMap, target, targetRoot, head)
		result.RefreshedUser += refreshed
		result.RefreshErrors = append(result.RefreshErrors, errs...)
	}
	if projectRoot != "" {
		for _, target := range s.enabledTargets() {
			targetRoot := projectTargetRoot(target, projectRoot)
			refreshed, errs := s.refreshManagedRoot(skillMap, target, targetRoot, head)
			result.RefreshedProject += refreshed
			result.RefreshErrors = append(result.RefreshErrors, errs...)
		}
	}

	if len(result.RefreshErrors) > 0 {
		return result, fmt.Errorf("refresh managed installs: %s", strings.Join(result.RefreshErrors, "; "))
	}
	return result, nil
}

func (s *Service) SyncStatus(projectRoot string) (SyncStatus, error) {
	if !gitrepo.IsRepo(s.Config.RepoPath) {
		return SyncStatus{}, fmt.Errorf("%w: %q", domain.ErrRepoNotFound, s.Config.RepoPath)
	}

	dirty, err := gitrepo.IsDirty(s.Config.RepoPath)
	if err != nil {
		return SyncStatus{}, err
	}
	head, _ := gitrepo.HeadCommit(s.Config.RepoPath)

	userOutdated := 0
	for _, target := range s.enabledTargets() {
		userOutdated += s.countOutdatedManagedRoot(s.targetRoot(target), head)
	}
	projectOutdated := 0
	if projectRoot != "" {
		for _, target := range s.enabledTargets() {
			projectOutdated += s.countOutdatedManagedRoot(projectTargetRoot(target, projectRoot), head)
		}
	}

	status := SyncStatus{
		RepoDirty:       dirty,
		OutdatedUser:    userOutdated,
		OutdatedProject: projectOutdated,
	}
	if gitrepo.HasRemote(s.Config.RepoPath) {
		if err := gitrepo.Fetch(s.Config.RepoPath); err == nil {
			assessment, err := gitrepo.AssessSyncState(s.Config.RepoPath)
			if err == nil {
				status.RepoState = assessment.State
				status.LocalAhead = assessment.State == gitrepo.SyncStateLocalAhead
				status.RemoteAhead = assessment.State == gitrepo.SyncStateRemoteAhead
				status.Diverged = assessment.State == gitrepo.SyncStateDiverged
				status.BootstrapNeeded = assessment.State == gitrepo.SyncStateBootstrapEmptyRemote
			} else {
				status.RemoteCheckError = err
			}
		} else {
			status.RemoteCheckError = err
		}
	}
	status.NeedsSync = status.RepoDirty || status.LocalAhead || status.RemoteAhead || status.Diverged || status.BootstrapNeeded || status.OutdatedUser > 0 || status.OutdatedProject > 0
	return status, nil

}

func (s *Service) Doctor() (DoctorReport, error) {
	var checks []DoctorCheck

	// Check repo
	repoOK := gitrepo.IsRepo(s.Config.RepoPath)
	checks = append(checks, DoctorCheck{
		Name:   "Repository",
		OK:     repoOK,
		Detail: s.Config.RepoPath,
	})

	syncOK, syncDetail := s.doctorSyncReadiness()
	checks = append(checks, DoctorCheck{
		Name:   "Sync Readiness",
		OK:     syncOK,
		Detail: syncDetail,
	})

	// Check registry loads
	skills, regErr := registry.Load(s.Config.RepoPath)
	checks = append(checks, DoctorCheck{
		Name:   "Registry",
		OK:     regErr == nil,
		Detail: detailOrErr(fmt.Sprintf("%d skills", len(skills)), regErr),
	})

	// Check target directories
	for _, target := range domain.AllTargets() {
		root := s.targetRoot(target)
		detail := root
		if root == "" {
			detail = "disabled"
		}
		checks = append(checks, DoctorCheck{
			Name:   fmt.Sprintf("Target: %s", target),
			OK:     true,
			Detail: detail,
		})
	}

	// Check convergence: are all managed installs present in registry?
	if regErr == nil {
		regSet := make(map[domain.SkillName]bool)
		for _, sk := range skills {
			regSet[sk.Name] = true
		}

		orphanCount := 0
		for _, target := range s.enabledTargets() {
			root := s.targetRoot(target)
			for _, name := range install.ScanManaged(root) {
				if !regSet[name] {
					orphanCount++
				}
			}
		}

		converged := orphanCount == 0
		detail := "converged"
		if !converged {
			detail = fmt.Sprintf("%d managed installs missing from registry", orphanCount)
		}
		checks = append(checks, DoctorCheck{
			Name:   "Convergence",
			OK:     converged,
			Detail: detail,
		})
	}

	allOK := true
	for _, c := range checks {
		if !c.OK {
			allOK = false
			break
		}
	}

	return DoctorReport{Checks: checks, AllOK: allOK}, nil
}

func (s *Service) ImportPath(sourcePath string, targets []domain.Target, autoCommit bool) (ImportResult, error) {
	if !gitrepo.IsRepo(s.Config.RepoPath) {
		return ImportResult{}, fmt.Errorf("%w: %q", domain.ErrRepoNotFound, s.Config.RepoPath)
	}

	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return ImportResult{}, fmt.Errorf("resolve source path: %w", err)
	}
	if len(targets) == 0 {
		targets = s.inferImportTargets(absSource)
	}

	result, err := importer.Import(importer.ImportParams{
		SourceDir: absSource,
		RepoPath:  s.Config.RepoPath,
		Targets:   targets,
	})
	if err != nil {
		return ImportResult{}, err
	}

	commitCreated := false
	if autoCommit {
		relPath := filepath.Join("skills", string(result.Skill.Name))
		if err := gitrepo.AddPathsAndCommit(s.Config.RepoPath, []string{relPath}, "Add skill: "+string(result.Skill.Name)); err != nil {
			return ImportResult{}, err
		}
		commitCreated = true
	}

	return ImportResult{
		SkillName:     result.Skill.Name,
		DestDir:       result.DestDir,
		RepoPath:      filepath.Join("skills", string(result.Skill.Name)),
		CommitCreated: commitCreated,
	}, nil
}

func (s *Service) ListImportCandidates() ([]ImportCandidateView, error) {
	candidates, err := importer.DiscoverCandidates(s.Config.Targets)
	if err != nil {
		return nil, err
	}
	views := importCandidateViews(candidates)
	return s.applyRepoImportStatus(views, true)
}

func (s *Service) ListImportCandidatesFromDir(dir string) ([]ImportCandidateView, error) {
	candidates, err := importer.DiscoverCandidatesInDir(dir, s.Config.Targets.ConfiguredTargets())
	if err != nil {
		return nil, err
	}
	views := importCandidateViews(candidates)
	return s.applyRepoImportStatus(views, false)
}

func importCandidateViews(candidates []importer.Candidate) []ImportCandidateView {
	views := make([]ImportCandidateView, 0, len(candidates))
	for _, candidate := range candidates {
		views = append(views, ImportCandidateView{
			SkillName: candidate.SkillName,
			SourceDir: candidate.SourceDir,
			Targets:   candidate.Targets,
			Ready:     candidate.Ready,
			Problem:   candidate.Problem,
			Duplicate: candidate.Duplicate,
			FromRoots: append([]domain.Target(nil), candidate.FromRoots...),
			Orphan:    candidate.Orphan,
		})
	}
	return views
}

func (s *Service) applyRepoImportStatus(views []ImportCandidateView, excludeExisting bool) ([]ImportCandidateView, error) {
	existing, err := s.repoSkillNames()
	if err != nil {
		return nil, err
	}

	filtered := make([]ImportCandidateView, 0, len(views))
	for _, view := range views {
		if !existing[view.SkillName] {
			filtered = append(filtered, view)
			continue
		}
		if excludeExisting {
			continue
		}
		view.Ready = false
		view.Problem = "already in repo"
		filtered = append(filtered, view)
	}
	return filtered, nil
}

func (s *Service) repoSkillNames() (map[domain.SkillName]bool, error) {
	skills, err := registry.Load(s.Config.RepoPath)
	if err != nil {
		return nil, err
	}
	existing := make(map[domain.SkillName]bool, len(skills))
	for _, skill := range skills {
		existing[skill.Name] = true
	}
	return existing, nil
}

func (s *Service) targetRoot(target domain.Target) string {
	return s.Config.Targets.Path(target)
}

func (s *Service) scanActual() reconcile.ActualState {
	actual := reconcile.NewActualState()

	for _, target := range s.enabledTargets() {
		root := s.targetRoot(target)

		// Scan all subdirs for installed skills
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			name := domain.SkillName(e.Name())
			switch target {
			case domain.TargetClaude:
				actual.Claude[name] = true
			case domain.TargetCodex:
				actual.Codex[name] = true
			}
			if install.HasMarker(name, root) {
				actual.SetManaged(name, target)
			}
		}
	}

	return actual
}

func (s *Service) scanProjectActual(projectRoot string) reconcile.ActualState {
	actual := reconcile.NewActualState()
	if projectRoot == "" {
		return actual
	}

	for _, target := range s.enabledTargets() {
		root := projectTargetRoot(target, projectRoot)
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			name := domain.SkillName(e.Name())
			switch target {
			case domain.TargetClaude:
				actual.Claude[name] = true
			case domain.TargetCodex:
				actual.Codex[name] = true
			}
		}
	}

	return actual
}

func (s *Service) installWithCommit(skill domain.Skill, target domain.Target, targetRoot string) error {
	commit, _ := gitrepo.HeadCommit(s.Config.RepoPath)
	return install.Install(s.Config.RepoPath, skill, target, targetRoot, commit)
}

func (s *Service) refreshManagedRoot(skillMap map[domain.SkillName]domain.Skill, target domain.Target, targetRoot, head string) (int, []string) {
	if targetRoot == "" {
		return 0, nil
	}
	refreshed := 0
	var errs []string
	for _, name := range install.ScanManaged(targetRoot) {
		skill, ok := skillMap[name]
		if !ok || !skill.SupportsTarget(target) {
			continue
		}
		if !s.markerNeedsRefresh(name, targetRoot, head) {
			continue
		}
		if err := install.Install(s.Config.RepoPath, skill, target, targetRoot, head); err != nil {
			errs = append(errs, fmt.Sprintf("%s %s: %v", name, target, err))
			continue
		}
		refreshed++
	}
	return refreshed, errs
}

func (s *Service) countOutdatedManagedRoot(targetRoot, head string) int {
	if targetRoot == "" {
		return 0
	}
	count := 0
	for _, name := range install.ScanManaged(targetRoot) {
		if s.markerNeedsRefresh(name, targetRoot, head) {
			count++
		}
	}
	return count
}

func (s *Service) markerNeedsRefresh(name domain.SkillName, targetRoot, head string) bool {
	marker, err := install.ReadMarker(name, targetRoot)
	if err != nil {
		return true
	}
	return marker.RepoCommit != head
}

func projectTargetRoot(target domain.Target, projectRoot string) string {
	switch target {
	case domain.TargetClaude:
		return filepath.Join(projectRoot, ".claude", "skills")
	case domain.TargetCodex:
		return filepath.Join(projectRoot, ".codex", "skills")
	default:
		return ""
	}
}

func (s *Service) ProjectInstall(name domain.SkillName, target domain.Target, projectRoot string) error {
	skill, err := registry.LoadOne(s.Config.RepoPath, name)
	if err != nil {
		return err
	}
	if err := s.ensureTargetEnabled(target); err != nil {
		return err
	}
	if !skill.SupportsTarget(target) {
		return fmt.Errorf("%w: skill %q does not support %q", domain.ErrUnsupportedTarget, name, target)
	}
	targetRoot := projectTargetRoot(target, projectRoot)
	commit, _ := gitrepo.HeadCommit(s.Config.RepoPath)
	return install.Install(s.Config.RepoPath, skill, target, targetRoot, commit)
}

func (s *Service) ProjectRemove(name domain.SkillName, target domain.Target, projectRoot string) error {
	if err := s.ensureTargetEnabled(target); err != nil {
		return err
	}
	targetRoot := projectTargetRoot(target, projectRoot)
	return install.Remove(name, targetRoot)
}

func (s *Service) ProjectList(projectRoot string) ([]SkillView, error) {
	// Load registry for cross-referencing
	skills, regErr := registry.Load(s.Config.RepoPath)
	skillMap := make(map[domain.SkillName]domain.Skill)
	if regErr == nil {
		for _, sk := range skills {
			skillMap[sk.Name] = sk
		}
	}

	seen := make(map[domain.SkillName]*SkillView)
	for _, target := range s.enabledTargets() {
		root := projectTargetRoot(target, projectRoot)
		names := scanInstalledSkills(root)
		for _, name := range names {
			v, ok := seen[name]
			if !ok {
				skill, found := skillMap[name]
				if !found {
					skill = domain.Skill{
						Name: name,
					}
				}
				v = &SkillView{Skill: skill}
				seen[name] = v
			}
			switch target {
			case domain.TargetClaude:
				v.ProjectClaude = true
			case domain.TargetCodex:
				v.ProjectCodex = true
			}
		}
	}

	var views []SkillView
	for _, v := range seen {
		views = append(views, *v)
	}
	return views, nil
}

func (s *Service) targetEnabled(target domain.Target) bool {
	return s.Config.Targets.Enabled(target)
}

func (s *Service) enabledTargets() []domain.Target {
	return s.Config.Targets.ConfiguredTargets()
}

func (s *Service) inferImportTargets(sourcePath string) []domain.Target {
	var targets []domain.Target
	for _, target := range s.enabledTargets() {
		root := s.targetRoot(target)
		if root == "" {
			continue
		}
		rel, err := filepath.Rel(root, sourcePath)
		if err != nil {
			continue
		}
		if rel == "." || (!strings.HasPrefix(rel, "..") && rel != "..") {
			targets = append(targets, target)
		}
	}
	return targets
}

func (s *Service) ensureTargetEnabled(target domain.Target) error {
	if s.targetEnabled(target) {
		return nil
	}
	return fmt.Errorf("%w: %q", domain.ErrTargetDisabled, target)
}

func scanInstalledSkills(targetRoot string) []domain.SkillName {
	entries, err := os.ReadDir(targetRoot)
	if err != nil {
		return nil
	}
	var names []domain.SkillName
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, domain.SkillName(e.Name()))
		}
	}
	return names
}

func detailOrErr(detail string, err error) string {
	if err != nil {
		return err.Error()
	}
	return detail
}

func syncReadinessDetail(readiness gitrepo.SyncReadiness, err error) string {
	if err != nil {
		if readiness.Branch != "" {
			return fmt.Sprintf("%s: %s", readiness.Branch, err.Error())
		}
		return err.Error()
	}
	return fmt.Sprintf("%s -> %s", readiness.Branch, readiness.Upstream)
}

func (s *Service) doctorSyncReadiness() (bool, string) {
	if !gitrepo.IsRepo(s.Config.RepoPath) {
		return false, detailOrErr("", fmt.Errorf("%w: %q", domain.ErrRepoNotFound, s.Config.RepoPath))
	}
	if !gitrepo.HasRemote(s.Config.RepoPath) {
		return true, "no remote configured"
	}
	if err := gitrepo.Fetch(s.Config.RepoPath); err != nil {
		return false, err.Error()
	}
	assessment, err := gitrepo.AssessSyncState(s.Config.RepoPath)
	if err != nil {
		return false, err.Error()
	}
	switch assessment.State {
	case gitrepo.SyncStateBootstrapEmptyRemote:
		return true, fmt.Sprintf("%s: empty remote; sync will publish first local commit", assessment.Branch)
	case gitrepo.SyncStateEmptyRemoteNoCommits:
		return true, fmt.Sprintf("%s: empty remote; waiting for first commit", assessment.Branch)
	case gitrepo.SyncStateNoUpstream:
		return false, fmt.Sprintf("%s: branch has no upstream", assessment.Branch)
	case gitrepo.SyncStateStaleUpstream:
		readiness, readinessErr := gitrepo.CheckSyncReadiness(s.Config.RepoPath)
		return false, syncReadinessDetail(readiness, readinessErr)
	case gitrepo.SyncStateDiverged:
		return false, fmt.Sprintf("%s: local and %s have diverged", assessment.Branch, assessment.Upstream)
	case gitrepo.SyncStateLocalAhead:
		return true, fmt.Sprintf("%s -> %s (local ahead; sync will push)", assessment.Branch, assessment.Upstream)
	case gitrepo.SyncStateRemoteAhead:
		return true, fmt.Sprintf("%s -> %s (remote ahead; sync will pull)", assessment.Branch, assessment.Upstream)
	case gitrepo.SyncStateUpToDate:
		return true, fmt.Sprintf("%s -> %s", assessment.Branch, assessment.Upstream)
	default:
		return true, string(assessment.State)
	}
}
