package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// loadRecents loads the recently watched clips.
func (m *Model) loadRecents() tea.Cmd {
	if m.recentsFunc == nil {
		return nil
	}
	m.loading = true
	m.loadingLabel = "Lade die zuletzt gesehenen Folgen …"
	return func() tea.Msg {
		episodes, err := m.recentsFunc(m.ctx, 50)
		return RecentsMsg{Episodes: episodes, Err: err}
	}
}

// loadBookmarks loads the bookmarked clips.
func (m *Model) loadBookmarks() tea.Cmd {
	if m.bookmarksFunc == nil {
		return nil
	}
	m.loading = true
	m.loadingLabel = "Lade die Merkliste …"
	return func() tea.Msg {
		episodes, err := m.bookmarksFunc(m.ctx, 50)
		return BookmarksMsg{Episodes: episodes, Err: err}
	}
}

// loadCalendar loads the upcoming entries.
func (m *Model) loadCalendar() tea.Cmd {
	return m.loadCalendarInto(false)
}

// loadCalendarQuiet loads the calendar for the header without opening it
// and without a spinner.
func (m *Model) loadCalendarQuiet() tea.Cmd {
	return m.loadCalendarInto(true)
}

// loadCalendarInto loads the upcoming entries. Quiet keeps the current screen.
func (m *Model) loadCalendarInto(quiet bool) tea.Cmd {
	if m.calendarFunc == nil {
		return nil
	}
	if !quiet {
		m.loading = true
		m.loadingLabel = "Lade den Kalender …"
	}
	limit := 100
	if quiet {
		limit = 20
	}
	return func() tea.Msg {
		entries, err := m.calendarFunc(m.ctx, limit)
		return CalendarMsg{Entries: entries, Err: err, Background: quiet}
	}
}

// loadLive loads the live state.
func (m *Model) loadLive() tea.Cmd {
	if m.liveFunc == nil {
		return nil
	}
	m.loading = true
	m.liveLoading = true
	m.loadingLabel = "Lade den Live-Stand …"
	return func() tea.Msg {
		live, err := m.liveFunc(m.ctx)
		return LiveMsg{Live: live, Err: err}
	}
}

// loadSimilar loads the clips related to an episode.
func (m *Model) loadSimilar(episode orangeflix.Episode) tea.Cmd {
	if m.similarFunc == nil {
		return nil
	}
	m.loading = true
	m.loadingLabel = "Suche ähnliche Folgen …"
	return func() tea.Msg {
		episodes, err := m.similarFunc(m.ctx, episode.Identifier, 10)
		return SimilarMsg{Source: episode.Title, Episodes: episodes, Err: err}
	}
}

// loadHost loads the page of a guest.
func (m *Model) loadHost(identifier string) tea.Cmd {
	if m.hostFunc == nil {
		return nil
	}
	m.loading = true
	m.loadingLabel = "Lade den Gast …"
	return func() tea.Msg {
		host, err := m.hostFunc(m.ctx, identifier)
		return HostMsg{Host: host, Err: err}
	}
}

// loadAnnouncements loads the active announcements.
func (m *Model) loadAnnouncements() tea.Cmd {
	if m.announcementsFunc == nil {
		return nil
	}
	return func() tea.Msg {
		items, err := m.announcementsFunc(m.ctx, "overview", 10)
		return AnnouncementsMsg{Items: items, Err: err}
	}
}

// openRecents opens the recently watched clips.
func (m *Model) openRecents() (tea.Model, tea.Cmd) {
	m.clipList = clipList{kind: clipListRecents, title: "Weiterschauen", backScreen: ScreenFormats}
	m.screen = ScreenClips
	return m, m.loadRecents()
}

// openBookmarks opens the bookmark list.
func (m *Model) openBookmarks() (tea.Model, tea.Cmd) {
	m.clipList = clipList{kind: clipListBookmarks, title: "Merkliste", backScreen: ScreenFormats}
	m.screen = ScreenClips
	return m, m.loadBookmarks()
}

// openCalendar opens the calendar.
func (m *Model) openCalendar() (tea.Model, tea.Cmd) {
	m.screen = ScreenCalendar
	return m, m.loadCalendar()
}

// openLive opens the live screen.
func (m *Model) openLive() (tea.Model, tea.Cmd) {
	m.closeChat()
	m.liveBack = ScreenFormats
	m.screen = ScreenLive
	return m, m.loadLive()
}

// openAnnouncements opens the announcements.
func (m *Model) openAnnouncements() (tea.Model, tea.Cmd) {
	m.announcements = nil
	m.loading = true
	m.loadingLabel = "Lade Ankündigungen …"
	m.screen = ScreenAnnouncements
	return m, m.loadAnnouncements()
}

// openSimilar opens the similar clips of the current episode.
func (m *Model) openSimilar() (tea.Model, tea.Cmd) {
	episode, ok := m.currentEpisode()
	if !ok {
		m.message = "Keine Folge ausgewählt."
		m.isError = true
		return m, nil
	}
	m.clipList = clipList{
		kind:       clipListSimilar,
		title:      "Ähnliche Folgen",
		source:     episode.Title,
		backScreen: m.screen,
	}
	m.screen = ScreenClips
	return m, m.loadSimilar(episode)
}

// openHostChoice picks a guest of the current episode.
func (m *Model) openHostChoice() (tea.Model, tea.Cmd) {
	episode, ok := m.currentEpisode()
	if !ok || len(episode.Participants) == 0 {
		m.message = "Keine Gäste bekannt."
		m.isError = true
		return m, nil
	}
	if len(episode.Participants) == 1 {
		return m.openHost(episode.Participants[0])
	}
	m.showParticipants = true
	m.participantCursor = 0
	return m, nil
}

// openHost opens the page of a guest.
func (m *Model) openHost(participant orangeflix.Participant) (tea.Model, tea.Cmd) {
	m.clipList = clipList{
		kind:       clipListHost,
		title:      "Gast: " + participant.Name,
		source:     participant.ID,
		backScreen: m.screen,
	}
	m.screen = ScreenClips
	m.showParticipants = false
	return m, m.loadHost(participant.ID)
}

// toggleBookmark adds or removes the current clip from the list.
func (m *Model) toggleBookmark() (tea.Model, tea.Cmd) {
	episode, ok := m.currentEpisode()
	if !ok {
		return m, nil
	}
	if m.setBookmarkFunc == nil {
		m.message = "Die Merkliste steht nicht zur Verfügung."
		m.isError = true
		return m, nil
	}
	on := !episode.Bookmarked
	m.setEpisodeBookmarked(episode.Identifier, on)
	return m, func() tea.Msg {
		err := m.setBookmarkFunc(m.ctx, episode.Identifier, on)
		return BookmarkMsg{Identifier: episode.Identifier, On: on, Err: err}
	}
}

// setEpisodeBookmarked updates every visible copy of the episode.
func (m *Model) setEpisodeBookmarked(identifier string, on bool) {
	for index := range m.episodes {
		if m.episodes[index].Identifier == identifier {
			m.episodes[index].Bookmarked = on
		}
	}
	for index := range m.clipList.episodes {
		if m.clipList.episodes[index].Identifier == identifier {
			m.clipList.episodes[index].Bookmarked = on
		}
	}
	for index := range m.searchResults {
		if m.searchResults[index].Identifier == identifier {
			m.searchResults[index].Bookmarked = on
		}
	}
}

// removeClip removes one clip from the visible list.
func (m *Model) removeClip(identifier string) {
	remaining := m.clipList.episodes[:0]
	for _, episode := range m.clipList.episodes {
		if episode.Identifier != identifier {
			remaining = append(remaining, episode)
		}
	}
	m.clipList.episodes = remaining
	if m.clipList.cursor >= len(m.clipList.episodes) {
		m.clipList.cursor = max(0, len(m.clipList.episodes)-1)
	}
	m.clipList.scroll = 0
}

// downloadCurrentClip loads the clip under the cursor.
func (m *Model) downloadCurrentClip() (tea.Model, tea.Cmd) {
	episode, ok := m.currentEpisode()
	if !ok {
		m.message = "Keine Folge ausgewählt."
		m.isError = true
		return m, nil
	}
	return m.downloadSingle(episode)
}

// downloadSingle loads one episode and shows the download list.
func (m *Model) downloadSingle(episode orangeflix.Episode) (tea.Model, tea.Cmd) {
	if m.downloadEpisodeFunc == nil {
		m.message = "Der Download steht nicht zur Verfügung."
		m.isError = true
		return m, nil
	}
	m.jobs = nil
	m.summary = summary{}
	m.message = ""
	m.isError = false
	m.screen = ScreenQueue
	return m, func() tea.Msg {
		return RunDoneMsg{Err: m.downloadEpisodeFunc(m.ctx, episode)}
	}
}

// playLive starts a live stream or replay.
func (m *Model) playLive(mode string) (tea.Model, tea.Cmd) {
	if m.live == nil || m.playLiveFunc == nil {
		m.message = "Die Wiedergabe steht nicht zur Verfügung."
		m.isError = true
		return m, nil
	}
	if mode == "replay" {
		// The live socket would mix with the transcript of the replay.
		m.closeChat()
		m.chatStatus = "Chat-Mitschnitt wird geladen …"
	}
	live := m.live
	m.loading = true
	m.loadingLabel = "Starte Wiedergabe …"
	return m, func() tea.Msg {
		return PlayLiveMsg{Err: m.playLiveFunc(m.ctx, live, mode)}
	}
}
