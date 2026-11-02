package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// pointAt draws the screen and moves the pointer onto a text, without a
// button.
func pointAt(t *testing.T, model *Model, text string) (*Model, int) {
	t.Helper()
	for y, line := range strings.Split(ansi.Strip(model.View()), "\n") {
		index := strings.Index(line, text)
		if index < 0 || strings.Contains(line[:index], "╭") {
			continue
		}
		x := ansi.StringWidth(line[:index])
		return update(t, model, tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionMotion, Button: tea.MouseButtonNone}), y
	}
	t.Fatalf("%q is not on the screen:\n%s", text, ansi.Strip(model.View()))
	return model, -1
}

func colors(t *testing.T) {
	t.Helper()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	applyTheme(ThemeDark)
}

func TestThePointerMarksTheEntryOfTheMenu(t *testing.T) {
	model := episodesModel()
	model = clickOn(t, model, "Folge 419", tea.MouseButtonRight)
	if model.context == nil || model.context.cursor != 0 {
		t.Fatalf("menu not open on the first entry")
	}
	model, _ = pointAt(t, model, "Ähnliche Folgen")
	want := -1
	for index, item := range model.context.items {
		if item.label == "Ähnliche Folgen" {
			want = index
		}
	}
	if model.context.cursor != want {
		t.Errorf("cursor = %d, want %d", model.context.cursor, want)
	}
	// Das Markieren durch den Zeiger zeigt der Balken der Auswahl.
	for _, line := range strings.Split(ansi.Strip(model.View()), "\n") {
		if strings.Contains(line, "Ähnliche Folgen") && !strings.Contains(line, "▌ Ähnliche Folgen") {
			t.Errorf("the entry under the pointer is not marked: %q", line)
		}
	}
	// enter führt den Eintrag unter dem Zeiger aus.
	model = press(t, model, "enter")
	if model.context != nil || model.screen != ScreenClips {
		t.Errorf("enter did not run the hovered entry: screen %v", model.screen)
	}
}

func TestThePointerMarksTheEntriesOfEveryWindow(t *testing.T) {
	model := episodesModel()
	model = press(t, model, "enter")
	model, _ = pointAt(t, model, actionCancel)
	if model.actionCursor != 2 {
		t.Errorf("action cursor = %d, want 2", model.actionCursor)
	}

	model = press(t, model, "esc")
	model = press(t, model, ":")
	model, _ = pointAt(t, model, "Nach Jahr filtern")
	if items := model.filteredPaletteItems(); items[model.paletteCursor].label != "Nach Jahr filtern" {
		t.Errorf("palette cursor on %q", items[model.paletteCursor].label)
	}
}

func TestThePageBehindAWindowIgnoresThePointer(t *testing.T) {
	model := episodesModel()
	model = press(t, model, "enter")
	model.View()
	before := model.actionCursor
	// Ganz links liegt die Seite, nicht das Fenster.
	model = update(t, model, tea.MouseMsg{X: 1, Y: 6, Action: tea.MouseActionMotion})
	if model.actionCursor != before || model.episodeCursor != 0 {
		t.Errorf("the page reacted: action %d, episode %d", model.actionCursor, model.episodeCursor)
	}
}

func TestButtonsLightUnderThePointer(t *testing.T) {
	colors(t)
	surface := sequenceOf(theme.Selected)
	model := testModel()

	model, y := pointAt(t, model, "3 Merkliste")
	line := strings.Split(model.View(), "\n")[y]
	if !strings.Contains(line, surface) || !strings.Contains(ansi.Strip(line), "3 Merkliste") {
		t.Errorf("the tab is not lit: %q", line)
	}
	if width := ansi.StringWidth(line); width != model.width {
		t.Errorf("the lit line is %d wide, want %d", width, model.width)
	}

	model, y = pointAt(t, model, "? ")
	line = strings.Split(model.View(), "\n")[y]
	lit := surface + sequenceOf(theme.AccentBold)
	if !strings.Contains(line, lit) {
		t.Errorf("the footer key is not lit: %q", line)
	}
	if width := ansi.StringWidth(line); width != model.width {
		t.Errorf("the lit footer is %d wide, want %d", width, model.width)
	}

	// Ohne Zeiger leuchtet nichts in der Fußzeile.
	model = update(t, model, tea.MouseMsg{X: 50, Y: 10, Action: tea.MouseActionMotion})
	if strings.Contains(strings.Split(model.View(), "\n")[y], surface) {
		t.Error("the footer stays lit after the pointer left")
	}
}

func TestTheEscOfAWindowIsAButton(t *testing.T) {
	colors(t)
	model := episodesModel()
	model = press(t, model, "enter")
	lines := strings.Split(ansi.Strip(model.View()), "\n")
	for y, line := range lines {
		if !strings.Contains(line, "Folge 420") || !strings.Contains(line, "esc") {
			continue
		}
		x := ansi.StringWidth(line[:strings.LastIndex(line, "esc")])
		model = update(t, model, tea.MouseMsg{X: x + 1, Y: y, Action: tea.MouseActionMotion})
		if !strings.Contains(strings.Split(model.View(), "\n")[y], sequenceOf(theme.Selected)) {
			t.Error("esc is not lit")
		}
		model = update(t, model, tea.MouseMsg{X: x + 1, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
		if model.showAction {
			t.Error("a click on esc does not close the window")
		}
		return
	}
	t.Fatal("the title of the window is missing")
}

func TestListRowsShowThePointer(t *testing.T) {
	model := testModel()
	model, y := pointAt(t, model, "MAXimal Musik")
	line := strings.Split(ansi.Strip(model.View()), "\n")[y]
	if !strings.Contains(line, "▎") {
		t.Errorf("the row under the pointer has no bar: %q", line)
	}
	// Der Zeiger wählt in Listen nichts aus.
	formats := model.visibleFormats()
	if formats[model.formatCursor].Title == "MAXimal Musik" {
		t.Error("the pointer moved the cursor of the list")
	}
}
