package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/revunix/orangeflix-dl/internal/app"
	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// runKey presses a key and runs the command that comes back once.
func runKey(t *testing.T, model *Model, key string) *Model {
	t.Helper()
	updated, cmd := model.Update(keyMsgFor(key))
	model = updated.(*Model)
	if cmd == nil {
		t.Fatalf("%q returned no command", key)
	}
	return update(t, model, cmd())
}

func TestWWatchesTheNewestEpisodeOfAFormat(t *testing.T) {
	model := testModel()
	format := model.filteredFormats()[model.formatCursor]
	var asked int
	model.latestEpisodeFunc = func(_ context.Context, pid int) (orangeflix.Episode, error) {
		asked = pid
		return orangeflix.Episode{Identifier: "neu", Title: "Folge 99"}, nil
	}
	var played []orangeflix.Episode
	model.watchEpisodesFunc = func(_ context.Context, episodes []orangeflix.Episode) error {
		played = episodes
		return nil
	}
	model.loadEpisodes = func(context.Context, int) ([]orangeflix.Episode, error) {
		t.Error("the whole list was loaded")
		return nil, nil
	}

	updated, cmd := model.Update(keyMsgFor("w"))
	model = updated.(*Model)
	if !model.loading || !strings.Contains(model.loadingLabel, format.Title) {
		t.Errorf("no loading state: %v %q", model.loading, model.loadingLabel)
	}
	updated, play := model.Update(cmd())
	model = updated.(*Model)
	if asked != format.PID {
		t.Errorf("pid = %d, want %d", asked, format.PID)
	}
	if model.screen != ScreenFormats {
		t.Errorf("the format was opened: %v", model.screen)
	}
	// Der Player läuft im Befehl, den die Antwort zurückgibt.
	if play == nil {
		t.Fatal("no player command")
	}
	play()
	if len(played) != 1 || played[0].Identifier != "neu" || played[0].PID != format.PID || played[0].FormatName != format.Title {
		t.Errorf("played = %+v", played)
	}
}

func TestShiftDLoadsTheNewestEpisode(t *testing.T) {
	model := testModel()
	format := model.filteredFormats()[model.formatCursor]
	model.latestEpisodeFunc = func(context.Context, int) (orangeflix.Episode, error) {
		return orangeflix.Episode{Identifier: "neu", Title: "Folge 99"}, nil
	}
	var loaded orangeflix.Episode
	model.downloadEpisodeFunc = func(_ context.Context, episode orangeflix.Episode) error {
		loaded = episode
		return nil
	}
	updated, cmd := model.Update(keyMsgFor("D"))
	model = updated.(*Model)
	updated, run := model.Update(cmd())
	model = updated.(*Model)
	if model.screen != ScreenQueue {
		t.Fatalf("screen = %v, want the downloads", model.screen)
	}
	// Der Download läuft im Befehl, den die Antwort zurückgibt.
	run()
	if loaded.Identifier != "neu" || loaded.PID != format.PID {
		t.Errorf("loaded = %+v", loaded)
	}
}

func TestTheNewestEpisodeOfAnEmptyFormat(t *testing.T) {
	model := testModel()
	format := model.filteredFormats()[model.formatCursor]
	model.latestEpisodeFunc = func(context.Context, int) (orangeflix.Episode, error) {
		return orangeflix.Episode{}, app.ErrNoEpisode
	}
	model = runKey(t, model, "w")
	if !model.isError || model.message != format.Title+" hat noch keine Folge." || model.loading {
		t.Errorf("message %q, error %v, loading %v", model.message, model.isError, model.loading)
	}
}

func TestTheNewestEpisodeFallsBackToTheList(t *testing.T) {
	model := testModel()
	now := time.Now()
	model.loadEpisodes = func(context.Context, int) ([]orangeflix.Episode, error) {
		return []orangeflix.Episode{
			{Identifier: "alt", Number: 1, Date: now.Add(-48 * time.Hour)},
			{Identifier: "neu", Number: 2, Date: now},
		}, nil
	}
	updated, cmd := model.Update(keyMsgFor("w"))
	model = updated.(*Model)
	msg, ok := cmd().(LatestMsg)
	if !ok || msg.Err != nil || msg.Episode.Identifier != "neu" || !msg.Watch {
		t.Errorf("msg = %+v", msg)
	}
}

func TestTheFormatMenuOffersTheNewestEpisode(t *testing.T) {
	model := testModel()
	model = clickOn(t, model, "MAXimal Musik", tea.MouseButtonRight)
	labels := []string{}
	for _, item := range model.context.items {
		labels = append(labels, item.label)
	}
	joined := strings.Join(labels, "|")
	if !strings.Contains(joined, "Neueste Folge (") || !strings.Contains(joined, ") ansehen") || !strings.Contains(joined, ") herunterladen") {
		t.Errorf("items = %s", joined)
	}
	model = press(t, model, "esc")
	if line := strings.Join(model.footer(160), ""); !strings.Contains(line, "w neueste ansehen") {
		t.Errorf("the footer does not offer the newest episode: %q", line)
	}
}
