package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// TestTheMenuAsksWhatShouldHappen checks that a click on an episode opens
// the menu with the choices.
func TestTheMenuAsksWhatShouldHappen(t *testing.T) {
	model := testModel()
	model.screen = ScreenEpisodes
	model.episodes = []orangeflix.Episode{
		{Identifier: "a", Number: 1, Title: "Eins", Date: time.Now()},
		{Identifier: "b", Number: 2, Title: "Zwei", Date: time.Now().Add(-time.Hour)},
	}

	model = press(t, model, "enter")
	if !model.showAction {
		t.Fatal("das Menü öffnet sich nicht")
	}
	view := model.View()
	for _, want := range []string{actionDownload, actionWatch, actionCancel} {
		if !strings.Contains(view, want) {
			t.Errorf("das Menü hat den Eintrag %q nicht:\n%s", want, view)
		}
	}
	// Die Pfeiltaste bewegt die Markierung.
	model = press(t, model, "down")
	if model.actionCursor != 1 {
		t.Errorf("cursor = %d, want 1", model.actionCursor)
	}
	// Escape schliesst das Menü.
	model = press(t, model, "esc")
	if model.showAction {
		t.Error("das Menü bleibt offen")
	}
}

// TestTheWindowHasItsOwnSurface checks that the window of the menu has the
// raised surface of a window and not the black of the page.
func TestTheWindowHasItsOwnSurface(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	applyTheme(ThemeDark)

	model := testModel()
	lines := model.actionPopup(100)
	window := sequenceOf(theme.Window)
	panel := sequenceOf(theme.Panel)
	// Die erste und die letzte Zeile sind die Kanten aus Halbblöcken, sie
	// stehen auf der Seite.
	if !strings.Contains(lines[0], "▄") || !strings.Contains(lines[len(lines)-1], "▀") {
		t.Errorf("the window has no edges: %q … %q", lines[0], lines[len(lines)-1])
	}
	for _, line := range lines[1 : len(lines)-1] {
		if !strings.Contains(line, window) {
			t.Errorf("die Zeile hat nicht die Fläche des Fensters: %q", line)
			break
		}
		if panel != window && strings.Contains(line, panel) {
			t.Errorf("die Zeile hat die Fläche der Seite: %q", line)
			break
		}
	}
}

// TestTheBackgroundStaysVisible checks that the page behind the menu is
// still drawn, only darker.
func TestTheBackgroundStaysVisible(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	applyTheme(ThemeDark)
	model := testModel()
	model.screen = ScreenEpisodes
	model.width = 120
	model.height = 24
	model.episodes = []orangeflix.Episode{
		{Identifier: "a", Number: 420, Title: "Folge 420", Date: time.Now()},
	}
	model = press(t, model, "enter")
	view := model.View()
	if !strings.Contains(stripANSI(view), "Folgen 1") {
		t.Errorf("der Hintergrund fehlt:\n%s", view)
	}
	// Über der Seite liegt der Schleier, das Fenster steht davor.
	if scrim := sequenceOf(theme.Scrim); scrim == "" || !strings.Contains(view, scrim) {
		t.Errorf("der Hintergrund ist nicht abgedunkelt:\n%q", view)
	}
}

// TestTheMenuDownloadsThePickedEpisode checks that the menu loads the
// episode that was picked and not the whole format.
func TestTheMenuDownloadsThePickedEpisode(t *testing.T) {
	model := testModel()
	model.screen = ScreenEpisodes
	model.currentFormat = &orangeflix.Format{PID: 2, Slug: "pantoffeltv", Title: "Pantoffelkino"}
	model.episodes = []orangeflix.Episode{
		{Identifier: "a", Number: 1, Title: "Eins", Date: time.Now()},
		{Identifier: "b", Number: 2, Title: "Zwei", Date: time.Now().Add(-time.Hour)},
	}
	model.episodeCursor = 1
	model = press(t, model, "enter")

	identifiers := model.actionIdentifiers()
	if len(identifiers) != 1 || identifiers[0] != "b" {
		t.Fatalf("identifiers = %v, want the picked episode", identifiers)
	}
}

// TestTheMenuWatchesThePickedEpisode checks that the choice for mpv plays
// the episode that was picked.
func TestTheMenuWatchesThePickedEpisode(t *testing.T) {
	model := testModel()
	model.screen = ScreenEpisodes
	model.episodes = []orangeflix.Episode{
		{Identifier: "a", Number: 1, Title: "Eins", Date: time.Now()},
		{Identifier: "b", Number: 2, Title: "Zwei", Date: time.Now().Add(-time.Hour)},
	}
	model.episodeCursor = 1
	model = press(t, model, "enter")
	model = press(t, model, "down") // "Mit mpv ansehen"
	model = press(t, model, "enter")

	if model.showAction {
		t.Error("das Menü bleibt offen")
	}
	// Der Player wurde gefragt: ohne Player gibt es eine Meldung.
	if model.message == "" {
		t.Error("es gibt keine Meldung nach dem Player")
	}
}
