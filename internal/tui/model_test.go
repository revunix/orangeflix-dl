package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/revunix/orangeflix-dl/internal/app"
	"github.com/revunix/orangeflix-dl/internal/config"
	"github.com/revunix/orangeflix-dl/internal/orangeflix"
	"github.com/revunix/orangeflix-dl/internal/store"
)

// testModel builds a model for the tests.
func testModel() *Model {
	model := NewModel(context.Background(), Options{
		Config:  &config.Options{Quality: "best", Submodule: 1},
		Version: "test",
	})
	model.width = 100
	model.height = 40
	model.screen = ScreenFormats
	return model
}

// press sends a key to the model.
func press(t *testing.T, model *Model, key string) *Model {
	t.Helper()
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	if key == "enter" {
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	}
	if key == "up" {
		msg = tea.KeyMsg{Type: tea.KeyUp}
	}
	if key == "down" {
		msg = tea.KeyMsg{Type: tea.KeyDown}
	}
	if key == "esc" {
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	}
	if key == "ctrl+t" {
		msg = tea.KeyMsg{Type: tea.KeyCtrlT}
	}
	if key == "ctrl+v" {
		msg = tea.KeyMsg{Type: tea.KeyCtrlV}
	}
	if key == "tab" {
		msg = tea.KeyMsg{Type: tea.KeyTab}
	}
	if key == "shift+tab" {
		msg = tea.KeyMsg{Type: tea.KeyShiftTab}
	}
	if key == " " {
		msg = tea.KeyMsg{Type: tea.KeySpace}
	}
	if key == "J" {
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}}
	}
	updated, _ := model.Update(msg)
	next, ok := updated.(*Model)
	if !ok {
		t.Fatalf("the model has the wrong type")
	}
	return next
}

func TestFormatListMovesTheCursor(t *testing.T) {
	model := testModel()
	model = press(t, model, "down")
	if model.formatCursor != 1 {
		t.Errorf("cursor = %d, want 1", model.formatCursor)
	}
	model = press(t, model, "up")
	if model.formatCursor != 0 {
		t.Errorf("cursor = %d, want 0", model.formatCursor)
	}
	// The cursor stays inside the list.
	for i := 0; i < 100; i++ {
		model = press(t, model, "down")
	}
	if model.formatCursor >= len(model.formats) {
		t.Errorf("cursor = %d, the list has %d formats", model.formatCursor, len(model.formats))
	}
}

func TestFormatListFilters(t *testing.T) {
	model := testModel()
	model = press(t, model, "/")
	if !model.filtering {
		t.Fatal("the filter is not active")
	}
	model.filterInput.SetValue("mediatheke")
	model = press(t, model, "enter")
	if model.filtering {
		t.Error("the filter is still active")
	}
	formats := model.filteredFormats()
	if len(formats) != 1 || formats[0].Slug != "mediatheke" {
		t.Errorf("formats = %+v", formats)
	}

	// The escape clears the filter.
	model = press(t, model, "esc")
	if model.filter != "" {
		t.Errorf("filter = %q", model.filter)
	}
}

func TestFormatListOpensAFormat(t *testing.T) {
	model := testModel()
	model.filter = "mediatheke"
	model = press(t, model, "enter")
	if !model.loading {
		t.Error("the model does not load the episodes")
	}
	if model.currentFormat != nil {
		t.Error("the format was opened without the episodes")
	}
}

func TestEpisodeListMarksTheEpisodes(t *testing.T) {
	model := testModel()
	model.screen = ScreenEpisodes
	// The list is held in the order of the API, the newest episode comes
	// first in the view.
	model.episodes = []orangeflix.Episode{
		{Identifier: "mt419", Number: 419, Title: "Folge 419"},
		{Identifier: "mt420", Number: 420, Title: "Folge 420"},
	}
	model = press(t, model, " ")
	if !model.selected["mt420"] {
		t.Errorf("selected = %v", model.selected)
	}
	model = press(t, model, "a")
	if !model.selected["mt419"] {
		t.Errorf("selected = %v", model.selected)
	}
	identifiers := model.selectedIdentifiers()
	if len(identifiers) != 2 {
		t.Errorf("identifiers = %v", identifiers)
	}
}

func TestEpisodeListSelectsEverythingWithoutAMarking(t *testing.T) {
	model := testModel()
	model.screen = ScreenEpisodes
	model.episodes = []orangeflix.Episode{
		{Identifier: "mt420", Number: 420},
		{Identifier: "mt419", Number: 419},
	}
	identifiers := model.selectedIdentifiers()
	if len(identifiers) != 2 {
		t.Errorf("identifiers = %v", identifiers)
	}
}

func TestHelpOverlayOpensAndCloses(t *testing.T) {
	model := testModel()
	model = press(t, model, "?")
	if !model.showHelp {
		t.Fatal("the help is not shown")
	}
	if !strings.Contains(model.View(), "Hilfe") {
		t.Error("the view does not hold the help")
	}
	// Escape schließt das Hilfefenster.
	model = press(t, model, "esc")
	if model.showHelp {
		t.Error("the help is still shown")
	}
}

func TestStartScreenNeedsTheCredentials(t *testing.T) {
	model := testModel()
	model.screen = ScreenStart
	model.userInput.SetValue("")
	model.passInput.SetValue("")
	model = press(t, model, "enter")
	if model.screen != ScreenStart {
		t.Error("the model left the start screen without credentials")
	}
	if !model.isError {
		t.Error("the missing credentials were not reported")
	}
}

func TestStartScreenStoresTheCredentials(t *testing.T) {
	model := testModel()
	model.screen = ScreenStart
	model.userInput.SetValue("user@example.com")
	model.passInput.SetValue("secret")
	model = press(t, model, "enter")
	if model.screen != ScreenFormats {
		t.Errorf("screen = %v", model.screen)
	}
	if model.config.Username != "user@example.com" {
		t.Errorf("username = %q", model.config.Username)
	}
}

func TestViewRendersTheFormatList(t *testing.T) {
	model := testModel()
	view := model.View()
	if !strings.Contains(view, "Formate") {
		t.Errorf("view = %q", view)
	}
	if !strings.Contains(view, "orangeflix") {
		t.Errorf("view = %q", view)
	}
}

func TestFormatListGroupsTheFormatsByStatus(t *testing.T) {
	model := testModel()
	// Die Liste scrollt, darum werden die Gruppen in den Zeilen geprüft.
	var groups []string
	for _, row := range model.formatRows() {
		if row.format == nil {
			groups = append(groups, row.header)
		}
	}
	for _, label := range []string{"AKTIV", "SAISONAL", "ARCHIVIERT"} {
		found := false
		for _, group := range groups {
			if group == label {
				found = true
			}
		}
		if !found {
			t.Errorf("the list does not hold the group %q: %v", label, groups)
		}
	}
}

func TestFormatListHoldsTheMarkedFormatInTheSecondColumn(t *testing.T) {
	model := testModel()
	model.width = 120
	view := model.View()
	if !strings.Contains(view, "Slug") {
		t.Errorf("the view does not hold the details:\n%s", view)
	}
}

func TestFormatListKeepsTheSecondColumnOnANarrowTerminal(t *testing.T) {
	model := testModel()
	model.width = 60
	view := model.View()
	if strings.Contains(view, "Slug") {
		t.Errorf("the view shows the details on a narrow terminal:\n%s", view)
	}
}

func TestEpisodeListFiltersByYear(t *testing.T) {
	model := testModel()
	model.screen = ScreenEpisodes
	model.episodes = []orangeflix.Episode{
		{Identifier: "mt420", Number: 420, Title: "Folge 420", Date: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)},
		{Identifier: "mt300", Number: 300, Title: "Folge 300", Date: time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC)},
	}
	model = press(t, model, "J")
	if !model.yearFiltering {
		t.Fatal("the year filter is not active")
	}
	model.yearInput.SetValue("2025")
	model = press(t, model, "enter")
	if model.episodeYear != "2025" {
		t.Errorf("year = %q", model.episodeYear)
	}
	episodes := model.filteredEpisodes()
	if len(episodes) != 1 || episodes[0].Identifier != "mt300" {
		t.Errorf("episodes = %+v", episodes)
	}

	// The escape removes the filter.
	model = press(t, model, "esc")
	if model.episodeYear != "" {
		t.Errorf("year = %q", model.episodeYear)
	}
}

func TestThemeSwitches(t *testing.T) {
	model := testModel()
	model.screen = ScreenStart
	model.theme = ThemeAuto
	model = press(t, model, "ctrl+t")
	if model.theme != ThemeLight {
		t.Errorf("theme = %q", model.theme)
	}
	model = press(t, model, "ctrl+t")
	if model.theme != ThemeDark {
		t.Errorf("theme = %q", model.theme)
	}
	model = press(t, model, "ctrl+t")
	if model.theme != ThemeAuto {
		t.Errorf("theme = %q", model.theme)
	}
}

// TestTheLoginFieldsTakeEveryLetter checks that letters reach the fields and
// do not switch the appearance.
func TestTheLoginFieldsTakeEveryLetter(t *testing.T) {
	model := testModel()
	model.screen = ScreenStart
	model.userInput.SetValue("")
	model.field = 0
	model.focusField()
	theme := model.theme
	for _, letter := range "thd@x.de" {
		model = press(t, model, string(letter))
	}
	if got := model.userInput.Value(); got != "thd@x.de" {
		t.Fatalf("e-mail = %q", got)
	}
	if model.theme != theme {
		t.Fatalf("theme switched to %q", model.theme)
	}
}

func TestQueueShowsTheProgress(t *testing.T) {
	model := testModel()
	model.screen = ScreenQueue
	model.jobs = []job{{
		Label:     "Die.Mediatheke.S2026E420",
		Path:      "/tmp/Die.Mediatheke.S2026E420.mp4",
		Done:      50,
		Total:     100,
		State:     "lädt",
		Episode:   "mt420",
		PID:       23,
		startedAt: time.Now().Add(-10 * time.Second),
	}}
	view := model.View()
	if !strings.Contains(view, "50%") {
		t.Errorf("the view does not hold the progress:\n%s", view)
	}
	if !strings.Contains(view, meterSymbol) {
		t.Errorf("the view does not hold the meter:\n%s", view)
	}
	if !strings.Contains(view, "noch") {
		t.Errorf("the view does not hold the remaining time:\n%s", view)
	}
}

func TestEventUpdatesTheQueue(t *testing.T) {
	model := testModel()
	model.screen = ScreenQueue

	updated, _ := model.Update(EventMsg{Event: app.Event{Kind: app.EventQueued, Label: "Folge", EpisodeID: "mt1", PID: 23}})
	model = updated.(*Model)
	if len(model.jobs) != 1 {
		t.Fatalf("jobs = %+v", model.jobs)
	}

	updated, _ = model.Update(EventMsg{Event: app.Event{Kind: app.EventProgress, EpisodeID: "mt1", Done: 10, Total: 100}})
	model = updated.(*Model)
	if model.jobs[0].Done != 10 || model.jobs[0].Total != 100 {
		t.Errorf("job = %+v", model.jobs[0])
	}

	updated, _ = model.Update(EventMsg{Event: app.Event{Kind: app.EventDone, EpisodeID: "mt1"}})
	model = updated.(*Model)
	if model.summary.done != 1 {
		t.Errorf("done = %d", model.summary.done)
	}
	if model.jobs[0].State != "fertig" {
		t.Errorf("state = %q", model.jobs[0].State)
	}
}

func TestSelectedIdentifiers(t *testing.T) {
	model := testModel()
	model.screen = ScreenEpisodes
	model.episodes = []orangeflix.Episode{
		{Identifier: "mt420", Number: 420},
		{Identifier: "mt419", Number: 419},
	}
	model.selected["mt419"] = true
	identifiers := model.selectedIdentifiers()
	if len(identifiers) != 1 || identifiers[0] != "mt419" {
		t.Errorf("identifiers = %v", identifiers)
	}
}

func TestHumanDuration(t *testing.T) {
	cases := map[int64]string{
		1:    "weniger als einer Minute",
		60:   "1 Minute",
		300:  "5 Minuten",
		3600: "1 Stunde",
	}
	for seconds, want := range cases {
		if got := humanDuration(seconds); got != want {
			t.Errorf("humanDuration(%d) = %q, want %q", seconds, got, want)
		}
	}
}

func TestEveryScreenRenders(t *testing.T) {
	model := testModel()

	model.screen = ScreenSplash
	if !strings.Contains(model.View(), "starte") {
		t.Error("the splash does not render")
	}

	model.screen = ScreenStart
	model.userInput.SetValue("user@example.com")
	model.passInput.SetValue("secret")
	view := model.View()
	if !strings.Contains(view, "Zugangsdaten") {
		t.Error("the start screen does not render")
	}
	if !strings.Contains(view, "Erscheinungsbild") {
		t.Error("the start screen does not show the theme")
	}

	model.screen = ScreenFormats
	if !strings.Contains(model.View(), "Formate") {
		t.Error("the format list does not render")
	}

	model.screen = ScreenEpisodes
	model.episodes = []orangeflix.Episode{
		{Identifier: "mt420", Number: 420, Title: "Folge 420", Date: time.Now()},
	}
	view = model.View()
	if !strings.Contains(view, "Folgen") {
		t.Error("the episode list does not render")
	}
	if !strings.Contains(view, "420") || !strings.Contains(view, "Folge 420") {
		t.Errorf("the episode list does not hold the episode:\n%s", view)
	}

	model.screen = ScreenQueue
	model.jobs = []job{{Label: "Folge", Path: "/tmp/x.mp4", State: "fertig", Episode: "mt420", PID: 23}}
	if !strings.Contains(model.View(), "Downloads") {
		t.Error("the queue does not render")
	}

	model.screen = ScreenSummary
	model.summary.done = 3
	model.summary.skipped = 1
	model.summary.failed = 0
	view = model.View()
	if !strings.Contains(view, "Fertig") {
		t.Error("the summary does not render")
	}
	if !strings.Contains(view, "geladen") {
		t.Errorf("the summary does not hold the counters:\n%s", view)
	}
}

func TestHelpWindowListsTheKeys(t *testing.T) {
	model := testModel()
	model.showHelp = true
	// Die Hilfe steht auf einer Seite.
	keys := ""
	for _, line := range model.helpPopup(120) {
		keys += line
	}
	for _, hint := range []string{"Formate", "Folgen", "Downloads", "Allgemein"} {
		if !strings.Contains(keys, hint) {
			t.Errorf("the help does not mention %q", hint)
		}
	}
}

func TestLibraryFilterShowsOnlyTheRememberedFormats(t *testing.T) {
	model := testModel()
	model.library = []store.FormatEntry{{PID: 23, Slug: "mediatheke", Title: "Die Mediatheke"}}
	model.libraryOnly = true
	formats := model.visibleFormats()
	if len(formats) != 1 || formats[0].PID != 23 {
		t.Errorf("formats = %+v", formats)
	}
	view := model.View()
	if !strings.Contains(view, "Bibliothek") {
		t.Errorf("the view does not name the library:\n%s", view)
	}
}

func TestEpisodeStateShowsTheState(t *testing.T) {
	model := testModel()
	model.screen = ScreenEpisodes
	model.episodes = []orangeflix.Episode{
		{Identifier: "mt420", Number: 420, Title: "Folge 420"},
		{Identifier: "mt419", Number: 419, Title: "Folge 419"},
	}
	model.known = map[string]store.KnownEpisode{
		"mt420": {EpisodeRef: store.EpisodeRef{Identifier: "mt420"}, DownloadedAt: timePtr(time.Now())},
		"mt419": {EpisodeRef: store.EpisodeRef{Identifier: "mt419"}, FirstSeen: time.Now()},
	}
	if !model.episodeLoaded("mt420") {
		t.Error("the episode is not marked as loaded")
	}
	if model.episodeLoaded("mt419") {
		t.Error("the episode is marked as loaded")
	}
	if !model.episodeNew("mt419") {
		t.Error("the episode is not marked as new")
	}
	view := model.View()
	if !strings.Contains(view, "geladen") {
		t.Errorf("the view does not show the state:\n%s", view)
	}
}

func TestClipInfoIsShown(t *testing.T) {
	model := testModel()
	model.screen = ScreenEpisodes
	model.episodes = []orangeflix.Episode{
		{Identifier: "mt420", Number: 420, Title: "Folge 420"},
	}
	model.clips = map[string][]orangeflix.File{
		"mt420": {
			{Desc: "HD 1080p", Dimensions: "1920x1080", Size: 826564514},
			{Desc: "HD 720p", Dimensions: "1280x720", Size: 319830856},
		},
	}
	view := model.View()
	if !strings.Contains(view, "Qualitäten") {
		t.Errorf("the view does not show the qualities:\n%s", view)
	}
	if !strings.Contains(view, "HD 1080p") || !strings.Contains(view, "MiB") {
		t.Errorf("the view does not show the files:\n%s", view)
	}
}

func TestYearGroupsTheEpisodes(t *testing.T) {
	model := testModel()
	model.screen = ScreenEpisodes
	model.episodes = []orangeflix.Episode{
		{Identifier: "mt420", Number: 420, Date: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)},
		{Identifier: "mt300", Number: 300, Date: time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC)},
	}
	groups := model.episodeGroups()
	if len(groups) != 2 {
		t.Fatalf("groups = %+v", groups)
	}
	// The newest episode comes first, so the newest year is the first group.
	if groups[0].year != "2026" || groups[1].year != "2025" {
		t.Errorf("groups = %+v", groups)
	}
	view := model.View()
	if !strings.Contains(view, "2026") {
		t.Errorf("the view does not show the year:\n%s", view)
	}
}

// timePtr returns a pointer to a time.
func timePtr(value time.Time) *time.Time {
	return &value
}

func TestThemeColorsFollowTheSite(t *testing.T) {
	// The colors come from the stylesheet of orangeflix.de.
	applyTheme(ThemeDark)
	if got := theme.Background.GetBackground(); got != lipgloss.Color("#09090b") {
		t.Errorf("the dark background = %v", got)
	}
	if got := theme.Accent.GetForeground(); got != lipgloss.Color("#f5a019") {
		t.Errorf("the dark brand color = %v", got)
	}
	if got := theme.Text.GetForeground(); got != lipgloss.Color("#fafafa") {
		t.Errorf("the dark text color = %v", got)
	}

	applyTheme(ThemeLight)
	if got := theme.Background.GetBackground(); got != lipgloss.Color("#f8fafc") {
		t.Errorf("the light background = %v", got)
	}
	if got := theme.Accent.GetForeground(); got != lipgloss.Color("#9a5b00") {
		t.Errorf("the light brand color = %v", got)
	}
	if got := theme.Text.GetForeground(); got != lipgloss.Color("#18181b") {
		t.Errorf("the light text color = %v", got)
	}
	applyTheme(ThemeDark)
}

func TestThemeSwitchPaintsTheBackground(t *testing.T) {
	// Without a terminal lipgloss drops the colors, so the profile is forced.
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

	applyTheme(ThemeDark)
	model := testModel()
	model.width = 80
	model.height = 20
	view := model.View()
	if !strings.Contains(view, "\x1b[48;2;9;9;11m") {
		t.Error("the view does not paint the background")
	}
}

func TestLayoutUsesTheWholeTerminal(t *testing.T) {
	// The head line and the background reach the edge of the terminal.
	model := testModel()
	model.width = 200
	model.height = 30
	view := model.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 30 {
		t.Errorf("the view has %d lines, want 30", len(lines))
	}
	for _, line := range lines {
		if visibleWidth(line) != 200 {
			t.Fatalf("a line is %d wide, want 200: %q", visibleWidth(line), line)
		}
	}
}

func TestLayoutFallsBackToOneColumn(t *testing.T) {
	model := testModel()
	model.width = 80
	model.height = 20
	model.screen = ScreenEpisodes
	model.episodes = []orangeflix.Episode{
		{Identifier: "mt420", Number: 420, Title: "Folge 420"},
	}
	view := model.View()
	if strings.Contains(view, "Nummer:") {
		t.Errorf("the narrow terminal shows the second column:\n%s", view)
	}
}

func TestFooterStaysAtTheBottom(t *testing.T) {
	for _, height := range []int{12, 14, 20, 30, 50} {
		model := testModel()
		model.width = 100
		model.height = height
		model.screen = ScreenEpisodes
		model.episodes = []orangeflix.Episode{
			{Identifier: "mt420", Number: 420, Title: "Folge 420"},
		}
		view := model.View()
		lines := strings.Split(view, "\n")
		if len(lines) != height {
			t.Errorf("bei Höhe %d hat die Ansicht %d Zeilen", height, len(lines))
		}
		// Die Fußzeile steht in den letzten Zeilen.
		hints := lines[len(lines)-1]
		if len(lines) > 1 {
			hints = lines[len(lines)-2] + lines[len(lines)-1]
		}
		if !strings.Contains(hints, "wählen") && !strings.Contains(hints, "suchen") &&
			!strings.Contains(hints, "laden") && !strings.Contains(hints, "starten") {
			t.Errorf("bei Höhe %d ist die Fußzeile nicht unten: %q", height, hints)
		}
	}
}

func TestFooterSurvivesALongBody(t *testing.T) {
	model := testModel()
	model.width = 100
	model.height = 16
	model.screen = ScreenEpisodes
	for i := 0; i < 60; i++ {
		model.episodes = append(model.episodes, orangeflix.Episode{
			Identifier:  "mt" + itoa(int64(i)),
			Number:      i,
			Title:       "Folge",
			Description: strings.Repeat("Text ", 40),
		})
	}
	view := model.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 16 {
		t.Errorf("die Ansicht hat %d Zeilen, want 16", len(lines))
	}
	// Die Fußzeile kann über mehrere Zeilen laufen.
	hints := lines[len(lines)-2] + lines[len(lines)-1]
	if !strings.Contains(hints, "wählen") && !strings.Contains(hints, "laden") {
		t.Errorf("die Fußzeile fehlt: %q", hints)
	}
}

func TestStartScreenIsSkippedWithAToken(t *testing.T) {
	// Ein gespeichertes Token lässt den Startbildschirm entfallen.
	model := NewModel(context.Background(), Options{
		Config:   &config.Options{Quality: "best", Submodule: 1},
		Version:  "test",
		HasToken: true,
	})
	if !model.credentials {
		t.Error("der Startbildschirm wird trotz Token gezeigt")
	}
}

func TestStartScreenIsSkippedWithCredentials(t *testing.T) {
	model := NewModel(context.Background(), Options{
		Config: &config.Options{
			Username:  "user@example.com",
			Password:  "secret",
			Quality:   "best",
			Submodule: 1,
		},
		Version: "test",
	})
	if !model.credentials {
		t.Error("der Startbildschirm wird trotz Zugangsdaten gezeigt")
	}
}

func TestStartScreenIsShownWithoutAnything(t *testing.T) {
	model := NewModel(context.Background(), Options{
		Config:  &config.Options{Quality: "best", Submodule: 1},
		Version: "test",
	})
	if model.credentials {
		t.Error("der Startbildschirm wird ohne Zugangsdaten übersprungen")
	}
}

func TestHelpWindowIsCentered(t *testing.T) {
	model := testModel()
	model.width = 100
	model.height = 24
	model.showHelp = true
	lines := strings.Split(stripANSI(model.View()), "\n")

	// Das Fenster beginnt mit der oberen Kante aus Halbblöcken und endet mit
	// der unteren.
	top, bottom := -1, -1
	for index, line := range lines {
		if top < 0 && strings.Contains(line, "▄▄▄▄") {
			top = index
		}
		if top >= 0 && strings.Contains(line, "▀▀▀▀") {
			bottom = index
		}
	}
	if top < 0 || bottom < 0 {
		t.Fatalf("das Hilfefenster fehlt:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.Contains(lines[top+1], "Hilfe") {
		t.Errorf("die Kopfzeile fehlt: %q", lines[top+1])
	}

	// Das Fenster steht mittig: der Abstand links und rechts ist gleich.
	edge := []rune(lines[top])
	left, right := -1, -1
	for index, r := range edge {
		if r == '▄' {
			if left < 0 {
				left = index
			}
			right = len(edge) - index - 1
		}
	}
	if abs(left-right) > 1 {
		t.Errorf("das Fenster steht nicht mittig: %d links, %d rechts", left, right)
	}

	// Das Fenster liegt in der Mitte: über und unter ihm bleibt Platz.
	if top <= 1 {
		t.Errorf("das Fenster klebt oben: %d", top)
	}
	if len(lines)-bottom <= 2 {
		t.Errorf("das Fenster klebt unten: %d von %d", bottom, len(lines))
	}
}

func TestHelpWindowFitsItsText(t *testing.T) {
	model := testModel()
	model.width = 100
	model.height = 24
	model.showHelp = true
	// Nur die Zeilen des Hilfefensters werden geprüft.
	for _, line := range model.helpPopup(100) {
		if strings.Contains(line, "…") {
			t.Errorf("ein Text wurde abgeschnitten: %q", line)
		}
	}
}

// abs returns the distance of a number from zero.
func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func TestQualityWindowPicksTheQuality(t *testing.T) {
	model := testModel()
	model.width = 100
	model.height = 24
	model.screen = ScreenEpisodes
	model.episodes = []orangeflix.Episode{
		{Identifier: "mt420", Number: 420, Title: "Folge 420"},
	}
	model.clips = map[string][]orangeflix.File{
		"mt420": {
			{Desc: "HD 1080p", Dimensions: "1920x1080", Size: 826564514},
			{Desc: "HD 720p", Dimensions: "1280x720", Size: 319830856},
		},
	}

	var chosen string
	model.setQuality = func(quality string) { chosen = quality }

	model = press(t, model, "i")
	if !model.showQuality {
		t.Fatal("das Qualitätsfenster ist nicht offen")
	}
	if len(model.qualityEntries()) != 2 {
		t.Fatalf("entries = %+v", model.qualityEntries())
	}

	// Der Cursor wandert auf die zweite Qualität und enter übernimmt sie.
	model = press(t, model, "down")
	model = press(t, model, "enter")
	if model.showQuality {
		t.Error("das Qualitätsfenster ist noch offen")
	}
	if chosen != "mobile" {
		t.Errorf("quality = %q, want mobile", chosen)
	}
	if model.config.Quality != "mobile" {
		t.Errorf("config quality = %q", model.config.Quality)
	}
}

func TestPathWindowChangesTheDownloadsDir(t *testing.T) {
	model := testModel()
	model.width = 100
	model.height = 30

	dir := t.TempDir()
	state, err := store.Open(filepath.Join(dir, "library.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer state.Close()
	model.store = state

	var chosen string
	model.setDownloadsDir = func(value string) { chosen = value }

	model = press(t, model, "p")
	if !model.showPath {
		t.Fatal("das Fenster für den Speicherort ist nicht offen")
	}
	if view := model.View(); !strings.Contains(view, "Speicherort") {
		t.Errorf("das Fenster fehlt in der Ansicht:\n%s", view)
	}

	target := filepath.Join(dir, "serien")
	model.pathInput.SetValue(target)
	model = press(t, model, "enter")
	if model.showPath {
		t.Error("das Fenster ist noch offen")
	}
	if chosen != target {
		t.Errorf("chosen = %q, want %q", chosen, target)
	}
	if model.config.DownloadsDir != target {
		t.Errorf("config dir = %q", model.config.DownloadsDir)
	}
	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		t.Errorf("der Ordner wurde nicht angelegt: %v", err)
	}
	stored, err := state.Setting(store.SettingDownloadsDir)
	if err != nil || stored != target {
		t.Errorf("stored = %q, err = %v", stored, err)
	}
}

func TestPathWindowRejectsAnUnwrittenFolder(t *testing.T) {
	model := testModel()
	model.width = 100
	model.height = 30
	model = press(t, model, "p")

	file := filepath.Join(t.TempDir(), "datei")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	model.pathInput.SetValue(file)
	model = press(t, model, "enter")
	if !model.showPath {
		t.Error("das Fenster wurde trotz Fehler geschlossen")
	}
	if model.pathError == "" {
		t.Error("die Fehlermeldung fehlt")
	}
}

func TestPathWindowPastesFromClipboard(t *testing.T) {
	model := testModel()
	model.width = 100
	model.height = 30
	model = press(t, model, "p")

	clipboard := "  ~/Serien  \n"
	saved := clipboardRead
	clipboardRead = func() (string, error) { return clipboard, nil }
	t.Cleanup(func() { clipboardRead = saved })

	model = press(t, model, "ctrl+v")
	if got := model.pathInput.Value(); got != "~/Serien" {
		t.Errorf("value = %q, want %q", got, "~/Serien")
	}

	// The clipboard is inserted at the cursor, not only at the end.
	model.pathInput.SetValue("~/Serien")
	model.pathInput.SetCursor(2)
	clipboard = "Archiv/"
	model = press(t, model, "ctrl+v")
	if got := model.pathInput.Value(); got != "~/Archiv/Serien" {
		t.Errorf("value = %q, want %q", got, "~/Archiv/Serien")
	}
	if pos := model.pathInput.Position(); pos != 9 {
		t.Errorf("cursor = %d, want 9", pos)
	}
}

func TestPathWindowReportsAnEmptyClipboard(t *testing.T) {
	model := testModel()
	model.width = 100
	model.height = 30
	model = press(t, model, "p")
	model.pathInput.SetValue("~/Serien")

	saved := clipboardRead
	clipboardRead = func() (string, error) { return " \n", nil }
	t.Cleanup(func() { clipboardRead = saved })

	model = press(t, model, "ctrl+v")
	if model.pathError == "" {
		t.Error("die Fehlermeldung fehlt")
	}
	if got := model.pathInput.Value(); got != "~/Serien" {
		t.Errorf("value = %q", got)
	}

	clipboardRead = func() (string, error) { return "", errors.New("kein Zugriff") }
	model = press(t, model, "ctrl+v")
	if !strings.Contains(model.pathError, "kein Zugriff") {
		t.Errorf("pathError = %q", model.pathError)
	}
}

func TestPathWindowListsTheFolders(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"Serien", "Filme"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "notiz.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	model := testModel()
	model.width = 100
	model.height = 40
	model = press(t, model, "p")
	model.pathInput.SetValue(root)
	model.loadPathDirs()

	want := []string{"..", "Filme", "Serien"}
	if !slices.Equal(model.pathDirs, want) {
		t.Fatalf("pathDirs = %v, want %v", model.pathDirs, want)
	}
	view := model.View()
	for _, name := range []string{"Filme", "Serien"} {
		if !strings.Contains(view, name+"/") {
			t.Errorf("die Ansicht zeigt %q nicht:\n%s", name, view)
		}
	}
	if strings.Contains(view, "notiz.txt") {
		t.Errorf("die Ansicht zeigt eine Datei:\n%s", view)
	}
}

func TestPathWindowExplainsAMissingFolder(t *testing.T) {
	model := testModel()
	model.width = 100
	model.height = 40
	model = press(t, model, "p")
	model.pathInput.SetValue(filepath.Join(t.TempDir(), "fehlt"))
	model.loadPathDirs()

	if len(model.pathDirs) != 0 {
		t.Fatalf("pathDirs = %v", model.pathDirs)
	}
	if view := model.View(); !strings.Contains(view, "existiert noch nicht") {
		t.Errorf("der Hinweis fehlt:\n%s", view)
	}
}

func TestPathWindowOpensAndLeavesFolders(t *testing.T) {
	root := t.TempDir()
	serien := filepath.Join(root, "Serien")
	if err := os.MkdirAll(filepath.Join(serien, "Staffel 1"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	model := testModel()
	model.width = 100
	model.height = 40
	model = press(t, model, "p")
	model.pathInput.SetValue(root)
	model.loadPathDirs()

	// ".." steht vorn, darunter folgt der Ordner.
	model = press(t, model, "down")
	if model.pathCursor != 1 {
		t.Fatalf("cursor = %d, want 1", model.pathCursor)
	}
	model = press(t, model, "tab")
	if got := model.pathInput.Value(); got != serien {
		t.Errorf("value = %q, want %q", got, serien)
	}
	if want := []string{"..", "Staffel 1"}; !slices.Equal(model.pathDirs, want) {
		t.Errorf("pathDirs = %v, want %v", model.pathDirs, want)
	}

	// shift+tab geht zurück und markiert den Ordner, aus dem man kam.
	model = press(t, model, "shift+tab")
	if got := model.pathInput.Value(); got != root {
		t.Errorf("value = %q, want %q", got, root)
	}
	if model.pathDirs[model.pathCursor] != "Serien" {
		t.Errorf("markiert ist %q, want Serien", model.pathDirs[model.pathCursor])
	}

	// Der ".."-Eintrag führt ebenfalls eine Ebene hoch.
	model = press(t, model, "tab")
	model.pathDirs = model.pathDirs[:1]
	model.pathCursor = 0
	model = press(t, model, "tab")
	if got := model.pathInput.Value(); got != root {
		t.Errorf("value = %q, want %q", got, root)
	}
}

func TestPathWindowScrollsTheFolderList(t *testing.T) {
	root := t.TempDir()
	for index := range 12 {
		if err := os.Mkdir(filepath.Join(root, fmt.Sprintf("ordner-%02d", index)), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	model := testModel()
	model.width = 100
	model.height = 40
	model = press(t, model, "p")
	model.pathInput.SetValue(root)
	model.loadPathDirs()

	for range 10 {
		model = press(t, model, "down")
	}
	if model.pathCursor != 10 {
		t.Fatalf("cursor = %d, want 10", model.pathCursor)
	}
	if model.pathOffset == 0 {
		t.Error("die Liste ist nicht gescrollt")
	}
	view := model.View()
	if !strings.Contains(view, "ordner-09/") {
		t.Errorf("die markierte Zeile fehlt:\n%s", view)
	}
	if !strings.Contains(view, "von 13") {
		t.Errorf("der Zähler fehlt:\n%s", view)
	}
}

func TestDisplayPathShortensTheHomeFolder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got := displayPath(home); got != "~" {
		t.Errorf("displayPath(home) = %q", got)
	}
	if got := displayPath(filepath.Join(home, "Videos", "Serien")); got != "~/Videos/Serien" {
		t.Errorf("displayPath = %q", got)
	}
	if got := displayPath("/tmp"); got != "/tmp" {
		t.Errorf("displayPath = %q", got)
	}
}

func TestFormatListHasNoSlug(t *testing.T) {
	model := testModel()
	model.width = 120
	model.height = 24
	view := model.View()
	if strings.Contains(view, "SLUG") {
		t.Errorf("die Übersicht zeigt den Slug:\n%s", view)
	}
	// Der Slug steht in den Angaben zum markierten Format. Die Liste ist
	// nach der letzten Folge sortiert, das markierte Format wechselt also.
	formats := model.visibleFormats()
	if len(formats) == 0 {
		t.Fatal("die Liste ist leer")
	}
	if !strings.Contains(view, formats[model.formatCursor].Slug) {
		t.Errorf("die Übersicht zeigt die Formate nicht:\n%s", view)
	}
}

func TestFooterHasNoDoubledHint(t *testing.T) {
	model := testModel()
	model.width = 120
	model.height = 24
	view := model.View()
	if strings.Count(view, "↑/↓ wählen") != 1 {
		t.Errorf("der Hinweis steht mehr als einmal:\n%s", view)
	}
}

// TestNewCredentialsCheckTheAccessAgain checks that the account box drops the
// state of the old account and shows the answer for the new one.
func TestNewCredentialsCheckTheAccessAgain(t *testing.T) {
	model := testModel()
	model.profile = &app.ProfileSummary{HasAccess: true, Lifetime: true}
	model.screen = ScreenStart
	model.userInput.SetValue("neu@example.org")
	model.passInput.SetValue("geheim")
	model.loadProfileState = func(context.Context) (*app.ProfileSummary, error) {
		return nil, errors.New("falsches Passwort")
	}
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(*Model)
	if model.profile != nil {
		t.Fatal("the state of the old account is still shown")
	}
	if !strings.Contains(model.profileLine(), "wird geprüft") {
		t.Fatalf("profile line while loading: %q", model.profileLine())
	}
	updated, _ = model.Update(ProfileMsg{Err: errors.New("falsches Passwort")})
	model = updated.(*Model)
	if !strings.Contains(model.profileLine(), "Anmeldung fehlgeschlagen") {
		t.Fatalf("profile line after the error: %q", model.profileLine())
	}
}

// TestTheLoginFormTakesTheSecondFactor checks that the code of the form goes
// to the application once and that the form comes back when the account asks
// for a code.
func TestTheLoginFormTakesTheSecondFactor(t *testing.T) {
	model := testModel()
	model.screen = ScreenStart
	var given string
	model.setTotp = func(code string) { given = code }
	model.userInput.SetValue("user@example.org")
	model.passInput.SetValue("geheim")
	for range 2 {
		model = press(t, model, "tab")
	}
	if model.field != 2 || !model.totpInput.Focused() {
		t.Fatalf("field = %d, the code field is not focused", model.field)
	}
	for _, digit := range "123456" {
		model = press(t, model, string(digit))
	}
	model = press(t, model, "enter")
	if given != "123456" {
		t.Fatalf("code handed over = %q", given)
	}
	if model.totpInput.Value() != "" {
		t.Fatalf("the code stays in the form: %q", model.totpInput.Value())
	}

	updated, _ := model.Update(ProfileMsg{Err: fmt.Errorf("%w: bitte eintragen", app.ErrTotpRequired)})
	model = updated.(*Model)
	if model.screen != ScreenStart || model.field != 2 {
		t.Fatalf("screen = %v, field = %d; the form must ask for the code", model.screen, model.field)
	}
	if !strings.Contains(model.message, "2FA-Code") {
		t.Fatalf("message = %q", model.message)
	}
	if !strings.Contains(model.profileLine(), "2FA-Code nötig") {
		t.Fatalf("profile line = %q", model.profileLine())
	}
}

// TestTheHelpScrollsWhenItIsTooLow checks that a low terminal shows the rest
// of the help with the arrows.
func TestTheHelpScrollsWhenItIsTooLow(t *testing.T) {
	model := testModel()
	model.width = 80
	model.height = 24
	model = press(t, model, "?")
	text := stripANSI(strings.Join(model.helpPopup(80), "\n"))
	if !strings.Contains(text, "blättern") {
		t.Fatalf("the foot does not offer to scroll:\n%s", text)
	}
	if strings.Contains(text, "Erscheinungsbild") {
		t.Fatal("the end of the help is visible without scrolling")
	}
	for range 20 {
		model = press(t, model, "down")
	}
	if !model.showHelp {
		t.Fatal("the arrow closed the help")
	}
	text = stripANSI(strings.Join(model.helpPopup(80), "\n"))
	if !strings.Contains(text, "Erscheinungsbild") {
		t.Fatalf("the end of the help is not reached:\n%s", text)
	}
	model = press(t, model, "esc")
	if model.showHelp {
		t.Fatal("esc does not close the help")
	}
}

// TestTheHelpStartsWithTheOpenScreen checks that the keys of the open screen
// stand first and the keys carry the accent color.
func TestTheHelpStartsWithTheOpenScreen(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)
	applyTheme(ThemeDark)
	defer applyTheme(ThemeDark)
	model := testModel()
	model.screen = ScreenEpisodes
	lines := model.helpPopup(120)
	// Oben die Kante, die Kopfzeile und ein Abstand, dann die Gruppen.
	if !strings.Contains(stripANSI(lines[1]), "Hilfe") {
		t.Fatalf("title: %q", stripANSI(lines[1]))
	}
	if !strings.Contains(stripANSI(lines[3]), "Folgen · hier") {
		t.Fatalf("first group: %q", stripANSI(lines[3]))
	}
	accent := sequenceOf(theme.AccentBold)
	if accent == "" {
		t.Fatal("the accent style has no color")
	}
	if !strings.Contains(lines[4], accent) {
		t.Fatalf("the keys are not in the accent color: %q", lines[4])
	}
}
