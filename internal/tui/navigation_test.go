package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
	"github.com/revunix/orangeflix-dl/internal/store"
)

// update sends one message and returns the model.
func update(t *testing.T, model *Model, msg tea.Msg) *Model {
	t.Helper()
	updated, _ := model.Update(msg)
	next, ok := updated.(*Model)
	if !ok {
		t.Fatalf("the model has the wrong type")
	}
	return next
}

// clickOn draws the screen, looks for a text and clicks on its first cell.
// The titles of the boxes are skipped: the detail box carries the name of
// the marked episode in its frame.
func clickOn(t *testing.T, model *Model, text string, button tea.MouseButton) *Model {
	t.Helper()
	for y, line := range strings.Split(ansi.Strip(model.View()), "\n") {
		index := strings.Index(line, text)
		if index < 0 || strings.Contains(line[:index], "╭") {
			continue
		}
		x := ansi.StringWidth(line[:index])
		return update(t, model, tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: button})
	}
	t.Fatalf("%q is not on the screen:\n%s", text, ansi.Strip(model.View()))
	return model
}

func episodesModel() *Model {
	model := testModel()
	model.screen = ScreenEpisodes
	model.currentFormat = &orangeflix.Format{PID: 23, Slug: "mediatheke", Title: "Die Mediatheke"}
	model.episodes = []orangeflix.Episode{
		{Identifier: "mt419", Number: 419, Title: "Folge 419", Date: time.Now().Add(-time.Hour)},
		{Identifier: "mt420", Number: 420, Title: "Folge 420", Date: time.Now()},
	}
	return model
}

func TestTheFooterKeepsOneLineAndTheMainKeys(t *testing.T) {
	hints := []keyHint{
		hintShow("↑↓", "wählen", prioUseful),
		hint("enter", "öffnen", prioMain),
		hint("v", "nur Bibliothek", prioExtra),
		hint("p", "Ordner", prioPalette),
		hint("?", "Hilfe", prioAlways),
	}
	line, spots := layoutFooter(30, hints)
	if width := ansi.StringWidth(line); width > 30 {
		t.Errorf("the footer is %d wide: %q", width, line)
	}
	for _, want := range []string{"enter öffnen", "? mehr"} {
		if !strings.Contains(line, want) {
			t.Errorf("the footer lost %q: %q", want, line)
		}
	}
	if strings.Contains(line, "Ordner") {
		t.Errorf("a palette command stands in the footer: %q", line)
	}
	// Die Stellen zeigen auf die Tasten.
	for _, spot := range spots {
		if spot.run != "" && !strings.HasPrefix(ansi.Strip(ansi.Cut(line, spot.x0, spot.x1)), spot.run) {
			t.Errorf("spot %+v does not point at its key in %q", spot, line)
		}
	}

	wide, _ := layoutFooter(200, hints)
	if !strings.Contains(wide, "v nur Bibliothek") || !strings.Contains(wide, "? Hilfe") {
		t.Errorf("a wide footer drops hints: %q", wide)
	}
}

func TestEpisodeHintsFollowTheMarking(t *testing.T) {
	model := episodesModel()
	model.selected = map[string]bool{"mt419": true, "mt420": true}
	line := strings.Join(model.footer(160), "\n")
	if !strings.Contains(line, "d 2 laden") || !strings.Contains(line, "w 2 ansehen") {
		t.Errorf("the footer does not count the marked episodes: %q", line)
	}

	// esc nimmt erst die Markierung weg, dann geht es zurück.
	model = press(t, model, "esc")
	if model.screen != ScreenEpisodes || model.markedCount() != 0 {
		t.Fatalf("esc did not clear the marks first: screen %v, marked %d", model.screen, model.markedCount())
	}
	model = press(t, model, "esc")
	if model.screen != ScreenFormats {
		t.Errorf("screen = %v, want the formats", model.screen)
	}
}

func TestShiftAMarksAllAndNone(t *testing.T) {
	model := episodesModel()
	model = press(t, model, "A")
	if model.markedCount() != 2 {
		t.Fatalf("marked = %d", model.markedCount())
	}
	model = press(t, model, "A")
	if model.markedCount() != 0 {
		t.Errorf("A did not clear the marks: %d", model.markedCount())
	}
}

func TestDigitsSwitchTheTabs(t *testing.T) {
	model := testModel()
	model.calendarFunc = func(context.Context, int) ([]orangeflix.V3CalendarEntry, error) { return nil, nil }

	model = press(t, model, "4")
	if model.screen != ScreenCalendar {
		t.Fatalf("screen = %v, want the calendar", model.screen)
	}
	view := model.View()
	if !strings.Contains(view, "[4 Kalender]") {
		t.Errorf("the open tab is not marked:\n%s", view)
	}
	model = press(t, model, "2")
	if model.screen != ScreenClips || model.clipList.kind != clipListRecents {
		t.Errorf("2 does not open the recents: %v %v", model.screen, model.clipList.kind)
	}
	model = press(t, model, "1")
	if model.screen != ScreenFormats {
		t.Errorf("1 does not open the formats: %v", model.screen)
	}

	// Ohne Downloads gibt es den sechsten Reiter nicht.
	if strings.Contains(model.View(), "6 Downloads") {
		t.Error("the downloads tab is shown without downloads")
	}
}

func TestDigitsBelongToAFieldThatIsTyped(t *testing.T) {
	model := testModel()
	model = press(t, model, "/")
	model = press(t, model, "2")
	if model.screen != ScreenFormats || model.filterInput.Value() != "2" {
		t.Errorf("the digit left the filter: screen %v, filter %q", model.screen, model.filterInput.Value())
	}
}

func TestTheSearchTakesEveryLetter(t *testing.T) {
	model := testModel()
	model = press(t, model, "f")
	for _, key := range []string{"q", "j", "k", "?", "1"} {
		model = press(t, model, key)
	}
	if model.quitting || model.showHelp || model.screen != ScreenSearch {
		t.Fatalf("a letter left the search: quitting %v, help %v, screen %v", model.quitting, model.showHelp, model.screen)
	}
	if got := model.searchInput.Value(); got != "qjk?1" {
		t.Errorf("query = %q", got)
	}
}

func TestEnterOpensASearchHit(t *testing.T) {
	model := testModel()
	model = press(t, model, "f")
	model.searchInput.SetValue("Kino")
	model.searchQuery = "Kino"
	model.searchResults = []orangeflix.Episode{{Identifier: "pk1", Title: "Kino 1"}, {Identifier: "pk2", Title: "Kino 2"}}
	model = press(t, model, "down")
	model = press(t, model, "enter")
	if !model.showAction {
		t.Fatal("enter does not open the hit")
	}
	if ids := model.actionIdentifiers(); len(ids) != 1 || ids[0] != "pk2" {
		t.Errorf("identifiers = %v", ids)
	}
}

func TestThePaletteRunsACommand(t *testing.T) {
	model := testModel()
	model.calendarFunc = func(context.Context, int) ([]orangeflix.V3CalendarEntry, error) { return nil, nil }
	model = update(t, model, tea.KeyMsg{Type: tea.KeyCtrlK})
	if !model.showPalette {
		t.Fatal("ctrl+k does not open the palette")
	}
	for _, key := range []string{"k", "a", "l"} {
		model = press(t, model, key)
	}
	items := model.filteredPaletteItems()
	if len(items) == 0 || items[0].label != "Kalender" {
		t.Fatalf("items = %+v", items)
	}
	if view := ansi.Strip(model.View()); !strings.Contains(view, "Befehle") || !strings.Contains(view, "Kalender") {
		t.Errorf("the palette is not drawn:\n%s", view)
	}
	model = press(t, model, "enter")
	if model.showPalette || model.screen != ScreenCalendar {
		t.Errorf("the command did not run: palette %v, screen %v", model.showPalette, model.screen)
	}
}

func TestThePaletteOffersTheKeysOfTheScreen(t *testing.T) {
	model := episodesModel()
	model = press(t, model, ":")
	labels := []string{}
	for _, item := range model.filteredPaletteItems() {
		labels = append(labels, item.label)
	}
	joined := strings.Join(labels, " | ")
	for _, want := range []string{"Folge herunterladen", "Nach Jahr filtern", "Merkliste", "Programm beenden"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the palette misses %q: %s", want, joined)
		}
	}
}

func TestDeletingAFileAsksFirst(t *testing.T) {
	model := episodesModel()
	now := time.Now()
	model.known = map[string]store.KnownEpisode{
		"mt420": {EpisodeRef: store.EpisodeRef{Identifier: "mt420"}, DownloadedAt: &now, Path: "/tmp/nicht-da.mp4"},
	}
	model = press(t, model, "x")
	if model.confirm == nil {
		t.Fatal("x does not ask")
	}
	if view := ansi.Strip(model.View()); !strings.Contains(view, "Datei löschen?") || !strings.Contains(view, "/tmp/nicht-da.mp4") {
		t.Errorf("the question is not shown:\n%s", view)
	}
	// Die Auswahl steht auf „Abbrechen“: ein schnelles enter löscht nichts.
	model = press(t, model, "enter")
	if model.confirm != nil || !model.episodeLoaded("mt420") {
		t.Errorf("enter deleted the file or left the question open")
	}
}

func TestTheQueueGoesBackWithoutQuitting(t *testing.T) {
	model := testModel()
	model.screen = ScreenQueue
	model.jobs = []job{{Label: "Folge", Episode: "mt1", State: "lädt"}}
	model = press(t, model, "esc")
	if model.quitting || model.screen != ScreenFormats {
		t.Fatalf("esc in the queue: quitting %v, screen %v", model.quitting, model.screen)
	}
	// Endet der Download woanders, bleibt der Bildschirm stehen.
	model = update(t, model, RunDoneMsg{})
	if model.screen != ScreenFormats || !strings.Contains(model.message, "Downloads fertig") {
		t.Errorf("screen %v, message %q", model.screen, model.message)
	}
	model = press(t, model, "6")
	if model.screen != ScreenQueue {
		t.Errorf("6 does not open the downloads: %v", model.screen)
	}
}

func TestQOnTheSummaryQuits(t *testing.T) {
	model := testModel()
	model.screen = ScreenSummary
	updated, cmd := model.Update(keyMsgFor("q"))
	model = updated.(*Model)
	if !model.quitting || cmd == nil {
		t.Error("q does not quit on the summary")
	}
}

func TestTheMousePicksAndOpensAnEpisode(t *testing.T) {
	model := episodesModel()
	if model.episodeCursor != 0 {
		t.Fatalf("cursor = %d", model.episodeCursor)
	}
	// Die neueste Folge steht oben; Folge 419 ist die zweite Zeile.
	model = clickOn(t, model, "Folge 419", tea.MouseButtonLeft)
	if model.episodeCursor != 1 {
		t.Fatalf("the click did not pick the episode: cursor %d", model.episodeCursor)
	}
	if model.showAction {
		t.Fatal("the first click opened the menu")
	}
	model = clickOn(t, model, "Folge 419", tea.MouseButtonLeft)
	if !model.showAction {
		t.Fatal("the second click did not open the menu")
	}
	// Ein Klick neben das Fenster schließt es. Das Programm zeichnet vor
	// jedem Klick neu.
	model.View()
	model = update(t, model, tea.MouseMsg{X: 0, Y: 5, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if model.showAction {
		t.Error("a click beside the menu does not close it")
	}
	// Die rechte Taste öffnet das Kontextmenü; daraus wird markiert.
	model = clickOn(t, model, "Folge 420", tea.MouseButtonRight)
	if model.context == nil {
		t.Fatal("the right click did not open the menu")
	}
	model = clickOn(t, model, "Markieren", tea.MouseButtonLeft)
	if !model.selected["mt420"] || model.context != nil {
		t.Errorf("the menu did not mark: %v, open %v", model.selected, model.context != nil)
	}
}

func TestTheContextMenuOpensAtThePointer(t *testing.T) {
	model := episodesModel()
	model = clickOn(t, model, "Folge 419", tea.MouseButtonRight)
	if model.context == nil || model.episodeCursor != 1 {
		t.Fatalf("menu %v, cursor %d", model.context != nil, model.episodeCursor)
	}
	clickY := model.mouseY
	lines := strings.Split(ansi.Strip(model.View()), "\n")
	found := -1
	for y, line := range lines {
		if strings.Contains(line, "Herunterladen") {
			found = y
			break
		}
	}
	// Das Menü beginnt unter dem Zeiger: Streifen, Titel, Leerzeile, dann
	// der erste Eintrag.
	if found != clickY+1+windowContentRow {
		t.Errorf("the first entry is on line %d, the click was on %d:\n%s", found, clickY, strings.Join(lines, "\n"))
	}
	for _, want := range []string{"Ansehen mit mpv", "Auf die Merkliste setzen", "Ähnliche Folgen", "Titel kopieren"} {
		if !strings.Contains(strings.Join(lines, "\n"), want) {
			t.Errorf("the menu misses %q", want)
		}
	}
	// Die Taste eines Eintrags führt ihn aus.
	model = press(t, model, "b")
	if model.context != nil {
		t.Error("b did not run the entry")
	}
}

func TestTheContextMenuOfAFormatAndTheKey(t *testing.T) {
	model := testModel()
	model = clickOn(t, model, "MAXimal Musik", tea.MouseButtonRight)
	if model.context == nil {
		t.Fatal("the right click on a format did not open the menu")
	}
	labels := []string{}
	for _, item := range model.context.items {
		labels = append(labels, item.label)
	}
	if labels[0] != "Folgen öffnen" || !strings.Contains(strings.Join(labels, "|"), "Zur Bibliothek hinzufügen") {
		t.Errorf("items = %v", labels)
	}
	if model.context.title != "MAXimal Musik" {
		t.Errorf("title = %q", model.context.title)
	}
	model = press(t, model, "esc")
	if model.context != nil {
		t.Fatal("esc did not close the menu")
	}

	// „.“ öffnet das Menü der markierten Zeile ohne Maus, in der Mitte.
	model = press(t, model, ".")
	if model.context == nil || model.context.anchored {
		t.Fatalf("the key did not open a centered menu")
	}
}

func TestTheContextMenuOffersTheMarkedEpisodes(t *testing.T) {
	model := episodesModel()
	model.selected = map[string]bool{"mt419": true, "mt420": true}
	model = press(t, model, ".")
	joined := ""
	for _, item := range model.context.items {
		joined += item.label + "|"
	}
	if !strings.Contains(joined, "2 markierte herunterladen") || !strings.Contains(joined, "Markierung entfernen") {
		t.Errorf("items = %s", joined)
	}
}

func TestTheContextMenuOfASearchHitNamesNoKeys(t *testing.T) {
	model := testModel()
	model = press(t, model, "f")
	model.searchQuery = "Kino"
	model.searchInput.SetValue("Kino")
	model.searchResults = []orangeflix.Episode{{Identifier: "pk1", Title: "Kino 1", FormatName: "Pantoffelkino"}}
	model = clickOn(t, model, "Kino 1", tea.MouseButtonRight)
	if model.context == nil {
		t.Fatal("no menu on the hit")
	}
	for _, item := range model.context.items {
		if item.key != "" {
			t.Errorf("the search menu names the key %q, which would be typed", item.key)
		}
	}
	model = clickOn(t, model, "Auf die Merkliste setzen", tea.MouseButtonLeft)
	if model.context != nil || !model.searchResults[0].Bookmarked {
		// Ohne Merkliste meldet das Programm das; die Liste bleibt dann.
		if model.message == "" {
			t.Errorf("the bookmark entry did nothing")
		}
	}
}

func TestTheMouseClicksTheMenuAndTheWheelMoves(t *testing.T) {
	model := episodesModel()
	model = press(t, model, "enter")
	model = clickOn(t, model, actionWatch, tea.MouseButtonLeft)
	if model.showAction {
		t.Error("the click on the entry did not run it")
	}

	model = update(t, model, tea.MouseMsg{X: 10, Y: 10, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})
	if model.episodeCursor != 1 {
		t.Errorf("the wheel did not move: cursor %d", model.episodeCursor)
	}
	model = update(t, model, tea.MouseMsg{X: 10, Y: 10, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp})
	if model.episodeCursor != 0 {
		t.Errorf("the wheel did not move back: cursor %d", model.episodeCursor)
	}
}

func TestTheMouseUsesTheTabsAndTheFooter(t *testing.T) {
	model := testModel()
	model.calendarFunc = func(context.Context, int) ([]orangeflix.V3CalendarEntry, error) { return nil, nil }
	model = clickOn(t, model, "4 Kalender", tea.MouseButtonLeft)
	if model.screen != ScreenCalendar {
		t.Fatalf("the click on the tab did not switch: %v", model.screen)
	}
	model = clickOn(t, model, "esc zurück", tea.MouseButtonLeft)
	if model.screen != ScreenFormats {
		t.Errorf("the click on the footer did not run esc: %v", model.screen)
	}
	if !strings.Contains(ansi.Strip(model.View()), "ctrl+k Befehle") {
		t.Error("the footer hides the palette")
	}
	model = clickOn(t, model, "? ", tea.MouseButtonLeft)
	if !model.showHelp {
		t.Error("the click on ? did not open the help")
	}
}

func TestMouseCanBeSwitchedOff(t *testing.T) {
	for value, want := range map[string]bool{"": true, "1": true, "0": false, "off": false, "Aus": false} {
		if got := mouseEnabled(value); got != want {
			t.Errorf("mouseEnabled(%q) = %v, want %v", value, got, want)
		}
	}
}
