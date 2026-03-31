package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) isQuitKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "ctrl+c", "q", "f10":
		return true
	default:
		return msg.Type == tea.KeyF10
	}
}

func isInvertSelectionKey(msg tea.KeyMsg) bool {
	key := strings.ToLower(msg.String())
	switch key {
	case "*", "kp*", "kp_multiply", "keypad*", "keypad_multiply", "numpad*", "numpad_multiply":
		return true
	}

	return msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == '*'
}

func isHomeNavigationKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyHome, tea.KeyCtrlHome, tea.KeyShiftHome, tea.KeyCtrlShiftHome, tea.KeyCtrlA:
		return true
	}

	key := normalizeNavigationKeyName(msg.String())
	if strings.HasSuffix(key, "+home") {
		return true
	}
	if isRawHomeNavigationSequence(key) {
		return true
	}
	switch key {
	case "home", "ctrl+a", "kp_home", "keypad_home", "numpad_home", "find", "pos1", "kp7", "kp_7", "keypad7", "keypad_7", "numpad7", "numpad_7":
		return true
	}

	return false
}

func isEndNavigationKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyEnd, tea.KeyCtrlEnd, tea.KeyShiftEnd, tea.KeyCtrlShiftEnd, tea.KeyCtrlE:
		return true
	}

	key := normalizeNavigationKeyName(msg.String())
	if strings.HasSuffix(key, "+end") {
		return true
	}
	if isRawEndNavigationSequence(key) {
		return true
	}
	switch key {
	case "end", "ctrl+e", "kp_end", "keypad_end", "numpad_end", "select", "kp1", "kp_1", "keypad1", "keypad_1", "numpad1", "numpad_1":
		return true
	}

	return false
}

func normalizeNavigationKeyName(key string) string {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	if strings.HasPrefix(normalized, "esc[") {
		normalized = "[" + strings.TrimPrefix(normalized, "esc[")
	} else if strings.HasPrefix(normalized, "esco") {
		normalized = "o" + strings.TrimPrefix(normalized, "esco")
	}
	normalized = strings.TrimPrefix(normalized, "esc+")
	for strings.HasPrefix(normalized, "\x1b") {
		normalized = strings.TrimPrefix(normalized, "\x1b")
	}
	normalized = strings.TrimPrefix(normalized, "^")
	if strings.HasPrefix(normalized, "[[") {
		normalized = strings.TrimPrefix(normalized, "[")
	}
	return normalized
}

func isRawHomeNavigationSequence(key string) bool {
	switch key {
	case "[h", "oh", "[1~", "[7~":
		return true
	default:
		return false
	}
}

func isRawEndNavigationSequence(key string) bool {
	switch key {
	case "[f", "of", "[4~", "[8~":
		return true
	default:
		return false
	}
}
