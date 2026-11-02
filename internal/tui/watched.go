package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// WatchedMsg carries the answer to marking episodes as watched or not.
type WatchedMsg struct {
	// Done are the episodes the account took over.
	Done    []string
	Watched bool
	// Previous holds the share of every episode before the change, so a
	// failure can put it back.
	Previous map[string]int
	Err      error
}

// isWatched tells whether an episode counts as watched.
func (m *Model) isWatched(identifier string) bool {
	return m.episodeProgress(identifier) >= watchedPercent
}

// watchedTargets returns the episodes the key works on: the marked ones in
// the episode list, otherwise the one under the cursor.
func (m *Model) watchedTargets() []orangeflix.Episode {
	if m.screen == ScreenEpisodes && m.markedCount() > 0 {
		marked := []orangeflix.Episode{}
		for _, episode := range m.filteredEpisodes() {
			if m.selected[episode.Identifier] {
				marked = append(marked, episode)
			}
		}
		return marked
	}
	if episode, ok := m.currentEpisode(); ok {
		return []orangeflix.Episode{episode}
	}
	return nil
}

// toggleWatched marks the episodes as watched; when all of them are
// watched already, it takes the mark away.
func (m *Model) toggleWatched() (tea.Model, tea.Cmd) {
	targets := m.watchedTargets()
	watched := false
	for _, episode := range targets {
		if !m.isWatched(episode.Identifier) {
			watched = true
			break
		}
	}
	return m.setWatched(targets, watched)
}

// setWatched changes the watch state on the account. The list shows the
// new state at once and goes back to the old one when the request fails.
func (m *Model) setWatched(targets []orangeflix.Episode, watched bool) (tea.Model, tea.Cmd) {
	if len(targets) == 0 {
		m.message = "Keine Folge ausgewählt."
		m.isError = true
		return m, nil
	}
	if m.setWatchedFunc == nil {
		m.message = "Der Stand des Kontos steht nicht zur Verfügung."
		m.isError = true
		return m, nil
	}
	percent := 0
	if watched {
		percent = 100
	}
	previous := map[string]int{}
	for _, episode := range targets {
		previous[episode.Identifier] = m.episodeProgress(episode.Identifier)
		m.setProgressPercent(episode.Identifier, percent)
	}
	set := m.setWatchedFunc
	ctx := m.ctx
	return m, func() tea.Msg {
		done := []string{}
		for _, episode := range targets {
			if err := set(ctx, episode, watched); err != nil {
				return WatchedMsg{Done: done, Watched: watched, Previous: previous, Err: err}
			}
			done = append(done, episode.Identifier)
		}
		return WatchedMsg{Done: done, Watched: watched, Previous: previous}
	}
}

// handleWatched takes over the answer of the account.
func (m *Model) handleWatched(message WatchedMsg) (tea.Model, tea.Cmd) {
	if message.Err != nil {
		done := map[string]bool{}
		for _, identifier := range message.Done {
			done[identifier] = true
		}
		for identifier, percent := range message.Previous {
			if !done[identifier] {
				m.setProgressPercent(identifier, percent)
			}
		}
		m.message = "Der Stand konnte nicht gespeichert werden: " + message.Err.Error()
		m.isError = true
		return m, nil
	}
	state := "gesehen"
	if !message.Watched {
		state = "ungesehen"
	}
	if len(message.Done) == 1 {
		m.message = "Als " + state + " markiert."
	} else {
		m.message = fmt.Sprintf("%d Folgen als %s markiert.", len(message.Done), state)
	}
	m.isError = false
	return m, nil
}

// setProgressPercent writes the watched share into every list that shows
// the episode.
func (m *Model) setProgressPercent(identifier string, percent int) {
	for index := range m.episodes {
		if m.episodes[index].Identifier == identifier {
			m.episodes[index].ProgressPercent = percent
		}
	}
	for index := range m.clipList.episodes {
		if m.clipList.episodes[index].Identifier == identifier {
			m.clipList.episodes[index].ProgressPercent = percent
		}
	}
	for index := range m.searchResults {
		if m.searchResults[index].Identifier == identifier {
			m.searchResults[index].ProgressPercent = percent
		}
	}
}

// watchedItem is the menu entry for one episode.
func (m *Model) watchedItem(episode orangeflix.Episode) menuItem {
	if m.isWatched(episode.Identifier) {
		return menuItem{label: "Als ungesehen markieren", key: "g", run: func() (tea.Model, tea.Cmd) {
			return m.setWatched([]orangeflix.Episode{episode}, false)
		}}
	}
	return menuItem{label: "Als gesehen markieren", key: "g", run: func() (tea.Model, tea.Cmd) {
		return m.setWatched([]orangeflix.Episode{episode}, true)
	}}
}

// watchedHint is the footer hint of the key.
func (m *Model) watchedHint() keyHint {
	targets := m.watchedTargets()
	all := len(targets) > 0
	for _, episode := range targets {
		if !m.isWatched(episode.Identifier) {
			all = false
			break
		}
	}
	state := "gesehen"
	if all {
		state = "ungesehen"
	}
	label, long := state, "Als "+state+" markieren"
	if len(targets) > 1 {
		label = fmt.Sprintf("%d %s", len(targets), state)
		long = fmt.Sprintf("%d markierte als %s markieren", len(targets), state)
	}
	return hintLong("g", label, long, prioExtra)
}
