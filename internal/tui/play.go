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
	return m.watchList(m.playableEpisodes())
}

// watchList plays the given episodes.
func (m *Model) watchList(episodes []orangeflix.Episode) (tea.Model, tea.Cmd) {
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

// screenEpisodes returns the visible episodes of the current screen.
func (m *Model) screenEpisodes() []orangeflix.Episode {
	switch m.screen {
	case ScreenEpisodes:
		return m.filteredEpisodes()
	case ScreenClips:
		return m.clipList.episodes
	case ScreenSearch:
		return m.searchResults
	}
	return nil
}

// currentEpisode returns the episode under the cursor of the current list.
func (m *Model) currentEpisode() (orangeflix.Episode, bool) {
	episodes := m.screenEpisodes()
	if len(episodes) == 0 {
		return orangeflix.Episode{}, false
	}
	index := 0
	switch m.screen {
	case ScreenEpisodes:
		index = m.episodeCursor
	case ScreenClips:
		index = m.clipList.cursor
	case ScreenSearch:
		index = m.searchCursor
	}
	if index < 0 || index >= len(episodes) {
		index = 0
	}
	return episodes[index], true
}

// detailEpisode returns the episode shown in the detail pane.
func (m *Model) detailEpisode() (orangeflix.Episode, bool) {
	if m.screen != ScreenEpisodes && m.screen != ScreenClips {
		return orangeflix.Episode{}, false
	}
	return m.currentEpisode()
}

// watchCurrentClip plays the clip under the cursor.
func (m *Model) watchCurrentClip() (tea.Model, tea.Cmd) {
	episode, ok := m.currentEpisode()
	if !ok {
		m.message = "Keine Folge ausgewählt."
		m.isError = true
		return m, nil
	}
	return m.watchList([]orangeflix.Episode{episode})
}

// playableEpisodes returns the episodes that are to be played: the marked
// ones, otherwise the episode under the cursor.
func (m *Model) playableEpisodes() []orangeflix.Episode {
	episodes := m.screenEpisodes()
	if m.screen == ScreenClips || m.screen == ScreenSearch {
		if episode, ok := m.currentEpisode(); ok {
			return []orangeflix.Episode{episode}
		}
		return nil
	}
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
