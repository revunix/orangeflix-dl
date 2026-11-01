package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// WatchMsg carries the result of the start of the playback.
type WatchMsg struct {
	// Err is the reason, warum es nicht geklappt hat.
	Err error
}

// watchEpisodes plays the marked episodes with mpv. Ohne Markierung wird
// die Folge unter dem Cursor abgespielt. Der Player läuft im Hintergrund,
// die Oberfläche bleibt stehen.
func (m *Model) watchEpisodes() (tea.Model, tea.Cmd) {
	episodes := m.playableEpisodes()
	if len(episodes) == 0 {
		m.message = "Keine Folge ausgewählt."
		m.isError = true
		return m, nil
	}
	if m.watchEpisodesFunc == nil {
		m.message = "Das Abspielen steht in dieser Fassung nicht zur Verfügung."
		m.isError = true
		return m, nil
	}
	m.loading = true
	m.loadingLabel = "Starte den Player …"
	return m, func() tea.Msg {
		err := m.watchEpisodesFunc(m.ctx, episodes)
		return WatchMsg{Err: err}
	}
}

// playableEpisodes returns the episodes that are to be played: the marked
// ones, otherwise the episode under the cursor.
func (m *Model) playableEpisodes() []orangeflix.Episode {
	episodes := m.filteredEpisodes()
	selected := make([]orangeflix.Episode, 0, len(episodes))
	for _, episode := range episodes {
		if m.selected[episode.Identifier] {
			selected = append(selected, episode)
		}
	}
	if len(selected) > 0 {
		return selected
	}
	if len(episodes) == 0 {
		return nil
	}
	index := m.episodeCursor
	if index < 0 || index >= len(episodes) {
		index = 0
	}
	return []orangeflix.Episode{episodes[index]}
}
