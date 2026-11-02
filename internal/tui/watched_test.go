package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// watchedCall is one request to the account.
type watchedCall struct {
	id      string
	watched bool
}

// recordWatched lets the model mark episodes and remembers the requests.
func recordWatched(model *Model, fail error) *[]watchedCall {
	calls := []watchedCall{}
	model.setWatchedFunc = func(_ context.Context, episode orangeflix.Episode, watched bool) error {
		calls = append(calls, watchedCall{episode.Identifier, watched})
		return fail
	}
	return &calls
}

// pressAndRun presses a key and hands the answer of its command back.
func pressAndRun(t *testing.T, model *Model, key string) *Model {
	t.Helper()
	updated, cmd := model.Update(keyMsgFor(key))
	model = updated.(*Model)
	if cmd == nil {
		t.Fatalf("%q returned no command", key)
	}
	return update(t, model, cmd())
}

func TestGMarksTheEpisodeAsWatchedAndBack(t *testing.T) {
	model := episodesModel()
	calls := recordWatched(model, nil)
	id := model.filteredEpisodes()[model.episodeCursor].Identifier

	// Die Liste zeigt den Stand sofort, noch vor der Antwort.
	updated, cmd := model.Update(keyMsgFor("g"))
	model = updated.(*Model)
	if !model.isWatched(id) || !strings.Contains(ansi.Strip(model.View()), "✓ gesehen") {
		t.Fatalf("the list does not show the episode as watched")
	}
	model = update(t, model, cmd())
	if len(*calls) != 1 || (*calls)[0] != (watchedCall{id, true}) {
		t.Errorf("calls = %+v", *calls)
	}
	if model.message != "Als gesehen markiert." || model.isError {
		t.Errorf("message = %q", model.message)
	}
	if line := strings.Join(model.footer(200), ""); !strings.Contains(line, "g ungesehen") {
		t.Errorf("the footer does not offer the way back: %q", line)
	}

	model = pressAndRun(t, model, "g")
	if model.isWatched(id) || (*calls)[1] != (watchedCall{id, false}) {
		t.Errorf("g did not take the mark away: calls %+v", *calls)
	}
}

func TestGMarksAllMarkedEpisodes(t *testing.T) {
	model := episodesModel()
	calls := recordWatched(model, nil)
	model.episodes[0].ProgressPercent = 100
	model.selected = map[string]bool{"mt419": true, "mt420": true}
	if line := strings.Join(model.footer(200), ""); !strings.Contains(line, "g 2 gesehen") {
		t.Errorf("the footer does not count the marked episodes: %q", line)
	}
	model = pressAndRun(t, model, "g")
	if len(*calls) != 2 || !(*calls)[0].watched || !(*calls)[1].watched {
		t.Errorf("calls = %+v", *calls)
	}
	if !model.isWatched("mt419") || !model.isWatched("mt420") || model.message != "2 Folgen als gesehen markiert." {
		t.Errorf("watched %v %v, message %q", model.isWatched("mt419"), model.isWatched("mt420"), model.message)
	}
}

func TestAFailedMarkGoesBack(t *testing.T) {
	model := episodesModel()
	model.episodes[1].ProgressPercent = 39
	recordWatched(model, errors.New("offline"))
	model.episodeCursor = 0
	id := model.filteredEpisodes()[0].Identifier
	before := model.episodeProgress(id)
	model = pressAndRun(t, model, "g")
	if model.episodeProgress(id) != before {
		t.Errorf("progress = %d, want %d back", model.episodeProgress(id), before)
	}
	if !model.isError || !strings.Contains(model.message, "offline") {
		t.Errorf("message = %q", model.message)
	}
}

func TestTheMenuMarksAsWatched(t *testing.T) {
	model := episodesModel()
	calls := recordWatched(model, nil)
	model = clickOn(t, model, "Folge 419", tea.MouseButtonRight)
	updated, cmd := model.runContextItem(indexOfLabel(t, model, "Als gesehen markieren"))
	model = updated.(*Model)
	model = update(t, model, cmd())
	if len(*calls) != 1 || (*calls)[0] != (watchedCall{"mt419", true}) {
		t.Errorf("calls = %+v", *calls)
	}
	if !model.isWatched("mt419") || model.isWatched("mt420") {
		t.Errorf("the menu marked the wrong episode")
	}
}

func TestASearchHitCanBeMarkedAsWatched(t *testing.T) {
	model := testModel()
	calls := recordWatched(model, nil)
	model = press(t, model, "f")
	model.searchQuery = "Kino"
	model.searchInput.SetValue("Kino")
	model.searchResults = []orangeflix.Episode{{Identifier: "pk1", Title: "Kino 1"}}
	model = clickOn(t, model, "Kino 1", tea.MouseButtonRight)
	updated, cmd := model.runContextItem(indexOfLabel(t, model, "Als gesehen markieren"))
	model = updated.(*Model)
	model = update(t, model, cmd())
	if len(*calls) != 1 || !model.isWatched("pk1") {
		t.Errorf("calls %+v, watched %v", *calls, model.isWatched("pk1"))
	}
}

func indexOfLabel(t *testing.T, model *Model, label string) int {
	t.Helper()
	for index, item := range model.context.items {
		if item.label == label {
			return index
		}
	}
	t.Fatalf("the menu has no %q", label)
	return -1
}
