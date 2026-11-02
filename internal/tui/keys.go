package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/revunix/orangeflix-dl/internal/app"
	"github.com/revunix/orangeflix-dl/internal/config"
	"github.com/revunix/orangeflix-dl/internal/store"
)

// handleKey processes one key press.
func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		m.quitting = true
		return m, tea.Quit
	}
	if m.showHelp {
		return m.handleHelpKey(msg)
	}
	if m.showQuality {
		return m.handleQualityKey(msg)
	}
	if m.showAction {
		return m.handleActionKey(msg)
	}
	if m.showParticipants {
		return m.handleParticipantKey(msg)
	}
	if m.showPath {
		return m.handlePathKey(msg)
	}
	if m.screen == ScreenLive && m.chatFocused {
		return m.handleChatKey(msg)
	}

	switch m.screen {
	case ScreenSplash:
		if m.credentials {
			m.screen = ScreenFormats
			m.loadLibrary()
		} else {
			m.screen = ScreenStart
		}
		return m, nil
	case ScreenStart:
		return m.handleStartKey(msg)
	case ScreenFormats:
		return m.handleFormatKey(msg)
	case ScreenEpisodes:
		return m.handleEpisodeKey(msg)
	case ScreenQueue:
		return m.handleQueueKey(msg)
	case ScreenSummary:
		return m.handleSummaryKey(msg)
	case ScreenSearch:
		return m.handleSearchKey(msg)
	case ScreenClips:
		return m.handleClipListKey(msg)
	case ScreenCalendar:
		return m.handleCalendarKey(msg)
	case ScreenLive:
		return m.handleLiveKey(msg)
	case ScreenAnnouncements:
		return m.handleAnnouncementKey(msg)
	}
	return m, nil
}

// handleSearchKey processes the keys of the search.
func (m *Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		query := strings.TrimSpace(m.searchInput.Value())
		if query == "" {
			return m, nil
		}
		m.searchQuery = query
		return m, m.runSearch(query)
	case "up", "k":
		if m.searchCursor > 0 {
			m.searchCursor--
		}
	case "down", "j":
		if m.searchCursor < len(m.searchResults)-1 {
			m.searchCursor++
		}
	case "esc":
		m.screen = ScreenFormats
		m.searchInput.Blur()
		return m, nil
	case "?":
		m.showHelp = true
		return m, nil
	case "q":
		m.quitting = true
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

// handleStartKey processes the keys of the start screen.
func (m *Model) handleStartKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "down":
		m.field = (m.field + 1) % startFields
		return m, m.focusField()
	case "shift+tab", "up":
		m.field = (m.field + startFields - 1) % startFields
		return m, m.focusField()
	case "ctrl+t":
		// Die Buchstaben gehören den Feldern: mit h, d und t wäre manche
		// E-Mail-Adresse nicht einzugeben.
		switch m.theme {
		case ThemeLight:
			m.setTheme(ThemeDark)
		case ThemeDark:
			m.setTheme(ThemeAuto)
		default:
			m.setTheme(ThemeLight)
		}
		return m, nil
	case "enter":
		user := strings.TrimSpace(m.userInput.Value())
		password := m.passInput.Value()
		if user == "" || password == "" {
			m.message = "E-Mail und Passwort werden gebraucht."
			m.isError = true
			return m, nil
		}
		m.config.Username = user
		m.config.Password = password
		if m.setCredentials != nil {
			m.setCredentials(user, password)
		}
		// Der Code gilt nur für diese eine Anmeldung.
		if code := strings.TrimSpace(m.totpInput.Value()); code != "" && m.setTotp != nil {
			m.setTotp(code)
		}
		m.totpInput.SetValue("")
		m.credentials = true
		// Der Abo-Stand des alten Kontos gilt nicht mehr.
		m.profile = nil
		m.profileErr = nil
		m.message = ""
		m.isError = false
		m.screen = ScreenFormats
		// Mit den neuen Zugangsdaten wird der Abo-Stand neu abgefragt und
		// der Katalog neu geladen.
		return m, tea.Batch(m.loadLibrary(), m.loadProfile(), m.loadFormats(), m.loadCalendarQuiet())
	case "esc":
		m.quitting = true
		return m, tea.Quit
	}
	var cmd tea.Cmd
	switch m.field {
	case 0:
		m.userInput, cmd = m.userInput.Update(msg)
	case 1:
		m.passInput, cmd = m.passInput.Update(msg)
	case 2:
		m.totpInput, cmd = m.totpInput.Update(msg)
	}
	return m, cmd
}

// startFields is the number of the fields of the start form.
const startFields = 3

// focusField moves the cursor into the active field.
func (m *Model) focusField() tea.Cmd {
	m.userInput.Blur()
	m.passInput.Blur()
	m.totpInput.Blur()
	switch m.field {
	case 0:
		return m.userInput.Focus()
	case 1:
		return m.passInput.Focus()
	case 2:
		return m.totpInput.Focus()
	}
	return nil
}

// handleFormatKey processes the keys of the format list.
func (m *Model) handleFormatKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filtering {
		switch msg.String() {
		case "enter":
			m.filter = m.filterInput.Value()
			m.filtering = false
			m.filterInput.Blur()
			m.formatCursor = 0
			m.formatScroll = 0
			return m, nil
		case "esc":
			m.filtering = false
			m.filter = ""
			m.filterInput.Blur()
			m.filterInput.SetValue("")
			m.formatScroll = 0
			return m, nil
		}
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		return m, cmd
	}

	formats := m.filteredFormats()
	switch msg.String() {
	case "up", "k":
		if m.formatCursor > 0 {
			m.formatCursor--
		}
	case "down", "j":
		if m.formatCursor < len(formats)-1 {
			m.formatCursor++
		}
	case "pgup":
		m.formatCursor -= 10
	case "pgdown":
		m.formatCursor += 10
	case "home":
		m.formatCursor = 0
	case "end":
		m.formatCursor = len(formats) - 1
	case "/":
		m.filtering = true
		return m, m.filterInput.Focus()
	case "f":
		m.screen = ScreenSearch
		m.searchInput.SetValue(m.searchQuery)
		return m, m.searchInput.Focus()
	case "esc":
		if m.filter != "" {
			m.filter = ""
			m.filterInput.SetValue("")
			m.formatCursor = 0
			m.formatScroll = 0
			return m, nil
		}
	case "enter":
		return m.openFormat()
	case "b":
		m.libraryOnly = !m.libraryOnly
		m.formatCursor = 0
		m.formatScroll = 0
		return m, nil
	case "l":
		m.showLogBox = !m.showLogBox
		m.logOffset = 0
		return m, nil
	case "c":
		return m.openRecents()
	case "m":
		return m.openBookmarks()
	case "K":
		return m.openCalendar()
	case "L":
		return m.openLive()
	case "!":
		return m.openAnnouncements()
	case "a":
		return m.addCurrentFormat()
	case "n":
		return m.checkLibrary()
	case "d":
		return m.downloadNewOfCurrentFormat()
	case "s":
		m.screen = ScreenStart
		m.field = 0
		return m, m.focusField()
	case "p":
		return m.openPathPopup()
	case "?":
		m.showHelp = true
		return m, nil
	case "q":
		m.quitting = true
		return m, tea.Quit
	}
	m.clampFormatCursor(len(formats))
	return m, nil
}

// clampFormatCursor keeps the cursor inside the list.
func (m *Model) clampFormatCursor(total int) {
	if total == 0 {
		m.formatCursor = 0
		return
	}
	if m.formatCursor < 0 {
		m.formatCursor = 0
	}
	if m.formatCursor >= total {
		m.formatCursor = total - 1
	}
}

// handleEpisodeKey processes the keys of the episode list.
func (m *Model) handleEpisodeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.yearFiltering {
		switch msg.String() {
		case "enter":
			m.episodeYear = m.yearInput.Value()
			m.yearFiltering = false
			m.yearInput.Blur()
			m.episodeCursor = 0
			m.episodeScroll = 0
			return m, nil
		case "esc":
			m.yearFiltering = false
			m.episodeYear = ""
			m.yearInput.Blur()
			m.yearInput.SetValue("")
			return m, nil
		}
		var cmd tea.Cmd
		m.yearInput, cmd = m.yearInput.Update(msg)
		return m, cmd
	}

	episodes := m.filteredEpisodes()
	switch msg.String() {
	case "up", "k":
		if m.episodeCursor > 0 {
			m.episodeCursor--
		}
	case "down", "j":
		if m.episodeCursor < len(episodes)-1 {
			m.episodeCursor++
		}
	case "pgup":
		m.episodeCursor -= 10
	case "pgdown":
		m.episodeCursor += 10
	case "home":
		m.episodeCursor = 0
	case "end":
		m.episodeCursor = len(episodes) - 1
	case " ":
		if len(episodes) > 0 {
			episode := episodes[m.episodeCursor]
			m.selected[episode.Identifier] = !m.selected[episode.Identifier]
		}
	case "a":
		for _, episode := range episodes {
			m.selected[episode.Identifier] = true
		}
	case "y":
		m.copyEpisodeTitle()
	case "i":
		m.showQuality = true
		m.qualityCursor = m.currentQualityIndex()
		return m, m.loadClipInfo()
	case "J":
		m.yearFiltering = true
		return m, m.yearInput.Focus()
	case "p":
		return m.openPathPopup()
	case "x":
		m.deleteEpisodeFile()
	case "w":
		return m.watchEpisodes()
	case "b":
		return m.toggleBookmark()
	case "r":
		return m.openSimilar()
	case "h":
		return m.openHostChoice()
	case "enter":
		// Ein Klick auf die Folge öffnet das Menü, das fragt, ob sie
		// geladen oder angesehen werden soll.
		m.openActionMenu()
		return m, nil
	case "d":
		return m.startDownload()
	case "esc":
		if m.episodeYear != "" {
			m.episodeYear = ""
			m.yearInput.SetValue("")
			return m, nil
		}
		m.screen = ScreenFormats
		return m, nil
	case "?":
		m.showHelp = true
		return m, nil
	case "q":
		m.quitting = true
		return m, tea.Quit
	}
	if m.episodeCursor < 0 {
		m.episodeCursor = 0
	}
	if len(episodes) > 0 && m.episodeCursor >= len(episodes) {
		m.episodeCursor = len(episodes) - 1
	}
	return m, nil
}

// handleActionKey processes the keys of the menu that asks what should
// happen with the episode.
func (m *Model) handleActionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	entries := actionEntries()
	switch msg.String() {
	case "up", "k":
		if m.actionCursor > 0 {
			m.actionCursor--
		}
	case "down", "j":
		if m.actionCursor < len(entries)-1 {
			m.actionCursor++
		}
	case "enter":
		choice := entries[m.actionCursor]
		m.closeActionMenu()
		switch choice {
		case actionWatch:
			if m.screen == ScreenClips {
				return m.watchCurrentClip()
			}
			return m.watchEpisodes()
		case actionCancel:
			return m, nil
		}
		if m.screen == ScreenClips {
			return m.downloadCurrentClip()
		}
		return m.startDownloadWith(m.actionIdentifiers())
	case "esc":
		m.closeActionMenu()
		return m, nil
	case "q":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// handleQueueKey processes the keys of the queue.
func (m *Model) handleQueueKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "l":
		m.showLog = !m.showLog
	case "y":
		m.copyJobPath()
	case "o":
		m.openJobFolder()
	case "x":
		m.deleteJobFile()
	case "?":
		m.showHelp = true
	case "q", "esc":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// handlePathKey processes the keys of the storage location window.
func (m *Model) handlePathKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closePathPopup()
		return m, nil
	case "enter":
		return m.applyPath()
	case "tab":
		m.enterPathDir()
		return m, nil
	case "shift+tab":
		m.leavePathDir()
		return m, nil
	case "up":
		m.movePathCursor(-1)
		return m, nil
	case "down":
		m.movePathCursor(1)
		return m, nil
	case "pgup":
		m.movePathCursor(-m.pathVisibleRows())
		return m, nil
	case "pgdown":
		m.movePathCursor(m.pathVisibleRows())
		return m, nil
	case "ctrl+v":
		m.pastePath()
		return m, nil
	}
	m.pathError = ""
	before := m.pathInput.Value()
	var cmd tea.Cmd
	m.pathInput, cmd = m.pathInput.Update(msg)
	// Die Liste folgt dem Pfad, sobald sich der Text ändert. Ein
	// Cursorschritt lässt die Markierung stehen.
	if m.pathInput.Value() != before {
		m.loadPathDirs()
	}
	return m, cmd
}

// openPathPopup opens the window for the storage location. The current
// folder is filled in, so a change starts from what is in use.
func (m *Model) openPathPopup() (tea.Model, tea.Cmd) {
	m.showPath = true
	m.pathError = ""
	value := ""
	if m.config != nil {
		value = m.config.DownloadsDir
	}
	m.pathInput.SetValue(value)
	m.pathInput.CursorEnd()
	m.loadPathDirs()
	return m, m.pathInput.Focus()
}

// closePathPopup hides the window and clears its state.
func (m *Model) closePathPopup() {
	m.showPath = false
	m.pathError = ""
	m.pathDirs = nil
	m.pathCursor = 0
	m.pathOffset = 0
	m.pathListNote = ""
	m.pathInput.Blur()
}

// applyPath takes over the folder that is typed into the window.
func (m *Model) applyPath() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.pathInput.Value())
	if value == "" {
		m.pathError = "Bitte einen Ordner angeben."
		return m, nil
	}
	dir := config.ExpandPath(value)
	if absolute, err := filepath.Abs(dir); err == nil {
		dir = absolute
	}
	if err := config.EnsureWritableDir(dir); err != nil {
		m.pathError = "Nicht beschreibbar: " + err.Error()
		return m, nil
	}
	m.useDownloadsDir(dir)
	m.closePathPopup()
	m.message = "Speicherort: " + dir
	m.isError = false
	return m, nil
}

// pastePath inserts the clipboard at the cursor of the storage location
// window. Terminals normally send cmd+v as a bracketed paste, which reaches
// the field as typed input; ctrl+v is read here because the text input's
// paste command would send a message the update loop does not route.
func (m *Model) pastePath() {
	text, err := clipboardRead()
	if err != nil {
		m.pathError = "Zwischenablage: " + err.Error()
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		m.pathError = "Die Zwischenablage ist leer."
		return
	}
	m.pathError = ""
	// The input sanitizes tabs and newlines and keeps the cursor behind the
	// pasted text.
	m.pathInput, _ = m.pathInput.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)})
	m.loadPathDirs()
}

// useDownloadsDir hands the folder to the application and keeps it for the
// next start.
func (m *Model) useDownloadsDir(dir string) {
	if m.config != nil {
		m.config.DownloadsDir = dir
	}
	if m.setDownloadsDir != nil {
		m.setDownloadsDir(dir)
	}
	if m.store != nil {
		_ = m.store.SetSetting(store.SettingDownloadsDir, dir)
	}
}

// handleHelpKey processes the keys of the help window. The arrows move
// through the pages of a long help.
func (m *Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Die Pfeile blättern, wenn die Hilfe nicht auf eine Seite passt; die
	// Hilfe begrenzt den Wert beim Zeichnen.
	switch msg.String() {
	case "up", "k":
		m.helpScroll = max(0, m.helpScroll-1)
	case "down", "j":
		m.helpScroll++
	case "pgup":
		m.helpScroll = max(0, m.helpScroll-10)
	case "pgdown", " ":
		m.helpScroll += 10
	case "esc", "q", "enter", "?":
		m.showHelp = false
		m.helpScroll = 0
	}
	return m, nil
}

// handleQualityKey processes the keys of the quality window.
func (m *Model) handleQualityKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	entries := m.qualityEntries()
	switch msg.String() {
	case "up", "k":
		if m.qualityCursor > 0 {
			m.qualityCursor--
		}
	case "down", "j":
		if m.qualityCursor < len(entries)-1 {
			m.qualityCursor++
		}
	case "enter":
		if len(entries) == 0 {
			m.showQuality = false
			return m, nil
		}
		entry := entries[m.qualityCursor]
		name := qualityName(entry.desc)
		if m.setQuality != nil {
			m.setQuality(name)
		}
		if m.config != nil {
			m.config.Quality = name
		}
		m.message = "Qualität: " + entry.desc
		m.isError = false
		m.showQuality = false
		return m, nil
	case "esc":
		m.showQuality = false
		return m, nil
	case "?":
		m.showHelp = true
		return m, nil
	case "q":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// handleSummaryKey processes the keys of the summary.
func (m *Model) handleSummaryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc", "q":
		m.screen = ScreenFormats
		m.loadLibrary()
	case "?":
		m.showHelp = true
	}
	return m, nil
}

// handleClipListKey processes the keys of the generic clip screen.
func (m *Model) handleClipListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.clipList.cursor > 0 {
			m.clipList.cursor--
		}
	case "down", "j":
		if m.clipList.cursor < len(m.clipList.episodes)-1 {
			m.clipList.cursor++
		}
	case "pgup":
		m.clipList.cursor -= 10
	case "pgdown":
		m.clipList.cursor += 10
	case "home":
		m.clipList.cursor = 0
	case "end":
		m.clipList.cursor = len(m.clipList.episodes) - 1
	case "enter":
		m.openActionMenu()
		return m, nil
	case "w":
		return m.watchCurrentClip()
	case "d":
		return m.downloadCurrentClip()
	case "b":
		return m.toggleBookmark()
	case "i":
		m.showQuality = true
		m.qualityCursor = m.currentQualityIndex()
		return m, m.loadClipInfo()
	case "p":
		return m.openPathPopup()
	case "esc":
		m.screen = m.clipList.backScreen
		if m.screen == ScreenSplash || m.screen == ScreenStart {
			m.screen = ScreenFormats
		}
		return m, nil
	case "?":
		m.showHelp = true
		return m, nil
	case "q":
		m.quitting = true
		return m, tea.Quit
	}
	if m.clipList.cursor < 0 {
		m.clipList.cursor = 0
	}
	if len(m.clipList.episodes) > 0 && m.clipList.cursor >= len(m.clipList.episodes) {
		m.clipList.cursor = len(m.clipList.episodes) - 1
	}
	return m, nil
}

// handleParticipantKey picks a guest of the current episode.
func (m *Model) handleParticipantKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	episode, ok := m.currentEpisode()
	if !ok {
		m.showParticipants = false
		return m, nil
	}
	switch msg.String() {
	case "up", "k":
		if m.participantCursor > 0 {
			m.participantCursor--
		}
	case "down", "j":
		if m.participantCursor < len(episode.Participants)-1 {
			m.participantCursor++
		}
	case "enter":
		if m.participantCursor >= 0 && m.participantCursor < len(episode.Participants) {
			return m.openHost(episode.Participants[m.participantCursor])
		}
	case "esc":
		m.showParticipants = false
		return m, nil
	case "q":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// handleCalendarKey processes the keys of the calendar.
func (m *Model) handleCalendarKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.calendarCursor > 0 {
			m.calendarCursor--
		}
	case "down", "j":
		if m.calendarCursor < len(m.calendar)-1 {
			m.calendarCursor++
		}
	case "pgup":
		m.calendarCursor -= 10
	case "pgdown":
		m.calendarCursor += 10
	case "home":
		m.calendarCursor = 0
	case "end":
		m.calendarCursor = len(m.calendar) - 1
	case "enter":
		if m.calendarCursor >= 0 && m.calendarCursor < len(m.calendar) && m.calendar[m.calendarCursor].Type == "live" {
			m.liveBack = ScreenCalendar
			m.screen = ScreenLive
			return m, m.loadLive()
		}
	case "o":
		if m.calendarCursor >= 0 && m.calendarCursor < len(m.calendar) {
			return m, m.openURL(m.calendar[m.calendarCursor].URL)
		}
	case "y":
		if m.calendarCursor >= 0 && m.calendarCursor < len(m.calendar) {
			m.copyText(m.calendar[m.calendarCursor].Title)
		}
	case "esc":
		m.screen = ScreenFormats
		return m, nil
	case "?":
		m.showHelp = true
		return m, nil
	case "q":
		m.quitting = true
		return m, tea.Quit
	}
	if m.calendarCursor < 0 {
		m.calendarCursor = 0
	}
	if len(m.calendar) > 0 && m.calendarCursor >= len(m.calendar) {
		m.calendarCursor = len(m.calendar) - 1
	}
	return m, nil
}

// handleLiveKey processes the keys of the live screen.
func (m *Model) handleLiveKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "w":
		if m.live != nil && m.live.IsLive && m.live.HasAccess() {
			return m.playLive("live")
		}
		if m.live != nil && m.live.Replay != nil && m.live.Replay.Possible && m.live.Replay.CanAccess {
			return m.playLive("replay")
		}
		m.message = "Zurzeit läuft keine Sendung, die abgespielt werden kann."
		m.isError = true
	case "r":
		if m.live != nil && m.live.Replay != nil && m.live.Replay.Possible && m.live.Replay.CanAccess {
			return m.playLive("replay")
		}
		m.message = "Keine Wiederholung verfügbar."
		m.isError = true
	case "c":
		if !m.chatConnected {
			m.chatStatus = "Der Chat ist nicht verbunden."
			return m, nil
		}
		m.chatFocused = true
		return m, m.chatInput.Focus()
	case "up", "k":
		if m.chatScroll < len(m.chatMessages) {
			m.chatScroll++
		}
	case "down", "j":
		if m.chatScroll > 0 {
			m.chatScroll--
		}
	case "R":
		return m, m.loadLive()
	case "esc":
		m.closeChat()
		m.screen = m.liveBack
		if m.screen == ScreenSplash || m.screen == ScreenStart {
			m.screen = ScreenFormats
		}
		return m, nil
	case "?":
		m.showHelp = true
		return m, nil
	case "q":
		m.closeChat()
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// handleChatKey processes the keys while the chat input has the focus.
func (m *Model) handleChatKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		return m.sendChat()
	case "esc":
		m.chatFocused = false
		m.chatInput.Blur()
		return m, nil
	case "up", "pgup":
		if m.chatScroll < len(m.chatMessages) {
			m.chatScroll++
		}
		return m, nil
	case "down", "pgdown":
		if m.chatScroll > 0 {
			m.chatScroll--
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.chatInput, cmd = m.chatInput.Update(msg)
	return m, cmd
}

// handleAnnouncementKey processes the keys of the announcements.
func (m *Model) handleAnnouncementKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.announceCursor > 0 {
			m.announceCursor--
		}
	case "down", "j":
		if m.announceCursor < len(m.announcements)-1 {
			m.announceCursor++
		}
	case "o":
		if m.announceCursor >= 0 && m.announceCursor < len(m.announcements) {
			return m, m.openURL(m.announcements[m.announceCursor].URL)
		}
	case "esc":
		m.screen = ScreenFormats
		return m, nil
	case "?":
		m.showHelp = true
		return m, nil
	case "q":
		m.quitting = true
		return m, tea.Quit
	}
	if m.announceCursor < 0 {
		m.announceCursor = 0
	}
	if len(m.announcements) > 0 && m.announceCursor >= len(m.announcements) {
		m.announceCursor = len(m.announcements) - 1
	}
	return m, nil
}

// openURL opens a link in the browser.
func (m *Model) openURL(raw string) tea.Cmd {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		m.message = "Kein Link vorhanden."
		m.isError = true
		return nil
	}
	if strings.HasPrefix(raw, "/") {
		raw = "https://orangeflix.de" + raw
	}
	return func() tea.Msg {
		if err := openURL(raw); err != nil {
			return EventMsg{Event: app.Event{Kind: app.EventLog, Message: "Der Link konnte nicht geöffnet werden: " + err.Error()}}
		}
		return EventMsg{Event: app.Event{Kind: app.EventLog, Message: "Link geöffnet."}}
	}
}

// copyText puts a text into the clipboard.
func (m *Model) copyText(text string) {
	if err := clipboardWrite(text); err != nil {
		m.message = "Der Text konnte nicht kopiert werden: " + err.Error()
		m.isError = true
		return
	}
	m.message = "Kopiert."
	m.isError = false
}

// openURL starts the browser of the system.
func openURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return errors.New("no URL")
	}
	var command string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		command = "open"
		args = []string{raw}
	case "windows":
		command = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", raw}
	default:
		command = "xdg-open"
		args = []string{raw}
	}
	return exec.Command(command, args...).Start()
}

// copyEpisodeTitle copies the title of the marked episode into the clipboard.
func (m *Model) copyEpisodeTitle() {
	episodes := m.filteredEpisodes()
	if len(episodes) == 0 {
		return
	}
	episode := episodes[m.episodeCursor]
	if err := clipboardWrite(episode.Title); err != nil {
		m.message = "Der Titel konnte nicht kopiert werden: " + err.Error()
		m.isError = true
		return
	}
	m.message = "Der Titel wurde kopiert."
	m.isError = false
}

// copyJobPath copies the path of the last file into the clipboard.
func (m *Model) copyJobPath() {
	job := m.currentJob()
	if job == nil {
		return
	}
	if err := clipboardWrite(job.Path); err != nil {
		m.message = "Der Pfad konnte nicht kopiert werden: " + err.Error()
		m.isError = true
		return
	}
	m.message = "Der Pfad wurde kopiert."
	m.isError = false
}

// openJobFolder opens the folder of the last file in the file manager.
func (m *Model) openJobFolder() {
	job := m.currentJob()
	if job == nil || job.Path == "" {
		return
	}
	if err := openFolder(job.Path); err != nil {
		m.message = "Der Ordner konnte nicht geöffnet werden: " + err.Error()
		m.isError = true
		return
	}
	m.message = "Der Ordner wurde geöffnet."
	m.isError = false
}

// deleteJobFile removes the file of the last download and offers the episode
// again.
func (m *Model) deleteJobFile() {
	job := m.currentJob()
	if job == nil || job.Path == "" {
		return
	}
	if err := os.Remove(job.Path); err != nil && !os.IsNotExist(err) {
		m.message = "Die Datei konnte nicht gelöscht werden: " + err.Error()
		m.isError = true
		return
	}
	m.forgetEpisode(job.PID, job.Episode)
	m.message = "Die Datei wurde gelöscht, die Folge wird wieder angeboten."
	m.isError = false
}

// deleteEpisodeFile removes the file of the marked episode.
func (m *Model) deleteEpisodeFile() {
	episodes := m.filteredEpisodes()
	if len(episodes) == 0 || m.store == nil {
		return
	}
	episode := episodes[m.episodeCursor]
	known, err := m.store.Episodes(episode.PID)
	if err != nil {
		return
	}
	for _, entry := range known {
		if entry.Identifier != episode.Identifier || entry.Path == "" {
			continue
		}
		if err := os.Remove(entry.Path); err != nil && !os.IsNotExist(err) {
			m.message = "Die Datei konnte nicht gelöscht werden: " + err.Error()
			m.isError = true
			return
		}
		_ = m.store.ClearDownloaded(episode.PID, episode.Identifier)
		m.message = "Die Datei wurde gelöscht, die Folge wird wieder angeboten."
		m.isError = false
		return
	}
	m.message = "Die Folge wurde noch nicht geladen."
	m.isError = false
}

// forgetEpisode removes the download mark of an episode.
func (m *Model) forgetEpisode(pid int, identifier string) {
	if m.store == nil || identifier == "" {
		return
	}
	_ = m.store.ClearDownloaded(pid, identifier)
}

// currentJob returns the job of the running or the last download.
func (m *Model) currentJob() *job {
	if len(m.jobs) == 0 {
		return nil
	}
	return &m.jobs[len(m.jobs)-1]
}

// clipboardWrite puts a text into the clipboard.
func clipboardWrite(text string) error {
	return clipboard.WriteAll(text)
}

// clipboardRead reads the clipboard. The tests replace it because they must
// not depend on the clipboard of the machine.
var clipboardRead = clipboard.ReadAll

// openFolder opens the folder of a file in the file manager of the system.
func openFolder(path string) error {
	folder := folderOf(path)
	var command string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		command = "open"
		args = []string{folder}
	case "windows":
		command = "explorer"
		args = []string{folder}
	default:
		command = "xdg-open"
		args = []string{folder}
	}
	if _, err := os.Stat(folder); err != nil {
		return fmt.Errorf("der Ordner %s fehlt", folder)
	}
	return exec.Command(command, args...).Start()
}

// folderOf returns the folder of a file.
func folderOf(path string) string {
	index := strings.LastIndexByte(path, '/')
	if index <= 0 {
		return "."
	}
	return path[:index]
}
