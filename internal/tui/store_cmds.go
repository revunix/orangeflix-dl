package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/revunix/orangeflix-dl/internal/store"
)

// loadLibrary reads the stored formats.
func (m *Model) loadLibrary() tea.Cmd {
	if m.store == nil {
		return nil
	}
	return func() tea.Msg {
		entries, err := m.store.Formats()
		if err != nil {
			return LibraryMsg{}
		}
		return LibraryMsg{Entries: entries}
	}
}

// loadFormats reads the catalog of the formats. The live catalog of the API
// is preferred, the catalog of the program is the fallback.
func (m *Model) loadFormats() tea.Cmd {
	if m.loadFormatsCatalog == nil {
		return nil
	}
	return func() tea.Msg {
		formats, err := m.loadFormatsCatalog(m.ctx)
		return FormatsMsg{Formats: formats, Err: err}
	}
}

// loadProfile reads the state of the subscription.
func (m *Model) loadProfile() tea.Cmd {
	if m.loadProfileState == nil {
		return nil
	}
	m.profileLoading = true
	return func() tea.Msg {
		profile, err := m.loadProfileState(m.ctx)
		return ProfileMsg{Profile: profile, Err: err}
	}
}

// runSearch looks for clips in the whole catalog.
func (m *Model) runSearch(query string) tea.Cmd {
	if m.runSearchCatalog == nil {
		return nil
	}
	m.loading = true
	m.loadingLabel = "Suche nach " + query + " ..."
	return func() tea.Msg {
		episodes, err := m.runSearchCatalog(m.ctx, query, 50)
		return SearchMsg{Query: query, Episodes: episodes, Err: err}
	}
}

// openFormat loads the episodes of the marked format.
func (m *Model) openFormat() (tea.Model, tea.Cmd) {
	formats := m.filteredFormats()
	if len(formats) == 0 {
		return m, nil
	}
	format := formats[m.formatCursor]
	m.loading = true
	m.loadingLabel = "Lade " + format.Title + " ..."
	m.message = ""
	m.isError = false
	m.episodeYear = ""
	m.episodeScroll = 0
	m.known = map[string]store.KnownEpisode{}
	pid := format.PID
	return m, tea.Batch(m.spinner.Tick, m.loadKnown(pid), func() tea.Msg {
		if m.loadEpisodes == nil {
			return EpisodesMsg{Format: format}
		}
		episodes, err := m.loadEpisodes(m.ctx, pid)
		return EpisodesMsg{Format: format, Episodes: episodes, Err: err}
	})
}

// addCurrentFormat stores the marked format in the library.
func (m *Model) addCurrentFormat() (tea.Model, tea.Cmd) {
	formats := m.filteredFormats()
	if len(formats) == 0 {
		return m, nil
	}
	format := formats[m.formatCursor]
	if m.addFormat == nil {
		return m, nil
	}
	return m, func() tea.Msg {
		if _, err := m.addFormat(m.ctx, format.Slug); err != nil {
			return CheckDoneMsg{Err: err}
		}
		return LibraryMsg{Entries: m.libraryEntries()}
	}
}

// libraryEntries reads the stored formats.
func (m *Model) libraryEntries() []store.FormatEntry {
	if m.store == nil {
		return nil
	}
	entries, err := m.store.Formats()
	if err != nil {
		return nil
	}
	return entries
}

// checkLibrary looks for new episodes in the stored formats.
func (m *Model) checkLibrary() (tea.Model, tea.Cmd) {
	if m.check == nil {
		return m, nil
	}
	m.message = "Suche nach neuen Folgen ..."
	m.isError = false
	return m, func() tea.Msg {
		results, err := m.check(m.ctx)
		return CheckDoneMsg{Results: results, Err: err}
	}
}

// downloadNewOfCurrentFormat loads the new episodes of the marked format.
func (m *Model) downloadNewOfCurrentFormat() (tea.Model, tea.Cmd) {
	formats := m.filteredFormats()
	if len(formats) == 0 {
		return m, nil
	}
	format := formats[m.formatCursor]
	entry, ok := m.libraryEntry(format.PID)
	if !ok {
		m.message = "Das Format ist nicht in der Bibliothek. Mit 'a' merken."
		m.isError = true
		return m, nil
	}
	if m.downloadNew == nil {
		return m, nil
	}
	m.message = "Lade die neuen Folgen von " + format.Title + " ..."
	m.isError = false
	m.jobs = nil
	m.summary = summary{}
	m.screen = ScreenQueue
	return m, func() tea.Msg {
		err := m.downloadNew(m.ctx, entry)
		return RunDoneMsg{Err: err}
	}
}

// selectedIdentifiers returns the episodes that are marked. Without a
// marking every episode is downloaded.
func (m *Model) selectedIdentifiers() []string {
	episodes := m.filteredEpisodes()
	var identifiers []string
	for _, episode := range episodes {
		if m.selected[episode.Identifier] {
			identifiers = append(identifiers, episode.Identifier)
		}
	}
	if len(identifiers) == 0 {
		for _, episode := range episodes {
			identifiers = append(identifiers, episode.Identifier)
		}
	}
	return identifiers
}

// startDownload starts the download of the selected episodes.
func (m *Model) startDownload() (tea.Model, tea.Cmd) {
	return m.startDownloadWith(m.selectedIdentifiers())
}

// startDownloadWith starts the download of the given episodes.
func (m *Model) startDownloadWith(identifiers []string) (tea.Model, tea.Cmd) {
	if m.currentFormat == nil {
		return m, nil
	}
	if len(identifiers) == 0 {
		return m, nil
	}
	if m.setSelection != nil {
		m.setSelection(identifiers)
	}
	if m.config != nil {
		m.config.Format = m.currentFormat.Slug
		m.config.Episode = ""
		m.config.EpisodeRange = ""
		m.config.Year = ""
		m.config.All = false
	}
	m.jobs = nil
	m.summary = summary{}
	m.message = ""
	m.isError = false
	m.screen = ScreenQueue
	if m.run == nil {
		return m, nil
	}
	return m, func() tea.Msg {
		err := m.run(m.ctx)
		return RunDoneMsg{Err: err}
	}
}
