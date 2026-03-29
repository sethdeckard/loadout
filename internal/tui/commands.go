package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sethdeckard/loadout/internal/app"
	"github.com/sethdeckard/loadout/internal/config"
	"github.com/sethdeckard/loadout/internal/domain"
	"github.com/sethdeckard/loadout/internal/fsx"
)

// Messages
type loadSkillsMsg struct {
	views []app.SkillView
	err   error
}

type previewMsg struct {
	preview app.SkillPreview
	err     error
}

type importPreviewMsg struct {
	preview app.ImportPreview
	err     error
}

type toggleResultMsg struct {
	name     domain.SkillName
	target   domain.Target
	equipped bool
	err      error
}

type syncResultMsg struct {
	result app.SyncResult
	err    error
}

type syncStatusMsg struct {
	status app.SyncStatus
	err    error
}

type doctorResultMsg struct {
	report app.DoctorReport
	err    error
}

type saveSettingsResultMsg struct {
	cfg config.Config
	err error
}

type loadImportCandidatesMsg struct {
	views []app.ImportCandidateView
	err   error
}

type startImportMsg struct {
	dir   string
	views []app.ImportCandidateView
	err   error
}

type importSkillResultMsg struct {
	result app.ImportResult
	err    error
}

type deleteEligibilityMsg struct {
	eligibility app.DeleteEligibility
	err         error
}

type deleteSkillResultMsg struct {
	result app.DeleteSkillResult
	err    error
}

type commitRepoPathResultMsg struct {
	err error
}

// Commands
func loadSkillsCmd(svc *app.Service, projectRoot string) tea.Cmd {
	return func() tea.Msg {
		var (
			views []app.SkillView
			err   error
		)
		if projectRoot != "" {
			views, err = svc.ListSkillsForProject(projectRoot)
		} else {
			views, err = svc.ListSkills()
		}
		return loadSkillsMsg{views: views, err: err}
	}
}

func loadPreviewCmd(svc *app.Service, name domain.SkillName) tea.Cmd {
	return func() tea.Msg {
		preview, err := svc.PreviewSkill(name)
		return previewMsg{preview: preview, err: err}
	}
}

func loadLocalPreviewCmd(svc *app.Service, name domain.SkillName, targetRoot string) tea.Cmd {
	return func() tea.Msg {
		preview, err := svc.PreviewLocalSkill(name, targetRoot)
		return previewMsg{preview: preview, err: err}
	}
}

func loadImportSourcePreviewCmd(svc *app.Service, sourceDir string, targets []domain.Target) tea.Cmd {
	return func() tea.Msg {
		preview, err := svc.PreviewImportSource(sourceDir, targets)
		if err != nil {
			return previewMsg{err: err}
		}
		return previewMsg{
			preview: app.SkillPreview{
				Skill:    preview.Skill,
				Markdown: preview.Markdown,
			},
		}
	}
}

func loadImportPreviewCmd(svc *app.Service, sourceDir string, targets []domain.Target) tea.Cmd {
	return func() tea.Msg {
		preview, err := svc.PreviewImportSource(sourceDir, targets)
		return importPreviewMsg{preview: preview, err: err}
	}
}

func toggleTargetCmd(svc *app.Service, name domain.SkillName, target domain.Target, equipped bool) tea.Cmd {
	return func() tea.Msg {
		err := svc.ToggleSkillTarget(name, target)
		return toggleResultMsg{name: name, target: target, equipped: equipped, err: err}
	}
}

func syncCmd(svc *app.Service, projectRoot string) tea.Cmd {
	return func() tea.Msg {
		result, err := svc.SyncRepoWithResult(projectRoot)
		return syncResultMsg{result: result, err: err}
	}
}

func loadSyncStatusCmd(svc *app.Service, projectRoot string) tea.Cmd {
	return func() tea.Msg {
		status, err := svc.SyncStatus(projectRoot)
		return syncStatusMsg{status: status, err: err}
	}
}

func doctorCmd(svc *app.Service) tea.Cmd {
	return func() tea.Msg {
		report, err := svc.Doctor()
		return doctorResultMsg{report: report, err: err}
	}
}

func saveSettingsCmd(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		err := config.Save(config.DefaultPath(), cfg)
		return saveSettingsResultMsg{cfg: cfg, err: err}
	}
}

func loadImportCandidatesCmd(svc *app.Service) tea.Cmd {
	return func() tea.Msg {
		views, err := svc.ListImportCandidates()
		return loadImportCandidatesMsg{views: views, err: err}
	}
}

func loadScopedImportCandidatesCmd(svc *app.Service, projectRoot string) tea.Cmd {
	if projectRoot != "" {
		return loadImportCandidatesFromDirCmd(svc, projectRoot)
	}
	return loadImportCandidatesCmd(svc)
}

func startImportCmd(svc *app.Service, projectRoot string) tea.Cmd {
	return func() tea.Msg {
		dir, err := os.Getwd()
		if err != nil {
			dir = fsx.HomeOrRoot()
		}
		if projectRoot != "" {
			dir = projectRoot
		}
		msg := loadScopedImportCandidatesCmd(svc, projectRoot)().(loadImportCandidatesMsg)
		return startImportMsg{dir: dir, views: msg.views, err: msg.err}
	}
}

func importSkillCmd(svc *app.Service, sourceDir string, targets []domain.Target, autoCommit bool) tea.Cmd {
	return func() tea.Msg {
		result, err := svc.ImportPath(sourceDir, targets, autoCommit)
		return importSkillResultMsg{result: result, err: err}
	}
}

func deleteEligibilityCmd(svc *app.Service, name domain.SkillName, projectRoot string) tea.Cmd {
	return func() tea.Msg {
		eligibility, err := svc.DeleteSkillEligibility(name, projectRoot)
		return deleteEligibilityMsg{eligibility: eligibility, err: err}
	}
}

func deleteSkillCmd(svc *app.Service, name domain.SkillName, projectRoot string, autoCommit bool) tea.Cmd {
	return func() tea.Msg {
		result, err := svc.DeleteSkill(name, projectRoot, autoCommit)
		return deleteSkillResultMsg{result: result, err: err}
	}
}

func commitRepoPathCmd(svc *app.Service, repoPath, message string) tea.Cmd {
	return func() tea.Msg {
		err := svc.CommitRepoPath(repoPath, message)
		return commitRepoPathResultMsg{err: err}
	}
}

type bulkImportResultMsg struct {
	imported   []domain.SkillName
	skipped    int
	errors     []string
	autoCommit bool
}

func bulkImportCmd(svc *app.Service, candidates []app.ImportCandidateView, skipped int, autoCommit bool) tea.Cmd {
	return func() tea.Msg {
		var imported []domain.SkillName
		var errors []string
		for _, c := range candidates {
			result, err := svc.ImportPath(c.SourceDir, c.Targets, autoCommit)
			if err != nil {
				errors = append(errors, string(c.SkillName)+": "+err.Error())
				continue
			}
			imported = append(imported, result.SkillName)
		}
		return bulkImportResultMsg{
			imported:   imported,
			skipped:    skipped,
			errors:     errors,
			autoCommit: autoCommit,
		}
	}
}

type projectToggleResultMsg struct {
	name     domain.SkillName
	target   domain.Target
	equipped bool
	err      error
}

type projectImportHintMsg struct {
	readyCount int
	err        error
}

type userImportHintMsg struct {
	readyCount int
	err        error
}

type loadBrowseDirMsg struct {
	dir     string
	entries []string
	err     error
}

func loadBrowseDirCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return loadBrowseDirMsg{dir: dir, err: err}
		}
		var dirs []string
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			if strings.HasPrefix(name, ".") && name != ".claude" && name != ".codex" {
				continue
			}
			if entry.IsDir() {
				dirs = append(dirs, entry.Name())
			}
		}
		sort.Strings(dirs)
		return loadBrowseDirMsg{dir: dir, entries: dirs}
	}
}

func loadImportCandidatesFromDirCmd(svc *app.Service, dir string) tea.Cmd {
	return func() tea.Msg {
		dir, err := filepath.Abs(dir)
		if err != nil {
			return loadImportCandidatesMsg{err: err}
		}
		views, err := svc.ListImportCandidatesFromDir(dir)
		return loadImportCandidatesMsg{views: views, err: err}
	}
}

func projectToggleCmd(svc *app.Service, name domain.SkillName, target domain.Target, projectRoot string, installed bool) tea.Cmd {
	return func() tea.Msg {
		var err error
		if installed {
			err = svc.ProjectRemove(name, target, projectRoot)
		} else {
			err = svc.ProjectInstall(name, target, projectRoot)
		}
		return projectToggleResultMsg{name: name, target: target, equipped: !installed, err: err}
	}
}

func projectImportHintCmd(svc *app.Service, projectRoot string) tea.Cmd {
	return func() tea.Msg {
		views, err := svc.ListImportCandidatesFromDir(projectRoot)
		if err != nil {
			return projectImportHintMsg{err: err}
		}
		readyCount := 0
		for _, view := range views {
			if view.Ready {
				readyCount++
			}
		}
		return projectImportHintMsg{readyCount: readyCount}
	}
}

func userImportHintCmd(svc *app.Service) tea.Cmd {
	return func() tea.Msg {
		views, err := svc.ListImportCandidates()
		if err != nil {
			return userImportHintMsg{err: err}
		}
		readyCount := 0
		for _, view := range views {
			if view.Ready {
				readyCount++
			}
		}
		return userImportHintMsg{readyCount: readyCount}
	}
}
