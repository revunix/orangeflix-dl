package tui

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// mouseReport is one mouse report of the terminal in the SGR form, like
// "[<65;61;27M" for the wheel down at column 61 and row 27.
var mouseReport = regexp.MustCompile(`\x1b?\[<(\d+);(\d+);(\d+)([Mm])`)

// strayWindow is how long after a mouse event a key that is made only of
// the characters of a report counts as a piece of one.
const strayWindow = 150 * time.Millisecond

// strayMouse recognizes mouse reports that reached the program as typed
// text. When the wheel turns fast, the terminal sends many reports at once;
// one that is cut between two reads is not taken for the mouse: its escape
// becomes the alt key, and its pieces arrive as letters, a "[" alone or
// "<65;61;27M". Such a key holds nothing but the characters of reports.
// It is a piece when it came with alt, when it holds a whole report, or
// when a mouse event came just before it. The wheel turns that are in it
// come back as up and down, so the scrolling goes on.
func strayMouse(msg tea.KeyMsg, sinceMouse time.Duration) (wheel []string, stray bool) {
	// Eingefügter Text ist immer Text.
	if msg.Type != tea.KeyRunes || len(msg.Runes) == 0 || msg.Paste {
		return nil, false
	}
	text := string(msg.Runes)
	if !onlyReportCharacters(text) {
		return nil, false
	}
	switch {
	case mouseReport.MatchString(text), looksLikeReportPiece(text):
	// Auf das abgetrennte Escape folgt immer „[“: alt mit einer Ziffer ist
	// eine Taste, etwa für die Emoji.
	case msg.Alt:
		if !strings.HasPrefix(text, "[") {
			return nil, false
		}
	case sinceMouse >= 0 && sinceMouse < strayWindow:
	default:
		return nil, false
	}
	for _, match := range mouseReport.FindAllStringSubmatch(text, -1) {
		button, err := strconv.Atoi(match[1])
		if err != nil || button&64 == 0 || match[4] != "M" {
			continue
		}
		if button&1 == 0 {
			wheel = append(wheel, "up")
		} else {
			wheel = append(wheel, "down")
		}
	}
	return wheel, true
}

// onlyReportCharacters tells a text made only of what a report holds:
// digits, semicolons, the bracket, the less-than sign, the final M and the
// escape.
func onlyReportCharacters(text string) bool {
	for _, r := range text {
		switch {
		case r >= '0' && r <= '9', r == ';', r == '[', r == '<', r == 'M', r == 'm', r == '\x1b':
		default:
			return false
		}
	}
	return true
}

// looksLikeReportPiece tells a piece of a cut report without a mouse event
// before it: at least three characters with a semicolon and a digit, so a
// typed ";" or a number stays text; typing sends one key at a time.
func looksLikeReportPiece(text string) bool {
	return len(text) >= 3 && strings.Contains(text, ";") && strings.ContainsAny(text, "0123456789")
}

// sinceMouse is the time since the last mouse event, -1 before the first.
func (m *Model) sinceMouse() time.Duration {
	if m.lastMouse.IsZero() {
		return -1
	}
	return time.Since(m.lastMouse)
}
