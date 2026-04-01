package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sethdeckard/loadout/internal/app"
	"github.com/sethdeckard/loadout/internal/domain"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.commitPromptActive() {
			return m.handleCommitPromptKey(msg)
		}
		if m.inImportScreen() {
			return m.handleImportKey(msg)
		}
		if m.inSettingsScreen() {
			return m.handleSettingsKey(msg)
		}
		return m.handleKey(msg)

	case loadSkillsMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.skills = msg.views
		m.applyFilter()
		if m.deleteConfirming() {
			found := false
			for _, view := range m.skills {
				if view.Skill.Name == m.deleteSkillName {
					found = true
					break
				}
			}
			if !found {
				m.clearDeleteState()
			}
		}
		// Auto-load preview for selected skill
		var cmds []tea.Cmd
		if sel := m.selectedSkill(); sel != nil {
			cmds = append(cmds, m.previewCmdForSkill(sel))
		}
		if m.inProjectMode() {
			cmds = append(cmds, projectImportHintCmd(m.svc, m.projectRoot))
		} else {
			cmds = append(cmds, userImportHintCmd(m.svc))
		}
		if len(cmds) == 0 {
			return m, nil
		}
		return m, tea.Batch(cmds...)

	case projectImportHintMsg:
		if msg.err == nil {
			m.projectHintCount = msg.readyCount
		}
		return m, nil

	case userImportHintMsg:
		if msg.err == nil {
			m.userHintCount = msg.readyCount
		}
		return m, nil

	case previewMsg:
		if msg.err != nil {
			m.preview = nil
			return m, nil
		}
		m.preview = &msg.preview
		m.detailScroll = 0
		return m, nil

	case importPreviewMsg:
		if msg.err != nil {
			m.importPreview = nil
			m.status = "preview error: " + msg.err.Error()
			return m, nil
		}
		m.importPreview = &msg.preview
		m.importPreviewScroll = 0
		return m, nil

	case toggleResultMsg:
		if msg.err != nil {
			m.status = "error: " + msg.err.Error()
			return m, nil
		}
		if msg.equipped {
			m.status = "equipped " + string(msg.name) + " for " + string(msg.target)
		} else {
			m.status = "unequipped " + string(msg.name) + " from " + string(msg.target)
		}
		return m, m.refreshAndLoad()

	case syncResultMsg:
		if msg.err != nil {
			m.status = "sync error: " + msg.err.Error()
			return m, nil
		}
		m.syncAttention = false
		switch {
		case msg.result.RefreshedTotal() > 0:
			m.status = syncResultStatus(msg.result)
		case msg.result.RepoChanged:
			m.status = "sync complete; managed installs already current"
		default:
			m.status = "sync complete; no changes"
		}
		return m, m.refreshAndLoad()

	case syncStatusMsg:
		if msg.err == nil {
			m.syncAttention = msg.status.NeedsSync
		}
		return m, nil

	case doctorResultMsg:
		if msg.err != nil {
			m.status = "doctor error: " + msg.err.Error()
			return m, nil
		}
		m.doctor = &msg.report
		m.status = "doctor complete"
		return m, nil

	case deleteEligibilityMsg:
		if msg.err != nil {
			m.status = "delete error: " + msg.err.Error()
			return m, nil
		}
		m.beginDeleteConfirm(msg.eligibility.SkillName, msg.eligibility.Blockers)
		m.doctor = nil
		if msg.eligibility.Deletable {
			m.status = "type " + string(msg.eligibility.SkillName) + " to confirm deletion"
			return m, nil
		}
		m.status = "delete blocked: " + strings.Join(msg.eligibility.Blockers, ", ")
		return m, nil

	case deleteSkillResultMsg:
		if msg.err != nil {
			m.clearDeleteState()
			m.status = "delete error: " + msg.err.Error()
			return m, nil
		}
		if msg.result.CommitCreated {
			m.clearDeleteState()
			m.preview = nil
			m.detailScroll = 0
			m.status = "deleted " + string(msg.result.SkillName) + " and committed repo change"
			m.loading = true
			return m, loadSkillsCmd(m.svc, m.projectRoot)
		}
		m.beginCommitPrompt(
			"delete",
			msg.result.SkillName,
			msg.result.DeletedPath,
			"Delete skill: "+string(msg.result.SkillName),
			"deleted "+string(msg.result.SkillName)+" and committed repo change",
			"deleted "+string(msg.result.SkillName)+" without committing repo change",
			false,
		)
		m.preview = nil
		m.detailScroll = 0
		m.status = "deleted " + string(msg.result.SkillName) + "; commit repo change?"
		return m, nil

	case saveSettingsResultMsg:
		if msg.err != nil {
			m.status = "settings error: " + msg.err.Error()
			return m, nil
		}
		m.svc.Config = msg.cfg
		m.err = nil
		m.doctor = nil
		m.preview = nil
		m.detailScroll = 0
		m.filtering = false
		m.closeSettings()
		m.status = "settings saved"
		return m, m.refreshCurrentScreen()

	case loadImportCandidatesMsg:
		m.loading = false
		if msg.err != nil {
			m.importPreview = nil
			m.status = "import discovery error: " + msg.err.Error()
			return m, nil
		}
		m.imports = msg.views
		if m.cursor >= len(m.imports) {
			m.cursor = max(0, len(m.imports)-1)
		}
		if cmd := m.loadSelectedImportPreviewCmd(); cmd != nil {
			return m, cmd
		}
		m.importPreview = nil
		return m, nil

	case startImportMsg:
		m.loading = false
		m.importStartDir = msg.dir
		if msg.err != nil {
			m.importPreview = nil
			m.status = "import discovery error: " + msg.err.Error()
			return m, nil
		}
		m.importBrowsing = false
		m.importCustomDir = ""
		m.imports = msg.views
		if len(msg.views) == 0 {
			m.importPreview = nil
			return m, nil
		}
		if cmd := m.moveImportCursor(0); cmd != nil {
			return m, cmd
		}
		return m, nil

	case importSkillResultMsg:
		if msg.err != nil {
			m.status = "import error: " + msg.err.Error()
			return m, nil
		}
		if msg.result.CommitCreated {
			m.closeImport()
			m.status = "imported " + string(msg.result.SkillName) + " and committed repo change"
			return m, m.refreshAndLoad()
		}
		m.beginCommitPrompt(
			"import",
			msg.result.SkillName,
			msg.result.RepoPath,
			"Add skill: "+string(msg.result.SkillName),
			"imported "+string(msg.result.SkillName)+" and committed repo change",
			"imported "+string(msg.result.SkillName)+" without committing repo change",
			true,
		)
		m.status = "imported " + string(msg.result.SkillName) + "; commit repo change?"
		return m, nil

	case bulkImportResultMsg:
		m.loading = false
		m.resetBulkImport()
		statusText := formatBulkImportStatus(len(msg.imported), msg.skipped, len(msg.errors), msg.autoCommit)
		if !msg.autoCommit && len(msg.imported) > 0 {
			commitMsg := fmt.Sprintf("Add %d skills", len(msg.imported))
			m.beginCommitPrompt(
				"import",
				domain.SkillName(fmt.Sprintf("%d skills", len(msg.imported))),
				"skills",
				commitMsg,
				statusText,
				statusText,
				false,
			)
			m.status = statusText + "; commit repo changes?"
			return m, m.refreshImportSource()
		}
		m.status = statusText
		return m, tea.Batch(
			m.refreshImportSource(),
			loadSkillsCmd(m.svc, m.projectRoot),
		)

	case commitRepoPathResultMsg:
		if msg.err != nil {
			m.status = "commit error: " + msg.err.Error()
			return m, nil
		}
		return m.finishCommitPrompt(true)

	case projectToggleResultMsg:
		if msg.err != nil {
			m.status = "error: " + msg.err.Error()
			return m, nil
		}
		if msg.equipped {
			m.status = "equipped " + string(msg.name) + " for " + string(msg.target)
		} else {
			m.status = "unequipped " + string(msg.name) + " from " + string(msg.target)
		}
		return m, m.refreshAndLoad()

	case loadBrowseDirMsg:
		if msg.err != nil {
			m.status = "browse error: " + msg.err.Error()
			return m, nil
		}
		m.browseDir = msg.dir
		m.browseDirEntries = msg.entries
		m.browseCursor = 0
		return m, nil
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.commitPromptActive() {
		return m.handleCommitPromptKey(msg)
	}
	if m.deleteConfirming() {
		return m.handleDeleteConfirmKey(msg)
	}

	// In filter mode, handle text input
	if m.filtering {
		switch msg.String() {
		case "esc":
			m.filtering = false
			m.filter = ""
			m.applyFilter()
			return m, nil
		case "enter":
			m.filtering = false
			return m, nil
		case "backspace":
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
				m.applyFilter()
			}
			return m, nil
		default:
			if len(msg.String()) == 1 {
				m.filter += msg.String()
				m.applyFilter()
			}
			return m, nil
		}
	}

	action := classifyKey(msg)

	switch action {
	case keyQuit:
		return m, tea.Quit

	case keyUp:
		if m.showHelp {
			m.helpScroll = max(0, m.helpScroll-1)
			return m, nil
		}
		if m.focusPane == paneDetails {
			m.detailScroll = max(0, m.detailScroll-1)
			return m, nil
		}
		return m, m.moveSkillCursor(m.cursor - 1)

	case keyDown:
		if m.showHelp {
			m.helpScroll++
			return m, nil
		}
		if m.focusPane == paneDetails {
			m.detailScroll++
			return m, nil
		}
		return m, m.moveSkillCursor(m.cursor + 1)

	case keyTop:
		if m.showHelp {
			m.helpScroll = 0
			return m, nil
		}
		if m.focusPane == paneDetails {
			m.detailScroll = 0
			return m, nil
		}
		return m, m.moveSkillCursor(0)

	case keyBottom:
		if m.showHelp {
			m.helpScroll = 999999 // clamped in view
			return m, nil
		}
		if m.focusPane == paneDetails {
			m.detailScroll = 999999 // clamped in view
			return m, nil
		}
		return m, m.moveSkillCursor(len(m.filtered) - 1)

	case keyPageUp:
		if m.showHelp {
			m.helpScroll = max(0, m.helpScroll-pageStep(m.mainBodyHeight()))
			return m, nil
		}
		if m.focusPane == paneDetails {
			m.detailScroll = max(0, m.detailScroll-pageStep(m.detailContentHeight()))
			return m, nil
		}
		return m, m.moveSkillCursor(m.cursor - pageStep(m.skillListVisibleItems(m.skillListContentHeight())))

	case keyPageDown:
		if m.showHelp {
			m.helpScroll += pageStep(m.mainBodyHeight())
			return m, nil
		}
		if m.focusPane == paneDetails {
			m.detailScroll += pageStep(m.detailContentHeight())
			return m, nil
		}
		return m, m.moveSkillCursor(m.cursor + pageStep(m.skillListVisibleItems(m.skillListContentHeight())))

	case keyLeft:
		m.focusPane = paneSkills
		return m, nil

	case keyRight:
		m.focusPane = paneDetails
		return m, nil

	case keyFilter:
		m.filtering = true
		m.filter = ""
		return m, nil

	case keyFilterClear:
		m.filter = ""
		m.applyFilter()
		m.showHelp = false
		m.doctor = nil
		return m, nil

	case keyClaude:
		if m.blockUnmanagedSelectionAction() {
			return m, nil
		}
		if m.inProjectMode() {
			return m, m.projectToggle(domain.TargetClaude)
		}
		return m, m.toggleTarget(domain.TargetClaude)

	case keyCodex:
		if m.blockUnmanagedSelectionAction() {
			return m, nil
		}
		if m.inProjectMode() {
			return m, m.projectToggle(domain.TargetCodex)
		}
		return m, m.toggleTarget(domain.TargetCodex)

	case keyAll:
		if m.blockUnmanagedSelectionAction() {
			return m, nil
		}
		if m.inProjectMode() {
			sel := m.selectedSkill()
			if sel == nil {
				return m, nil
			}
			var cmds []tea.Cmd
			allInstalled := true
			for _, t := range m.enabledTargets() {
				if sel.Skill.SupportsTarget(t) {
					installed := projectInstalledForTarget(*sel, t)
					if !installed {
						allInstalled = false
					}
				}
			}
			for _, t := range m.enabledTargets() {
				if sel.Skill.SupportsTarget(t) {
					installed := projectInstalledForTarget(*sel, t)
					if allInstalled {
						cmds = append(cmds, projectToggleCmd(m.svc, sel.Skill.Name, t, m.projectRoot, true))
					} else if !installed {
						cmds = append(cmds, projectToggleCmd(m.svc, sel.Skill.Name, t, m.projectRoot, false))
					}
				}
			}
			if len(cmds) > 0 {
				if allInstalled {
					m.status = "unequipping project targets..."
				} else {
					m.status = "equipping project targets..."
				}
				return m, tea.Batch(cmds...)
			}
			m.status = "no supported project targets to update"
			return m, nil
		}
		sel := m.selectedSkill()
		if sel == nil {
			return m, nil
		}
		var cmds []tea.Cmd
		allInstalled := true
		for _, t := range m.enabledTargets() {
			if sel.Skill.SupportsTarget(t) {
				installed := skillInstalledForTarget(*sel, t)
				if !installed {
					allInstalled = false
				}
			}
		}
		for _, t := range m.enabledTargets() {
			if sel.Skill.SupportsTarget(t) {
				installed := skillInstalledForTarget(*sel, t)
				if allInstalled {
					cmds = append(cmds, toggleTargetCmd(m.svc, sel.Skill.Name, t, false))
				} else if !installed {
					cmds = append(cmds, toggleTargetCmd(m.svc, sel.Skill.Name, t, true))
				}
			}
		}
		if len(cmds) > 0 {
			if allInstalled {
				m.status = "unequipping supported targets..."
			} else {
				m.status = "equipping supported targets..."
			}
			return m, tea.Batch(cmds...)
		}
		m.status = "no supported targets to update"
		return m, nil

	case keySync:
		m.status = "syncing repo and refreshing managed installs..."
		return m, syncCmd(m.svc, m.projectRoot)

	case keyDoctor:
		m.status = "running doctor..."
		return m, doctorCmd(m.svc)

	case keyHelp:
		m.showHelp = !m.showHelp
		m.helpScroll = 0
		return m, nil

	case keyScope:
		if m.detectedProject == "" {
			m.status = "no project detected (need git repo with .claude/ or .codex/)"
			return m, nil
		}
		if m.inProjectMode() {
			// Switch to user
			m.projectRoot = ""
			m.projectHintCount = 0
			m.doctor = nil
			m.preview = nil
			m.detailScroll = 0
			m.focusPane = paneSkills
			m.status = ""
			return m, m.refreshAndLoad()
		}
		// Switch to project
		m.projectRoot = m.detectedProject
		m.userHintCount = 0
		m.doctor = nil
		m.preview = nil
		m.detailScroll = 0
		m.focusPane = paneSkills
		m.status = ""
		m.loading = true
		return m, tea.Batch(
			loadSkillsCmd(m.svc, m.projectRoot),
			projectImportHintCmd(m.svc, m.projectRoot),
		)

	case keySettings:
		m.openSettings()
		return m, nil

	case keyImport:
		return m, m.openImport()

	case keyDelete:
		return m, m.beginDeleteForSelection()
	}

	return m, nil
}

func (m Model) handleCommitPromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		return m.finishCommitPrompt(false)
	case "enter":
		if m.commitPrompt == nil {
			return m, nil
		}
		m.status = m.commitPrompt.ActionKind + " commit in progress..."
		return m, commitRepoPathCmd(m.svc, m.commitPrompt.RepoPath, m.commitPrompt.CommitMessage)
	}
	return m, nil
}

func (m Model) finishCommitPrompt(committed bool) (tea.Model, tea.Cmd) {
	if m.commitPrompt == nil {
		return m, nil
	}
	status := m.commitPrompt.UncommittedStatus
	if committed {
		status = m.commitPrompt.CommittedStatus
	}
	closeImport := m.commitPrompt.CloseImport
	m.clearCommitPrompt()
	m.preview = nil
	m.detailScroll = 0
	if closeImport {
		m.closeImport()
	}
	m.status = status
	m.loading = true
	return m, loadSkillsCmd(m.svc, m.projectRoot)
}

func (m Model) handleDeleteConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.clearDeleteState()
		m.status = "delete cancelled"
		return m, nil
	case "backspace":
		if len(m.deleteInput) > 0 {
			m.deleteInput = m.deleteInput[:len(m.deleteInput)-1]
		}
		return m, nil
	case "tab":
		if !m.deleteReady {
			return m, nil
		}
		m.deleteCommit = !m.deleteCommit
		if m.deleteCommit {
			m.status = "delete auto-commit enabled"
		} else {
			m.status = "delete auto-commit disabled"
		}
		return m, nil
	case "enter":
		if !m.deleteReady {
			m.status = "delete blocked: uninstall managed copies first"
			return m, nil
		}
		if m.deleteInput != string(m.deleteSkillName) {
			m.status = "delete blocked: type " + string(m.deleteSkillName) + " exactly"
			return m, nil
		}
		m.status = "deleting " + string(m.deleteSkillName) + "..."
		return m, deleteSkillCmd(m.svc, m.deleteSkillName, m.detectedProject, m.deleteCommit)
	default:
		if len(msg.String()) == 1 {
			m.deleteInput += msg.String()
		}
		return m, nil
	}
}

func (m Model) handleImportKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.importBrowsing {
		return m.handleBrowseKey(msg)
	}

	if m.bulkImportConfirming() {
		switch msg.String() {
		case "enter":
			var ready []app.ImportCandidateView
			for _, c := range m.imports {
				if c.Ready {
					ready = append(ready, c)
				}
			}
			skipped := m.bulkImport.SkippedCount
			autoCommit := m.bulkImport.AutoCommit
			m.loading = true
			m.status = fmt.Sprintf("importing %d skills...", len(ready))
			return m, bulkImportCmd(m.svc, ready, skipped, autoCommit)
		case "esc":
			m.resetBulkImport()
			m.status = "import all cancelled"
			return m, nil
		case "q", "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}

	switch classifyKey(msg) {
	case keyQuit:
		return m, tea.Quit
	case keyUp:
		if m.showHelp {
			m.helpScroll = max(0, m.helpScroll-1)
			return m, nil
		}
		if m.focusPane == paneDetails {
			m.importPreviewScroll = max(0, m.importPreviewScroll-1)
			return m, nil
		}
		if m.cursor > 0 {
			return m, m.moveImportCursor(m.cursor - 1)
		}
		return m, nil
	case keyDown:
		if m.showHelp {
			m.helpScroll++
			return m, nil
		}
		if m.focusPane == paneDetails {
			m.importPreviewScroll++
			return m, nil
		}
		if m.cursor < len(m.imports)-1 {
			return m, m.moveImportCursor(m.cursor + 1)
		}
		return m, nil
	case keyTop:
		if m.showHelp {
			m.helpScroll = 0
			return m, nil
		}
		if m.focusPane == paneDetails {
			m.importPreviewScroll = 0
			return m, nil
		}
		return m, m.moveImportCursor(0)
	case keyBottom:
		if m.showHelp {
			m.helpScroll = 999999
			return m, nil
		}
		if m.focusPane == paneDetails {
			m.importPreviewScroll = 999999
			return m, nil
		}
		return m, m.moveImportCursor(max(0, len(m.imports)-1))
	case keyPageUp:
		if m.showHelp {
			m.helpScroll = max(0, m.helpScroll-pageStep(m.mainBodyHeight()))
			return m, nil
		}
		if m.focusPane == paneDetails {
			m.importPreviewScroll = max(0, m.importPreviewScroll-pageStep(m.importPreviewContentHeight()))
			return m, nil
		}
		return m, m.moveImportCursor(m.cursor - pageStep(m.importListVisibleItems(m.importContentHeight())))
	case keyPageDown:
		if m.showHelp {
			m.helpScroll += pageStep(m.mainBodyHeight())
			return m, nil
		}
		if m.focusPane == paneDetails {
			m.importPreviewScroll += pageStep(m.importPreviewContentHeight())
			return m, nil
		}
		return m, m.moveImportCursor(m.cursor + pageStep(m.importListVisibleItems(m.importContentHeight())))
	case keyLeft:
		m.focusPane = paneSkills
		return m, nil
	case keyRight:
		m.focusPane = paneDetails
		return m, nil
	case keyHelp:
		m.showHelp = !m.showHelp
		m.helpScroll = 0
		return m, nil
	case keyScope:
		return m, m.toggleImportScope()
	case keyImport:
		return m.importSelectedCandidate()
	}

	switch msg.String() {
	case "esc", "p":
		m.closeImport()
		return m, nil
	case "c":
		m.importCommit = !m.importCommit
		if m.importCommit {
			m.status = "auto-commit enabled"
		} else {
			m.status = "auto-commit disabled"
		}
		return m, nil
	case "enter":
		return m.importSelectedCandidate()
	case "b":
		return m, m.openBrowse()
	case "A":
		if m.showHelp {
			return m, nil
		}
		readyCount := 0
		skippedCount := 0
		for _, c := range m.imports {
			if c.Ready {
				readyCount++
			} else {
				skippedCount++
			}
		}
		if readyCount == 0 {
			m.status = "no ready candidates to import"
			return m, nil
		}
		m.bulkImport = &bulkImportState{
			ReadyCount:   readyCount,
			SkippedCount: skippedCount,
			AutoCommit:   m.importCommit,
		}
		return m, nil
	}

	return m, nil
}

func (m Model) handleBrowseKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch classifyKey(msg) {
	case keyQuit:
		return m, tea.Quit
	case keyUp:
		if m.showHelp {
			m.helpScroll = max(0, m.helpScroll-1)
			return m, nil
		}
		if m.browseCursor > 0 {
			m.browseCursor--
		}
		return m, nil
	case keyDown:
		if m.showHelp {
			m.helpScroll++
			return m, nil
		}
		if m.browseCursor < len(m.browseDirEntries) {
			m.browseCursor++
		}
		return m, nil
	case keyTop:
		if m.showHelp {
			m.helpScroll = 0
			return m, nil
		}
		m.browseCursor = 0
		return m, nil
	case keyBottom:
		if m.showHelp {
			m.helpScroll = 999999
			return m, nil
		}
		m.browseCursor = len(m.browseDirEntries)
		return m, nil
	case keyPageUp:
		if m.showHelp {
			m.helpScroll = max(0, m.helpScroll-pageStep(m.mainBodyHeight()))
			return m, nil
		}
		m.browseCursor = max(0, m.browseCursor-pageStep(m.importListVisibleItems(m.importContentHeight())))
		return m, nil
	case keyPageDown:
		if m.showHelp {
			m.helpScroll += pageStep(m.mainBodyHeight())
			return m, nil
		}
		m.browseCursor = min(len(m.browseDirEntries), m.browseCursor+pageStep(m.importListVisibleItems(m.importContentHeight())))
		return m, nil
	case keyHelp:
		m.showHelp = !m.showHelp
		m.helpScroll = 0
		return m, nil
	case keyScope:
		return m, m.toggleImportScope()
	}

	switch msg.String() {
	case "enter":
		if m.browseCursor == 0 {
			parent := filepath.Dir(m.browseDir)
			if parent != m.browseDir {
				return m, loadBrowseDirCmd(parent)
			}
			return m, nil
		}
		entryIndex := m.browseCursor - 1
		if entryIndex >= 0 && entryIndex < len(m.browseDirEntries) {
			subdir := filepath.Join(m.browseDir, m.browseDirEntries[entryIndex])
			return m, loadBrowseDirCmd(subdir)
		}
		return m, nil
	case "backspace":
		parent := filepath.Dir(m.browseDir)
		if parent != m.browseDir {
			return m, loadBrowseDirCmd(parent)
		}
		return m, nil
	case "s":
		m.importCustomDir = m.browseDir
		m.closeBrowse()
		m.cursor = 0
		m.loading = true
		m.importPreview = nil
		return m, loadImportCandidatesFromDirCmd(m.svc, m.browseDir)
	case "esc":
		m.closeBrowse()
		return m, nil
	}

	return m, nil
}

func (m Model) importSelectedCandidate() (tea.Model, tea.Cmd) {
	candidate := m.selectedImportCandidate()
	if candidate == nil {
		return m, nil
	}
	if !candidate.Ready {
		m.status = "import blocked: " + candidate.Problem
		return m, nil
	}
	m.status = "importing " + string(candidate.SkillName) + "..."
	return m, importSkillCmd(m.svc, candidate.SourceDir, candidate.Targets, m.importCommit)
}

func (m Model) handleSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.settingsEditing {
		switch msg.String() {
		case "esc":
			m.cancelSettingsEdit()
			return m, nil
		case "enter":
			m.commitSettingsEdit()
			return m, nil
		case "backspace":
			value := m.selectedSettingsValue()
			if len(value) > 0 {
				m.setSelectedSettingsValue(value[:len(value)-1])
			}
			return m, nil
		default:
			if len(msg.String()) == 1 {
				m.setSelectedSettingsValue(m.selectedSettingsValue() + msg.String())
			}
			return m, nil
		}
	}

	switch classifyKey(msg) {
	case keyQuit:
		return m, tea.Quit
	case keyUp:
		if m.showHelp {
			m.helpScroll = max(0, m.helpScroll-1)
			return m, nil
		}
		if m.settingsField > 0 {
			m.settingsField--
		}
		return m, nil
	case keyDown:
		if m.showHelp {
			m.helpScroll++
			return m, nil
		}
		if m.settingsField < settingsFieldCount-1 {
			m.settingsField++
		}
		return m, nil
	case keyTop:
		if m.showHelp {
			m.helpScroll = 0
			return m, nil
		}
		m.settingsField = settingsFieldRepo
		return m, nil
	case keyBottom:
		if m.showHelp {
			m.helpScroll = 999999
			return m, nil
		}
		m.settingsField = settingsFieldCount - 1
		return m, nil
	case keyPageUp:
		if m.showHelp {
			m.helpScroll = max(0, m.helpScroll-pageStep(m.mainBodyHeight()))
			return m, nil
		}
		m.settingsField = settingsField(max(0, int(m.settingsField)-pageStep(m.settingsVisibleFields(m.mainBodyHeight()))))
		return m, nil
	case keyPageDown:
		if m.showHelp {
			m.helpScroll += pageStep(m.mainBodyHeight())
			return m, nil
		}
		next := int(m.settingsField) + pageStep(m.settingsVisibleFields(m.mainBodyHeight()))
		last := int(settingsFieldCount - 1)
		if next > last {
			next = last
		}
		m.settingsField = settingsField(next)
		return m, nil
	case keyHelp:
		m.showHelp = !m.showHelp
		m.helpScroll = 0
		return m, nil
	case keySettings:
		m.closeSettings()
		return m, nil
	case keyFilterClear:
		m.closeSettings()
		return m, nil
	case keySave:
		cfg, err := m.settingsConfig()
		if err != nil {
			m.status = "settings error: " + err.Error()
			return m, nil
		}
		m.status = "saving settings..."
		return m, saveSettingsCmd(cfg)
	}

	if msg.String() == "enter" {
		if m.selectedSettingsFieldEditable() {
			m.startSettingsEdit()
		} else {
			m.toggleSelectedSettingsTarget()
		}
		return m, nil
	}

	return m, nil
}

func (m *Model) refreshCurrentScreen() tea.Cmd {
	if m.inImportScreen() {
		m.loading = true
		return tea.Batch(startImportCmd(m.svc, m.projectRoot), loadSyncStatusCmd(m.svc, m.projectRoot))
	}
	if m.inProjectMode() {
		m.clearDeleteState()
		m.loading = true
		return tea.Batch(
			loadSkillsCmd(m.svc, m.projectRoot),
			projectImportHintCmd(m.svc, m.projectRoot),
			loadSyncStatusCmd(m.svc, m.projectRoot),
		)
	}
	return m.refreshAndLoad()
}

func syncResultStatus(result app.SyncResult) string {
	parts := []string{"sync complete"}
	if result.Bootstrapped {
		parts = append(parts, "published first local commit")
	} else {
		if result.Pushed {
			parts = append(parts, "pushed repo changes")
		}
		if result.Pulled {
			parts = append(parts, "pulled repo changes")
		}
	}
	if result.RefreshedUser > 0 {
		parts = append(parts, pluralize(result.RefreshedUser, "user install", "user installs")+" refreshed")
	}
	if result.RefreshedProject > 0 {
		parts = append(parts, pluralize(result.RefreshedProject, "project install", "project installs")+" refreshed")
	}
	return strings.Join(parts, "; ")
}

func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", count, plural)
}

func formatBulkImportStatus(imported, skipped, errors int, autoCommit bool) string {
	var parts []string
	parts = append(parts, "imported "+pluralize(imported, "skill", "skills"))
	if skipped > 0 {
		parts = append(parts, "skipped "+pluralize(skipped, "blocked", "blocked"))
	}
	if errors > 0 {
		parts = append(parts, pluralize(errors, "error", "errors"))
	}
	status := strings.Join(parts, "; ")
	if !autoCommit && imported > 0 {
		status += " without committing repo changes"
	}
	return status
}
