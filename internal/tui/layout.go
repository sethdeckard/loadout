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
