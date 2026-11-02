package tui

import (
	"context"
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/revunix/orangeflix-dl/internal/app"
	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// LatestMsg carries the newest episode of a format and what should happen
// with it.
type LatestMsg struct {
	Format  orangeflix.Format
	Episode orangeflix.Episode
	Watch   bool
	Err     error
}

// latestOfCurrentFormat plays or loads the newest episode of the marked
// format, without opening the format. Only the first page of the feed is
// read.
func (m *Model) latestOfCurrentFormat(watch bool) (tea.Model, tea.Cmd) {
	formats := m.filteredFormats()
	if len(formats) == 0 || m.formatCursor >= len(formats) {
		return m, nil
	}
	format := formats[m.formatCursor]
	fetch := m.latestEpisodeFunc
	if fetch == nil && m.loadEpisodes != nil {
		// Ohne die schnelle Abfrage wird die ganze Liste geladen.
		fetch = func(ctx context.Context, pid int) (orangeflix.Episode, error) {
			episodes, err := m.loadEpisodes(ctx, pid)
			if err != nil {
				return orangeflix.Episode{}, err
			}
			newest, ok := app.NewestEpisode(episodes)
			if !ok {
				return orangeflix.Episode{}, app.ErrNoEpisode
			}
			return newest, nil
		}
	}
	if fetch == nil {
		m.message = "Die Folgen stehen nicht zur Verfügung."
		m.isError = true
		return m, nil
	}
	m.loading = true
	m.loadingLabel = "Suche die neueste Folge von " + format.Title + " …"
	return m, func() tea.Msg {
		episode, err := fetch(m.ctx, format.PID)
		return LatestMsg{Format: format, Episode: episode, Watch: watch, Err: err}
	}
}

// handleLatest starts the player or the download for the newest episode.
func (m *Model) handleLatest(message LatestMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if message.Err != nil {
		m.isError = true
		if errors.Is(message.Err, app.ErrNoEpisode) {
			m.message = message.Format.Title + " hat noch keine Folge."
		} else {
			m.message = message.Err.Error()
		}
		return m, nil
	}
	episode := message.Episode
	if episode.PID == 0 {
		episode.PID = message.Format.PID
	}
	if episode.FormatName == "" {
		episode.FormatName = message.Format.Title
	}
	m.message = ""
	m.isError = false
	if message.Watch {
		model, cmd := m.watchList([]orangeflix.Episode{episode})
		if !m.isError {
			m.message = "Starte " + episodeTitle(episode) + " …"
		}
		return model, cmd
	}
	return m.downloadSingle(episode)
}
