package tui

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sethdeckard/loadout/internal/app"
	"github.com/sethdeckard/loadout/internal/config"
	"github.com/sethdeckard/loadout/internal/domain"
	"github.com/sethdeckard/loadout/internal/fsx"
	"github.com/sethdeckard/loadout/internal/reconcile"
)

type paneID int

const (
	paneSkills paneID = iota
	paneDetails
)

type screenID int

const (
	screenInventory screenID = iota
	screenImport
	screenSettings
)

type settingsField int

const (
	settingsFieldRepo settingsField = iota
	settingsFieldClaudeEnabled
	settingsFieldClaudePath
	settingsFieldCodexEnabled
	settingsFieldCodexPath
	settingsFieldImportAutoCommit
	settingsFieldDeleteAutoCommit
	settingsFieldCount
)

type settingsForm struct {
	RepoPath          string
	ClaudeEnabled     bool
	ClaudePath        string
	CodexEnabled      bool
	CodexPath         string
	ImportAutoCommit  bool
	DeleteAutoCommit  bool
	originalValue     string
	originalBoolValue bool
}

type bulkImportState struct {
	ReadyCount   int
	SkippedCount int
	AutoCommit   bool
}

type commitPromptState struct {
	ActionKind        string
	SkillName         domain.SkillName
	RepoPath          string
	CommitMessage     string
	CommittedStatus   string
	UncommittedStatus string
	CloseImport       bool
}

type Model struct {
	svc *app.Service

	// Data
	skills        []app.SkillView
	filtered      []app.SkillView
	preview       *app.SkillPreview
	importPreview *app.ImportPreview
	doctor        *app.DoctorReport
	imports       []app.ImportCandidateView

	// Project scope
	detectedProject  string // project root detected at startup (for toggle)
	projectRoot      string // empty = user mode, non-empty = project mode
	projectHintCount int
	syncAttention    bool

	// UI state
	cursor          int
	filter          string
	filtering       bool
	showHelp        bool
	helpScroll      int
	status          string
	err             error
	loading         bool
	focusPane       paneID // which pane has focus (default: paneSkills)
	detailScroll    int    // vertical scroll offset for details pane
	screen          screenID
	importCommit    bool
	settings        settingsForm
	settingsField   settingsField
	settingsEditing bool
	commitPrompt    *commitPromptState
	deleteSkillName domain.SkillName
	deleteInput     string
	deleteCommit    bool
	deleteReady     bool
	deleteBlockers  []string
	bulkImport      *bulkImportState

	// Directory browser state
	importBrowsing   bool     // true = directory browser active
	browseDir        string   // current directory being browsed
	browseDirEntries []string // sorted child directory names (including hidden)
	browseCursor     int      // cursor in directory browser
	importCustomDir  string   // non-empty = scanning this dir instead of configured roots
	importStartDir   string   // working directory used as the default import root

	// Dimensions
	width  int
	height int
}

func NewModel(svc *app.Service, projectRoot string, userScope bool) Model {
	m := Model{
		svc:             svc,
		detectedProject: projectRoot,
		loading:         true,
	}
	if projectRoot != "" && !userScope {
		m.projectRoot = projectRoot
		m.status = "project scope: " + shortenHomePath(projectRoot)
	}
	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		loadSkillsCmd(m.svc, m.projectRoot),
		loadSyncStatusCmd(m.svc, m.projectRoot),
	}
	if m.projectRoot != "" {
		cmds = append(cmds, projectImportHintCmd(m.svc, m.projectRoot))
	}
	return tea.Batch(cmds...)
}

func (m Model) inProjectMode() bool {
	return m.projectRoot != ""
}

func (m Model) inSettingsScreen() bool {
	return m.screen == screenSettings
}

func (m Model) inImportScreen() bool {
	return m.screen == screenImport
}

func (m Model) selectedSkill() *app.SkillView {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return nil
	}
	return &m.filtered[m.cursor]
}

func (m Model) selectedSkillIsUnmanaged() bool {
	sel := m.selectedSkill()
	if sel == nil {
		return false
	}
	for _, flag := range sel.Flags {
		if flag == reconcile.StatusUnmanaged {
			return true
		}
	}
	return false
}

func (m *Model) blockUnmanagedSelectionAction() bool {
	sel := m.selectedSkill()
	if sel == nil {
		return false
	}
	if !m.selectedSkillIsUnmanaged() {
		return false
	}
	m.status = string(sel.Skill.Name) + " is not in repo; press i to import it"
	return true
}

func (m Model) selectedImportCandidate() *app.ImportCandidateView {
	if len(m.imports) == 0 || m.cursor >= len(m.imports) {
		return nil
	}
	return &m.imports[m.cursor]
}

func (m Model) previewCmdForSkill(sel *app.SkillView) tea.Cmd {
	if sel.LocalRoot != "" {
		return loadLocalPreviewCmd(m.svc, sel.Skill.Name, sel.LocalRoot)
	}
	return loadPreviewCmd(m.svc, sel.Skill.Name)
}

func (m *Model) applyFilter() {
	if m.filter == "" {
		m.filtered = m.skills
		return
	}
	f := strings.ToLower(m.filter)
	m.filtered = nil
	for _, v := range m.skills {
		if strings.Contains(strings.ToLower(string(v.Skill.Name)), f) ||
			matchTags(v.Skill.Tags, f) {
			m.filtered = append(m.filtered, v)
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func matchTags(tags []string, query string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), query) {
			return true
		}
	}
	return false
}

func (m *Model) loadSettingsFromConfig() {
	m.settings = settingsForm{
		RepoPath:         m.svc.Config.RepoPath,
		ClaudeEnabled:    m.svc.Config.Targets.Claude.Enabled,
		ClaudePath:       m.svc.Config.Targets.Claude.Path,
		CodexEnabled:     m.svc.Config.Targets.Codex.Enabled,
		CodexPath:        m.svc.Config.Targets.Codex.Path,
		ImportAutoCommit: m.svc.Config.RepoActions.ImportAutoCommit,
		DeleteAutoCommit: m.svc.Config.RepoActions.DeleteAutoCommit,
	}
}

func (m *Model) openSettings() {
	m.clearDeleteState()
	m.clearCommitPrompt()
	m.screen = screenSettings
	m.showHelp = false
	m.filtering = false
	m.settingsField = settingsFieldRepo
	m.settingsEditing = false
	m.loadSettingsFromConfig()
}

func (m *Model) openImport() tea.Cmd {
	m.clearDeleteState()
	m.clearCommitPrompt()
	m.screen = screenImport
	m.showHelp = false
	m.filtering = false
	m.importCommit = m.svc.Config.RepoActions.ImportAutoCommit
	m.resetImportState()
	return m.startImportForCurrentScope()
}

func (m *Model) closeSettings() {
	m.clearDeleteState()
	m.clearCommitPrompt()
	m.screen = screenInventory
	m.settingsEditing = false
	m.settings.originalValue = ""
	m.settings.originalBoolValue = false
}

func (m *Model) closeImport() {
	m.clearDeleteState()
	m.clearCommitPrompt()
	m.screen = screenInventory
	m.resetImportState()
}

func (m *Model) resetImportState() {
	m.importBrowsing = false
	m.importCustomDir = ""
	m.imports = nil
	m.importPreview = nil
	m.browseDir = ""
	m.browseCursor = 0
	m.browseDirEntries = nil
	m.importStartDir = ""
	m.cursor = 0
}

func (m Model) deleteConfirming() bool {
	return m.deleteSkillName != ""
}

func (m Model) commitPromptActive() bool {
	return m.commitPrompt != nil
}

func (m *Model) clearDeleteState() {
	m.deleteSkillName = ""
	m.deleteInput = ""
	m.deleteReady = false
	m.deleteBlockers = nil
}

func (m *Model) clearCommitPrompt() {
	m.commitPrompt = nil
}

func (m Model) bulkImportConfirming() bool {
	return m.bulkImport != nil
}

func (m *Model) resetBulkImport() {
	m.bulkImport = nil
}

func (m *Model) openBrowse() tea.Cmd {
	dir := m.importCustomDir
	if dir == "" {
		dir = m.importStartDir
	}
	if dir == "" {
		dir, _ = os.Getwd()
	}
	if dir == "" {
		dir = fsx.HomeOrRoot()
	}
	return m.openBrowseAt(dir)
}

func (m *Model) openBrowseAt(dir string) tea.Cmd {
	m.importBrowsing = true
	m.browseCursor = 0
	m.importPreview = nil
	m.browseDir = dir
	return loadBrowseDirCmd(dir)
}

func (m *Model) closeBrowse() {
	m.importBrowsing = false
	m.browseCursor = 0
}

func (m *Model) reloadImportForCurrentScope(status string) tea.Cmd {
	m.resetImportState()
	m.status = status
	return m.startImportForCurrentScope()
}

func (m *Model) refreshImportSource() tea.Cmd {
	m.imports = nil
	m.importPreview = nil
	m.cursor = 0
	m.loading = true
	if m.importCustomDir != "" {
		return loadImportCandidatesFromDirCmd(m.svc, m.importCustomDir)
	}
	return startImportCmd(m.svc, m.projectRoot)
}

func (m *Model) startImportForCurrentScope() tea.Cmd {
	if m.inProjectMode() {
		dir := m.projectRoot
		if dir == "" {
			dir = m.detectedProject
		}
		if dir == "" {
			dir, _ = os.Getwd()
		}
		if dir == "" {
			dir = fsx.HomeOrRoot()
		}
		m.loading = false
		m.importStartDir = dir
		return m.openBrowseAt(dir)
	}
	m.loading = true
	return startImportCmd(m.svc, m.projectRoot)
}

func (m Model) loadSelectedImportPreviewCmd() tea.Cmd {
	sel := m.selectedImportCandidate()
	if sel == nil {
		return nil
	}
	return loadImportPreviewCmd(m.svc, sel.SourceDir, sel.Targets)
}

func (m *Model) moveImportCursor(cursor int) tea.Cmd {
	m.cursor = clampIndex(cursor, len(m.imports))
	return m.loadSelectedImportPreviewCmd()
}

func (m *Model) moveSkillCursor(cursor int) tea.Cmd {
	cursor = clampIndex(cursor, len(m.filtered))
	if cursor == m.cursor {
		return nil
	}
	m.cursor = cursor
	m.clearDeleteState()
	m.detailScroll = 0
	m.doctor = nil
	if sel := m.selectedSkill(); sel != nil {
		return m.previewCmdForSkill(sel)
	}
	return nil
}

func (m *Model) toggleImportScope() tea.Cmd {
	if m.detectedProject == "" {
		m.status = "no project detected (need git repo with .claude/ or .codex/)"
		return nil
	}
	if m.inProjectMode() {
		m.projectRoot = ""
		return m.reloadImportForCurrentScope("import scope: user")
	}
	m.projectRoot = m.detectedProject
	return m.reloadImportForCurrentScope("import scope: project")
}

func (m Model) selectedSettingsValue() string {
	switch m.settingsField {
	case settingsFieldRepo:
		return m.settings.RepoPath
	case settingsFieldClaudePath:
		return m.settings.ClaudePath
	case settingsFieldCodexPath:
		return m.settings.CodexPath
	default:
		return ""
	}
}

func (m *Model) setSelectedSettingsValue(value string) {
	switch m.settingsField {
	case settingsFieldRepo:
		m.settings.RepoPath = value
	case settingsFieldClaudePath:
		m.settings.ClaudePath = value
	case settingsFieldCodexPath:
		m.settings.CodexPath = value
	}
}

func (m *Model) startSettingsEdit() {
	if !m.selectedSettingsFieldEditable() {
		return
	}
	m.settingsEditing = true
	m.settings.originalValue = m.selectedSettingsValue()
}

func (m *Model) commitSettingsEdit() {
	m.settingsEditing = false
	m.settings.originalValue = ""
}

func (m *Model) cancelSettingsEdit() {
	m.setSelectedSettingsValue(m.settings.originalValue)
	m.settingsEditing = false
	m.settings.originalValue = ""
}

func (m Model) selectedSettingsFieldEditable() bool {
	switch m.settingsField {
	case settingsFieldRepo, settingsFieldClaudePath, settingsFieldCodexPath:
		return true
	default:
		return false
	}
}

func (m *Model) toggleSelectedSettingsTarget() {
	switch m.settingsField {
	case settingsFieldClaudeEnabled:
		m.settings.ClaudeEnabled = !m.settings.ClaudeEnabled
	case settingsFieldCodexEnabled:
		m.settings.CodexEnabled = !m.settings.CodexEnabled
	case settingsFieldImportAutoCommit:
		m.settings.ImportAutoCommit = !m.settings.ImportAutoCommit
	case settingsFieldDeleteAutoCommit:
		m.settings.DeleteAutoCommit = !m.settings.DeleteAutoCommit
	}
}

func (m *Model) beginDeleteConfirm(name domain.SkillName, blockers []string) {
	m.clearCommitPrompt()
	m.deleteSkillName = name
	m.deleteInput = ""
	m.deleteCommit = m.svc.Config.RepoActions.DeleteAutoCommit
	m.deleteReady = len(blockers) == 0
	m.deleteBlockers = append([]string(nil), blockers...)
}

func (m *Model) beginCommitPrompt(actionKind string, skillName domain.SkillName, repoPath, commitMessage, committedStatus, uncommittedStatus string, closeImport bool) {
	m.clearDeleteState()
	m.commitPrompt = &commitPromptState{
		ActionKind:        actionKind,
		SkillName:         skillName,
		RepoPath:          repoPath,
		CommitMessage:     commitMessage,
		CommittedStatus:   committedStatus,
		UncommittedStatus: uncommittedStatus,
		CloseImport:       closeImport,
	}
}

func (m *Model) beginDeleteForSelection() tea.Cmd {
	sel := m.selectedSkill()
	if sel == nil {
		return nil
	}
	if m.selectedSkillIsUnmanaged() {
		m.status = string(sel.Skill.Name) + " is not in repo; press i to import it"
		return nil
	}
	m.status = "checking delete eligibility for " + string(sel.Skill.Name) + "..."
	return deleteEligibilityCmd(m.svc, sel.Skill.Name, m.detectedProject)
}

func (m *Model) settingsConfig() (config.Config, error) {
	repoPath := strings.TrimSpace(expandHome(m.settings.RepoPath))
	claudePath := strings.TrimSpace(expandHome(m.settings.ClaudePath))
	codexPath := strings.TrimSpace(expandHome(m.settings.CodexPath))

	if repoPath == "" {
		return config.Config{}, errRequiredSetting("repo path")
	}
	if m.settings.ClaudeEnabled && claudePath == "" {
		return config.Config{}, errRequiredSetting("claude skills path")
	}
	if m.settings.CodexEnabled && codexPath == "" {
		return config.Config{}, errRequiredSetting("codex skills path")
	}

	return config.Config{
		RepoPath: repoPath,
		Targets: config.TargetPaths{
			Claude: config.TargetConfig{
				Enabled: m.settings.ClaudeEnabled,
				Path:    claudePath,
			},
			Codex: config.TargetConfig{
				Enabled: m.settings.CodexEnabled,
				Path:    codexPath,
			},
		},
		RepoActions: config.RepoActions{
			ImportAutoCommit: m.settings.ImportAutoCommit,
			DeleteAutoCommit: m.settings.DeleteAutoCommit,
		},
	}, nil
}

func (m *Model) refreshAndLoad() tea.Cmd {
	m.clearDeleteState()
	m.clearCommitPrompt()
	m.loading = true
	cmds := []tea.Cmd{
		loadSkillsCmd(m.svc, ""),
		loadSyncStatusCmd(m.svc, m.projectRoot),
	}
	if m.inProjectMode() {
		cmds[0] = loadSkillsCmd(m.svc, m.projectRoot)
		cmds = append(cmds, projectImportHintCmd(m.svc, m.projectRoot))
	}
	if sel := m.selectedSkill(); sel != nil {
		cmds = append(cmds, m.previewCmdForSkill(sel))
	}
	return tea.Batch(cmds...)
}

func (m *Model) projectToggle(target domain.Target) tea.Cmd {
	sel := m.selectedSkill()
	if sel == nil {
		return nil
	}
	var installed bool
	switch target {
	case domain.TargetClaude:
		installed = sel.ProjectClaude
	case domain.TargetCodex:
		installed = sel.ProjectCodex
	}
	if !sel.Skill.SupportsTarget(target) {
		m.status = string(sel.Skill.Name) + " does not support " + string(target)
		return nil
	}
	action := "equipping"
	if installed {
		action = "unequipping"
	}
	m.status = action + " " + string(target) + " for " + string(sel.Skill.Name) + "..."
	return projectToggleCmd(m.svc, sel.Skill.Name, target, m.projectRoot, installed)
}

func (m *Model) toggleTarget(target domain.Target) tea.Cmd {
	sel := m.selectedSkill()
	if sel == nil {
		return nil
	}
	if !sel.Skill.SupportsTarget(target) {
		m.status = string(sel.Skill.Name) + " does not support " + string(target)
		return nil
	}
	var installed bool
	switch target {
	case domain.TargetClaude:
		installed = sel.InstalledClaude
	case domain.TargetCodex:
		installed = sel.InstalledCodex
	}
	action := "equipping"
	if installed {
		action = "unequipping"
	}
	m.status = action + " " + string(target) + " for " + string(sel.Skill.Name) + "..."
	return toggleTargetCmd(m.svc, sel.Skill.Name, target, !installed)
}

type requiredSettingError string

func (e requiredSettingError) Error() string {
	return string(e)
}

func errRequiredSetting(name string) error {
	return requiredSettingError(name + " is required")
}

func expandHome(path string) string {
	if path == "" || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == "~" {
		return home
	}
	if len(path) > 1 && path[1] == '/' {
		return home + path[1:]
	}
	return path
}

func (m Model) enabledTargets() []domain.Target {
	return m.svc.Config.Targets.ConfiguredTargets()
}
