package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// TestTheMessageStandsInTheFooter checks that the message of the program
// belongs into the bar with the keys and not as a naked line under them.
func TestTheMessageStandsInTheFooter(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	applyTheme(ThemeDark)

	model := testModel()
	model.screen = ScreenFormats
	model.message = "3 neue Folge(n) gefunden."

	lines := model.footer(100)
	last := lines[len(lines)-1]
	if !strings.Contains(last, "3 neue Folge(n) gefunden.") {
		t.Errorf("the message is not in the footer: %q", last)
	}
	// Der Text steht rechts in der Leiste, nicht in einer eigenen Zeile.
	for _, line := range lines[:len(lines)-1] {
		if strings.Contains(line, "gefunden") {
			t.Errorf("the message has a line of its own: %q", line)
		}
	}
	// Die Marke steht vor dem Text.
	if !strings.Contains(last, "●") {
		t.Errorf("the message has no marker: %q", last)
	}
}

// TestALongMessageGetsItsOwnLine checks that a long message does not push
// the keys out of the bar.
func TestALongMessageGetsItsOwnLine(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	applyTheme(ThemeDark)

	model := testModel()
	model.screen = ScreenFormats
	model.message = strings.Repeat("Fehler ", 30)

	lines := model.footer(60)
	if len(lines) < 2 {
		t.Fatalf("lines = %d", len(lines))
	}
	if !strings.Contains(lines[0], "Fehler") {
		t.Errorf("the message has no line of its own: %q", lines[0])
	}
	// Die Leiste der Meldung hat ihre eigene Fläche und einen Randbalken.
	if !strings.Contains(lines[0], sequenceOf(theme.Bar)) {
		t.Errorf("the message has no bar: %q", lines[0])
	}
	if !strings.Contains(lines[0], "▌") {
		t.Errorf("the message has no marker: %q", lines[0])
	}
	// Die Tasten stehen weiter unten.
	found := false
	for _, line := range lines[1:] {
		if strings.Contains(line, "wählen") || strings.Contains(line, "beenden") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("the keys are missing: %q", lines)
	}
	// Der Text wird gekürzt, damit er die Breite nicht sprengt.
	if width := ansi.StringWidth(lines[0]); width > 60 {
		t.Errorf("the message is too long (%d): %q", width, lines[0])
	}
}

// TestAnErrorMessageIsMarked checks that a failure stands out.
func TestAnErrorMessageIsMarked(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	applyTheme(ThemeDark)

	model := testModel()
	model.screen = ScreenFormats
	model.message = "Die Anmeldung ist fehlgeschlagen."
	model.isError = true

	lines := model.footer(100)
	last := lines[len(lines)-1]
	if !strings.Contains(last, "✗") {
		t.Errorf("the error has no marker: %q", last)
	}
	if !strings.Contains(last, sequenceOf(theme.Danger)) {
		t.Errorf("the error has no color: %q", last)
	}
}
