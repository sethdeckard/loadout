package tui

import "github.com/charmbracelet/lipgloss"

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)

	footerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 1)

	footerKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Bold(true)

	paneHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)

	paneFooterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	paneFooterKeyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Bold(true)

	paneFooterActionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("114")).
				Bold(true)

	paneFooterDestructiveStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("203")).
					Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	enabledStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("114"))

	disabledStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("248"))

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	statusOKStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("114"))

	statusWarnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	statusInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("81"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238"))

	focusBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62"))

	filterPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("212"))

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 1)

	modalBackdropStyle = lipgloss.NewStyle().
				Faint(true)

	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color("214")).
			Padding(1, 2)

	modalTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15"))

	modalPrimaryStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("114"))
)
