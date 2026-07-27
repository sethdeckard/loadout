package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	compactWidthThreshold = 60
	importSplitWidth      = 90

	importBrowseHeaderLines   = 2
	importListBaseHeaderLines = 2
	settingsPreambleLines     = 5
	settingsActionLines       = 5
)

func (m Model) mainBodyHeight() int {
	headerHeight := lipgloss.Height(m.renderHeader())
	footerHeight := lipgloss.Height(m.renderFooter())
	return max(1, m.height-headerHeight-footerHeight-2)
}

func (m Model) skillListContentHeight() int {
	bodyHeight := m.mainBodyHeight()
	if m.width < compactWidthThreshold {
		listOuterHeight, _ := compactPaneHeights(bodyHeight)
		return contentHeightForPane(listOuterHeight, borderStyle)
	}

	border := borderStyle
	if m.focusPane == paneSkills {
		border = focusBorderStyle
	}
	return contentHeightForPane(bodyHeight, border)
}

// skillListHeader returns the lines above the skill rows: the pane title, the
// filter echo while filtering, and the blank separator as the empty final line.
// renderSkillList and the budget below share it so the reserved header height is
// the height actually rendered.
func (m Model) skillListHeader(width int) string {
	lines := []string{paneHeaderStyle.Render("Skills Inventory")}
	if m.filtering || m.filter != "" {
		// The filter is unbounded user input. Keep the echo to a single line,
		// showing the tail so the characters just typed stay visible; wrapping it
		// would eat the rows the budget below reserves.
		echo := truncateCellsLeft(m.filter, max(1, width-filterEchoLabelWidth))
		lines = append(lines, dimStyle.Render("Filter: ")+echo)
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

// filterEchoLabelWidth is the display width of the "Filter: " label.
const filterEchoLabelWidth = 8

// skillListHeaderLines measures the rendered header rather than assuming a line
// count, so the reserved height always matches what renderSkillList draws.
func (m Model) skillListHeaderLines(width int) int {
	return countLines(wrapContent(m.skillListHeader(width), width))
}

// skillListPaneBudget splits a pane's content height into the number of skill
// rows to draw and the lines reserved for the action panel. The renderer and the
// paging math both consume this, so the two cannot drift apart: everything is
// counted in rendered lines, and the footer is measured wrapped to pane width.
//
// The list has priority. When the pane is too short for both, the action panel
// yields lines rather than squeezing the list out entirely.
func (m Model) skillListPaneBudget(width, height int) (rows, footerHeight int) {
	footerHeight = countLines(wrapContent(m.renderPaneFooterActions(width, m.inProjectMode()), width))
	headerLines := m.skillListHeaderLines(width)
	if height-headerLines-footerHeight < 1 {
		footerHeight = max(0, height-headerLines-1)
	}
	return max(1, height-headerLines-footerHeight), footerHeight
}

// usesCompactLayout reports whether View's body falls back to renderCompact,
// which draws a bare windowed list with no pane frame or action panel. It
// mirrors the height checks at the top of renderWide and renderNarrow.
func (m Model) usesCompactLayout() bool {
	bodyHeight := m.mainBodyHeight()
	frameHeight := paneFrameHeight(borderStyle)
	if m.width < compactWidthThreshold {
		return bodyHeight <= (frameHeight*2)+1+2
	}
	return bodyHeight <= frameHeight+compactBodyThreshold
}

// skillListRows is how many skill rows the current layout can draw, for callers
// that page the cursor rather than render. Compact mode has no pane frame or
// action panel, so budgeting it like a framed pane would shrink the page step to
// a single item.
func (m Model) skillListRows() int {
	if m.usesCompactLayout() {
		return max(1, m.mainBodyHeight()-1) // one line for the title
	}
	rows, _ := m.skillListPaneBudget(m.skillListContentWidth(), m.skillListContentHeight())
	return rows
}

// skillListContentWidth is the wrap width of the skill list pane, matching the
// active layout the way detailContentWidth does for the details pane.
func (m Model) skillListContentWidth() int {
	if m.width < compactWidthThreshold {
		return max(1, m.width-4)
	}
	leftW, _, _ := m.wideColumnWidths()
	return leftW
}

func (m Model) detailContentHeight() int {
	bodyHeight := m.mainBodyHeight()
	if m.width < compactWidthThreshold {
		_, detailOuterHeight := compactPaneHeights(bodyHeight)
		return contentHeightForPane(detailOuterHeight, borderStyle)
	}

	border := borderStyle
	if m.focusPane == paneDetails {
		border = focusBorderStyle
	}
	return contentHeightForPane(bodyHeight, border)
}

func (m Model) importContentHeight() int {
	bodyHeight := m.mainBodyHeight()
	width := max(1, m.width-4)
	if width < importSplitWidth {
		return max(1, contentHeightForPane(bodyHeight, borderStyle)/2)
	}
	return contentHeightForPane(bodyHeight, borderStyle)
}

func (m Model) importPreviewContentHeight() int {
	bodyHeight := m.mainBodyHeight()
	width := max(1, m.width-4)
	if width < importSplitWidth {
		return max(1, contentHeightForPane(bodyHeight, borderStyle)-contentHeightForPane(bodyHeight, borderStyle)/2-1)
	}
	return contentHeightForPane(bodyHeight, borderStyle)
}

// wideColumnWidths returns the three column widths of the wide (3-pane) layout.
// Shared by renderWide and detailContentWidth so the rendered pane and the
// scroll-bound measurement agree on the center width.
func (m Model) wideColumnWidths() (leftW, centerW, rightW int) {
	leftW = m.width * 28 / 100
	rightW = m.width * 24 / 100
	centerW = m.width - leftW - rightW - 6 // borders
	if leftW < 20 {
		leftW = 20
	}
	if rightW < 18 {
		rightW = 18
	}
	if centerW < 20 {
		centerW = 20
	}
	return leftW, centerW, rightW
}

// importPaneWidths returns the left/right widths of the side-by-side import
// layout. Shared by renderImport and importPreviewContentWidth.
func (m Model) importPaneWidths() (leftW, rightW int) {
	w := max(1, m.width-4)
	leftW = w * 38 / 100
	if leftW < 28 {
		leftW = 28
	}
	rightW = w - leftW - 2
	if rightW < 30 {
		rightW = 30
		leftW = max(20, w-rightW-2)
	}
	return leftW, rightW
}

// detailContentWidth is the wrap width of the details pane, matching the active
// layout (narrow stacked vs wide center column). Compact falls back to the same
// narrow width; maxDetailScroll guards the not-scrollable cases.
func (m Model) detailContentWidth() int {
	if m.width < compactWidthThreshold {
		return max(1, m.width-4)
	}
	_, centerW, _ := m.wideColumnWidths()
	return centerW
}

// importPreviewContentWidth is the wrap width of the import preview pane,
// matching importPreviewContentHeight's stacked vs side-by-side branch.
func (m Model) importPreviewContentWidth() int {
	w := max(1, m.width-4)
	if w < importSplitWidth {
		return w
	}
	_, rightW := m.importPaneWidths()
	return rightW
}

// maxDetailScroll is the largest valid detailScroll offset for the current
// content and pane size (0 when nothing scrolls).
func (m Model) maxDetailScroll() int {
	content, scrollable := m.detailScrollContent(m.detailContentWidth())
	if !scrollable {
		return 0
	}
	return maxScrollOffset(content, m.detailContentHeight())
}

// maxImportPreviewScroll is the largest valid importPreviewScroll offset.
func (m Model) maxImportPreviewScroll() int {
	content, scrollable := m.importPreviewScrollContent(m.importPreviewContentWidth())
	if !scrollable {
		return 0
	}
	return maxScrollOffset(content, m.importPreviewContentHeight())
}

// maxHelpScroll is the largest valid helpScroll offset for the active help text.
func (m Model) maxHelpScroll() int {
	content := wrapContent(normalStyle.Render(m.activeHelpText()), m.width)
	return maxHelpScrollOffset(content, m.mainBodyHeight())
}

// clampScrollOffsets re-bounds every scroll offset into [0, max]. Called after
// the window size changes, since a previously valid offset can exceed the new
// max (the per-key handlers already clamp on the way down).
func (m *Model) clampScrollOffsets() {
	m.detailScroll = min(max(0, m.detailScroll), m.maxDetailScroll())
	m.importPreviewScroll = min(max(0, m.importPreviewScroll), m.maxImportPreviewScroll())
	m.helpScroll = min(max(0, m.helpScroll), m.maxHelpScroll())
}

// importListContentWidth is the wrap width of the import list pane, matching
// renderImport's stacked vs side-by-side branch.
func (m Model) importListContentWidth() int {
	w := max(1, m.width-4)
	if w < importSplitWidth {
		return w
	}
	leftW, _ := m.importPaneWidths()
	return leftW
}

// importListRows is how many import rows the current layout can draw, for
// callers that page the cursor rather than render.
func (m Model) importListRows() int {
	return m.importListVisibleItems(m.importListContentWidth(), m.importContentHeight())
}

func (m Model) importListVisibleItems(width, height int) int {
	// Measure the footer wrapped, the way renderListPaneWithFooter reserves it:
	// the import actions wrap in a narrow pane, and an unwrapped count would
	// hand the body more lines than it actually gets.
	bodyHeight := max(1, height-countLines(wrapContent(m.renderImportPaneFooter(), width)))
	if m.importBrowsing {
		return max(1, bodyHeight-importBrowseHeaderLines)
	}

	headerLines := importListBaseHeaderLines
	if m.importCustomDir != "" {
		headerLines++
	}

	// Import rows render as a title line plus a source path line.
	return max(1, (bodyHeight-headerLines)/2)
}

func (m Model) settingsVisibleFields(height int) int {
	return max(1, height-settingsPreambleLines-settingsActionLines)
}

func compactPaneHeights(bodyHeight int) (listOuterHeight int, detailOuterHeight int) {
	frameHeight := paneFrameHeight(borderStyle)
	if bodyHeight <= (frameHeight*2)+1+2 {
		return 1, 1
	}

	available := bodyHeight - 1
	if available <= 0 {
		return 1, 1
	}

	listOuterHeight = available / 2
	detailOuterHeight = available - listOuterHeight
	return listOuterHeight, detailOuterHeight
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func pageStep(visibleItems int) int {
	return max(1, visibleItems/2)
}

func clampIndex(index, total int) int {
	if total <= 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= total {
		return total - 1
	}
	return index
}

func windowRange(totalItems, visibleItems, selected int) (int, int) {
	if totalItems <= 0 || visibleItems <= 0 {
		return 0, 0
	}
	if totalItems <= visibleItems {
		return 0, totalItems
	}

	selected = clampIndex(selected, totalItems)
	start := selected - (visibleItems / 2)
	maxStart := totalItems - visibleItems
	if start < 0 {
		start = 0
	}
	if start > maxStart {
		start = maxStart
	}
	return start, start + visibleItems
}
