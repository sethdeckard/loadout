package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sethdeckard/loadout/internal/app"
	"github.com/sethdeckard/loadout/internal/config"
	"github.com/sethdeckard/loadout/internal/domain"
	"github.com/sethdeckard/loadout/internal/reconcile"
)

var (
	testProject = filepath.Join(os.TempDir(), "project")
	testHome    = filepath.Join(os.TempDir(), "home", "user")
	testShared  = filepath.Join(os.TempDir(), "shared")
	testRepo    = filepath.Join(os.TempDir(), "repo")
	testClaude  = filepath.Join(os.TempDir(), "claude")
	testCodex   = filepath.Join(os.TempDir(), "codex")
)

func testModel() Model {
	m := Model{
		svc: &app.Service{
			Config: config.Config{
				RepoPath: testRepo,
				Targets: config.TargetPaths{
					Claude: config.TargetConfig{Enabled: true, Path: testClaude},
					Codex:  config.TargetConfig{Enabled: true, Path: testCodex},
				},
				RepoActions: config.RepoActions{
					ImportAutoCommit: true,
					DeleteAutoCommit: true,
				},
			},
		},
		width:  120,
		height: 40,
		skills: []app.SkillView{
			{
				Skill: domain.Skill{
					Name:    "swift-refactor",
					Tags:    []string{"swift", "ios"},
					Targets: []domain.Target{domain.TargetClaude, domain.TargetCodex},
					Path:    "skills/swift-refactor",
				},
				Flags: []reconcile.StatusFlag{reconcile.StatusInactive},
			},
			{
				Skill: domain.Skill{
					Name:    "rails-review",
					Tags:    []string{"ruby"},
					Targets: []domain.Target{domain.TargetClaude},
					Path:    "skills/rails-review",
				},
				InstalledClaude: true,
				Flags:           []reconcile.StatusFlag{reconcile.StatusCurrent},
			},
		},
	}
	m.applyFilter()
	return m
}

func addTestSkills(m *Model, count int) {
	for i := 0; i < count; i++ {
		name := domain.SkillName("skill-" + string(rune('a'+(i%26))) + string(rune('a'+((i/26)%26))))
		m.skills = append(m.skills, app.SkillView{
			Skill: domain.Skill{
				Name:    name,
				Targets: []domain.Target{domain.TargetClaude},
				Path:    "skills/" + string(name),
			},
			Flags: []reconcile.StatusFlag{reconcile.StatusInactive},
		})
	}
	m.applyFilter()
}

func importCandidates(count int) []app.ImportCandidateView {
	candidates := make([]app.ImportCandidateView, 0, count)
	for i := 0; i < count; i++ {
		name := domain.SkillName("import-" + string(rune('a'+(i%26))) + string(rune('a'+((i/26)%26))))
		candidates = append(candidates, app.ImportCandidateView{
			SkillName: name,
			SourceDir: filepath.Join("/tmp", string(name)),
			Targets:   []domain.Target{domain.TargetClaude},
			Ready:     true,
			FromRoots: []domain.Target{domain.TargetClaude},
		})
	}
	return candidates
}

func browseEntries(count int) []string {
	entries := make([]string, 0, count)
	for i := 0; i < count; i++ {
		entries = append(entries, "dir-"+string(rune('a'+(i%26)))+string(rune('a'+((i/26)%26))))
	}
	return entries
}

func ctrlKey(t tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: t}
}

func fieldLabel(field settingsField) string {
	switch field {
	case settingsFieldRepo:
		return "Repo Path"
	case settingsFieldClaudeEnabled:
		return "Claude Enabled"
	case settingsFieldClaudePath:
		return "Claude Skills Path"
	case settingsFieldCodexEnabled:
		return "Codex Enabled"
	case settingsFieldCodexPath:
		return "Codex Skills Path"
	case settingsFieldImportAutoCommit:
		return "Import Auto-Commit"
	case settingsFieldDeleteAutoCommit:
		return "Delete Auto-Commit"
	default:
		return ""
	}
}

func TestNavigationDown(t *testing.T) {
	m := testModel()
	if m.cursor != 0 {
		t.Fatalf("initial cursor = %d", m.cursor)
	}

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model := m2.(Model)
	if model.cursor != 1 {
		t.Errorf("cursor after j = %d, want 1", model.cursor)
	}
}

func TestNavigationUp(t *testing.T) {
	m := testModel()
	m.cursor = 1

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model := m2.(Model)
	if model.cursor != 0 {
		t.Errorf("cursor after k = %d, want 0", model.cursor)
	}
}

func TestNavigationBounds(t *testing.T) {
	m := testModel()
	m.cursor = 0

	// Up at top stays at 0
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model := m2.(Model)
	if model.cursor != 0 {
		t.Errorf("cursor after k at top = %d, want 0", model.cursor)
	}

	// Down at bottom stays at bottom
	m.cursor = len(m.filtered) - 1
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model = m2.(Model)
	if model.cursor != len(m.filtered)-1 {
		t.Errorf("cursor after j at bottom = %d, want %d", model.cursor, len(m.filtered)-1)
	}
}

func TestClassifyKey_PageKeys(t *testing.T) {
	if got := classifyKey(ctrlKey(tea.KeyCtrlU)); got != keyPageUp {
		t.Fatalf("ctrl+u action = %v, want %v", got, keyPageUp)
	}
	if got := classifyKey(ctrlKey(tea.KeyCtrlD)); got != keyPageDown {
		t.Fatalf("ctrl+d action = %v, want %v", got, keyPageDown)
	}
}

func TestNavigation_CtrlDPagesSkillList(t *testing.T) {
	m := testModel()
	addTestSkills(&m, 24)

	step := pageStep(m.skillListVisibleItems(m.skillListContentHeight()))
	m2, _ := m.Update(ctrlKey(tea.KeyCtrlD))
	model := m2.(Model)

	if got, want := model.cursor, step; got != want {
		t.Fatalf("cursor after ctrl+d = %d, want %d", got, want)
	}
}

func TestNavigation_CtrlUClampsSkillList(t *testing.T) {
	m := testModel()
	addTestSkills(&m, 24)
	m.cursor = 1

	m2, _ := m.Update(ctrlKey(tea.KeyCtrlU))
	model := m2.(Model)

	if got := model.cursor; got != 0 {
		t.Fatalf("cursor after ctrl+u = %d, want 0", got)
	}
}

func TestFilterMode(t *testing.T) {
	m := testModel()

	// Enter filter mode
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	model := m2.(Model)
	if !model.filtering {
		t.Error("expected filtering = true")
	}

	// Type characters
	m3, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	model = m3.(Model)
	if model.filter != "s" {
		t.Errorf("filter = %q, want 's'", model.filter)
	}

	m4, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	model = m4.(Model)
	if model.filter != "sw" {
		t.Errorf("filter = %q, want 'sw'", model.filter)
	}

	// Filter should reduce list
	if len(model.filtered) != 1 {
		t.Errorf("filtered = %d, want 1", len(model.filtered))
	}
	if model.filtered[0].Skill.Name != "swift-refactor" {
		t.Errorf("filtered[0].ID = %q", model.filtered[0].Skill.Name)
	}

	// Escape clears filter
	m5, _ := model.Update(tea.KeyMsg{Type: tea.KeyEscape})
	model = m5.(Model)
	if model.filtering {
		t.Error("expected filtering = false after esc")
	}
	if model.filter != "" {
		t.Errorf("filter = %q, want empty", model.filter)
	}
	if len(model.filtered) != 2 {
		t.Errorf("filtered = %d, want 2 after clear", len(model.filtered))
	}
}

func TestJumpTopBottom(t *testing.T) {
	m := testModel()

	// Jump to bottom
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	model := m2.(Model)
	if model.cursor != 1 {
		t.Errorf("cursor after G = %d, want 1", model.cursor)
	}

	// Jump to top
	m3, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model = m3.(Model)
	if model.cursor != 0 {
		t.Errorf("cursor after g = %d, want 0", model.cursor)
	}
}

func TestHelpToggle(t *testing.T) {
	m := testModel()

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	model := m2.(Model)
	if !model.showHelp {
		t.Error("expected showHelp = true")
	}

	m3, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	model = m3.(Model)
	if model.showHelp {
		t.Error("expected showHelp = false")
	}
}

func TestLoadSkillsMsg(t *testing.T) {
	m := Model{width: 120, height: 40, loading: true}
	views := []app.SkillView{
		{Skill: domain.Skill{Name: "s1"}},
	}

	m2, _ := m.Update(loadSkillsMsg{views: views})
	model := m2.(Model)
	if model.loading {
		t.Error("expected loading = false")
	}
	if len(model.skills) != 1 {
		t.Errorf("skills = %d, want 1", len(model.skills))
	}
}

func TestLoadSkillsMsg_ProjectModeAlsoRefreshesHint(t *testing.T) {
	m := Model{
		width:       120,
		height:      40,
		loading:     true,
		projectRoot: testProject,
		svc: &app.Service{
			Config: config.Config{
				RepoPath: testRepo,
				Targets: config.TargetPaths{
					Claude: config.TargetConfig{Enabled: true, Path: testClaude},
					Codex:  config.TargetConfig{Enabled: true, Path: testCodex},
				},
			},
		},
	}
	views := []app.SkillView{
		{Skill: domain.Skill{Name: "s1"}},
	}

	m2, cmd := m.Update(loadSkillsMsg{views: views})
	model := m2.(Model)
	if model.loading {
		t.Error("expected loading = false")
	}
	if cmd == nil {
		t.Fatal("expected project mode refresh commands")
	}
}

func TestLoadSkillsMsgError(t *testing.T) {
	m := Model{width: 120, height: 40, loading: true}

	m2, _ := m.Update(loadSkillsMsg{err: domain.ErrRepoNotFound})
	model := m2.(Model)
	if model.err == nil {
		t.Error("expected err to be set")
	}
}

func TestViewRendersWithoutPanic(t *testing.T) {
	m := testModel()
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view")
	}
}

func TestViewError(t *testing.T) {
	m := testModel()
	m.err = domain.ErrRepoNotFound
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view")
	}
}

func TestViewNarrow(t *testing.T) {
	m := testModel()
	m.width = 50
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view for narrow")
	}
}

func TestToggleTarget_UsesEquipLanguage(t *testing.T) {
	m := testModel()

	cmd := m.toggleTarget(domain.TargetClaude)
	if cmd == nil {
		t.Fatal("expected toggle command")
	}
	if got, want := m.status, "equipping claude for swift-refactor..."; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}

	m.cursor = 1
	cmd = m.toggleTarget(domain.TargetClaude)
	if cmd == nil {
		t.Fatal("expected toggle command")
	}
	if got, want := m.status, "unequipping claude for rails-review..."; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
}

func TestUpdate_ToggleResult_UsesEquipOutcome(t *testing.T) {
	m := testModel()

	m2, _ := m.Update(toggleResultMsg{
		name:     "swift-refactor",
		target:   domain.TargetClaude,
		equipped: true,
	})
	model := m2.(Model)
	if got, want := model.status, "equipped swift-refactor for claude"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}

	m3, _ := m.Update(toggleResultMsg{
		name:     "swift-refactor",
		target:   domain.TargetClaude,
		equipped: false,
	})
	model = m3.(Model)
	if got, want := model.status, "unequipped swift-refactor from claude"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
}

func TestRenderSkillList_UsesEquipActionLabels(t *testing.T) {
	m := testModel()
	m.cursor = 0

	list := m.renderSkillList(40, 12)
	if !strings.Contains(list, "c equip Claude (user)") {
		t.Fatalf("skill list missing equip action:\n%s", list)
	}
	if !strings.Contains(list, "x equip Codex (user)") {
		t.Fatalf("skill list missing codex action:\n%s", list)
	}
	if !strings.Contains(list, "a equip all (user)") {
		t.Fatalf("skill list missing bulk equip action:\n%s", list)
	}

	m.cursor = 1
	list = m.renderSkillList(40, 12)
	if !strings.Contains(list, "c unequip Claude (user)") {
		t.Fatalf("skill list missing unequip action:\n%s", list)
	}
	if !strings.Contains(list, "x equip Codex (n/a)") {
		t.Fatalf("skill list missing unavailable codex action:\n%s", list)
	}
	if !strings.Contains(list, "a equip all (n/a)") {
		t.Fatalf("skill list should keep dimmed unavailable bulk action:\n%s", list)
	}

	m.cursor = 0
	m.skills[0].InstalledClaude = true
	m.skills[0].InstalledCodex = true
	m.applyFilter()
	list = m.renderSkillList(40, 12)
	if !strings.Contains(list, "a unequip all (user)") {
		t.Fatalf("skill list should switch to bulk unequip when fully equipped:\n%s", list)
	}
}

func TestRenderFooter_OmitsPrimaryEquipActions(t *testing.T) {
	m := testModel()

	footer := m.renderFooter()
	if strings.Contains(footer, "equip Claude") || strings.Contains(footer, "equip Codex") || strings.Contains(footer, "equip all") {
		t.Fatalf("footer should not contain primary actions:\n%s", footer)
	}
	if !strings.Contains(footer, "settings") || !strings.Contains(footer, "project") {
		t.Fatalf("footer missing secondary actions:\n%s", footer)
	}
	if strings.Contains(footer, "delete") || strings.Contains(footer, "sync") {
		t.Fatalf("footer should not duplicate utility actions:\n%s", footer)
	}
	if !strings.Contains(footer, "import") {
		t.Fatalf("footer should show import action:\n%s", footer)
	}
}

func TestRenderHelp_UsesEquipLanguage(t *testing.T) {
	m := testModel()
	help := m.renderHelp(30)

	if !strings.Contains(help, "Equip or unequip the visible target rows") {
		t.Fatalf("help missing equip language:\n%s", help)
	}
	if !strings.Contains(help, "a               Equip all visible supported targets") {
		t.Fatalf("help missing bulk equip action:\n%s", help)
	}
	if strings.Contains(help, "Toggle Claude for selected skill") {
		t.Fatalf("help still contains toggle language:\n%s", help)
	}
	if !strings.Contains(help, "Delete selected repo skill") {
		t.Fatalf("help missing delete action:\n%s", help)
	}
	if !strings.Contains(help, "D               Delete selected repo skill") {
		t.Fatalf("help should advertise uppercase delete shortcut:\n%s", help)
	}
	if strings.Contains(help, "r               Delete selected repo skill") {
		t.Fatalf("help should not advertise lowercase delete shortcut:\n%s", help)
	}
}

func TestRenderSkillList_ShowsUppercaseDeleteAction(t *testing.T) {
	m := testModel()

	list := m.renderSkillList(40, 12)
	if !strings.Contains(list, "D delete repo copy") {
		t.Fatalf("skill list missing uppercase delete action:\n%s", list)
	}
	if strings.Contains(list, "r delete repo copy") {
		t.Fatalf("skill list should not show lowercase delete action:\n%s", list)
	}
}

func TestRenderSkillList_HidesActionsWhenEmpty(t *testing.T) {
	m := testModel()
	m.skills = nil
	m.applyFilter()

	list := m.renderSkillList(40, 12)
	if !strings.Contains(list, "No skills found") {
		t.Fatalf("empty skill list should show empty state:\n%s", list)
	}
	if strings.Contains(list, "equip all") || strings.Contains(list, "delete repo copy") {
		t.Fatalf("empty skill list should hide pane actions:\n%s", list)
	}
}

func TestRenderSkillList_KeepsCursorVisibleWhenPaged(t *testing.T) {
	m := testModel()
	addTestSkills(&m, 24)
	m.cursor = len(m.filtered) - 1

	list := m.renderSkillList(40, 12)
	selectedName := string(m.filtered[m.cursor].Skill.Name)
	firstName := string(m.filtered[0].Skill.Name)

	if !strings.Contains(list, "> [ ] "+selectedName) {
		t.Fatalf("skill list should keep selected row visible:\n%s", list)
	}
	if strings.Contains(list, firstName) {
		t.Fatalf("skill list should window away early rows when cursor is near bottom:\n%s", list)
	}
}

func TestRenderSkillList_SeparatesAndHighlightsDeleteAction(t *testing.T) {
	m := testModel()

	list := m.renderSkillList(40, 12)
	if fg := paneFooterDestructiveStyle.GetForeground(); fg == nil || fg == paneFooterActionStyle.GetForeground() {
		t.Fatalf("delete action should use a distinct destructive color style")
	}

	bulkIdx := strings.Index(list, "a equip all (user)")
	separatorIdx := strings.LastIndex(list, "────────────")
	deleteIdx := strings.Index(list, "D delete repo copy")
	if bulkIdx == -1 || separatorIdx == -1 || deleteIdx == -1 || !(bulkIdx < separatorIdx && separatorIdx < deleteIdx) {
		t.Fatalf("skill list should separate delete action from standard actions:\n%s", list)
	}
}

func TestModel_UserDeleteUsesUppercaseD(t *testing.T) {
	m := testModel()

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	model := m2.(Model)
	if cmd == nil {
		t.Fatal("expected delete eligibility command")
	}
	if got, want := model.status, "checking delete eligibility for swift-refactor..."; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
}

func TestModel_UnmanagedSelectionBlocksRepoActions(t *testing.T) {
	for _, tt := range []struct {
		name string
		key  tea.KeyMsg
	}{
		{"claude", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")}},
		{"codex", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")}},
		{"all", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}},
		{"delete", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := testModel()
			m.skills[0].InstalledClaude = true
			m.skills[0].Flags = []reconcile.StatusFlag{reconcile.StatusUnmanaged}
			m.skills[0].Skill.Targets = nil
			m.applyFilter()

			m2, cmd := m.Update(tt.key)
			model := m2.(Model)

			if cmd != nil {
				t.Fatal("expected no command for unmanaged selection")
			}
			if got, want := model.status, "swift-refactor is not in repo; press i to import it"; got != want {
				t.Fatalf("status = %q, want %q", got, want)
			}
			if model.deleteConfirming() {
				t.Fatal("expected delete flow to remain inactive")
			}
		})
	}
}

func TestModel_ProjectDeleteUsesUppercaseD(t *testing.T) {
	m := testModel()
	m.detectedProject = testProject
	m.projectRoot = testProject

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	model := m2.(Model)
	if cmd == nil {
		t.Fatal("expected delete eligibility command in project mode")
	}
	if got, want := model.status, "checking delete eligibility for swift-refactor..."; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
}

func TestModel_UserDeleteIgnoresLowercaseR(t *testing.T) {
	m := testModel()

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	model := m2.(Model)
	if cmd != nil {
		t.Fatal("expected lowercase r to do nothing in inventory mode")
	}
	if model.status != "" {
		t.Fatalf("status = %q, want empty", model.status)
	}
	if model.deleteConfirming() {
		t.Fatal("expected delete confirm to remain inactive")
	}
}

func TestDeleteEligibilityMsg_EntersConfirmState(t *testing.T) {
	m := testModel()

	m2, _ := m.Update(deleteEligibilityMsg{
		eligibility: app.DeleteEligibility{
			SkillName:   "swift-refactor",
			DeletedPath: "skills/swift-refactor",
			Deletable:   true,
		},
	})
	model := m2.(Model)
	if !model.deleteConfirming() {
		t.Fatal("expected delete confirm state")
	}
	if got, want := model.status, "type swift-refactor to confirm deletion"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
}

func TestDeleteEligibilityMsg_BlockedShowsReason(t *testing.T) {
	m := testModel()

	m2, _ := m.Update(deleteEligibilityMsg{
		eligibility: app.DeleteEligibility{
			SkillName: "swift-refactor",
			Deletable: false,
			Blockers:  []string{"managed user claude install exists"},
		},
	})
	model := m2.(Model)
	if !model.deleteConfirming() {
		t.Fatal("expected delete confirm state")
	}
	if model.deleteReady {
		t.Fatal("deleteReady = true, want false")
	}
	if got := model.View(); !strings.Contains(got, "managed user claude install exists") {
		t.Fatalf("delete modal missing blocker:\n%s", got)
	}
}

func TestDeleteConfirmKey_RequiresExactSkillName(t *testing.T) {
	m := testModel()
	m.beginDeleteConfirm("swift-refactor", nil)

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	model := m2.(Model)
	m3, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = m3.(Model)
	if got, want := model.status, "delete blocked: type swift-refactor exactly"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
}

func TestDeleteConfirmKey_AllowsTypingCommandLetters(t *testing.T) {
	m := testModel()
	m.beginDeleteConfirm("swift-refactor", nil)

	input := "swift-refactor"
	model := m
	for _, r := range input {
		next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		model = next.(Model)
	}

	if got := model.deleteInput; got != input {
		t.Fatalf("deleteInput = %q, want %q", got, input)
	}
}

func TestDeleteConfirmKey_TabTogglesAutoCommit(t *testing.T) {
	m := testModel()
	m.beginDeleteConfirm("swift-refactor", nil)

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := m2.(Model)
	if model.deleteCommit {
		t.Fatal("expected delete auto-commit to be disabled")
	}
	if got, want := model.status, "delete auto-commit disabled"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
}

func TestDeleteConfirmKey_EscapeCancels(t *testing.T) {
	m := testModel()
	m.beginDeleteConfirm("swift-refactor", nil)

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	model := m2.(Model)
	if model.deleteConfirming() {
		t.Fatal("expected delete confirm to be cleared")
	}
	if got, want := model.status, "delete cancelled"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
}

func TestDeleteSkillResult_EntersCommitPromptWhenNotCommitted(t *testing.T) {
	m := testModel()
	m.beginDeleteConfirm("swift-refactor", nil)

	m2, _ := m.Update(deleteSkillResultMsg{
		result: app.DeleteSkillResult{
			SkillName:     "swift-refactor",
			DeletedPath:   "skills/swift-refactor",
			CommitCreated: false,
		},
	})
	model := m2.(Model)
	if !model.commitPromptActive() {
		t.Fatal("expected commit prompt")
	}
	if got, want := model.commitPrompt.RepoPath, "skills/swift-refactor"; got != want {
		t.Fatalf("commitPrompt.RepoPath = %q, want %q", got, want)
	}
	if got, want := model.status, "deleted swift-refactor; commit repo change?"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
}

func TestCommitPrompt_EscapeLeavesChangeUncommitted(t *testing.T) {
	m := testModel()
	m.beginCommitPrompt("delete", "swift-refactor", "skills/swift-refactor", "Delete skill: swift-refactor", "deleted swift-refactor and committed repo change", "deleted swift-refactor without committing repo change", false)

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	model := m2.(Model)
	if model.commitPromptActive() {
		t.Fatal("expected commit prompt to clear")
	}
	if got, want := model.status, "deleted swift-refactor without committing repo change"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
}

func TestView_FitsTerminalHeight_WideShort(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 18
	m.preview = &app.SkillPreview{
		Skill:    m.filtered[0].Skill,
		Markdown: strings.Repeat("line\n", 80),
	}

	assertViewFitsHeight(t, m)
}

func TestView_FitsTerminalHeight_NarrowShort(t *testing.T) {
	m := testModel()
	m.width = 50
	m.height = 14
	m.preview = &app.SkillPreview{
		Skill:    m.filtered[0].Skill,
		Markdown: strings.Repeat("line\n", 40),
	}

	assertViewFitsHeight(t, m)
}

func TestView_FitsTerminalHeight_ProjectMode(t *testing.T) {
	m := testModel()
	m.width = 90
	m.height = 12
	m.detectedProject = testProject
	m.projectRoot = testProject
	m.skills[0].ProjectClaude = true
	m.skills[0].InstalledClaude = true
	m.applyFilter()

	assertViewFitsHeight(t, m)
}

func TestRenderProjectScopeList_UsesRepoInventory(t *testing.T) {
	m := testModel()
	m.detectedProject = testProject
	m.projectRoot = testProject

	list := m.renderSkillList(40, 12)
	if !strings.Contains(list, "swift-refactor") || !strings.Contains(list, "rails-review") {
		t.Fatalf("project list should still show repo inventory:\n%s", list)
	}
}

func TestRenderProjectScopeList_IncludesProjectActions(t *testing.T) {
	m := testModel()
	m.detectedProject = testProject
	m.projectRoot = testProject
	m.skills[0].ProjectClaude = true
	m.applyFilter()

	list := m.renderSkillList(40, 12)
	if !strings.Contains(list, "c unequip Claude (project)") {
		t.Fatalf("project list missing pane action:\n%s", list)
	}
	if !strings.Contains(list, "a equip all (project)") {
		t.Fatalf("project list should include project bulk action:\n%s", list)
	}
	if !strings.Contains(list, "D delete repo copy") {
		t.Fatalf("project list should include repo delete action:\n%s", list)
	}
	if strings.Contains(list, "import") {
		t.Fatalf("project list should not show standing import hint without ready skills:\n%s", list)
	}
}

func TestRenderProjectScopeList_HidesActionsWhenEmpty(t *testing.T) {
	m := testModel()
	m.detectedProject = testProject
	m.projectRoot = testProject
	m.skills = nil
	m.applyFilter()

	list := m.renderSkillList(40, 12)
	if !strings.Contains(list, "No skills found") {
		t.Fatalf("empty project list should show empty state:\n%s", list)
	}
	if strings.Contains(list, "equip all") || strings.Contains(list, "delete repo copy") {
		t.Fatalf("empty project list should hide pane actions:\n%s", list)
	}
}

func TestRenderProjectScopeList_ShowsNoImportHintWhenReadySkillsExist(t *testing.T) {
	m := testModel()
	m.detectedProject = testProject
	m.projectRoot = testProject
	m.projectHintCount = 2
	m.applyFilter()

	list := m.renderSkillList(40, 12)
	if strings.Contains(list, "import") {
		t.Fatalf("project list should not show project-level import hint:\n%s", list)
	}
}

func TestView_FitsTerminalHeight_Help(t *testing.T) {
	m := testModel()
	m.width = 80
	m.height = 10
	m.showHelp = true

	assertViewFitsHeight(t, m)
}

func TestView_FitsTerminalHeight_CompactFallback(t *testing.T) {
	m := testModel()
	m.width = 100
	m.height = 7
	m.preview = &app.SkillPreview{
		Skill:    m.filtered[0].Skill,
		Markdown: strings.Repeat("line\n", 20),
	}

	assertViewFitsHeight(t, m)
}

func TestSettings_OpenAndClose(t *testing.T) {
	m := testModel()

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	model := m2.(Model)
	if !model.inSettingsScreen() {
		t.Fatal("expected settings screen")
	}
	if got := model.settings.RepoPath; got != testRepo {
		t.Fatalf("repo path = %q, want %q", got, testRepo)
	}
	if !model.settings.ClaudeEnabled {
		t.Fatal("expected Claude to be enabled")
	}
	if !model.settings.ImportAutoCommit || !model.settings.DeleteAutoCommit {
		t.Fatalf("auto-commit settings = import:%v delete:%v, want both true", model.settings.ImportAutoCommit, model.settings.DeleteAutoCommit)
	}

	m3, _ := model.Update(tea.KeyMsg{Type: tea.KeyEscape})
	model = m3.(Model)
	if model.inSettingsScreen() {
		t.Fatal("expected inventory screen after esc")
	}
}

func TestSettings_ToggleTargetEnabled(t *testing.T) {
	m := testModel()
	m.openSettings()

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model := m2.(Model)
	if model.settingsField != settingsFieldClaudeEnabled {
		t.Fatalf("settingsField = %v, want ClaudeEnabled", model.settingsField)
	}

	m3, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = m3.(Model)
	if model.settings.ClaudeEnabled {
		t.Fatal("expected Claude to be disabled")
	}
	if model.settingsEditing {
		t.Fatal("enter on enabled field should toggle, not enter edit mode")
	}
}

func TestSettings_EditField(t *testing.T) {
	m := testModel()
	m.openSettings()

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := m2.(Model)
	if !model.settingsEditing {
		t.Fatal("expected editing mode")
	}

	m3, _ := model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	model = m3.(Model)
	if got := model.settings.RepoPath; got != testRepo[:len(testRepo)-1] {
		t.Fatalf("repo path after backspace = %q", got)
	}

	m4, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	model = m4.(Model)
	if got := model.settings.RepoPath; got != testRepo {
		t.Fatalf("repo path after typing = %q", got)
	}

	m5, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = m5.(Model)
	if model.settingsEditing {
		t.Fatal("expected edit mode to end on enter")
	}
}

func TestSettings_CancelFieldEdit(t *testing.T) {
	m := testModel()
	m.openSettings()

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := m2.(Model)
	m3, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	model = m3.(Model)
	m4, _ := model.Update(tea.KeyMsg{Type: tea.KeyEscape})
	model = m4.(Model)

	if model.settingsEditing {
		t.Fatal("expected edit mode to end on esc")
	}
	if got := model.settings.RepoPath; got != testRepo {
		t.Fatalf("repo path after cancel = %q, want %q", got, testRepo)
	}
}

func TestSettings_SaveUpdatesConfigAndWritesFile(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	m := testModel()
	m.openSettings()
	m.settings.RepoPath = "~/repo-new"
	m.settings.ImportAutoCommit = false
	m.settings.DeleteAutoCommit = false

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	model := m2.(Model)
	if cmd == nil {
		t.Fatal("expected save command")
	}
	if got, want := model.status, "saving settings..."; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}

	msg := cmd()
	m3, _ := model.Update(msg)
	model = m3.(Model)

	if model.inSettingsScreen() {
		t.Fatal("expected settings screen to close after save")
	}
	if got, want := model.svc.Config.RepoPath, filepath.Join(tempHome, "repo-new"); got != want {
		t.Fatalf("svc.Config.RepoPath = %q, want %q", got, want)
	}
	if got, want := model.status, "settings saved"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if model.svc.Config.RepoActions.ImportAutoCommit {
		t.Fatal("expected ImportAutoCommit to be false after save")
	}
	if model.svc.Config.RepoActions.DeleteAutoCommit {
		t.Fatal("expected DeleteAutoCommit to be false after save")
	}

	data, err := os.ReadFile(config.DefaultPath())
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !strings.Contains(string(data), filepath.Join(tempHome, "repo-new")) {
		t.Fatalf("saved config missing expanded repo path:\n%s", string(data))
	}
	if !strings.Contains(string(data), `"import_auto_commit": false`) || !strings.Contains(string(data), `"delete_auto_commit": false`) {
		t.Fatalf("saved config missing repo action defaults:\n%s", string(data))
	}
}

func TestSettings_SaveValidationError(t *testing.T) {
	m := testModel()
	m.openSettings()
	m.settings.RepoPath = "   "

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	model := m2.(Model)
	if cmd != nil {
		t.Fatal("expected no save command on validation error")
	}
	if got, want := model.status, "settings error: repo path is required"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
}

func TestSettings_SaveAllowsDisabledTargetWithoutPath(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	m := testModel()
	m.openSettings()
	m.settings.CodexEnabled = false
	m.settings.CodexPath = ""

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	model := m2.(Model)
	if cmd == nil {
		t.Fatal("expected save command")
	}

	msg := cmd()
	m3, _ := model.Update(msg)
	model = m3.(Model)
	if got := model.status; got != "settings saved" {
		t.Fatalf("status = %q, want settings saved", got)
	}
	if model.svc.Config.Targets.Codex.Enabled {
		t.Fatal("expected Codex to remain disabled after save")
	}
}

func TestRenderStatus_HidesDisabledTarget(t *testing.T) {
	m := testModel()
	m.svc.Config.Targets.Codex.Enabled = false

	status := m.renderStatus(60, 20)
	if strings.Contains(status, "Targets") {
		t.Fatalf("status should not show redundant targets heading:\n%s", status)
	}
	if strings.Contains(status, "Codex") {
		t.Fatalf("status should hide disabled target:\n%s", status)
	}
}

func TestRenderStatus_UsesLabelFirstTargetRows(t *testing.T) {
	m := testModel()
	m.skills[1].InstalledClaude = true
	m.applyFilter()
	m.cursor = 1

	status := m.renderStatus(60, 20)
	if !strings.Contains(status, "[x] Claude") {
		t.Fatalf("status should start rows with checkbox and target label:\n%s", status)
	}
	if !strings.Contains(status, "c unequip") {
		t.Fatalf("status should show compact action hint:\n%s", status)
	}
	if strings.Contains(status, "unequip Claude") {
		t.Fatalf("status should not repeat verbose action text:\n%s", status)
	}
	if strings.Contains(status, "c Claude") {
		t.Fatalf("status should not use key-first row formatting:\n%s", status)
	}
}

func TestRenderStatus_UnsupportedTargetsUseCompactNAHint(t *testing.T) {
	m := testModel()
	m.cursor = 1

	status := m.renderStatus(60, 20)
	if !strings.Contains(status, "[-] Codex") {
		t.Fatalf("status should show unavailable target marker:\n%s", status)
	}
	if !strings.Contains(status, "x n/a") {
		t.Fatalf("status should show compact unavailable hint:\n%s", status)
	}
}

func TestRenderHelp_IncludesSettingsShortcut(t *testing.T) {
	m := testModel()
	help := m.renderHelp(30)
	if !strings.Contains(help, "p               Open settings") {
		t.Fatalf("help missing settings shortcut:\n%s", help)
	}
	if !strings.Contains(help, "i               Open import for current scope") {
		t.Fatalf("help missing import shortcut:\n%s", help)
	}
	if !strings.Contains(help, "ctrl+u/d        Move or scroll half a page") {
		t.Fatalf("help missing page shortcut:\n%s", help)
	}
}

func TestView_FitsTerminalHeight_Settings(t *testing.T) {
	m := testModel()
	m.width = 80
	m.height = 12
	m.openSettings()

	assertViewFitsHeight(t, m)
}

func TestSettings_CtrlDPagesFields(t *testing.T) {
	m := testModel()
	m.openSettings()

	step := pageStep(m.settingsVisibleFields(m.mainBodyHeight()))
	m2, _ := m.Update(ctrlKey(tea.KeyCtrlD))
	model := m2.(Model)

	want := min(step, int(settingsFieldCount-1))
	if got := int(model.settingsField); got != want {
		t.Fatalf("settingsField after ctrl+d = %d, want %d", got, want)
	}
}

func TestRenderSettingsContent_KeepsSelectedFieldVisibleWhenPaged(t *testing.T) {
	m := testModel()
	m.openSettings()
	m.settingsField = settingsFieldDeleteAutoCommit

	content := m.renderSettingsContent(80, 12)
	if !strings.Contains(content, "> "+fieldLabel(settingsFieldDeleteAutoCommit)+":") {
		t.Fatalf("settings content should keep selected field visible:\n%s", content)
	}
	if strings.Contains(content, fieldLabel(settingsFieldRepo)+":") {
		t.Fatalf("settings content should window away early fields near the bottom:\n%s", content)
	}
}

func TestImport_OpenAndClose(t *testing.T) {
	m := testModel()

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	model := m2.(Model)
	if !model.inImportScreen() {
		t.Fatal("expected import screen")
	}
	if cmd == nil {
		t.Fatal("expected import startup command")
	}
	if !model.importCommit {
		t.Fatal("expected import auto-commit to seed from config")
	}

	m3, _ := model.Update(tea.KeyMsg{Type: tea.KeyEscape})
	model = m3.(Model)
	if model.inImportScreen() {
		t.Fatal("expected import screen to close")
	}
}

func TestImport_CtrlDPagesCandidates(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.imports = importCandidates(20)

	step := pageStep(m.importListVisibleItems(m.importContentHeight()))
	m2, _ := m.Update(ctrlKey(tea.KeyCtrlD))
	model := m2.(Model)

	if got, want := model.cursor, step; got != want {
		t.Fatalf("cursor after ctrl+d = %d, want %d", got, want)
	}
}

func TestImport_BrowseCtrlDPagesDirectories(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.importBrowsing = true
	m.browseDir = testHome
	m.browseDirEntries = browseEntries(20)

	step := pageStep(m.importListVisibleItems(m.importContentHeight()))
	m2, _ := m.Update(ctrlKey(tea.KeyCtrlD))
	model := m2.(Model)

	if got, want := model.browseCursor, step; got != want {
		t.Fatalf("browseCursor after ctrl+d = %d, want %d", got, want)
	}
}

func TestImport_OpenInProjectModeStartsBrowseAtProjectRoot(t *testing.T) {
	m := testModel()
	m.projectRoot = testProject

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	model := m2.(Model)
	if !model.inImportScreen() {
		t.Fatal("expected import screen")
	}
	if !model.importBrowsing {
		t.Fatal("expected browse mode in project scope")
	}
	if got, want := model.importStartDir, testProject; got != want {
		t.Fatalf("importStartDir = %q, want %q", got, want)
	}
	if got, want := model.browseDir, testProject; got != want {
		t.Fatalf("browseDir = %q, want %q", got, want)
	}
	if cmd == nil {
		t.Fatal("expected browse load command")
	}
}

func TestImport_StartShowsCandidatesFromCurrentDir(t *testing.T) {
	m := testModel()
	m.screen = screenImport

	views := []app.ImportCandidateView{{
		SkillName: "shared-skill",
		SourceDir: filepath.Join(testProject, "shared-skill"),
		Targets:   []domain.Target{domain.TargetClaude},
		Ready:     true,
	}}
	m2, cmd := m.Update(startImportMsg{dir: testProject, views: views})
	model := m2.(Model)
	if model.importBrowsing {
		t.Fatal("expected candidate mode when current dir has skills")
	}
	if model.importCustomDir != "" {
		t.Fatalf("importCustomDir = %q, want empty", model.importCustomDir)
	}
	if model.importStartDir != testProject {
		t.Fatalf("importStartDir = %q, want %q", model.importStartDir, testProject)
	}
	if len(model.imports) != 1 {
		t.Fatalf("imports = %d, want 1", len(model.imports))
	}
	if cmd == nil {
		t.Fatal("expected preview command")
	}
}

func TestImport_StartFallsBackToBrowseWhenCurrentDirHasNoSkills(t *testing.T) {
	m := testModel()
	m.screen = screenImport

	m2, cmd := m.Update(startImportMsg{dir: testProject})
	model := m2.(Model)
	if model.importBrowsing {
		t.Fatal("expected user import mode to remain in candidate list")
	}
	if model.importStartDir != testProject {
		t.Fatalf("importStartDir = %q, want %q", model.importStartDir, testProject)
	}
	if model.browseDir != "" {
		t.Fatalf("browseDir = %q, want empty", model.browseDir)
	}
	if cmd != nil {
		t.Fatal("expected no follow-up command without candidates")
	}
}

func TestImport_ToggleCommit(t *testing.T) {
	m := testModel()
	m.importCommit = true
	m.screen = screenImport

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	model := m2.(Model)
	if model.importCommit {
		t.Fatal("expected auto-commit to be disabled")
	}
}

func TestDeleteConfirm_SeedsCommitFromConfig(t *testing.T) {
	m := testModel()
	m.svc.Config.RepoActions.DeleteAutoCommit = true

	m.beginDeleteConfirm("swift-refactor", nil)
	if !m.deleteCommit {
		t.Fatal("expected delete auto-commit to seed from config")
	}

	m.deleteCommit = false
	m.beginDeleteConfirm("swift-refactor", nil)
	if !m.deleteCommit {
		t.Fatal("expected new delete flow to restore config default")
	}
}

func TestSettings_ToggleRepoActionDefaults(t *testing.T) {
	m := testModel()
	m.openSettings()
	m.settingsField = settingsFieldImportAutoCommit

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := m2.(Model)
	if model.settings.ImportAutoCommit {
		t.Fatal("expected ImportAutoCommit to toggle off")
	}

	model.settingsField = settingsFieldDeleteAutoCommit
	m3, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = m3.(Model)
	if model.settings.DeleteAutoCommit {
		t.Fatal("expected DeleteAutoCommit to toggle off")
	}
}

func TestImportSkillResult_EntersCommitPromptWhenNotCommitted(t *testing.T) {
	m := testModel()
	m.screen = screenImport

	m2, _ := m.Update(importSkillResultMsg{
		result: app.ImportResult{
			SkillName:     "shared-skill",
			RepoPath:      "skills/shared-skill",
			CommitCreated: false,
		},
	})
	model := m2.(Model)
	if !model.commitPromptActive() {
		t.Fatal("expected commit prompt")
	}
	if got, want := model.status, "imported shared-skill; commit repo change?"; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
	if !model.inImportScreen() {
		t.Fatal("expected import screen to remain active")
	}
}

func TestCommitPromptFooter_ShowsCommitActions(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.beginCommitPrompt("import", "shared-skill", "skills/shared-skill", "Add skill: shared-skill", "imported shared-skill and committed repo change", "imported shared-skill without committing repo change", true)

	footer := m.renderFooter()
	if !strings.Contains(footer, "commit") || !strings.Contains(footer, "leave uncommitted") {
		t.Fatalf("footer missing commit prompt actions:\n%s", footer)
	}
}

func TestView_ShowsCommitModal(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.beginCommitPrompt("import", "shared-skill", "skills/shared-skill", "Add skill: shared-skill", "imported shared-skill and committed repo change", "imported shared-skill without committing repo change", true)

	view := m.View()
	if !strings.Contains(view, "Repo Changed: commit now?") || !strings.Contains(view, "Enter to commit this repo change") {
		t.Fatalf("view missing commit modal:\n%s", view)
	}
	if !strings.Contains(view, "If you cancel, the repo stays dirty.") {
		t.Fatalf("view missing consequence copy:\n%s", view)
	}
}

func TestView_ShowsDeleteModal(t *testing.T) {
	m := testModel()
	m.beginDeleteConfirm("swift-refactor", nil)

	view := m.View()
	if !strings.Contains(view, "Delete Skill From Repo") || !strings.Contains(view, "Enter to delete this skill") {
		t.Fatalf("view missing delete modal:\n%s", view)
	}
}

func TestImport_LoadCandidates(t *testing.T) {
	m := testModel()
	views := []app.ImportCandidateView{{SkillName: "shared-skill", SourceDir: testShared, Ready: true}}

	m2, cmd := m.Update(loadImportCandidatesMsg{views: views})
	model := m2.(Model)
	if len(model.imports) != 1 {
		t.Fatalf("imports = %d, want 1", len(model.imports))
	}
	if cmd == nil {
		t.Fatal("expected preview load command")
	}
}

func TestImport_ViewContainsCandidate(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.imports = []app.ImportCandidateView{{
		SkillName: "shared-skill",
		SourceDir: testShared,
		Ready:     true,
		FromRoots: []domain.Target{domain.TargetClaude, domain.TargetCodex},
	}}

	v := m.View()
	if !strings.Contains(v, "shared-skill") {
		t.Fatalf("view missing candidate:\n%s", v)
	}
}

func TestView_FitsTerminalHeight_WithWrappedPaneContent(t *testing.T) {
	m := testModel()
	m.width = 98
	m.height = 22
	m.preview = &app.SkillPreview{
		Skill:    m.filtered[0].Skill,
		Markdown: strings.Repeat("Every piece of feedback must be a haiku. ", 12),
	}

	assertViewFitsHeight(t, m)
}

func TestView_CompactWidthDoesNotDuplicateFooter(t *testing.T) {
	m := testModel()
	m.width = 98
	m.height = 12

	view := m.View()
	if count := strings.Count(view, "h/l  focus"); count != 1 {
		t.Fatalf("view should render one app footer in compact mode, got %d:\n%s", count, view)
	}
}

func TestSelectedSkill(t *testing.T) {
	m := testModel()
	sel := m.selectedSkill()
	if sel == nil {
		t.Fatal("expected non-nil selectedSkill")
	}
	if sel.Skill.Name != "swift-refactor" {
		t.Errorf("selected ID = %q", sel.Skill.Name)
	}
}

func TestSelectedSkillEmpty(t *testing.T) {
	m := Model{}
	if m.selectedSkill() != nil {
		t.Error("expected nil for empty list")
	}
}

func TestNewModel_ProjectStartsInProjectScope(t *testing.T) {
	svc := &app.Service{
		Config: config.Config{
			RepoPath: testRepo,
			Targets: config.TargetPaths{
				Claude: config.TargetConfig{Enabled: true, Path: testClaude},
			},
		},
	}
	m := NewModel(svc, testProject, false)
	if !m.inProjectMode() {
		t.Error("expected project mode when projectRoot is provided")
	}
	if m.projectRoot != testProject {
		t.Errorf("projectRoot = %q, want %q", m.projectRoot, testProject)
	}
	if m.detectedProject != testProject {
		t.Errorf("detectedProject = %q, want %q", m.detectedProject, testProject)
	}
	if m.status == "" {
		t.Error("expected startup status message in project mode")
	}
	if !strings.Contains(m.status, "project scope:") {
		t.Errorf("status = %q, want it to contain %q", m.status, "project scope:")
	}
}

func TestNewModel_NoProjectStartsInUserScope(t *testing.T) {
	svc := &app.Service{
		Config: config.Config{
			RepoPath: testRepo,
			Targets: config.TargetPaths{
				Claude: config.TargetConfig{Enabled: true, Path: testClaude},
			},
		},
	}
	m := NewModel(svc, "", false)
	if m.inProjectMode() {
		t.Error("expected user mode when projectRoot is empty")
	}
	if m.status != "" {
		t.Errorf("status = %q, want empty in user mode", m.status)
	}
}

func TestNewModel_ProjectStartupInitLoadsProjectScope(t *testing.T) {
	svc := &app.Service{
		Config: config.Config{
			RepoPath: testRepo,
			Targets: config.TargetPaths{
				Claude: config.TargetConfig{Enabled: true, Path: testClaude},
			},
		},
	}
	m := NewModel(svc, testProject, false)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected Init() to return commands")
	}
}

func TestNewModel_UserScopePreservesDetectedProject(t *testing.T) {
	svc := &app.Service{
		Config: config.Config{
			RepoPath: testRepo,
			Targets: config.TargetPaths{
				Claude: config.TargetConfig{Enabled: true, Path: testClaude},
			},
		},
	}
	m := NewModel(svc, testProject, true)
	if m.inProjectMode() {
		t.Error("expected user mode with userScope=true")
	}
	if m.detectedProject != testProject {
		t.Errorf("detectedProject = %q, want %q", m.detectedProject, testProject)
	}
	if m.status != "" {
		t.Errorf("status = %q, want empty in user mode", m.status)
	}

	// Tab should still toggle to project scope since detectedProject is set
	m.width = 120
	m.height = 40
	m.skills = []app.SkillView{}
	m.applyFilter()
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := m2.(Model)
	if !model.inProjectMode() {
		t.Error("expected project mode after tab even with --user startup")
	}
	if model.projectRoot != testProject {
		t.Errorf("projectRoot = %q, want %q", model.projectRoot, testProject)
	}
}

func TestModel_ScopeToggle(t *testing.T) {
	m := testModel()
	m.detectedProject = filepath.Join(os.TempDir(), "my-project")
	m.projectRoot = filepath.Join(os.TempDir(), "my-project") // starts in project scope

	if !m.inProjectMode() {
		t.Error("expected project mode initially")
	}

	// Tab toggles to user mode
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := m2.(Model)
	if model.inProjectMode() {
		t.Error("expected user mode after tab")
	}
	if model.status != "" {
		t.Errorf("status = %q, want empty after entering user scope", model.status)
	}

	// Tab toggles back to project mode
	m3, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = m3.(Model)
	if !model.inProjectMode() {
		t.Error("expected project mode after second tab")
	}
	if model.projectRoot != filepath.Join(os.TempDir(), "my-project") {
		t.Errorf("projectRoot = %q, want %q", model.projectRoot, filepath.Join(os.TempDir(), "my-project"))
	}
	if model.status != "" {
		t.Errorf("status = %q, want empty after returning to project scope", model.status)
	}
}

func TestModel_ScopeTogglePreservesCursor(t *testing.T) {
	m := testModel()
	m.detectedProject = filepath.Join(os.TempDir(), "my-project")
	m.cursor = 1

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := m2.(Model)
	if got, want := model.cursor, 1; got != want {
		t.Fatalf("project cursor = %d, want %d", got, want)
	}

	m3, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = m3.(Model)
	if got, want := model.cursor, 1; got != want {
		t.Fatalf("user cursor after toggling back = %d, want %d", got, want)
	}
}

func TestModel_ScopeToggleNoProject(t *testing.T) {
	m := testModel()
	m.detectedProject = "" // No project detected

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := m2.(Model)
	if model.inProjectMode() {
		t.Error("should stay user when no project detected")
	}
	if model.status == "" {
		t.Error("expected status message about no project")
	}
}

func TestModel_ProjectModeAllowsSync(t *testing.T) {
	m := testModel()
	m.detectedProject = testProject
	m.projectRoot = testProject // already in project mode

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	model := m2.(Model)
	if cmd == nil {
		t.Fatal("expected sync command in project mode")
	}
	if got, want := model.status, "syncing repo and refreshing managed installs..."; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
}

func TestModel_ProjectModeAllowsImport(t *testing.T) {
	m := testModel()
	m.detectedProject = testProject
	m.projectRoot = testProject

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	model := m2.(Model)
	if !model.inImportScreen() {
		t.Fatal("expected import screen in project mode")
	}
	if cmd == nil {
		t.Fatal("expected import startup command")
	}
}

func TestModel_EquipAllOnlyInstallsMissingUserTargets(t *testing.T) {
	m := testModel()
	m.skills[0].InstalledClaude = true
	m.applyFilter()

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	model := m2.(Model)
	if cmd == nil {
		t.Fatal("expected equip-all command")
	}
	if got, want := model.status, "equipping supported targets..."; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}

	msg := cmd()
	result, ok := msg.(toggleResultMsg)
	if !ok {
		t.Fatalf("batch msg type = %T, want toggleResultMsg", msg)
	}
	if result.target != domain.TargetCodex {
		t.Fatalf("target = %q, want codex", result.target)
	}
	if !result.equipped {
		t.Fatal("expected equip-all to install missing target")
	}
}

func TestModel_EquipAllNoopsWhenUserTargetsAlreadyEquipped(t *testing.T) {
	m := testModel()
	m.skills[0].InstalledClaude = true
	m.skills[0].InstalledCodex = true
	m.applyFilter()

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	model := m2.(Model)
	if cmd == nil {
		t.Fatal("expected bulk unequip command")
	}
	if got, want := model.status, "unequipping supported targets..."; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}

	msg := cmd()
	result, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("cmd() type = %T, want tea.BatchMsg", msg)
	}
	if len(result) != 2 {
		t.Fatalf("batch len = %d, want 2", len(result))
	}
}

func TestModel_ProjectModeEquipAllOnlyInstallsMissingTargets(t *testing.T) {
	m := testModel()
	m.detectedProject = testProject
	m.projectRoot = testProject
	m.skills[0].ProjectClaude = true
	m.applyFilter()

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	model := m2.(Model)
	if cmd == nil {
		t.Fatal("expected equip commands")
	}
	if got, want := model.status, "equipping project targets..."; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}

	msg := cmd()
	result, ok := msg.(projectToggleResultMsg)
	if !ok {
		t.Fatalf("batch msg type = %T, want projectToggleResultMsg", msg)
	}
	if result.target != domain.TargetCodex {
		t.Fatalf("target = %q, want codex", result.target)
	}
	if !result.equipped {
		t.Fatal("expected project equip-all to install missing target")
	}
}

func TestModel_ProjectModeEquipAllNoopsWhenTargetsAlreadyEquipped(t *testing.T) {
	m := testModel()
	m.detectedProject = testProject
	m.projectRoot = testProject
	m.skills[0].ProjectClaude = true
	m.skills[0].ProjectCodex = true
	m.applyFilter()

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	model := m2.(Model)
	if cmd == nil {
		t.Fatal("expected bulk unequip command")
	}
	if got, want := model.status, "unequipping project targets..."; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}

	msg := cmd()
	result, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("cmd() type = %T, want tea.BatchMsg", msg)
	}
	if len(result) != 2 {
		t.Fatalf("batch len = %d, want 2", len(result))
	}
}

func TestFocus_DefaultSkills(t *testing.T) {
	m := testModel()
	if m.focusPane != paneSkills {
		t.Errorf("default focusPane = %d, want paneSkills", m.focusPane)
	}
}

func TestFocus_RightMovesToDetails(t *testing.T) {
	m := testModel()
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	model := m2.(Model)
	if model.focusPane != paneDetails {
		t.Errorf("focusPane after l = %d, want paneDetails", model.focusPane)
	}
}

func TestFocus_LeftMovesToSkills(t *testing.T) {
	m := testModel()
	m.focusPane = paneDetails
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	model := m2.(Model)
	if model.focusPane != paneSkills {
		t.Errorf("focusPane after h = %d, want paneSkills", model.focusPane)
	}
}

func TestFocus_JKScrollsDetails(t *testing.T) {
	m := testModel()
	m.focusPane = paneDetails
	m.detailScroll = 0

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model := m2.(Model)
	if model.detailScroll != 1 {
		t.Errorf("detailScroll after j = %d, want 1", model.detailScroll)
	}

	m3, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model = m3.(Model)
	if model.detailScroll != 0 {
		t.Errorf("detailScroll after k = %d, want 0", model.detailScroll)
	}
}

func TestFocus_CtrlDPagesDetails(t *testing.T) {
	m := testModel()
	m.focusPane = paneDetails

	step := pageStep(m.detailContentHeight())
	m2, _ := m.Update(ctrlKey(tea.KeyCtrlD))
	model := m2.(Model)

	if got, want := model.detailScroll, step; got != want {
		t.Fatalf("detailScroll after ctrl+d = %d, want %d", got, want)
	}
}

func TestHelp_CtrlUPagesHelp(t *testing.T) {
	m := testModel()
	m.showHelp = true
	m.helpScroll = pageStep(m.mainBodyHeight()) + 1

	m2, _ := m.Update(ctrlKey(tea.KeyCtrlU))
	model := m2.(Model)

	if got := model.helpScroll; got != 1 {
		t.Fatalf("helpScroll after ctrl+u = %d, want 1", got)
	}
}

func TestPreviewCmdForSkill_UsesLocalPreviewForUnmanagedRows(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "swift-refactor")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Swift Refactor\nPreview body."), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	m := testModel()
	m.skills[0].Flags = []reconcile.StatusFlag{reconcile.StatusUnmanaged}
	m.skills[0].LocalRoot = root
	m.applyFilter()

	cmd := m.previewCmdForSkill(m.selectedSkill())
	if cmd == nil {
		t.Fatal("expected local preview command")
	}
	msg := cmd()
	preview, ok := msg.(previewMsg)
	if !ok {
		t.Fatalf("cmd() type = %T, want previewMsg", msg)
	}
	if preview.err != nil {
		t.Fatalf("preview err = %v", preview.err)
	}
	if !strings.Contains(preview.preview.Markdown, "Preview body.") {
		t.Fatalf("Markdown = %q, want local preview content", preview.preview.Markdown)
	}
}

func TestFocus_JKMovesSkillCursor(t *testing.T) {
	m := testModel()
	m.focusPane = paneSkills

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model := m2.(Model)
	if model.cursor != 1 {
		t.Errorf("cursor after j in skills pane = %d, want 1", model.cursor)
	}
}

func TestFocus_ScrollResetOnSkillChange(t *testing.T) {
	m := testModel()
	m.detailScroll = 5
	m.focusPane = paneSkills

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model := m2.(Model)
	if model.detailScroll != 0 {
		t.Errorf("detailScroll after skill change = %d, want 0", model.detailScroll)
	}
}

func TestFocus_GTopResetsScrollInDetails(t *testing.T) {
	m := testModel()
	m.focusPane = paneDetails
	m.detailScroll = 10

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model := m2.(Model)
	if model.detailScroll != 0 {
		t.Errorf("detailScroll after g = %d, want 0", model.detailScroll)
	}
}

func TestTruncateToWindow_Fits(t *testing.T) {
	content := "line1\nline2\nline3"
	visible, offset, above, below := truncateToWindow(content, 5, 0)
	if visible != content {
		t.Errorf("visible = %q", visible)
	}
	if offset != 0 || above || below {
		t.Errorf("offset=%d above=%v below=%v", offset, above, below)
	}
}

func TestTruncateToWindow_Overflow(t *testing.T) {
	content := "1\n2\n3\n4\n5"
	visible, _, above, below := truncateToWindow(content, 3, 0)
	if above {
		t.Error("expected moreAbove=false at offset 0")
	}
	if !below {
		t.Error("expected moreBelow=true")
	}
	lines := strings.Split(visible, "\n")
	if len(lines) != 3 {
		t.Errorf("visible lines = %d, want 3", len(lines))
	}
}

func TestTruncateToWindow_ScrolledDown(t *testing.T) {
	content := "1\n2\n3\n4\n5"
	_, _, above, below := truncateToWindow(content, 3, 1)
	if !above {
		t.Error("expected moreAbove=true at offset 1")
	}
	if !below {
		t.Error("expected moreBelow=true")
	}
}

func TestTruncateToWindow_ClampOffset(t *testing.T) {
	content := "1\n2\n3\n4\n5"
	_, offset, above, below := truncateToWindow(content, 3, 100)
	if offset != 2 {
		t.Errorf("clamped offset = %d, want 2", offset)
	}
	if !above {
		t.Error("expected moreAbove=true")
	}
	if below {
		t.Error("expected moreBelow=false at bottom")
	}
}

func TestRenderDetails_FitsHeight(t *testing.T) {
	m := testModel()
	m.preview = &app.SkillPreview{
		Skill:    m.filtered[0].Skill,
		Markdown: strings.Repeat("line\n", 100),
	}

	paneHeight := 20
	for _, scroll := range []int{0, 5, 50, 999} {
		m.detailScroll = scroll
		output := m.renderDetails(60, paneHeight)
		lines := strings.Split(output, "\n")
		if len(lines) > paneHeight {
			t.Errorf("scroll=%d: rendered %d lines, max %d", scroll, len(lines), paneHeight)
		}
	}
}

func TestRenderDetails_OmitsPath(t *testing.T) {
	m := testModel()

	output := m.renderDetails(60, 20)
	if strings.Contains(output, "Path:") {
		t.Fatalf("details should not render path:\n%s", output)
	}
	if !strings.Contains(output, "Name:") || !strings.Contains(output, "Supports:") {
		t.Fatalf("details should still render skill metadata:\n%s", output)
	}
}

func TestRenderDetails_UnmanagedShowsImportBanner(t *testing.T) {
	m := testModel()
	m.skills[0].Flags = []reconcile.StatusFlag{reconcile.StatusUnmanaged}
	m.applyFilter()

	output := m.renderDetails(60, 20)
	if !strings.Contains(output, "`i` to import this unmanaged skill") {
		t.Fatalf("details should show unmanaged import banner:\n%s", output)
	}
	if !strings.Contains(output, "Name:") {
		t.Fatalf("details should still show metadata below banner:\n%s", output)
	}
}

func TestModel_ProjectModeView(t *testing.T) {
	m := testModel()
	m.detectedProject = testProject
	m.projectRoot = testProject
	m.skills[0].ProjectClaude = true
	m.skills[0].InstalledClaude = true
	m.projectHintCount = 2
	m.applyFilter()

	v := m.View()
	if v == "" {
		t.Error("expected non-empty view in project mode")
	}
	if !strings.Contains(v, "i import") || !strings.Contains(v, "Untracked skills: 2 ready") {
		t.Fatalf("view missing project import hint:\n%s", v)
	}
	if !strings.Contains(v, "Utilities") {
		t.Fatalf("view missing separate utilities panel:\n%s", v)
	}
	if strings.Contains(v, "refresh all targets") {
		t.Fatalf("view should not show bulk action in utilities:\n%s", v)
	}
	if strings.Contains(v, "tab  switch to user scope") {
		t.Fatalf("view should not show tab in right-side panels:\n%s", v)
	}
}

func TestRenderHeader_ProjectScopeUsesHomeShortPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Fatalf("UserHomeDir() error = %v, home = %q", err, home)
	}

	m := testModel()
	m.projectRoot = filepath.Join(home, "projects", "go", "loadout")

	header := m.renderHeader()
	if !strings.Contains(header, "Loadout [project scope: ~/projects/go/loadout]") {
		t.Fatalf("header should shorten home path:\n%s", header)
	}
	if strings.Contains(header, "project scope: "+m.projectRoot) {
		t.Fatalf("header should not render full home path:\n%s", header)
	}
}

func TestRenderHeader_UserScopeLabel(t *testing.T) {
	m := testModel()

	header := m.renderHeader()
	if !strings.Contains(header, "Loadout [user scope]") {
		t.Fatalf("header should show user scope label:\n%s", header)
	}
}

func TestView_ProjectScopeHeaderDoesNotRepeatScopeStatus(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Fatalf("UserHomeDir() error = %v, home = %q", err, home)
	}

	m := testModel()
	m.projectRoot = filepath.Join(home, "projects", "go", "loadout")

	v := m.View()
	if strings.Count(v, "project scope:") != 1 {
		t.Fatalf("view should render project scope only once in header:\n%s", v)
	}
}

func TestView_RightSideUsesTwoStackedPanes(t *testing.T) {
	m := testModel()
	m.width = 120
	m.height = 30

	v := m.View()
	if strings.Count(v, "Status") < 1 || strings.Count(v, "Utilities") < 1 {
		t.Fatalf("view should show separate status and utilities panes:\n%s", v)
	}
}

func TestRenderStatus_ProjectHintUsesSeparateSection(t *testing.T) {
	m := testModel()
	m.detectedProject = testProject
	m.projectRoot = testProject
	m.projectHintCount = 1
	m.applyFilter()

	status := m.renderStatus(40, 20)
	if strings.Contains(status, "Untracked skills: 1 ready") || strings.Contains(status, "Utilities") {
		t.Fatalf("status panel should be selection-specific only:\n%s", status)
	}
}

func TestRenderScopeInfoPanel_UserUtilities_ShowImportSyncDoctor(t *testing.T) {
	m := testModel()

	scope := m.renderScopeInfoPanel(40, 10)
	if strings.Contains(scope, "refresh all") {
		t.Fatalf("scope utilities should not show bulk skill action:\n%s", scope)
	}
	if !strings.Contains(scope, "i import") || !strings.Contains(scope, "s sync repo") || !strings.Contains(scope, "d doctor") {
		t.Fatalf("scope panel missing expected utility lines:\n%s", scope)
	}
	syncIdx := strings.Index(scope, "s sync repo")
	doctorIdx := strings.Index(scope, "d doctor")
	importIdx := strings.Index(scope, "i import")
	if syncIdx == -1 || doctorIdx == -1 || importIdx == -1 || !(syncIdx < doctorIdx && doctorIdx < importIdx) {
		t.Fatalf("user utilities should render sync, doctor, then import:\n%s", scope)
	}
}

func TestModel_ProjectModeView_HidesImportHintWithoutReadySkills(t *testing.T) {
	m := testModel()
	m.detectedProject = testProject
	m.projectRoot = testProject
	m.skills[0].ProjectClaude = true
	m.applyFilter()

	v := m.View()
	if strings.Contains(v, "Untracked skills:") {
		t.Fatalf("view should hide import hint without ready skills:\n%s", v)
	}
	if !strings.Contains(v, "i import") {
		t.Fatalf("view should keep import visible without ready skills:\n%s", v)
	}
}

func TestRenderScopeInfoPanel_ProjectModeHidesTabAction(t *testing.T) {
	m := testModel()
	m.detectedProject = testProject
	m.projectRoot = testProject

	scope := m.renderScopeInfoPanel(40, 10)
	if strings.Contains(scope, "tab") {
		t.Fatalf("scope panel should not mention tab:\n%s", scope)
	}
	if strings.Contains(scope, "\nUtilities\n\nUtilities") {
		t.Fatalf("scope panel should not duplicate utilities label:\n%s", scope)
	}
	if !strings.Contains(scope, "i import") || !strings.Contains(scope, "s sync repo") || !strings.Contains(scope, "d doctor") {
		t.Fatalf("scope panel missing project utility actions:\n%s", scope)
	}
	if strings.Contains(scope, "equip all") {
		t.Fatalf("scope panel should hide skill actions:\n%s", scope)
	}
}

func TestRenderScopeInfoPanel_ProjectModeShowsImportHintBelowImport(t *testing.T) {
	m := testModel()
	m.detectedProject = testProject
	m.projectRoot = testProject
	m.projectHintCount = 1

	scope := m.renderScopeInfoPanel(40, 10)
	importIdx := strings.Index(scope, "i import")
	hintIdx := strings.Index(scope, "Untracked skills: 1 ready")
	syncIdx := strings.Index(scope, "s sync repo")
	doctorIdx := strings.Index(scope, "d doctor")
	if importIdx == -1 || hintIdx == -1 || syncIdx == -1 || doctorIdx == -1 || !(syncIdx < doctorIdx && doctorIdx < importIdx && importIdx < hintIdx) {
		t.Fatalf("scope panel should render sync, doctor, import, then hint:\n%s", scope)
	}
	if strings.Contains(scope, "Untracked Skills") || strings.Contains(scope, "open import") {
		t.Fatalf("scope panel should use compact import hint copy:\n%s", scope)
	}
}

func TestRenderScopeInfoPanel_HighlightsSyncWhenAttentionNeeded(t *testing.T) {
	m := testModel()

	plain := m.renderScopeInfoPanel(40, 10)
	m.syncAttention = true
	highlighted := m.renderScopeInfoPanel(40, 10)
	if !strings.Contains(highlighted, "s sync repo *") {
		t.Fatalf("highlighted scope panel missing sync row:\n%s", highlighted)
	}
	if strings.Contains(plain, "sync repo *") {
		t.Fatalf("plain scope panel should not show sync attention marker:\n%s", plain)
	}
}

func TestRenderStatus_ProjectModeShowsCompactUserNote(t *testing.T) {
	m := testModel()
	m.detectedProject = testProject
	m.projectRoot = testProject
	m.skills[0].InstalledClaude = true
	m.applyFilter()

	status := m.renderStatus(40, 20)
	if !strings.Contains(status, "[g]") {
		t.Fatalf("status panel should show bracketed user marker:\n%s", status)
	}
	if !strings.Contains(status, "c equip project") {
		t.Fatalf("status panel should show project action hint in project mode:\n%s", status)
	}
	if strings.Contains(status, "(g)") || strings.Contains(status, "user install") || strings.Contains(status, "also user") {
		t.Fatalf("status panel should not use legacy user note text:\n%s", status)
	}
}

func TestRenderStatus_ProjectModeProjectInstallUsesCheckAndUnequipProject(t *testing.T) {
	m := testModel()
	m.detectedProject = testProject
	m.projectRoot = testProject
	m.skills[0].ProjectClaude = true
	m.applyFilter()

	status := m.renderStatus(40, 20)
	if !strings.Contains(status, "[x] Claude") {
		t.Fatalf("status panel should show project installs with check marker:\n%s", status)
	}
	if !strings.Contains(status, "c unequip project") {
		t.Fatalf("status panel should show project unequip hint:\n%s", status)
	}
	if strings.Contains(status, "[g] Claude") {
		t.Fatalf("status panel should not show user marker for project installs:\n%s", status)
	}
}

func TestProjectImportHintUsesDistinctInfoColor(t *testing.T) {
	if fg := statusInfoStyle.GetForeground(); fg == nil || fg == statusWarnStyle.GetForeground() {
		t.Fatalf("project import hint should use a distinct info color")
	}
}

func TestRenderFooter_ProjectModeShowsUserInstallMessage(t *testing.T) {
	m := testModel()
	m.width = 160
	m.detectedProject = testProject
	m.projectRoot = testProject
	m.skills[0].InstalledClaude = true
	m.applyFilter()

	footer := m.renderFooter()
	if !strings.Contains(footer, "this skill is installed in user scope") {
		t.Fatalf("footer should show user install message:\n%s", footer)
	}
}

func TestRenderFooter_HidesUserInstallMessageWhenNotApplicable(t *testing.T) {
	m := testModel()
	if strings.Contains(m.renderFooter(), "this skill is installed in user scope") {
		t.Fatalf("user footer message should be hidden outside project mode:\n%s", m.renderFooter())
	}

	m.detectedProject = testProject
	m.projectRoot = testProject
	m.applyFilter()
	if strings.Contains(m.renderFooter(), "this skill is installed in user scope") {
		t.Fatalf("user footer message should be hidden without user installs:\n%s", m.renderFooter())
	}
}

func TestRenderFooter_DropsUserInstallMessageWhenTooNarrow(t *testing.T) {
	m := testModel()
	m.width = 50
	m.detectedProject = testProject
	m.projectRoot = testProject
	m.skills[0].InstalledClaude = true
	m.applyFilter()

	footer := m.renderFooter()
	if strings.Contains(footer, "this skill is installed in user scope") {
		t.Fatalf("footer should drop user install message when width is tight:\n%s", footer)
	}
}

func TestImport_BrowseEnterAndExit(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.importStartDir = testProject

	// Press b to enter browse mode
	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	model := m2.(Model)
	if !model.importBrowsing {
		t.Fatal("expected browse mode after b")
	}
	if cmd == nil {
		t.Fatal("expected loadBrowseDirCmd")
	}

	// Simulate loadBrowseDirMsg
	m3, _ := model.Update(loadBrowseDirMsg{dir: testHome, entries: []string{".claude", "projects", "scripts"}})
	model = m3.(Model)
	if model.browseDir != testHome {
		t.Fatalf("browseDir = %q, want %q", model.browseDir, testHome)
	}
	if len(model.browseDirEntries) != 3 {
		t.Fatalf("browseDirEntries len = %d, want 3", len(model.browseDirEntries))
	}
	if model.browseDirEntries[0] != ".claude" {
		t.Fatalf("first entry = %q, want .claude", model.browseDirEntries[0])
	}

	// Esc cancels browse
	m4, _ := model.Update(tea.KeyMsg{Type: tea.KeyEscape})
	model = m4.(Model)
	if model.importBrowsing {
		t.Fatal("expected browse mode to exit on esc")
	}
}

func TestImport_BrowseStartsFromImportStartDir(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.importStartDir = testProject

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	model := m2.(Model)
	if !model.importBrowsing {
		t.Fatal("expected browse mode after b")
	}
	if model.browseDir != testProject {
		t.Fatalf("browseDir = %q, want %q", model.browseDir, testProject)
	}
	if cmd == nil {
		t.Fatal("expected loadBrowseDirCmd")
	}
}

func TestImport_BrowseEnterDirectory(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.importBrowsing = true
	m.browseDir = testHome
	m.browseDirEntries = []string{"projects", "scripts"}
	m.browseCursor = 1

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = m2
	if cmd == nil {
		t.Fatal("expected loadBrowseDirCmd for enter")
	}
}

func TestImport_BrowseDownReachesLastDirectory(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.importBrowsing = true
	m.browseDir = testHome
	m.browseDirEntries = []string{".claude", "bin", "projects"}
	m.browseCursor = 0

	for range 3 {
		m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		m = m2.(Model)
	}

	if got, want := m.browseCursor, len(m.browseDirEntries); got != want {
		t.Fatalf("browseCursor after moving down = %d, want %d", got, want)
	}

	v := m.renderImportListPane(40, 14)
	if !strings.Contains(v, "> projects/") {
		t.Fatalf("browse view should select last directory:\n%s", v)
	}
}

func TestImport_BrowseBottomSelectsLastDirectory(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.importBrowsing = true
	m.browseDir = testHome
	m.browseDirEntries = []string{".claude", "bin", "projects"}

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	m = m2.(Model)

	if got, want := m.browseCursor, len(m.browseDirEntries); got != want {
		t.Fatalf("browseCursor after bottom = %d, want %d", got, want)
	}

	v := m.renderImportListPane(40, 14)
	if !strings.Contains(v, "> projects/") {
		t.Fatalf("browse view should select last directory after bottom:\n%s", v)
	}
}

func TestImport_BrowseEnterParentDirectory(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.importBrowsing = true
	m.browseDir = filepath.Join(testHome, "projects")
	m.browseDirEntries = []string{"foo"}
	m.browseCursor = 0

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected loadBrowseDirCmd for parent entry")
	}
}

func TestImport_BrowseBackspace(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.importBrowsing = true
	m.browseDir = filepath.Join(testHome, "projects")
	m.browseDirEntries = []string{"foo"}
	m.browseCursor = 0

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if cmd == nil {
		t.Fatal("expected loadBrowseDirCmd for backspace")
	}
}

func TestImport_BrowseScanHere(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.importBrowsing = true
	m.browseDir = filepath.Join(testHome, "projects")
	m.browseDirEntries = []string{"foo"}
	m.browseCursor = 0

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	model := m2.(Model)
	if model.importBrowsing {
		t.Fatal("expected browse mode to exit on scan")
	}
	if model.importCustomDir != filepath.Join(testHome, "projects") {
		t.Fatalf("importCustomDir = %q", model.importCustomDir)
	}
	if !model.loading {
		t.Fatal("expected loading state")
	}
	if cmd == nil {
		t.Fatal("expected loadImportCandidatesFromDirCmd")
	}
}

func TestImport_ResetKeyIsNoOp(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.importCustomDir = "/some/dir"
	m.imports = []app.ImportCandidateView{{
		SkillName: "test-skill",
		Ready:     true,
		FromRoots: []domain.Target{domain.TargetClaude},
		SourceDir: "/some/dir/test-skill",
	}}

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	model := m2.(Model)
	if cmd != nil {
		t.Fatal("expected no command for removed reset key")
	}
	if model.importCustomDir != "/some/dir" {
		t.Fatal("expected importCustomDir to remain unchanged")
	}
	if model.loading {
		t.Fatal("did not expect loading state change")
	}
}

func TestImport_FooterShowsCommitState(t *testing.T) {
	m := testModel()
	m.screen = screenImport

	footer := m.renderFooter()
	if strings.Contains(footer, "auto-commit") {
		t.Fatalf("global footer should not show auto-commit (moved to pane footer), got:\n%s", footer)
	}
	if strings.Contains(footer, "commit: off") || strings.Contains(footer, "commit: on") {
		t.Fatalf("footer should not show commit state labels, got:\n%s", footer)
	}
}

func TestImport_FooterShowsBrowseKeys(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.importBrowsing = true

	footer := m.renderFooter()
	if strings.Contains(footer, "scan here") {
		t.Fatalf("global footer should not show scan here (moved to pane footer), got:\n%s", footer)
	}
	if !strings.Contains(footer, "project") {
		t.Fatalf("browse footer should show scope toggle, got:\n%s", footer)
	}
}

func TestImport_FooterShowsResetWhenCustomDir(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.importCustomDir = "/some/dir"

	footer := m.renderFooter()
	if strings.Contains(footer, "reset") {
		t.Fatalf("global footer should not show reset (moved to pane footer), got:\n%s", footer)
	}
}

func TestImport_PaneFooterShowsImportActions(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.imports = []app.ImportCandidateView{{
		SkillName: "shared-skill",
		Ready:     true,
		FromRoots: []domain.Target{domain.TargetClaude},
		SourceDir: filepath.Join(os.TempDir(), "skills", "shared-skill"),
	}}

	pane := m.renderImportListPane(40, 20)
	for _, label := range []string{"import", "import all", "auto-commit", "browse"} {
		if !strings.Contains(pane, label) {
			t.Errorf("pane footer missing %q:\n%s", label, pane)
		}
	}
}

func TestImport_PaneFooterShowsBrowseActions(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.importBrowsing = true
	m.browseDir = testHome

	pane := m.renderImportListPane(40, 20)
	for _, label := range []string{"open", "up", "scan here"} {
		if !strings.Contains(pane, label) {
			t.Errorf("pane footer missing %q:\n%s", label, pane)
		}
	}
}

func TestImport_PaneFooterShowsBrowseWhenEmpty(t *testing.T) {
	m := testModel()
	m.screen = screenImport

	pane := m.renderImportListPane(40, 20)
	if strings.Contains(pane, "import all") {
		t.Errorf("should not show import actions with no candidates:\n%s", pane)
	}
	if !strings.Contains(pane, "browse") {
		t.Errorf("should show browse even with no candidates:\n%s", pane)
	}
}

func TestImport_PaneFooterDoesNotShowResetWhenEmptyWithCustomDir(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.importCustomDir = "/some/scanned/dir"

	pane := m.renderImportListPane(40, 20)
	if strings.Contains(pane, "default root") {
		t.Errorf("should not show default root in simplified footer:\n%s", pane)
	}
	if !strings.Contains(pane, "browse") {
		t.Errorf("should show browse when empty with custom dir:\n%s", pane)
	}
}

func TestImport_PaneFooterVisibleWhenLoading(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.loading = true

	pane := m.renderImportListPane(80, 20)
	if !strings.Contains(pane, "browse") {
		t.Errorf("pane footer should show browse while loading:\n%s", pane)
	}
	if strings.Contains(pane, "import all") {
		t.Errorf("pane footer should not show import actions while loading:\n%s", pane)
	}
}

func TestImport_PaneFooterNoSeparator(t *testing.T) {
	for _, tt := range []struct {
		name  string
		setup func(*Model)
	}{
		{"browse", func(m *Model) { m.importBrowsing = true; m.browseDir = os.TempDir() }},
		{"import", func(m *Model) {
			m.imports = []app.ImportCandidateView{{SkillName: "s", Ready: true, FromRoots: []domain.Target{domain.TargetClaude}, SourceDir: filepath.Join(os.TempDir(), "s")}}
		}},
		{"empty", func(m *Model) {}},
		{"loading", func(m *Model) { m.loading = true }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := testModel()
			m.screen = screenImport
			tt.setup(&m)
			footer := m.renderImportPaneFooter()
			if strings.Contains(footer, "──") {
				t.Errorf("footer should not contain separator:\n%s", footer)
			}
		})
	}
}

func TestImport_PaneFooterRowCount(t *testing.T) {
	for _, tt := range []struct {
		name     string
		setup    func(*Model)
		wantRows int
	}{
		{"browse", func(m *Model) { m.importBrowsing = true; m.browseDir = os.TempDir() }, 2},
		{"import", func(m *Model) {
			m.imports = []app.ImportCandidateView{{SkillName: "s", Ready: true, FromRoots: []domain.Target{domain.TargetClaude}, SourceDir: filepath.Join(os.TempDir(), "s")}}
		}, 2},
		{"empty", func(m *Model) {}, 1},
		{"loading", func(m *Model) { m.loading = true }, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := testModel()
			m.screen = screenImport
			tt.setup(&m)
			footer := m.renderImportPaneFooter()
			got := strings.Count(footer, "\n") + 1
			if got != tt.wantRows {
				t.Errorf("footer rows = %d, want %d:\n%s", got, tt.wantRows, footer)
			}
		})
	}
}

func TestImport_ShortHeight(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.imports = []app.ImportCandidateView{{
		SkillName: "test-skill",
		Ready:     true,
		FromRoots: []domain.Target{domain.TargetClaude},
		SourceDir: filepath.Join(os.TempDir(), "test-skill"),
	}}

	pane := m.renderImportListPane(80, 4)
	lines := strings.Split(pane, "\n")
	// With height 4, the 2-row footer leaves 2 rows for body.
	// At minimum we need 1 body row visible.
	bodyLines := 0
	for _, line := range lines {
		if !strings.Contains(line, "import") && !strings.Contains(line, "browse") && !strings.Contains(line, "auto-commit") && line != "" {
			bodyLines++
		}
	}
	if bodyLines < 1 {
		t.Errorf("expected at least 1 body row with height=4, got:\n%s", pane)
	}
}

func TestRenderFooter_ShowsImportOnInventory(t *testing.T) {
	m := testModel()

	footer := m.renderFooter()
	if !strings.Contains(footer, "import") {
		t.Fatalf("inventory footer should show import:\n%s", footer)
	}
}

func TestRenderFooter_HelpShowsPageKeys(t *testing.T) {
	m := testModel()
	m.showHelp = true

	footer := m.renderFooter()
	if !strings.Contains(footer, "ctrl+u/d") {
		t.Fatalf("help footer should show page keys:\n%s", footer)
	}
}

func TestImport_LoadImportPreview(t *testing.T) {
	m := testModel()
	preview := app.ImportPreview{
		Skill: domain.Skill{
			Name:    "shared-skill",
			Targets: []domain.Target{domain.TargetClaude, domain.TargetCodex},
		},
		Markdown:  "# Shared Skill\nPreview",
		SourceDir: testShared,
		Ready:     true,
	}

	m2, _ := m.Update(importPreviewMsg{preview: preview})
	model := m2.(Model)
	if model.importPreview == nil {
		t.Fatal("expected import preview to be loaded")
	}
	if model.importPreview.Skill.Name != "shared-skill" {
		t.Fatalf("preview ID = %q", model.importPreview.Skill.Name)
	}
}

func TestImport_ViewShowsPreviewPane(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.imports = []app.ImportCandidateView{{
		SkillName: "shared-skill",
		SourceDir: testShared,
		Targets:   []domain.Target{domain.TargetClaude, domain.TargetCodex},
		Ready:     true,
		FromRoots: []domain.Target{domain.TargetClaude, domain.TargetCodex},
	}}
	m.importPreview = &app.ImportPreview{
		Skill: domain.Skill{
			Name:    "shared-skill",
			Targets: []domain.Target{domain.TargetClaude, domain.TargetCodex},
		},
		Markdown:  "# Shared Skill\nPreview",
		SourceDir: testShared,
		Ready:     true,
	}

	v := m.View()
	if !strings.Contains(v, "Preview") {
		t.Fatalf("view should contain preview pane:\n%s", v)
	}
	if !strings.Contains(v, "> import") || !strings.Contains(v, "Import") {
		t.Fatalf("view should use import naming:\n%s", v)
	}
	if !strings.Contains(v, "shared-skill") {
		t.Fatalf("view should contain import preview content:\n%s", v)
	}
}

func TestImport_BrowseViewShowsParentEntry(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.importBrowsing = true
	m.browseDir = filepath.Join(testHome, "projects")
	m.browseDirEntries = []string{"foo", "bar"}

	v := m.View()
	if !strings.Contains(v, "../") {
		t.Fatalf("browse view should contain parent entry:\n%s", v)
	}
}

func TestImport_RenderListPaneKeepsCursorVisibleWhenPaged(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.imports = importCandidates(20)
	m.cursor = len(m.imports) - 1

	pane := m.renderImportListPane(80, 14)
	selected := string(m.imports[m.cursor].SkillName)
	first := string(m.imports[0].SkillName)

	if !strings.Contains(pane, "> "+selected) {
		t.Fatalf("import pane should keep selected candidate visible:\n%s", pane)
	}
	if strings.Contains(pane, first) {
		t.Fatalf("import pane should window away early candidates near the bottom:\n%s", pane)
	}
}

func TestImport_RenderBrowsePaneKeepsCursorVisibleWhenPaged(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.importBrowsing = true
	m.browseDir = testHome
	m.browseDirEntries = browseEntries(20)
	m.browseCursor = len(m.browseDirEntries)

	pane := m.renderImportListPane(80, 14)
	selected := m.browseDirEntries[len(m.browseDirEntries)-1] + string(filepath.Separator)
	first := m.browseDirEntries[0] + string(filepath.Separator)

	if !strings.Contains(pane, "> "+selected) {
		t.Fatalf("browse pane should keep selected directory visible:\n%s", pane)
	}
	if strings.Contains(pane, first) {
		t.Fatalf("browse pane should window away early directories near the bottom:\n%s", pane)
	}
}

func TestImport_ProjectBrowsePreviewCopy(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.projectRoot = testProject
	m.importBrowsing = true
	m.browseDir = testProject

	v := m.View()
	if !strings.Contains(v, "Browse project files, then press s to scan this directory for skills.") {
		t.Fatalf("browse preview copy mismatch:\n%s", v)
	}
}

func TestImport_TabTogglesToProjectScope(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.detectedProject = testProject
	m.importBrowsing = true
	m.importCustomDir = filepath.Join(os.TempDir(), "custom")

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := m2.(Model)
	if model.projectRoot != testProject {
		t.Fatalf("projectRoot = %q, want %q", model.projectRoot, testProject)
	}
	if !model.importBrowsing {
		t.Fatal("expected browse mode in project scope")
	}
	if model.importCustomDir != "" {
		t.Fatalf("importCustomDir = %q, want empty", model.importCustomDir)
	}
	if model.loading {
		t.Fatal("did not expect loading state in project browse mode")
	}
	if model.status != "import scope: project" {
		t.Fatalf("status = %q", model.status)
	}
	if model.browseDir != testProject {
		t.Fatalf("browseDir = %q, want %q", model.browseDir, testProject)
	}
	if cmd == nil {
		t.Fatal("expected import reload command")
	}
}

func TestImport_TabTogglesToUserScope(t *testing.T) {
	m := testModel()
	m.screen = screenImport
	m.detectedProject = testProject
	m.projectRoot = testProject

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := m2.(Model)
	if model.projectRoot != "" {
		t.Fatalf("projectRoot = %q, want empty", model.projectRoot)
	}
	if !model.loading {
		t.Fatal("expected loading after scope toggle")
	}
	if model.status != "import scope: user" {
		t.Fatalf("status = %q", model.status)
	}
	if cmd == nil {
		t.Fatal("expected import reload command")
	}
}

func TestImport_TabWithoutDetectedProjectLeavesScopeUnchanged(t *testing.T) {
	m := testModel()
	m.screen = screenImport

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := m2.(Model)
	if model.projectRoot != "" {
		t.Fatalf("projectRoot = %q, want empty", model.projectRoot)
	}
	if model.status == "" || !strings.Contains(model.status, "no project detected") {
		t.Fatalf("status = %q, want no project detected message", model.status)
	}
	if cmd != nil {
		t.Fatal("expected no command without detected project")
	}
}

func importModelWithCandidates(ready, blocked int) Model {
	m := testModel()
	m.screen = screenImport
	for i := 0; i < ready; i++ {
		m.imports = append(m.imports, app.ImportCandidateView{
			SkillName: domain.SkillName("ready-" + string(rune('a'+i))),
			SourceDir: filepath.Join(os.TempDir(), "ready-"+string(rune('a'+i))),
			Targets:   []domain.Target{domain.TargetClaude},
			Ready:     true,
		})
	}
	for i := 0; i < blocked; i++ {
		m.imports = append(m.imports, app.ImportCandidateView{
			SkillName: domain.SkillName("blocked-" + string(rune('a'+i))),
			SourceDir: filepath.Join(os.TempDir(), "blocked-"+string(rune('a'+i))),
			Targets:   []domain.Target{domain.TargetClaude},
			Ready:     false,
			Problem:   "already in repo",
		})
	}
	return m
}

func pressKey(m Model, key string) Model {
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return m2.(Model)
}

func TestBulkImport_AOpensConfirmationWithReadyCandidates(t *testing.T) {
	m := importModelWithCandidates(3, 2)

	m = pressKey(m, "A")

	if !m.bulkImportConfirming() {
		t.Fatal("expected bulk import confirmation modal")
	}
	if m.bulkImport.ReadyCount != 3 {
		t.Errorf("ReadyCount = %d, want 3", m.bulkImport.ReadyCount)
	}
	if m.bulkImport.SkippedCount != 2 {
		t.Errorf("SkippedCount = %d, want 2", m.bulkImport.SkippedCount)
	}
}

func TestBulkImport_ANoopsWithNoReadyCandidates(t *testing.T) {
	m := importModelWithCandidates(0, 3)

	m = pressKey(m, "A")

	if m.bulkImportConfirming() {
		t.Fatal("expected no bulk import modal when no ready candidates")
	}
	if m.status != "no ready candidates to import" {
		t.Errorf("status = %q, want %q", m.status, "no ready candidates to import")
	}
}

func TestBulkImport_AIgnoredWhileHelpOpen(t *testing.T) {
	m := importModelWithCandidates(3, 0)
	m.showHelp = true

	m = pressKey(m, "A")

	if m.bulkImportConfirming() {
		t.Fatal("A should not open bulk import modal while help is open")
	}
}

func TestBulkImport_LowercaseADoesNothing(t *testing.T) {
	m := importModelWithCandidates(3, 0)

	m = pressKey(m, "a")

	if m.bulkImportConfirming() {
		t.Fatal("lowercase a should not open bulk import modal")
	}
}

func TestBulkImport_EscCancelsModal(t *testing.T) {
	m := importModelWithCandidates(3, 0)
	m = pressKey(m, "A")
	if !m.bulkImportConfirming() {
		t.Fatal("expected modal open")
	}

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	model := m2.(Model)

	if model.bulkImportConfirming() {
		t.Fatal("expected modal closed after esc")
	}
	if model.status != "import all cancelled" {
		t.Errorf("status = %q, want %q", model.status, "import all cancelled")
	}
}

func TestBulkImport_EnterStartsBatch(t *testing.T) {
	m := importModelWithCandidates(3, 1)
	m.importCommit = false
	m = pressKey(m, "A")

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := m2.(Model)

	if !model.loading {
		t.Error("expected loading = true")
	}
	if cmd == nil {
		t.Fatal("expected batch import command")
	}
	if !strings.Contains(model.status, "importing 3 skills") {
		t.Errorf("status = %q, want importing message", model.status)
	}
}

func TestBulkImport_ModalSwallowsNonQuitKeys(t *testing.T) {
	m := importModelWithCandidates(3, 0)
	m = pressKey(m, "A")

	for _, key := range []string{"j", "k", "?", "c", "b"} {
		m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		model := m2.(Model)
		if !model.bulkImportConfirming() {
			t.Errorf("key %q closed modal", key)
		}
		if cmd != nil {
			t.Errorf("key %q produced command", key)
		}
	}
}

func TestBulkImport_ModalQuitKeyWorks(t *testing.T) {
	m := importModelWithCandidates(3, 0)
	m = pressKey(m, "A")

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	_ = m2.(Model)
	if cmd == nil {
		t.Fatal("expected quit command from q key in modal")
	}
}

func TestBulkImport_ModalShowsCorrectCounts(t *testing.T) {
	m := importModelWithCandidates(5, 2)
	m = pressKey(m, "A")

	view := m.View()
	if !strings.Contains(view, "Import All Ready Skills?") {
		t.Error("modal missing title")
	}
	if !strings.Contains(view, "5 skills") {
		t.Errorf("modal missing ready count in view:\n%s", view)
	}
	if !strings.Contains(view, "2 blocked") {
		t.Errorf("modal missing skipped count in view:\n%s", view)
	}
}

func TestBulkImport_ModalShowsCommitDetailWhenAutoCommitOn(t *testing.T) {
	m := importModelWithCandidates(2, 0)
	m.importCommit = true
	m = pressKey(m, "A")

	view := m.View()
	if !strings.Contains(view, "One commit per imported skill") {
		t.Errorf("modal missing commit detail:\n%s", view)
	}
}

func TestBulkImport_ModalShowsUncommittedDetailWhenAutoCommitOff(t *testing.T) {
	m := importModelWithCandidates(2, 0)
	m.importCommit = false
	m = pressKey(m, "A")

	view := m.View()
	if !strings.Contains(view, "Repo changes left uncommitted") {
		t.Errorf("modal missing uncommitted detail:\n%s", view)
	}
}

func TestBulkImport_FooterShowsModalKeys(t *testing.T) {
	m := importModelWithCandidates(2, 0)
	m = pressKey(m, "A")

	footer := m.renderFooter()
	if !strings.Contains(footer, "confirm") || !strings.Contains(footer, "cancel") {
		t.Errorf("footer missing modal keys:\n%s", footer)
	}
}

func TestBulkImport_ResultStaysOnImportScreen(t *testing.T) {
	m := importModelWithCandidates(3, 1)
	m.loading = true

	m2, cmd := m.Update(bulkImportResultMsg{
		imported:   []domain.SkillName{"ready-a", "ready-b", "ready-c"},
		skipped:    1,
		autoCommit: true,
	})
	model := m2.(Model)

	if !model.inImportScreen() {
		t.Fatal("expected to stay on import screen")
	}
	if !model.loading {
		t.Fatal("expected loading=true after refreshImportSource")
	}
	if cmd == nil {
		t.Fatal("expected refresh command")
	}
	if !strings.Contains(model.status, "imported 3 skills") {
		t.Errorf("status = %q, want imported count", model.status)
	}
	if !strings.Contains(model.status, "skipped 1 blocked") {
		t.Errorf("status = %q, want skipped count", model.status)
	}

	msg := cmd()
	result, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("cmd() type = %T, want tea.BatchMsg", msg)
	}
	if len(result) != 2 {
		t.Fatalf("batch len = %d, want 2", len(result))
	}
	var sawLoadSkills bool
	for _, batchCmd := range result {
		if batchCmd == nil {
			continue
		}
		if _, ok := batchCmd().(loadSkillsMsg); ok {
			sawLoadSkills = true
		}
	}
	if !sawLoadSkills {
		t.Fatal("expected bulk import result to refresh inventory skills")
	}
}

func TestBulkImport_ResultNoSkips(t *testing.T) {
	m := importModelWithCandidates(2, 0)

	m2, _ := m.Update(bulkImportResultMsg{
		imported:   []domain.SkillName{"ready-a", "ready-b"},
		skipped:    0,
		autoCommit: true,
	})
	model := m2.(Model)

	if strings.Contains(model.status, "skipped") {
		t.Errorf("status should not mention skipped: %q", model.status)
	}
}

func TestBulkImport_ResultWithErrors(t *testing.T) {
	m := importModelWithCandidates(3, 1)

	m2, _ := m.Update(bulkImportResultMsg{
		imported:   []domain.SkillName{"ready-a"},
		skipped:    1,
		errors:     []string{"ready-b: permission denied", "ready-c: disk full"},
		autoCommit: true,
	})
	model := m2.(Model)

	if !strings.Contains(model.status, "imported 1 skill") {
		t.Errorf("status = %q, want imported count", model.status)
	}
	if !strings.Contains(model.status, "2 errors") {
		t.Errorf("status = %q, want error count", model.status)
	}
}

func TestBulkImport_ResultAutoCommitOff_OpensCommitPrompt(t *testing.T) {
	m := importModelWithCandidates(2, 0)

	m2, _ := m.Update(bulkImportResultMsg{
		imported:   []domain.SkillName{"ready-a", "ready-b"},
		skipped:    0,
		autoCommit: false,
	})
	model := m2.(Model)

	if !model.commitPromptActive() {
		t.Fatal("expected commit prompt when auto-commit is off")
	}
	if !strings.Contains(model.status, "commit repo changes?") {
		t.Errorf("status = %q, want commit prompt suffix", model.status)
	}
	if model.commitPrompt.RepoPath != "skills" {
		t.Errorf("commit prompt RepoPath = %q, want %q", model.commitPrompt.RepoPath, "skills")
	}
}

func TestBulkImport_HelpShowsImportAll(t *testing.T) {
	m := importModelWithCandidates(2, 0)
	m.showHelp = true

	view := m.View()
	if !strings.Contains(view, "Import all ready candidates") {
		t.Errorf("help missing import all entry:\n%s", view)
	}
}

func TestBulkImport_PaneFooterShowsImportAll(t *testing.T) {
	m := importModelWithCandidates(2, 0)

	pane := m.renderImportListPane(40, 20)
	if !strings.Contains(pane, "import all") {
		t.Errorf("pane footer missing import all:\n%s", pane)
	}
}

func TestFormatBulkImportStatus(t *testing.T) {
	tests := []struct {
		name       string
		imported   int
		skipped    int
		errors     int
		autoCommit bool
		want       string
	}{
		{"all success", 5, 2, 0, true, "imported 5 skills; skipped 2 blocked"},
		{"no skips", 3, 0, 0, true, "imported 3 skills"},
		{"with errors", 3, 2, 1, true, "imported 3 skills; skipped 2 blocked; 1 error"},
		{"multiple errors", 1, 0, 3, true, "imported 1 skill; 3 errors"},
		{"auto-commit off", 5, 0, 0, false, "imported 5 skills without committing repo changes"},
		{"auto-commit off with skips", 3, 2, 0, false, "imported 3 skills; skipped 2 blocked without committing repo changes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatBulkImportStatus(tt.imported, tt.skipped, tt.errors, tt.autoCommit)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func assertViewFitsHeight(t *testing.T, m Model) {
	t.Helper()

	view := m.View()
	if h := lipgloss.Height(view); h > m.height {
		t.Fatalf("view height = %d, want <= %d\nview:\n%s", h, m.height, view)
	}
}
