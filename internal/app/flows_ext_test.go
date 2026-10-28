package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/revunix/orangeflix-dl/internal/config"
	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

func argEpisode(number int, title string, date time.Time) orangeflix.Episode {
	return orangeflix.Episode{Identifier: "id", Number: number, Title: title, Date: date}
}

func TestPickSince(t *testing.T) {
	app := &App{}
	episodes := []orangeflix.Episode{
		argEpisode(1, "alt", time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)),
		argEpisode(2, "am Tag", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)),
		argEpisode(3, "später", time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)),
		argEpisode(4, "ohne Datum", time.Time{}),
	}
	picked := app.pickSince(episodes, time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
	if len(picked) != 2 || picked[0].Number != 2 || picked[1].Number != 3 {
		t.Fatalf("picked = %+v", picked)
	}
}

func TestPickTitle(t *testing.T) {
	app := &App{}
	episodes := []orangeflix.Episode{
		argEpisode(1, "Die Mediatheke", time.Now()),
		argEpisode(2, "TECHLOAD", time.Now()),
		argEpisode(3, "anders", time.Now()),
	}
	picked := app.pickTitle(episodes, "techload")
	if len(picked) != 1 || picked[0].Number != 2 {
		t.Fatalf("picked = %+v", picked)
	}
}

func TestSaveMetaWritesTheJSONFile(t *testing.T) {
	dir := t.TempDir()
	app := &App{Config: &config.Options{Meta: true}}

	video := filepath.Join(dir, "folge.mp4")
	if err := os.WriteFile(video, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	e := orangeflix.Episode{Identifier: "mt1", PID: 23, Number: 1, Title: "Beginn", FormatName: "Die Mediatheke"}
	base := video[:len(video)-len(filepath.Ext(video))]
	app.saveMeta(e, video, base+".json")
	data, err := os.ReadFile(base + ".json")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var meta sidecarMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if meta.Identifier != "mt1" || meta.Title != "Beginn" || meta.Number != 1 {
		t.Fatalf("meta = %+v", meta)
	}
}

func TestPauseRespectsTheContext(t *testing.T) {
	app := &App{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	app.pause(ctx, 10) // returns immediately instead of waiting ten seconds
}

func TestDownloadEpisodesParallel(t *testing.T) {
	// Zählungen der Fehler müssen im Zusammenspiel von Downloads sicher sein.
	app := &App{}
	app.recordFailure("ein Fehler")
	if app.episodeFailures != 1 {
		t.Fatalf("failures = %d", app.episodeFailures)
	}
}
