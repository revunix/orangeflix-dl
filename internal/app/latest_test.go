package app

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

func TestLatestEpisodeReadsOnlyTheFirstPage(t *testing.T) {
	requests := 0
	application, _ := moreApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/content/feed" {
			t.Errorf("path = %q", r.URL.Path)
		}
		requests++
		if offset := r.URL.Query().Get("offset"); offset != "" && offset != "0" {
			t.Errorf("offset = %q, want the first page", offset)
		}
		_, _ = w.Write([]byte(`{"ok":true,"clips":[
			{"id":"pk419","magazineId":2,"seqNr":419,"title":"Folge 419","projectTitle":"Pantoffelkino","time":1793000000},
			{"id":"pk420","magazineId":2,"seqNr":420,"title":"Folge 420","projectTitle":"Pantoffelkino","time":1793600000}
		]}`))
	})
	episode, err := application.LatestEpisode(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if episode.Identifier != "pk420" || episode.PID != 2 || episode.FormatName != "Pantoffelkino" {
		t.Errorf("episode = %+v", episode)
	}
	if requests != 1 {
		t.Errorf("requests = %d, want one page", requests)
	}
}

func TestLatestEpisodeOfAnEmptyFormat(t *testing.T) {
	application, _ := moreApp(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"clips":[]}`))
	})
	if _, err := application.LatestEpisode(context.Background(), 2); !errors.Is(err, ErrNoEpisode) {
		t.Errorf("err = %v, want ErrNoEpisode", err)
	}
}

func TestNewestEpisodeUsesTheDateThenTheNumber(t *testing.T) {
	day := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	episodes := []orangeflix.Episode{
		{Identifier: "old", Number: 900, Date: day.Add(-24 * time.Hour)},
		{Identifier: "a", Number: 10, Date: day},
		{Identifier: "b", Number: 11, Date: day},
	}
	newest, ok := NewestEpisode(episodes)
	if !ok || newest.Identifier != "b" {
		t.Errorf("newest = %+v", newest)
	}
	if _, ok := NewestEpisode(nil); ok {
		t.Error("an empty list has a newest episode")
	}
}
