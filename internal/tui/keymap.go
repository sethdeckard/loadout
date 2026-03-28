package tui

import tea "github.com/charmbracelet/bubbletea"

type keyAction int

const (
	keyNone keyAction = iota
	keyUp
	keyDown
	keyTop
	keyBottom
	keyFilter
	keyFilterClear
	keyClaude
	keyCodex
	keyAll
	keySync
	keyDoctor
	keyHelp
	keyScope
	keyImport
	keyDelete
	keySettings
	keySave
	keyLeft
	keyRight
	keyQuit
)

func classifyKey(msg tea.KeyMsg) keyAction {
	switch msg.String() {
	case "k", "up":
		return keyUp
	case "j", "down":
		return keyDown
	case "g", "home":
		return keyTop
	case "G", "end":
		return keyBottom
	case "/":
		return keyFilter
	case "esc":
		return keyFilterClear
	case "c":
		return keyClaude
	case "x":
		return keyCodex
	case "a":
		return keyAll
	case "s":
		return keySync
	case "d":
		return keyDoctor
	case "?":
		return keyHelp
	case "tab":
		return keyScope
	case "i":
		return keyImport
	case "D":
		return keyDelete
	case "p":
		return keySettings
	case "ctrl+s":
		return keySave
	case "h", "left":
		return keyLeft
	case "l", "right":
		return keyRight
	case "q", "ctrl+c":
		return keyQuit
	}
	return keyNone
}
