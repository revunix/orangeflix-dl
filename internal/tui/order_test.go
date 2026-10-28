package tui

import (
	"testing"
	"time"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// TestFilteredEpisodesOrderByDate checks that the list follows the release
// date instead of the episode number: the newest goes on top, the years
// stay together, and the number only breaks a tie.
func TestFilteredEpisodesOrderByDate(t *testing.T) {
	model := testModel()
	model.screen = ScreenEpisodes
	date := func(year int, month time.Month, day int) time.Time {
		return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	}
	// Der Feed liefert die Folgen in gemischter Ordnung.
	model.episodes = []orangeflix.Episode{
		{Identifier: "pk12", Number: 12, Title: "Alt", Date: date(2024, 3, 2)},
		{Identifier: "pk31", Number: 31, Title: "Neu", Date: date(2026, 1, 15)},
		{Identifier: "pk20", Number: 20, Title: "Mitte", Date: date(2025, 7, 5)},
		{Identifier: "pk11", Number: 11, Title: "Noch älter", Date: date(2024, 2, 1)},
		{Identifier: "pk30", Number: 30, Title: "ohne Datum"},
	}
	episodes := model.filteredEpisodes()
	want := []string{"pk31", "pk20", "pk12", "pk11", "pk30"}
	for index, identifier := range want {
		if episodes[index].Identifier != identifier {
			t.Fatalf("position %d = %s, want %s", index, episodes[index].Identifier, identifier)
		}
	}

	// Die Jahre stehen in zusammenlaufenden Gruppen, das neueste Jahr oben.
	groups := model.episodeGroups()
	years := []string{}
	for _, group := range groups {
		years = append(years, group.year)
	}
	if len(years) != 4 || years[0] != "2026" || years[1] != "2025" || years[2] != "2024" {
		t.Fatalf("years = %v", years)
	}

	// Die Zeile des Cursors stimmt mit der Auswahl überein.
	rows := model.episodeRows(groups)
	episodes = model.filteredEpisodes()
	if model.episodeCursor >= len(episodes) {
		t.Fatalf("cursor = %d", model.episodeCursor)
	}
	wanted := episodes[model.episodeCursor].Identifier
	found := false
	for _, row := range rows {
		if row.episode != nil && row.episode.Identifier == wanted {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("the row of the cursor %s is not in the list", wanted)
	}
}
