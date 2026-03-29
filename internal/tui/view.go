package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/sethdeckard/loadout/internal/app"
	"github.com/sethdeckard/loadout/internal/config"
	"github.com/sethdeckard/loadout/internal/domain"
	"github.com/sethdeckard/loadout/internal/reconcile"
)

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	if m.err != nil {
		return errorStyle.Render("Error: "+m.err.Error()) + "\n\nPress q to quit."
	}

	header := m.renderHeader()
	footer := m.renderFooter()
	headerHeight := lipgloss.Height(header)
	footerHeight := lipgloss.Height(footer)
	bodyHeight := m.height - headerHeight - footerHeight - 2

	var body string
	if bodyHeight > 0 {
		switch {
		case m.commitPromptActive() || m.deleteConfirming() || m.bulkImportConfirming():
			body = m.renderModalBody(bodyHeight)
		case m.inImportScreen() && m.showHelp:
			body = m.renderImportHelp(bodyHeight)
		case m.inImportScreen():
			body = m.renderImport(bodyHeight)
		case m.inSettingsScreen() && m.showHelp:
			body = m.renderSettingsHelp(bodyHeight)
		case m.inSettingsScreen():
			body = m.renderSettings(bodyHeight)
		case m.showHelp:
			body = m.renderHelp(bodyHeight)
		case m.width < 60:
			body = m.renderNarrow(bodyHeight)
		default:
			body = m.renderWide(bodyHeight)
		}
	}

	parts := []string{header}
	if body != "" {
		parts = append(parts, body)
	}
	parts = append(parts, footer)
	view := strings.Join(parts, "\n")
	if m.commitPromptActive() || m.deleteConfirming() {
		return modalBackdropStyle.Render(view)
	}
	return view
}

func (m Model) renderHeader() string {
	scopeLabel := "user scope"
	if m.inProjectMode() {
		scopeLabel = "project scope: " + shortenHomePath(m.projectRoot)
	}
	if m.inSettingsScreen() {
		scopeLabel += " > settings"
	} else if m.inImportScreen() {
		scopeLabel += " > import"
	}
	title := headerStyle.Render("Loadout [" + scopeLabel + "]")
	statusText := ""
	if m.status != "" {
		statusText = statusBarStyle.Render(m.status)
	}
	gap := m.width - lipgloss.Width(title) - lipgloss.Width(statusText)
	if gap < 0 {
		gap = 0
	}
	return title + strings.Repeat(" ", gap) + statusText
}

func shortenHomePath(path string) string {
	if path == "" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func (m Model) renderFooter() string {
	if m.showHelp {
		keys := []struct{ key, label string }{
			{"j/k", "scroll"},
			{"ctrl+u/d", "page"},
			{"esc", "close"},
			{"q", "quit"},
		}
		return renderFooterKeys(m.width, keys, "")
	}

	if m.filtering {
		return filterPromptStyle.Render("Filter: ") + m.filter + "█"
	}

	if m.commitPromptActive() {
		keys := []struct{ key, label string }{
			{"enter", "commit"},
			{"esc", "leave uncommitted"},
			{"q", "quit"},
		}
		return renderFooterKeys(m.width, keys, "")
	}

	if m.bulkImportConfirming() {
		keys := []struct{ key, label string }{
			{"enter", "confirm"},
			{"esc", "cancel"},
		}
		return renderFooterKeys(m.width, keys, "")
	}

	if m.inImportScreen() {
		if m.importBrowsing {
			keys := []struct{ key, label string }{
				{"j/k", "move"},
				{"ctrl+u/d", "page"},
				{"tab", toggleScopeLabel(m.inProjectMode())},
				{"esc", "cancel"},
				{"?", "help"},
				{"q", "quit"},
			}
			return renderFooterKeys(m.width, keys, "")
		}
		keys := []struct{ key, label string }{
			{"j/k", "move"},
			{"ctrl+u/d", "page"},
			{"tab", toggleScopeLabel(m.inProjectMode())},
			{"esc", "close"},
			{"?", "help"},
			{"q", "quit"},
		}
		return renderFooterKeys(m.width, keys, "")
	}

	if m.inSettingsScreen() {
		keys := []struct{ key, label string }{
			{"j/k", "move"},
			{"ctrl+u/d", "page"},
			{"enter", "edit/toggle"},
			{"ctrl+s", "save"},
			{"esc", "close"},
			{"?", "help"},
			{"q", "quit"},
		}
		return renderFooterKeys(m.width, keys, "")
	}

	if m.deleteConfirming() {
		keys := []struct{ key, label string }{
			{"type name", "confirm text"},
			{"enter", "delete"},
			{"tab", "commit"},
			{"esc", "cancel"},
			{"ctrl+c", "quit"},
		}
		if !m.deleteReady {
			keys = []struct{ key, label string }{
				{"esc", "close"},
				{"ctrl+c", "quit"},
			}
		}
		return renderFooterKeys(m.width, keys, "")
	}

	var secondaryKeys []struct{ key, label string }
	if m.inProjectMode() {
		secondaryKeys = []struct{ key, label string }{
			{"h/l", "focus"},
			{"/", "filter"},
			{"i", "import"},
			{"p", "settings"},
			{"tab", "user"},
			{"?", "help"},
			{"q", "quit"},
		}
	} else {
		secondaryKeys = []struct{ key, label string }{
			{"h/l", "focus"},
			{"/", "filter"},
			{"i", "import"},
			{"p", "settings"},
			{"tab", "project"},
			{"?", "help"},
			{"q", "quit"},
		}
	}

	return renderFooterKeys(m.width, secondaryKeys, m.footerMessage())
}

func renderFooterKeys(width int, keys []struct{ key, label string }, message string) string {
	var parts []string
	for _, k := range keys {
		parts = append(parts, footerKeyStyle.Render(k.key)+" "+footerStyle.Render(k.label))
	}
	left := footerStyle.Render(strings.Join(parts, "  "))
	if message == "" || width <= 0 {
		return left
	}
	right := statusWarnStyle.Render(message)
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		return left
	}
	return left + strings.Repeat(" ", gap) + right
}

func renderInlineActions(actions []struct{ key, label string }) string {
	var parts []string
	for _, a := range actions {
		parts = append(parts, paneFooterKeyStyle.Render(a.key)+" "+paneFooterActionStyle.Render(a.label))
	}
	return strings.Join(parts, "  ")
}

func toggleScopeLabel(projectMode bool) string {
	if projectMode {
		return "user"
	}
	return "project"
}

func (m Model) footerMessage() string {
	if !m.inProjectMode() {
		return ""
	}
	sel := m.selectedSkill()
	if sel == nil {
		return ""
	}
	if !sel.InstalledClaude && !sel.InstalledCodex {
		return ""
	}
	return "this skill is installed in user scope"
}

func (m Model) renderWide(height int) string {
	frameHeight := paneFrameHeight(borderStyle)
	if height <= frameHeight+compactBodyThreshold {
		return m.renderCompact(height)
	}

	// Three-pane layout
	leftW := m.width * 28 / 100
	rightW := m.width * 24 / 100
	centerW := m.width - leftW - rightW - 6 // borders
	if leftW < 20 {
		leftW = 20
	}
	if rightW < 18 {
		rightW = 18
	}
	if centerW < 20 {
		centerW = 20
	}

	leftBorder := borderStyle
	centerBorder := borderStyle
	if m.focusPane == paneSkills {
		leftBorder = focusBorderStyle
	} else {
		centerBorder = focusBorderStyle
	}

	leftContentHeight := contentHeightForPane(height, leftBorder)
	centerContentHeight := contentHeightForPane(height, centerBorder)
	rightOuterHeight := height
	statusOuterHeight := max(1, rightOuterHeight/2)
	utilitiesOuterHeight := max(1, rightOuterHeight-statusOuterHeight)
	statusContentHeight := contentHeightForPane(statusOuterHeight, borderStyle)
	utilitiesContentHeight := contentHeightForPane(utilitiesOuterHeight, borderStyle)

	left := m.renderSkillList(leftW, leftContentHeight)
	center := m.renderDetails(centerW, centerContentHeight)
	status := m.renderStatus(rightW, statusContentHeight)
	utilities := m.renderScopeInfoPanel(rightW, utilitiesContentHeight)

	leftPane := leftBorder.Width(leftW).Height(leftContentHeight).Render(left)
	centerPane := centerBorder.Width(centerW).Height(centerContentHeight).Render(center)
	statusPane := borderStyle.Width(rightW).Height(statusContentHeight).Render(status)
	utilitiesPane := borderStyle.Width(rightW).Height(utilitiesContentHeight).Render(utilities)
	rightPane := lipgloss.JoinVertical(lipgloss.Left, statusPane, utilitiesPane)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, centerPane, rightPane)
}

func (m Model) renderModalBody(height int) string {
	w := m.width - 4
	if w < 1 {
		w = 1
	}
	cardWidth := min(72, max(42, w-8))
	if cardWidth > w {
		cardWidth = w
	}

	var content string
	switch {
	case m.commitPromptActive():
		content = m.renderCommitPrompt(cardWidth-6, max(8, min(height-4, 14)))
	case m.deleteConfirming():
		content = m.renderDeleteConfirm(cardWidth-6, max(10, min(height-4, 16)))
	case m.bulkImportConfirming():
		content = m.renderBulkImportConfirm(cardWidth-6, max(8, min(height-4, 14)))
	}

	card := modalStyle.Width(cardWidth).Render(content)
	return lipgloss.Place(w, height, lipgloss.Center, lipgloss.Center, card)
}

func (m Model) renderNarrow(height int) string {
	frameHeight := paneFrameHeight(borderStyle)
	if height <= (frameHeight*2)+1+2 {
		return m.renderCompact(height)
	}

	available := height - 1
	if available <= 0 {
		return m.renderCompact(height)
	}

	listOuterHeight := available / 2
	detailOuterHeight := available - listOuterHeight
	w := m.width - 4
	if w < 1 {
		w = 1
	}

	listContentHeight := contentHeightForPane(listOuterHeight, borderStyle)
	detailContentHeight := contentHeightForPane(detailOuterHeight, borderStyle)
	list := m.renderSkillList(w, listContentHeight)
	detail := m.renderDetails(w, detailContentHeight)

	return borderStyle.Width(w).Height(listContentHeight).Render(list) + "\n" +
		borderStyle.Width(w).Height(detailContentHeight).Render(detail)
}

func (m Model) renderSkillList(width, height int) string {
	if height <= 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(paneHeaderStyle.Render("Skills Inventory"))
	b.WriteString("\n")

	if m.filtering || m.filter != "" {
		b.WriteString(dimStyle.Render("Filter: ") + m.filter)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	if m.loading {
		b.WriteString(dimStyle.Render("Loading..."))
		return m.renderListPaneWithFooter(b.String(), m.renderPaneFooterActions(m.inProjectMode()), width, height)
	}

	if len(m.filtered) == 0 {
		b.WriteString(dimStyle.Render("No skills found"))
		return clipWrappedContent(b.String(), width, height)
	}

	start, end := windowRange(len(m.filtered), m.skillListVisibleItems(height), m.cursor)
	for i := start; i < end; i++ {
		v := m.filtered[i]
		if b.Len() > height*width {
			break
		}

		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		checkbox := userCheckboxFor(v)
		if m.inProjectMode() {
			checkbox = checkboxFor(v)
		}

		line := cursor + checkbox + " " + string(v.Skill.Name)
		if i == m.cursor {
			b.WriteString(selectedStyle.Render(line))
		} else {
			b.WriteString(normalStyle.Render(line))
		}
		b.WriteString("\n")
	}

	return m.renderListPaneWithFooter(b.String(), m.renderPaneFooterActions(m.inProjectMode()), width, height)
}

func checkboxFor(v app.SkillView) string {
	if skillHasFlag(v, reconcile.StatusUnmanaged) && (v.InstalledClaude || v.InstalledCodex) {
		if v.ProjectClaude || v.ProjectCodex {
			return statusInfoStyle.Render("[*]")
		}
		return statusInfoStyle.Render("[g]")
	}
	if v.ProjectClaude || v.ProjectCodex {
		if v.InstalledClaude || v.InstalledCodex {
			return enabledStyle.Render("[*]")
		}
		return enabledStyle.Render("[x]")
	}
	if v.InstalledClaude || v.InstalledCodex {
		return statusWarnStyle.Render("[g]")
	}
	return disabledStyle.Render("[ ]")
}

func userCheckboxFor(v app.SkillView) string {
	if skillHasFlag(v, reconcile.StatusUnmanaged) && (v.InstalledClaude || v.InstalledCodex) {
		return statusInfoStyle.Render("[x]")
	}
	if v.InstalledClaude || v.InstalledCodex {
		return enabledStyle.Render("[x]")
	}
	return disabledStyle.Render("[ ]")
}

func skillHasFlag(v app.SkillView, flag reconcile.StatusFlag) bool {
	for _, existing := range v.Flags {
		if existing == flag {
			return true
		}
	}
	return false
}

func truncateToWindow(content string, height, offset int) (visible string, clampedOffset int, moreAbove bool, moreBelow bool) {
	lines := strings.Split(content, "\n")
	total := len(lines)

	if total <= height {
		return content, 0, false, false
	}

	maxOffset := total - height
	if offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}

	end := offset + height
	if end > total {
		end = total
	}

	return strings.Join(lines[offset:end], "\n"), offset, offset > 0, end < total
}

func (m Model) renderDetails(width, height int) string {
	if height <= 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(paneHeaderStyle.Render("Details"))
	b.WriteString("\n\n")

	sel := m.selectedSkill()
	if sel == nil {
		b.WriteString(dimStyle.Render("Select a skill to view details"))
		return clipWrappedContent(b.String(), width, height)
	}
	if m.selectedSkillIsUnmanaged() {
		b.WriteString(infoBannerStyle.Render("`i` to import this unmanaged skill"))
		b.WriteString("\n\n")
	}

	s := sel.Skill
	b.WriteString(labelStyle.Render("Name:        ") + valueStyle.Render(string(s.Name)) + "\n")
	if len(s.Tags) > 0 {
		b.WriteString(labelStyle.Render("Tags:        ") + valueStyle.Render(strings.Join(s.Tags, ", ")) + "\n")
	}
	targets := make([]string, len(s.Targets))
	for i, t := range s.Targets {
		targets[i] = string(t)
	}
	b.WriteString(labelStyle.Render("Supports:    ") + valueStyle.Render(strings.Join(targets, ", ")) + "\n")

	if s.Description != "" {
		b.WriteString("\n" + labelStyle.Render("Description") + "\n")
		b.WriteString(normalStyle.Render(s.Description) + "\n")
	}

	if m.preview != nil && m.preview.Skill.Name == s.Name && len(m.preview.Files) > 0 {
		b.WriteString("\n" + labelStyle.Render("Files") + "\n")
		for _, f := range m.preview.Files {
			b.WriteString("  " + dimStyle.Render(f) + "\n")
		}
	}

	if m.preview != nil && m.preview.Skill.Name == s.Name {
		b.WriteString("\n" + labelStyle.Render("Preview") + "\n")
		b.WriteString(dimStyle.Render(strings.Repeat("─", min(width, 40))) + "\n")
		for _, line := range strings.Split(m.preview.Markdown, "\n") {
			b.WriteString(normalStyle.Render(line) + "\n")
		}
	}

	content := wrapContent(b.String(), width)
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	// Content fits — no scrolling needed
	if totalLines <= height {
		return clipContent(content, height)
	}

	// Reserve lines for scroll indicators
	visibleHeight := height
	hasAbove := m.detailScroll > 0
	if hasAbove {
		visibleHeight-- // room for ▲
	}
	// Pessimistically reserve for ▼; we check after truncation
	visibleHeight--
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	visible, _, _, moreBelow := truncateToWindow(content, visibleHeight, m.detailScroll)

	if !moreBelow && hasAbove {
		// We reserved a line for ▼ but don't need it — reclaim it
		visibleHeight++
		visible, _, _, moreBelow = truncateToWindow(content, visibleHeight, m.detailScroll)
	}

	var result strings.Builder
	if hasAbove {
		result.WriteString(dimStyle.Render("▲") + "\n")
	}
	result.WriteString(visible)
	if moreBelow {
		result.WriteString("\n" + dimStyle.Render("▼"))
	}

	return result.String()
}

func (m Model) renderStatus(width, height int) string {
	var b strings.Builder

	if m.doctor != nil {
		b.WriteString(paneHeaderStyle.Render("Doctor Report"))
		b.WriteString("\n\n")
		for _, c := range m.doctor.Checks {
			icon := statusOKStyle.Render("OK")
			if !c.OK {
				icon = statusWarnStyle.Render("!!")
			}
			b.WriteString(fmt.Sprintf(" [%s] %s\n", icon, c.Name))
			b.WriteString(dimStyle.Render("       "+c.Detail) + "\n")
		}
		return clipWrappedContent(b.String(), width, height)
	}

	b.WriteString(paneHeaderStyle.Render("Status"))
	b.WriteString("\n\n")

	sel := m.selectedSkill()
	if sel == nil {
		return clipWrappedContent(b.String(), width, height)
	}

	b.WriteString(m.renderTargetRows(m.skillTargetRows(sel)))

	// Flags
	if len(sel.Flags) > 0 {
		b.WriteString("\n")
		for _, f := range sel.Flags {
			var s lipgloss.Style
			switch f {
			case reconcile.StatusCurrent:
				s = statusOKStyle
			case reconcile.StatusInactive:
				s = dimStyle
			default:
				s = statusWarnStyle
			}
			b.WriteString(" " + s.Render(displayStatusFlag(f)) + "\n")
		}
	}
	if sel.Orphaned {
		b.WriteString("\n" + dimStyle.Render(" press i to import and recover") + "\n")
	}
	return clipWrappedContent(b.String(), width, height)
}

func displayStatusFlag(flag reconcile.StatusFlag) string {
	if flag == reconcile.StatusUnmanaged {
		return "not in repo"
	}
	return string(flag)
}

func (m Model) renderScopeInfoPanel(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(paneHeaderStyle.Render("Utilities"))
	b.WriteString("\n\n")

	importLabel := dimStyle.Render("import")
	if (m.inProjectMode() && m.projectHintCount > 0) || (!m.inProjectMode() && m.userHintCount > 0) {
		importLabel = statusInfoStyle.Render("import")
	}
	syncLabel := dimStyle.Render("sync repo")
	if m.syncAttention {
		syncLabel = statusWarnStyle.Render("sync repo *")
	}
	if m.inProjectMode() {
		b.WriteString(footerKeyStyle.Render("s") + " " + syncLabel + "\n")
		b.WriteString(footerKeyStyle.Render("d") + " " + dimStyle.Render("doctor") + "\n")
		b.WriteString(footerKeyStyle.Render("i") + " " + importLabel)
		if m.projectHintCount > 0 {
			b.WriteString("\n")
			b.WriteString(statusInfoStyle.Render(fmt.Sprintf("Untracked skills: %d ready", m.projectHintCount)))
		}
		return clipWrappedContent(b.String(), width, height)
	}

	b.WriteString(footerKeyStyle.Render("s") + " " + syncLabel + "\n")
	b.WriteString(footerKeyStyle.Render("d") + " " + dimStyle.Render("doctor") + "\n")
	b.WriteString(footerKeyStyle.Render("i") + " " + importLabel)
	return clipWrappedContent(b.String(), width, height)
}

func (m Model) renderScrollableHelp(text string, height int) string {
	rendered := normalStyle.Render(text)
	wrapped := wrapContent(rendered, m.width)
	visible, _, moreAbove, moreBelow := truncateToWindow(wrapped, height, m.helpScroll)

	var b strings.Builder
	if moreAbove {
		b.WriteString(dimStyle.Render("▲") + "\n")
		// Re-truncate with one less line to make room for the indicator
		visible, _, _, moreBelow = truncateToWindow(wrapped, height-1, m.helpScroll)
	}
	b.WriteString(visible)
	if moreBelow {
		b.WriteString("\n" + dimStyle.Render("▼"))
	}
	return b.String()
}

func (m Model) renderHelp(height int) string {
	help := "\n" + Logo + `

  Navigation
    j/k, up/down    Move selection / scroll details
    ctrl+u/d        Move or scroll half a page
    g/G, home/end   Jump to top/bottom
    h/l, left/right Switch pane focus
    /               Start filter
    esc             Clear filter / close help

  Actions
    c/x             Equip or unequip the visible target rows
    a               Equip all visible supported targets
    i               Open import for current scope
    D               Delete selected repo skill
    s               Sync repository and refresh managed installs
    d               Doctor (health check)
    p               Open settings

  Scope
    tab             Toggle user/project install scope

  General
    ?               Toggle help
    q, ctrl+c       Quit
`
	return m.renderScrollableHelp(help, height)
}

func (m Model) renderImportHelp(height int) string {
	help := `
  Loadout Import

  Navigation
    j/k, up/down    Move between import candidates
    ctrl+u/d        Move half a page
    g/G, home/end   Jump to top/bottom

  Actions
    enter or i      Import selected candidate into repo
    A               Import all ready candidates (with confirmation)
    c               Toggle auto-commit
    b               Browse directories to scan
    r               Reset to configured roots
    tab             Toggle user/project app scope
    enter/esc       After uncommitted import, commit now or leave uncommitted
    esc or p        Close import

  Browse Mode
    enter           Open selected directory
    backspace       Go to parent directory
    s               Scan current directory
    tab             Reset to the current scope defaults
    esc             Cancel browse

  General
    ?               Toggle help
    q, ctrl+c       Quit
`
	return m.renderScrollableHelp(help, height)
}

func (m Model) renderSettingsHelp(height int) string {
	help := `
  Loadout Settings

  Navigation
    j/k, up/down    Move between fields
    ctrl+u/d        Move half a page
    g/G, home/end   Jump to top/bottom field

  Editing
    enter           Edit path field or toggle enabled field
    backspace       Delete character while editing
    esc             Cancel field edit / close settings
    ctrl+s          Save settings

  General
    p               Close settings
    ?               Toggle help
    q, ctrl+c       Quit
`
	return m.renderScrollableHelp(help, height)
}

const compactBodyThreshold = 6

func contentHeightForPane(totalHeight int, style lipgloss.Style) int {
	frameHeight := paneFrameHeight(style)
	contentHeight := totalHeight - frameHeight
	if contentHeight < 1 {
		return 1
	}
	return contentHeight
}

func paneFrameHeight(style lipgloss.Style) int {
	_, frameHeight := style.GetFrameSize()
	return frameHeight
}

func clipContent(content string, height int) string {
	if height <= 0 {
		return ""
	}

	lines := strings.Split(content, "\n")
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return ""
	}
	if len(lines) <= height {
		return strings.Join(lines, "\n")
	}
	if height == 1 {
		return dimStyle.Render("▼")
	}

	visible := append([]string{}, lines[:height-1]...)
	visible = append(visible, dimStyle.Render("▼"))
	return strings.Join(visible, "\n")
}

func clipWrappedContent(content string, width, height int) string {
	return clipContent(wrapContent(content, width), height)
}

func wrapContent(content string, width int) string {
	if width <= 0 {
		return content
	}
	return lipgloss.NewStyle().Width(width).Render(content)
}

func (m Model) renderCompact(height int) string {
	if height <= 0 {
		return ""
	}

	var sections []string
	sections = append(sections, clipContent(m.renderCompactList(), height))
	if height > 2 {
		remaining := height - lipgloss.Height(sections[0]) - 1
		if remaining > 0 {
			sections = append(sections, clipContent(m.renderCompactDetails(), remaining))
		}
	}
	return clipContent(strings.Join(sections, "\n"), height)
}

func (m Model) renderCompactList() string {
	var b strings.Builder
	b.WriteString(paneHeaderStyle.Render("Skills Inventory"))
	b.WriteString("\n")
	if len(m.filtered) == 0 {
		b.WriteString(dimStyle.Render("No skills found"))
		return b.String()
	}
	for i, skill := range m.filtered {
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		check := userCheckboxFor(skill)
		if m.inProjectMode() {
			check = checkboxFor(skill)
		}
		b.WriteString(prefix + check + " " + string(skill.Skill.Name) + "\n")
	}
	return b.String()
}

func (m Model) renderCompactDetails() string {
	return m.renderStatus(m.width, compactBodyThreshold)
}

func (m Model) renderCommitPrompt(width, height int) string {
	if height <= 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(modalTitleStyle.Render("Repo Changed: commit now?"))
	b.WriteString("\n\n")
	if m.commitPrompt == nil {
		b.WriteString(dimStyle.Render("No pending repo change"))
		return clipWrappedContent(b.String(), width, height)
	}

	prompt := m.commitPrompt
	actionLabel := prompt.ActionKind
	if actionLabel != "" {
		actionLabel = strings.ToUpper(actionLabel[:1]) + actionLabel[1:]
	}
	b.WriteString(labelStyle.Render("Action:      ") + valueStyle.Render(actionLabel) + "\n")
	b.WriteString(labelStyle.Render("Skill:       ") + valueStyle.Render(string(prompt.SkillName)) + "\n")
	b.WriteString(labelStyle.Render("Repo Path:   ") + valueStyle.Render(prompt.RepoPath) + "\n")
	b.WriteString(labelStyle.Render("Commit:      ") + valueStyle.Render(prompt.CommitMessage) + "\n")
	b.WriteString("\n" + dimStyle.Render("If you cancel, the repo stays dirty.") + "\n")
	b.WriteString("\n" + modalPrimaryStyle.Render("Enter to commit this repo change") + "\n")
	b.WriteString(dimStyle.Render("Esc to leave it uncommitted") + "\n")
	return clipWrappedContent(b.String(), width, height)
}

func (m Model) renderBulkImportConfirm(width, height int) string {
	if height <= 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(modalTitleStyle.Render("Import All Ready Skills?"))
	b.WriteString("\n\n")
	if m.bulkImport == nil {
		b.WriteString(dimStyle.Render("No pending bulk import"))
		return clipWrappedContent(b.String(), width, height)
	}

	b.WriteString(labelStyle.Render("Ready:       ") + valueStyle.Render(fmt.Sprintf("%d skills", m.bulkImport.ReadyCount)) + "\n")
	b.WriteString(labelStyle.Render("Skipped:     ") + valueStyle.Render(fmt.Sprintf("%d blocked", m.bulkImport.SkippedCount)) + "\n")
	if m.bulkImport.AutoCommit {
		b.WriteString("\n" + dimStyle.Render("One commit per imported skill.") + "\n")
	} else {
		b.WriteString("\n" + dimStyle.Render("Repo changes left uncommitted.") + "\n")
	}
	b.WriteString("\n" + modalPrimaryStyle.Render("Enter to import all") + "\n")
	b.WriteString(dimStyle.Render("Esc to cancel") + "\n")
	return clipWrappedContent(b.String(), width, height)
}

func targetActionLabel(installed bool, label string) string {
	if installed {
		return "unequip " + label
	}
	return "equip " + label
}

func (m Model) renderListPaneWithFooter(content, footer string, width, height int) string {
	if height <= 0 {
		return ""
	}
	content = strings.TrimRight(content, "\n")
	if footer == "" {
		return clipWrappedContent(content, width, height)
	}
	footerLines := strings.Count(footer, "\n") + 1
	footerHeight := min(footerLines, height)
	bodyHeight := height - footerHeight
	if bodyHeight < 1 {
		bodyHeight = 1
		footerHeight = max(0, height-1)
	}

	body := clipWrappedContent(content, width, bodyHeight)
	if footerHeight == 0 {
		return body
	}
	footerRendered := clipWrappedContent(footer, width, footerHeight)
	if body == "" {
		return footerRendered
	}
	return body + "\n" + footerRendered
}

func (m Model) renderPaneFooterActions(project bool) string {
	rows := m.currentSkillTargetRows()

	var b strings.Builder
	b.WriteString(dimStyle.Render(strings.Repeat("─", 12)) + "\n")
	lines := m.paneFooterActionLines(rows, project)
	if len(lines) == 0 {
		b.WriteString(paneFooterStyle.Render("no actions"))
		return b.String()
	}
	for i, line := range lines {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(line)
	}
	return b.String()
}

func (m Model) paneFooterActionLines(rows []targetRow, project bool) []string {
	var lines []string
	supportedCount := 0
	for _, row := range rows {
		if row.supported {
			supportedCount++
			lines = append(lines, paneFooterKeyStyle.Render(row.key)+" "+paneFooterActionStyle.Render(row.actionLabel))
			continue
		}

		lines = append(lines, paneFooterKeyStyle.Render(row.key)+" "+dimStyle.Render(actionLabelNA(row.actionLabel)))
	}
	bulkLabel, bulkEnabled := m.bulkActionState(project)
	lines = append(lines, m.bulkActionLine(supportedCount > 1 && bulkEnabled, bulkLabel))
	lines = append(lines, dimStyle.Render(strings.Repeat("─", 12)))
	lines = append(lines, paneFooterKeyStyle.Render("D")+" "+paneFooterDestructiveStyle.Render("delete repo copy"))
	return lines
}

func (m Model) bulkActionLine(enabled bool, label string) string {
	if enabled {
		return paneFooterKeyStyle.Render("a") + " " + paneFooterActionStyle.Render(label)
	}
	return paneFooterKeyStyle.Render("a") + " " + dimStyle.Render(actionLabelNA(label))
}

func (m Model) bulkActionState(project bool) (string, bool) {
	sel := m.selectedSkill()
	if sel == nil {
		if project {
			return "equip all (project)", false
		}
		return "equip all (user)", false
	}

	supportedCount := 0
	installedCount := 0
	for _, target := range m.enabledTargets() {
		if !sel.Skill.SupportsTarget(target) {
			continue
		}
		supportedCount++
		if project {
			if projectInstalledForTarget(*sel, target) {
				installedCount++
			}
			continue
		}
		if skillInstalledForTarget(*sel, target) {
			installedCount++
		}
	}

	if project {
		if supportedCount > 1 && installedCount == supportedCount {
			return "unequip all (project)", true
		}
		return "equip all (project)", supportedCount > 1
	}
	if supportedCount > 1 && installedCount == supportedCount {
		return "unequip all (user)", true
	}
	return "equip all (user)", supportedCount > 1
}

func (m Model) renderDeleteConfirm(width, height int) string {
	var b strings.Builder
	b.WriteString(modalTitleStyle.Render("Delete Skill From Repo"))
	b.WriteString("\n\n")

	sel := m.selectedSkill()
	if sel == nil {
		b.WriteString(dimStyle.Render("No skill selected"))
		return clipWrappedContent(b.String(), width, height)
	}

	b.WriteString(labelStyle.Render("Name:        ") + valueStyle.Render(string(m.deleteSkillName)) + "\n")
	b.WriteString(labelStyle.Render("Repo Path:   ") + valueStyle.Render(sel.Skill.Path) + "\n")

	if !m.deleteReady {
		b.WriteString(labelStyle.Render("Blocked") + "\n")
		for _, blocker := range m.deleteBlockers {
			b.WriteString(dimStyle.Render(" - "+blocker) + "\n")
		}
		b.WriteString("\n" + dimStyle.Render("Delete requires uninstalling managed copies first.") + "\n")
		b.WriteString("\n" + modalPrimaryStyle.Render("Esc to close") + "\n")
		return clipWrappedContent(b.String(), width, height)
	}

	mode := "off"
	if m.deleteCommit {
		mode = "on"
	}
	b.WriteString(labelStyle.Render("Auto-Commit: ") + valueStyle.Render(mode) + "\n")
	b.WriteString("\n" + labelStyle.Render("Confirm") + "\n")
	b.WriteString(dimStyle.Render("Type the exact skill name, then press enter.") + "\n")
	b.WriteString(valueStyle.Render(string(m.deleteSkillName)) + "\n")
	b.WriteString(selectedStyle.Render("> "+m.deleteInput+"█") + "\n")
	b.WriteString("\n" + dimStyle.Render("If you cancel, nothing is deleted.") + "\n")
	b.WriteString("\n" + modalPrimaryStyle.Render("Enter to delete this skill") + "\n")
	b.WriteString(dimStyle.Render("Tab to toggle auto-commit") + "\n")
	b.WriteString(dimStyle.Render("Esc to cancel") + "\n")
	return clipWrappedContent(b.String(), width, height)
}

func (m Model) currentSkillTargetRows() []targetRow {
	if sel := m.selectedSkill(); sel != nil {
		return m.skillTargetRows(sel)
	}
	return nil
}

func (m Model) renderSettings(height int) string {
	w := m.width - 4
	if w < 1 {
		w = 1
	}
	contentHeight := contentHeightForPane(height, borderStyle)
	return borderStyle.Width(w).Height(contentHeight).Render(m.renderSettingsContent(w, contentHeight))
}

func (m Model) renderImport(height int) string {
	w := m.width - 4
	if w < 1 {
		w = 1
	}
	if w < 90 {
		contentHeight := contentHeightForPane(height, borderStyle)
		return borderStyle.Width(w).Height(contentHeight).Render(m.renderImportContent(w, contentHeight))
	}

	leftW := w * 38 / 100
	if leftW < 28 {
		leftW = 28
	}
	rightW := w - leftW - 2
	if rightW < 30 {
		rightW = 30
		leftW = max(20, w-rightW-2)
	}
	leftContentHeight := contentHeightForPane(height, borderStyle)
	rightContentHeight := contentHeightForPane(height, borderStyle)
	left := borderStyle.Width(leftW).Height(leftContentHeight).Render(m.renderImportListPane(leftW, leftContentHeight))
	right := borderStyle.Width(rightW).Height(rightContentHeight).Render(m.renderImportPreviewPane(rightW, rightContentHeight))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m Model) renderImportContent(width, height int) string {
	list := m.renderImportListPane(width, max(1, height/2))
	preview := m.renderImportPreviewPane(width, max(1, height-height/2-1))
	if preview == "" {
		return list
	}
	return clipContent(list+"\n"+preview, height)
}

func (m Model) renderImportListPane(width, height int) string {
	var b strings.Builder
	if m.importBrowsing {
		b.WriteString(paneHeaderStyle.Render("Browse"))
	} else {
		b.WriteString(paneHeaderStyle.Render("Import"))
	}
	b.WriteString("\n")
	if m.importBrowsing {
		b.WriteString("\n")
		if len(m.browseDirEntries) == 0 {
			line := "> ../"
			if m.browseCursor > 0 {
				line = "  ../"
			}
			if m.browseCursor == 0 {
				b.WriteString(selectedStyle.Render(line) + "\n")
			} else {
				b.WriteString(normalStyle.Render(line) + "\n")
			}
			b.WriteString(dimStyle.Render("No subdirectories"))
			return m.renderListPaneWithFooter(b.String(), m.renderImportPaneFooter(), width, height)
		}

		start, end := windowRange(len(m.browseDirEntries)+1, m.importListVisibleItems(height), m.browseCursor)
		if start == 0 {
			parentLine := "  ../"
			if m.browseCursor == 0 {
				parentLine = "> ../"
				b.WriteString(selectedStyle.Render(parentLine) + "\n")
			} else {
				b.WriteString(normalStyle.Render(parentLine) + "\n")
			}
		}

		for i := max(0, start-1); i < min(len(m.browseDirEntries), end-1); i++ {
			name := m.browseDirEntries[i]
			cursor := "  "
			if i+1 == m.browseCursor {
				cursor = "> "
			}
			line := cursor + name + string(filepath.Separator)
			if i+1 == m.browseCursor {
				b.WriteString(selectedStyle.Render(line) + "\n")
			} else {
				b.WriteString(normalStyle.Render(line) + "\n")
			}
		}
		return m.renderListPaneWithFooter(b.String(), m.renderImportPaneFooter(), width, height)
	}

	if m.importCustomDir != "" {
		b.WriteString("\n" + dimStyle.Render("Scanned: "+m.importCustomDir) + "\n")
	}
	b.WriteString("\n")

	if m.loading {
		b.WriteString(dimStyle.Render("Loading..."))
		return m.renderListPaneWithFooter(b.String(), m.renderImportPaneFooter(), width, height)
	}
	if len(m.imports) == 0 {
		if m.importCustomDir != "" {
			b.WriteString(dimStyle.Render("No importable skills found in scanned directory") + "\n")
		} else {
			b.WriteString(dimStyle.Render("No importable skills found in configured user roots") + "\n")
		}
		return m.renderListPaneWithFooter(b.String(), m.renderImportPaneFooter(), width, height)
	}

	start, end := windowRange(len(m.imports), m.importListVisibleItems(height), m.cursor)
	for i := start; i < end; i++ {
		candidate := m.imports[i]
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		status := "ready"
		if candidate.Orphan && candidate.Ready {
			status = "recoverable"
		} else if !candidate.Ready {
			status = candidate.Problem
		}
		line := fmt.Sprintf("%s%s [%s] %s", cursor, candidate.SkillName, formatTargets(candidate.FromRoots), status)
		if i == m.cursor {
			b.WriteString(selectedStyle.Render(line) + "\n")
		} else {
			b.WriteString(normalStyle.Render(line) + "\n")
		}
		b.WriteString(dimStyle.Render("    "+candidate.SourceDir) + "\n")
	}

	return m.renderListPaneWithFooter(b.String(), m.renderImportPaneFooter(), width, height)
}

func (m Model) renderImportPaneFooter() string {
	if m.importBrowsing {
		row1 := renderInlineActions([]struct{ key, label string }{
			{"enter", "open"}, {"backspace", "up"},
		})
		row2 := renderInlineActions([]struct{ key, label string }{
			{"s", "scan here"},
		})
		return row1 + "\n" + row2
	}

	if !m.loading && len(m.imports) > 0 {
		row1 := renderInlineActions([]struct{ key, label string }{
			{"enter", "import"}, {"A", "import all"},
		})
		commitLabel := "auto-commit (OFF)"
		if m.importCommit {
			commitLabel = "auto-commit (ON)"
		}
		row2 := renderInlineActions([]struct{ key, label string }{
			{"b", "browse"}, {"c", commitLabel},
		})
		return row1 + "\n" + row2
	}

	return renderInlineActions([]struct{ key, label string }{
		{"b", "browse"},
	})
}

func (m Model) renderImportPreviewPane(width, height int) string {
	if height <= 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(paneHeaderStyle.Render("Preview"))
	b.WriteString("\n\n")
	if m.importBrowsing {
		if m.inProjectMode() {
			b.WriteString(dimStyle.Render("Browse project files, then press s to scan this directory for skills.") + "\n")
		} else {
			b.WriteString(dimStyle.Render("Browse to a directory, then press s to scan for local skills.") + "\n")
		}
		if m.browseDir != "" {
			b.WriteString("\n" + labelStyle.Render("Current Path") + "\n")
			b.WriteString(valueStyle.Render(m.browseDir))
		}
		return clipWrappedContent(b.String(), width, height)
	}

	if m.loading {
		b.WriteString(dimStyle.Render("Loading preview..."))
		return clipWrappedContent(b.String(), width, height)
	}

	if m.importPreview == nil {
		b.WriteString(dimStyle.Render("Select a discovered skill to preview before import."))
		return clipWrappedContent(b.String(), width, height)
	}

	preview := m.importPreview
	b.WriteString(labelStyle.Render("Name:        ") + valueStyle.Render(string(preview.Skill.Name)) + "\n")
	b.WriteString(labelStyle.Render("Source:      ") + valueStyle.Render(preview.SourceDir) + "\n")
	b.WriteString(labelStyle.Render("Targets:     ") + valueStyle.Render(formatTargets(preview.Skill.Targets)) + "\n")
	status := "ready"
	if !preview.Ready {
		status = preview.Problem
	}
	b.WriteString(labelStyle.Render("Status:      ") + valueStyle.Render(status) + "\n")
	if preview.Skill.Description != "" {
		b.WriteString("\n" + labelStyle.Render("Description") + "\n")
		b.WriteString(normalStyle.Render(preview.Skill.Description) + "\n")
	}
	if preview.Markdown != "" {
		b.WriteString("\n" + labelStyle.Render("Preview") + "\n")
		b.WriteString(dimStyle.Render(strings.Repeat("─", min(width, 40))) + "\n")
		for _, line := range strings.Split(preview.Markdown, "\n") {
			b.WriteString(normalStyle.Render(line) + "\n")
		}
	}
	return clipWrappedContent(b.String(), width, height)
}

func formatTargets(targets []domain.Target) string {
	if len(targets) == 0 {
		return ""
	}
	parts := make([]string, len(targets))
	for i, target := range targets {
		parts[i] = string(target)
	}
	return strings.Join(parts, "+")
}

func (m Model) renderSettingsContent(width, height int) string {
	var b strings.Builder
	b.WriteString(paneHeaderStyle.Render("Settings"))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("Edit the persisted Loadout config.") + "\n")
	b.WriteString(dimStyle.Render(shortenHomePath(config.DefaultPath())) + "\n\n")

	fields := []settingsField{
		settingsFieldRepo,
		settingsFieldClaudeEnabled,
		settingsFieldClaudePath,
		settingsFieldCodexEnabled,
		settingsFieldCodexPath,
		settingsFieldImportAutoCommit,
		settingsFieldDeleteAutoCommit,
	}
	start, end := windowRange(len(fields), m.settingsVisibleFields(height), int(m.settingsField))
	for _, field := range fields[start:end] {
		cursor := "  "
		if field == m.settingsField {
			cursor = "> "
		}

		line := cursor + m.settingsFieldLabel(field) + ": " + m.settingsFieldValue(field)
		switch {
		case field == m.settingsField && m.settingsEditing:
			b.WriteString(selectedStyle.Render(line+"█") + "\n")
		case field == m.settingsField:
			b.WriteString(selectedStyle.Render(line) + "\n")
		default:
			b.WriteString(normalStyle.Render(line) + "\n")
		}
	}

	b.WriteString("\n" + dimStyle.Render("Actions") + "\n")
	b.WriteString(dimStyle.Render(" enter   edit path or toggle target") + "\n")
	b.WriteString(dimStyle.Render(" ctrl+s  save settings") + "\n")
	b.WriteString(dimStyle.Render(" esc     close settings") + "\n")

	return clipWrappedContent(b.String(), width, height)
}

func (m Model) settingsFieldLabel(field settingsField) string {
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

func (m Model) settingsFieldValue(field settingsField) string {
	switch field {
	case settingsFieldRepo:
		return m.settings.RepoPath
	case settingsFieldClaudeEnabled:
		return enabledLabel(m.settings.ClaudeEnabled)
	case settingsFieldClaudePath:
		return m.settings.ClaudePath
	case settingsFieldCodexEnabled:
		return enabledLabel(m.settings.CodexEnabled)
	case settingsFieldCodexPath:
		return m.settings.CodexPath
	case settingsFieldImportAutoCommit:
		return enabledLabel(m.settings.ImportAutoCommit)
	case settingsFieldDeleteAutoCommit:
		return enabledLabel(m.settings.DeleteAutoCommit)
	default:
		return ""
	}
}

type targetRow struct {
	label       string
	key         string
	supported   bool
	installed   bool
	user        bool
	project     bool
	actionLabel string
	statusLabel string
}

func (m Model) skillTargetRows(sel *app.SkillView) []targetRow {
	var rows []targetRow
	for _, target := range m.enabledTargets() {
		userInstalled := skillInstalledForTarget(*sel, target)
		installed := userInstalled
		if m.inProjectMode() {
			installed = projectInstalledForTarget(*sel, target)
		}
		label := targetLabel(target)
		actionLabel := targetActionLabel(installed, label)
		statusLabel := actionLabel
		if m.inProjectMode() {
			actionLabel = "equip " + label + " (project)"
			if installed {
				actionLabel = "unequip " + label + " (project)"
			}
			statusLabel = ""
		} else {
			actionLabel += " (user)"
		}
		rows = append(rows, targetRow{
			label:       label,
			key:         targetKey(target),
			supported:   sel.Skill.SupportsTarget(target),
			installed:   installed,
			user:        userInstalled,
			project:     m.inProjectMode(),
			actionLabel: actionLabel,
			statusLabel: statusLabel,
		})
	}
	return rows
}

func actionLabelNA(label string) string {
	switch {
	case strings.HasSuffix(label, " (user)"):
		return strings.TrimSuffix(label, " (user)") + " (n/a)"
	case strings.HasSuffix(label, " (project)"):
		return strings.TrimSuffix(label, " (project)") + " (n/a)"
	default:
		return label + " (n/a)"
	}
}

func (m Model) renderTargetRows(rows []targetRow) string {
	var b strings.Builder
	if len(rows) == 0 {
		return dimStyle.Render(" no enabled targets")
	}
	for _, row := range rows {
		check := "[ ]"
		lineStyle := normalStyle
		if row.installed {
			check = enabledStyle.Render("[x]")
		} else if row.project && row.user {
			check = statusWarnStyle.Render("[g]")
		}

		actionHint := row.key + " " + compactTargetAction(row)
		var statusParts []string
		statusParts = append(statusParts, " "+check+" "+row.label)

		if !row.supported {
			check = dimStyle.Render("[-]")
			lineStyle = dimStyle
			statusParts[0] = " " + check + " " + row.label
			actionHint = row.key + " n/a"
		} else if row.project && row.statusLabel != "" {
			statusParts = append(statusParts, row.statusLabel)
		}

		statusParts = append(statusParts, actionHint)
		line := strings.Join(statusParts, "  ")
		b.WriteString(lineStyle.Render(line) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func compactTargetAction(row targetRow) string {
	if !row.supported {
		return "n/a"
	}
	if row.installed {
		if row.project {
			return "unequip project"
		}
		return "unequip"
	}
	if row.project {
		return "equip project"
	}
	return "equip"
}

func targetLabel(target domain.Target) string {
	tStr := string(target)
	return strings.ToUpper(tStr[:1]) + tStr[1:]
}

func targetKey(target domain.Target) string {
	switch target {
	case domain.TargetClaude:
		return "c"
	case domain.TargetCodex:
		return "x"
	default:
		return ""
	}
}

func skillInstalledForTarget(sel app.SkillView, target domain.Target) bool {
	switch target {
	case domain.TargetClaude:
		return sel.InstalledClaude
	case domain.TargetCodex:
		return sel.InstalledCodex
	default:
		return false
	}
}

func projectInstalledForTarget(sel app.SkillView, target domain.Target) bool {
	switch target {
	case domain.TargetClaude:
		return sel.ProjectClaude
	case domain.TargetCodex:
		return sel.ProjectCodex
	default:
		return false
	}
}

func enabledLabel(enabled bool) string {
	if enabled {
		return "yes"
	}
	return "no"
}
