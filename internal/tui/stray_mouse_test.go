package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func runes(text string, alt bool) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text), Alt: alt}
}

// wheelDown is a turn of the wheel that the program took for the mouse.
func wheelDown() tea.MouseMsg {
	return tea.MouseMsg{X: 20, Y: 20, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown}
}

func TestStrayMouseReportsAreNotTyped(t *testing.T) {
	model := longChat()
	model.View()
	model = press(t, model, "c")
	model.chatInput.SetValue("Hallo")
	model = update(t, model, wheelDown())
	for _, msg := range []tea.KeyMsg{
		// Die Stücke, die beim schnellen Drehen ankommen.
		runes("[", false), runes("[", false), runes("[", true),
		runes("<65;61;27M", false),
		runes("[<65;61;27M[<65;61;27M[<65;61;27M", true),
		runes("[<35;40;12M", true),
		runes("1;27M", false), runes("M", false), runes("<", false),
		runes("[<65;6", true),
		runes("\x1b[<0;10;5m", false),
	} {
		model = update(t, model, msg)
	}
	if got := model.chatInput.Value(); got != "Hallo" {
		t.Errorf("the field took a mouse report: %q", got)
	}
}

func TestPiecesWithoutAMouseEventBefore(t *testing.T) {
	model := longChat()
	model.View()
	model = press(t, model, "c")
	// Ganze Meldungen und Stücke mit alt erkennt das Programm auch ohne ein
	// Mausereignis davor.
	for _, msg := range []tea.KeyMsg{runes("[<65;61;27M", false), runes("[", true), runes("65;61;27M", false)} {
		model = update(t, model, msg)
	}
	if got := model.chatInput.Value(); got != "" {
		t.Errorf("the field took %q", got)
	}
}

func TestTheWheelInAStrayReportStillScrolls(t *testing.T) {
	model := longChat()
	model.View()
	model = update(t, model, runes("[<64;61;27M[<64;61;27M", true))
	if model.chatScroll != 6 {
		t.Errorf("scroll = %d, want two turns of three lines", model.chatScroll)
	}
	model = update(t, model, runes("[<65;61;27M", true))
	if model.chatScroll != 3 {
		t.Errorf("scroll = %d after a turn down", model.chatScroll)
	}
}

func TestTypedTextStaysText(t *testing.T) {
	// Ohne Mausereignis davor ist ein einzelnes Zeichen getippt.
	for _, text := range []string{"Hallo; 3M", "12", "[<3", "M", "a;b", ";", "1;", "<", "["} {
		if _, stray := strayMouse(runes(text, false), -1); stray {
			t.Errorf("%q was taken for the mouse", text)
		}
	}
	// Lange nach dem Scrollen auch.
	if _, stray := strayMouse(runes("[", false), time.Second); stray {
		t.Error("a [ a second after the wheel was taken for the mouse")
	}
	// Text mit Buchstaben ist nie eine Meldung, auch direkt nach der Maus.
	if _, stray := strayMouse(runes("Ja", false), time.Millisecond); stray {
		t.Error("letters right after the wheel were taken for the mouse")
	}
	// alt mit einer Ziffer ist eine Taste, auch direkt nach der Maus.
	if _, stray := strayMouse(runes("2", true), time.Millisecond); stray {
		t.Error("alt+2 was taken for the mouse")
	}
	// Eingefügter Text bleibt Text, auch wenn er wie eine Meldung aussieht.
	pasted := runes("[<65;61;27M", false)
	pasted.Paste = true
	if _, stray := strayMouse(pasted, time.Millisecond); stray {
		t.Error("a paste was taken for the mouse")
	}

	model := interactiveChat()
	model = press(t, model, "c")
	for _, text := range []string{"Hallo", ";", " ", "[", "3", "]"} {
		model = update(t, model, runes(text, false))
	}
	if got := model.chatInput.Value(); got != "Hallo; [3]" {
		t.Errorf("value = %q", got)
	}
}
