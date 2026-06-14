package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	compactWidthThreshold = 60
	importSplitWidth      = 90

	skillListBaseHeaderLines  = 3
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

func (m Model) skillListVisibleItems(height int) int {
	headerLines := skillListBaseHeaderLines
	if m.filtering || m.filter != "" {
		headerLines++
	}
	return max(1, height-headerLines-countLines(m.renderPaneFooterActions(m.inProjectMode())))
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

func (m Model) importListVisibleItems(height int) int {
	bodyHeight := max(1, height-countLines(m.renderImportPaneFooter()))
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
