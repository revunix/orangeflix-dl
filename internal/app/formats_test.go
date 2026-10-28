package app

import (
	"testing"
	"time"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// TestIsNewEpisode checks the age of an episode against the current date.
func TestIsNewEpisode(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		moment time.Time
		want   bool
	}{
		{now.Add(-24 * time.Hour), true},
		{now.Add(-6 * 24 * time.Hour), true},
		{now.Add(-8 * 24 * time.Hour), false},
		{now.Add(-90 * 24 * time.Hour), false},
		{now.Add(24 * time.Hour), true},
		{time.Time{}, false},
	}
	for _, item := range cases {
		if got := isNewEpisode(item.moment, now); got != item.want {
			t.Errorf("isNewEpisode(%v) = %v, want %v", item.moment, got, item.want)
		}
	}
}

// TestMagazineLastEpisodeReadsTheTimestamp checks the date of the live
// catalog and the guard against values that are no date.
func TestMagazineLastEpisodeReadsTheTimestamp(t *testing.T) {
	magazine := orangeflix.V3Magazine{}
	magazine.TimeRange.Max = time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC).Unix()
	moment, ok := magazineLastEpisode(magazine)
	if !ok {
		t.Fatal("the timestamp was not read")
	}
	if moment.Year() != 2026 || moment.Month() != 10 || moment.Day() != 20 {
		t.Errorf("moment = %v", moment)
	}

	// Ein Wert, der kein Datum ist, wird nicht genommen.
	magazine.TimeRange.Max = 42
	if _, ok := magazineLastEpisode(magazine); ok {
		t.Error("a value that is no date was taken")
	}
	magazine.TimeRange.Max = 0
	if _, ok := magazineLastEpisode(magazine); ok {
		t.Error("the empty value was taken")
	}
}

// TestTheCatalogKnowsTheDateOfTheLastEpisode checks that the catalog in
// the program carries the date as a time, so the list can be sorted by it.
func TestTheCatalogKnowsTheDateOfTheLastEpisode(t *testing.T) {
	for _, format := range orangeflix.Catalog {
		if format.LastEpisode == "" {
			continue
		}
		if format.LastEpisodeTime.IsZero() {
			t.Errorf("%s: the date of the last episode is missing", format.Slug)
		}
	}
}
