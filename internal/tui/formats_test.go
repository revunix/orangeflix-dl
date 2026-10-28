package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// TestTheFormatsAreSortedByTheLastEpisode checks that the newest format
// comes first in its group.
func TestTheFormatsAreSortedByTheLastEpisode(t *testing.T) {
	model := testModel()
	model.screen = ScreenFormats
	model.formats = []orangeflix.Format{
		{PID: 1, Slug: "alt", Title: "Alt", Status: orangeflix.StatusActive, LastEpisodeTime: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
		{PID: 2, Slug: "neu", Title: "Neu", Status: orangeflix.StatusActive, LastEpisodeTime: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
		{PID: 3, Slug: "dazwischen", Title: "Dazwischen", Status: orangeflix.StatusActive, LastEpisodeTime: time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)},
		{PID: 4, Slug: "ohne", Title: "Ohne Datum", Status: orangeflix.StatusActive},
	}
	rows := model.formatRows()
	order := []string{}
	for _, row := range rows {
		if row.format != nil {
			order = append(order, row.format.Slug)
		}
	}
	want := []string{"neu", "dazwischen", "alt", "ohne"}
	for index, slug := range want {
		if index >= len(order) || order[index] != slug {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

// TestTheFormatListShowsTheMarkers checks the marks for a new episode and
// for a format the user has already been in.
func TestTheFormatListShowsTheMarkers(t *testing.T) {
	model := testModel()
	model.screen = ScreenFormats
	model.formatWidth = 80
	format := orangeflix.Format{
		PID: 2, Slug: "pantoffeltv", Title: "Pantoffelkino",
		Status: orangeflix.StatusActive, NewEpisode: true, Opened: true,
	}
	line := model.formatLine(format)
	if !strings.Contains(line, "●") {
		t.Errorf("the row has no marker for the new episode: %q", line)
	}
	if !strings.Contains(line, "✓") {
		t.Errorf("the row has no marker for the visit: %q", line)
	}

	// Ein Format ohne neue Folge und ohne Besuch zeigt keine Marken.
	plain := model.formatLine(orangeflix.Format{
		PID: 19, Slug: "veto", Title: "Veto", Status: orangeflix.StatusActive,
	})
	if strings.Contains(plain, "●") || strings.Contains(plain, "✓") {
		t.Errorf("the row has markers it should not have: %q", plain)
	}
}

// TestTheFormatColumnsFallAway checks that the list keeps the title when
// the terminal is narrow.
func TestTheFormatColumnsFallAway(t *testing.T) {
	wide := measureFormatColumns(120, 2)
	if wide.seen == 0 || wide.new == 0 {
		t.Errorf("a wide terminal has no markers: %+v", wide)
	}
	narrow := measureFormatColumns(40, 2)
	if narrow.title < 10 {
		t.Errorf("the title has %d characters: %+v", narrow.title, narrow)
	}
	if narrow.count != 0 && narrow.last != 0 && narrow.seen != 0 && narrow.new != 0 {
		t.Errorf("the narrow list has all the columns: %+v", narrow)
	}
}
