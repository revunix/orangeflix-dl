package tui

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

// hints returns the keys of the current screen. They follow the state: a
// key that does nothing right now is not offered, a label says what the key
// does with the current choice. The footer shows as many as fit, the
// palette and the mouse use the same list.
func (m *Model) hints() []keyHint {
	// Ein offenes Fenster zeigt seine eigenen Tasten.
	switch {
	case m.confirm != nil:
		return []keyHint{
			hint("y", m.confirm.yes, prioAlways),
			hintLong("n", "abbrechen", "Abbrechen", prioAlways),
		}
	case m.context != nil:
		return []keyHint{
			hintShow("↑↓", "wählen", prioMain),
			hint("enter", "ausführen", prioAlways),
			hint("esc", "schließen", prioAlways),
		}
	case m.showPalette:
		return []keyHint{
			hintShow("↑↓", "wählen", prioMain),
			hint("enter", "ausführen", prioAlways),
			hint("esc", "schließen", prioAlways),
		}
	case m.showHelp:
		return []keyHint{hintShow("↑↓", "blättern", prioMain), hint("esc", "schließen", prioAlways)}
	case m.showQuality:
		return []keyHint{hintShow("↑↓", "wählen", prioMain), hint("enter", "übernehmen", prioAlways), hint("esc", "abbrechen", prioAlways)}
	case m.showAction:
		return []keyHint{
			hintShow("↑↓", "wählen", prioUseful), hint("enter", "bestätigen", prioAlways),
			hint("d", "laden", prioMain), hint("w", "ansehen", prioMain), hint("esc", "abbrechen", prioAlways),
		}
	case m.showParticipants:
		return []keyHint{hintShow("↑↓", "wählen", prioMain), hint("enter", "öffnen", prioAlways), hint("esc", "abbrechen", prioAlways)}
	case m.showPath:
		return []keyHint{
			hintShow("↑↓", "wählen", prioUseful), hint("tab", "öffnen", prioMain), hint("shift+tab", "hoch", prioUseful),
			hint("ctrl+v", "einfügen", prioExtra), hint("enter", "übernehmen", prioAlways), hint("esc", "abbrechen", prioAlways),
		}
	case m.screen == ScreenLive && m.chatFocused && m.chatReplyTo != nil:
		return []keyHint{hint("enter", "antworten", prioAlways), hintLong("esc", "Antwort abbrechen", "Antwort abbrechen", prioAlways), hintShow("↑↓", "blättern", prioUseful)}
	case m.screen == ScreenLive && m.chatFocused:
		list := []keyHint{hint("enter", "senden", prioAlways), hintLong("esc", "Eingabe verlassen", "Chat-Eingabe verlassen", prioAlways)}
		if m.sendStudioFunc != nil {
			if m.chatToStudio {
				list = append(list, hintLong("tab", "an den Chat", "An den Chat schreiben", prioMain))
			} else {
				list = append(list, hintLong("tab", "ans Studio", "An das Studio schreiben", prioMain))
			}
		}
		return append(list, hintShow("alt+1…9", "Emoji", prioUseful), hintShow("↑↓", "blättern", prioExtra))
	case m.screen == ScreenLive && m.chatSelectedID != "":
		list := []keyHint{hintShow("↑↓", "Nachricht wählen", prioMain)}
		if m.chatInteractive() {
			list = append(list,
				hintLong("a", "antworten", "Auf die Nachricht antworten", prioMain),
				hintLong("enter", "Reaktionen", "Menü der Nachricht mit den Reaktionen", prioMain),
			)
		}
		return append(list,
			hintLong("y", "kopieren", "Text der Nachricht kopieren", prioExtra),
			hintLong("esc", "fertig", "Nachricht abwählen", prioAlways),
			hintLong("?", "Hilfe", "Alle Tasten anzeigen", prioAlways),
		)
	case m.screen == ScreenFormats && m.filtering:
		return []keyHint{hint("enter", "übernehmen", prioAlways), hint("esc", "verwerfen", prioAlways)}
	case m.screen == ScreenEpisodes && m.yearFiltering:
		return []keyHint{hint("enter", "übernehmen", prioAlways), hint("esc", "verwerfen", prioAlways)}
	}
	return m.screenHints()
}

// screenHints returns the keys of the open screen without the windows.
func (m *Model) screenHints() []keyHint {
	var list []keyHint
	switch m.screen {
	case ScreenSplash:
		return []keyHint{hintShow("Taste", "überspringen", prioAlways)}
	case ScreenStart:
		return []keyHint{
			hint("enter", "anmelden", prioAlways), hint("tab", "nächstes Feld", prioMain),
			hintLong("ctrl+t", "Erscheinungsbild", "Erscheinungsbild wechseln", prioExtra), hint("esc", "beenden", prioUseful),
		}
	case ScreenFormats:
		list = m.formatHints()
	case ScreenEpisodes:
		list = m.episodeHints()
	case ScreenClips:
		list = m.clipHints()
	case ScreenSearch:
		// Das Feld hat die Tastatur: Buchstaben gehören in die Suche.
		action := "suchen"
		if m.searchOpensHit() {
			action = "öffnen"
		}
		return []keyHint{
			hint("enter", action, prioAlways), hintShow("↑↓", "Treffer", prioMain),
			hint("esc", "zurück", prioAlways), hintLong("ctrl+k", "Befehle", "Befehlspalette", prioUseful),
		}
	case ScreenCalendar:
		list = m.calendarHints()
	case ScreenLive:
		list = m.liveHints()
	case ScreenAnnouncements:
		list = []keyHint{hintShow("↑↓", "wählen", prioUseful)}
		if m.announceCursor >= 0 && m.announceCursor < len(m.announcements) && m.announcements[m.announceCursor].URL != "" {
			list = append(list, hintLong("o", "Link öffnen", "Link im Browser öffnen", prioMain))
		}
		list = append(list, hint("esc", "zurück", prioMain))
	case ScreenQueue:
		list = m.queueHints()
	case ScreenSummary:
		list = []keyHint{hintLong("enter", "zurück", "Zurück zu den Formaten", prioMain)}
		if job := m.currentJob(); job != nil && job.Path != "" {
			list = append(list, hintLong("o", "Ordner öffnen", "Ordner der letzten Datei öffnen", prioUseful))
		}
	}
	return append(list,
		hintLong("ctrl+k", "Befehle", "Befehlspalette", prioMain),
		hintLong("?", "Hilfe", "Alle Tasten anzeigen", prioAlways),
		hintLong("q", "beenden", "Programm beenden", prioExtra),
	)
}

// formatHints are the keys of the format list.
func (m *Model) formatHints() []keyHint {
	list := []keyHint{
		hintShow("↑↓", "wählen", prioUseful),
		hintLong("enter", "öffnen", "Format öffnen", prioMain),
	}
	if m.filter != "" {
		list = append(list, hintLong("esc", "Filter löschen", "Filter löschen", prioMain))
	}
	formats := m.filteredFormats()
	if len(formats) > 0 && m.formatCursor < len(formats) {
		list = append(list,
			hintLong("w", "neueste ansehen", "Neueste Folge des Formats ansehen", prioUseful),
			hintLong("D", "neueste laden", "Neueste Folge des Formats herunterladen", prioPalette),
		)
		if _, ok := m.libraryEntry(formats[m.formatCursor].PID); ok {
			list = append(list,
				hintLong("d", "neue laden", "Neue Folgen des Formats herunterladen", prioMain),
				hintLong("n", "prüfen", "Bibliothek nach neuen Folgen durchsuchen", prioExtra),
			)
		} else {
			list = append(list, hintLong("a", "zur Bibliothek", "Format zur Bibliothek hinzufügen", prioUseful))
		}
	}
	filterLabel := "nur Bibliothek"
	if m.libraryOnly {
		filterLabel = "alle Formate"
	}
	list = append(list,
		hintLong("/", "filtern", "Formate filtern", prioUseful),
		hintLong("f", "suchen", "Volltextsuche in allen Folgen", prioUseful),
		hintLong("v", filterLabel, "Ansicht: "+filterLabel, prioExtra),
		hintLong("p", "Ordner", "Speicherort ändern", prioPalette),
		hintLong("l", "Protokoll", "Protokoll ein- oder ausblenden", prioPalette),
		hintLong("s", "Konto", "Zugangsdaten ändern", prioPalette),
	)
	return list
}

// episodeHints are the keys of the episode list. With marked episodes the
// keys say how many they work on.
func (m *Model) episodeHints() []keyHint {
	marked := m.markedCount()
	list := []keyHint{hintShow("↑↓", "wählen", prioUseful)}
	episode, ok := m.currentEpisode()
	switch {
	case marked > 0:
		list = append(list,
			hintLong("d", fmt.Sprintf("%d laden", marked), fmt.Sprintf("%d markierte Folgen herunterladen", marked), prioMain),
			hintLong("w", fmt.Sprintf("%d ansehen", marked), fmt.Sprintf("%d markierte Folgen ansehen", marked), prioMain),
			hintLong("esc", "Markierung weg", "Markierung aufheben", prioMain),
		)
	case ok:
		list = append(list,
			hintLong("enter", "Menü", "Menü der Folge", prioUseful),
			hintLong("d", "laden", "Folge herunterladen", prioMain),
			hintLong("w", m.watchLabel(episode.Identifier), "Folge mit mpv ansehen", prioMain),
		)
	}
	if ok {
		list = append(list, hintLong("space", "markieren", "Folge markieren", prioUseful))
		bookmark, bookmarkLong := "merken", "Auf die Merkliste setzen"
		if episode.Bookmarked {
			bookmark, bookmarkLong = "entmerken", "Von der Merkliste nehmen"
		}
		list = append(list, hintLong("b", bookmark, bookmarkLong, prioExtra), m.watchedHint())
		if m.episodeLoaded(episode.Identifier) {
			list = append(list, hintLong("x", "Datei löschen", "Heruntergeladene Datei löschen", prioExtra))
		}
	}
	if m.episodeYear != "" {
		list = append(list, hintLong("esc", "Jahr aufheben", "Jahresfilter aufheben", prioMain))
	} else if marked == 0 {
		list = append(list, hintLong("esc", "zurück", "Zurück zu den Formaten", prioUseful))
	}
	list = append(list,
		hintLong("A", "alle", "Alle Folgen markieren oder die Markierung aufheben", prioPalette),
		hintLong("J", "Jahr", "Nach Jahr filtern", prioPalette),
		hintLong("i", "Qualität", "Qualität wählen", prioPalette),
		hintLong("r", "ähnliche", "Ähnliche Folgen", prioPalette),
		hintLong("y", "kopieren", "Titel kopieren", prioPalette),
		hintLong("p", "Ordner", "Speicherort ändern", prioPalette),
	)
	if ok && len(episode.Participants) > 0 {
		list = append(list, hintLong("h", "Gast", "Gast der Folge öffnen", prioPalette))
	}
	return list
}

// clipHints are the keys of the clip lists.
func (m *Model) clipHints() []keyHint {
	list := []keyHint{hintShow("↑↓", "wählen", prioUseful)}
	if episode, ok := m.currentEpisode(); ok {
		bookmark, bookmarkLong := "merken", "Auf die Merkliste setzen"
		if episode.Bookmarked {
			bookmark, bookmarkLong = "entmerken", "Von der Merkliste nehmen"
		}
		list = append(list,
			hintLong("w", m.watchLabel(episode.Identifier), "Folge mit mpv ansehen", prioMain),
			hintLong("d", "laden", "Folge herunterladen", prioMain),
			hintLong("enter", "Menü", "Menü der Folge", prioExtra),
			hintLong("b", bookmark, bookmarkLong, prioUseful),
			m.watchedHint(),
			hintLong("i", "Qualität", "Qualität wählen", prioPalette),
			hintLong("p", "Ordner", "Speicherort ändern", prioPalette),
		)
	}
	return append(list, hintLong("esc", "zurück", "Zurück", prioUseful))
}

// calendarHints are the keys of the calendar.
func (m *Model) calendarHints() []keyHint {
	list := []keyHint{hintShow("↑↓", "wählen", prioUseful)}
	if m.calendarCursor >= 0 && m.calendarCursor < len(m.calendar) {
		entry := m.calendar[m.calendarCursor]
		if entry.Type == "live" {
			list = append(list, hintLong("enter", "Live", "Live-Ansicht öffnen", prioMain))
		}
		if entry.URL != "" {
			list = append(list, hintLong("o", "Link", "Link im Browser öffnen", prioUseful))
		}
		list = append(list, hintLong("y", "kopieren", "Titel kopieren", prioExtra))
	}
	return append(list, hintLong("esc", "zurück", "Zurück zu den Formaten", prioUseful))
}

// liveHints are the keys of the live screen.
func (m *Model) liveHints() []keyHint {
	list := []keyHint{}
	live := m.live
	if live != nil && live.IsLive && live.HasAccess() {
		list = append(list, hintLong("enter", "ansehen", "Livesendung ansehen", prioMain))
	}
	if live != nil && live.Replay != nil && live.Replay.Possible && live.Replay.CanAccess {
		list = append(list, hintLong("r", "Replay", "Wiederholung ansehen", prioMain))
	}
	if m.chatConnected {
		list = append(list, hintLong("c", "Chat", "In den Chat schreiben", prioUseful))
		list = append(list, hintLong("s", "Nachricht wählen", "Nachricht wählen, um zu antworten oder zu reagieren", prioUseful))
	}
	if m.hasPoll() && !m.poll.closed {
		list = append(list, hintLong("1…9", "abstimmen", "Bei der Abstimmung eine Antwort wählen", prioMain))
	}
	if len(m.chatMessages) > 0 {
		list = append(list, hintShow("↑↓", "Chat blättern", prioExtra))
	}
	if m.chatScroll > 0 {
		list = append(list, hintLong("end", "neueste", "Chat ans Ende springen", prioMain))
	}
	return append(list,
		hintLong("R", "neu laden", "Live-Stand neu laden", prioExtra),
		hintLong("esc", "zurück", "Zurück", prioUseful),
	)
}

// queueHints are the keys of the download list.
func (m *Model) queueHints() []keyHint {
	list := []keyHint{}
	if job := m.currentJob(); job != nil && job.Path != "" {
		list = append(list,
			hintLong("o", "Ordner öffnen", "Ordner der letzten Datei öffnen", prioMain),
			hintLong("y", "Pfad kopieren", "Pfad der letzten Datei kopieren", prioUseful),
			hintLong("x", "Datei löschen", "Letzte Datei löschen", prioExtra),
		)
	}
	return append(list,
		hintLong("l", "Protokoll", "Protokoll ein- oder ausblenden", prioExtra),
		hintLong("esc", "zurück", "Zurück zu den Formaten (der Download läuft weiter)", prioMain),
	)
}

// watchLabel says whether a play starts or continues the episode.
func (m *Model) watchLabel(identifier string) string {
	if percent := m.episodeProgress(identifier); percent > 0 && percent < watchedPercent {
		return "weiterschauen"
	}
	return "ansehen"
}

// markedCount returns the number of the marked episodes in the list.
func (m *Model) markedCount() int {
	count := 0
	for _, episode := range m.filteredEpisodes() {
		if m.selected[episode.Identifier] {
			count++
		}
	}
	return count
}

// searchOpensHit tells that enter in the search opens the marked hit: the
// field still holds the text the hits were found with.
func (m *Model) searchOpensHit() bool {
	return len(m.searchResults) > 0 &&
		strings.TrimSpace(m.searchInput.Value()) == m.searchQuery
}

// navTab is one tab of the bar below the head line.
type navTab struct {
	key   string
	label string
}

// tabs returns the tabs of the bar. The downloads appear once something
// was loaded in this run.
func (m *Model) tabs() []navTab {
	tabs := []navTab{
		{"1", "Formate"},
		{"2", "Weiterschauen"},
		{"3", "Merkliste"},
		{"4", "Kalender"},
		{"5", "Live"},
	}
	if len(m.jobs) > 0 || m.screen == ScreenQueue || m.screen == ScreenSummary {
		tabs = append(tabs, navTab{"6", "Downloads"})
	}
	return tabs
}

// activeTab returns the key of the tab the open screen belongs to.
func (m *Model) activeTab() string {
	switch m.screen {
	case ScreenFormats, ScreenEpisodes, ScreenSearch:
		return "1"
	case ScreenClips:
		switch m.clipList.kind {
		case clipListRecents:
			return "2"
		case clipListBookmarks:
			return "3"
		}
	case ScreenCalendar:
		return "4"
	case ScreenLive:
		return "5"
	case ScreenQueue, ScreenSummary:
		return "6"
	}
	return ""
}

// showsTabs tells whether the bar of the tabs is drawn: not before the
// login.
func (m *Model) showsTabs() bool {
	return m.screen != ScreenSplash && m.screen != ScreenStart
}

// switchTab opens the screen of a tab.
func (m *Model) switchTab(key string) (tea.Model, tea.Cmd) {
	if m.screen == ScreenLive && key != "5" {
		m.closeChat()
		m.chatFocused = false
		m.chatInput.Blur()
	}
	switch key {
	case "1":
		m.screen = ScreenFormats
		return m, nil
	case "2":
		return m.openRecents()
	case "3":
		return m.openBookmarks()
	case "4":
		return m.openCalendar()
	case "5":
		if m.screen == ScreenLive {
			return m, m.loadLive()
		}
		return m.openLive()
	case "6":
		if len(m.jobs) == 0 && m.screen != ScreenSummary {
			m.message = "In dieser Sitzung wurde noch nichts heruntergeladen."
			m.isError = false
			return m, nil
		}
		m.screen = ScreenQueue
		return m, nil
	}
	return m, nil
}

// typing tells that a field has the keyboard, so letters and digits belong
// to it.
func (m *Model) typing() bool {
	switch m.screen {
	case ScreenSplash, ScreenStart, ScreenSearch:
		return true
	case ScreenLive:
		return m.chatFocused
	}
	return m.filtering || m.yearFiltering
}

// handleGlobalKey takes the keys that work on every screen: the tabs and
// the command palette. It reports whether it took the key.
func (m *Model) handleGlobalKey(msg tea.KeyMsg) (bool, tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+k" && m.screen != ScreenSplash && m.screen != ScreenStart && !m.chatFocused {
		model, cmd := m.openPalette()
		return true, model, cmd
	}
	if m.typing() {
		return false, m, nil
	}
	if key == ":" {
		model, cmd := m.openPalette()
		return true, model, cmd
	}
	if key == "." {
		model, cmd := m.openContextMenu(false)
		return true, model, cmd
	}
	// Läuft eine Abstimmung, gehören die Ziffern ihr: sie wählen eine
	// Antwort. Der Live-Bildschirm braucht die Reiter nicht.
	if m.screen == ScreenLive && m.hasPoll() && len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		return false, m, nil
	}
	for _, tab := range m.tabs() {
		if tab.key == key {
			model, cmd := m.switchTab(key)
			return true, model, cmd
		}
	}
	return false, m, nil
}

// pressKey runs a key as if it was typed. The palette and the mouse use it,
// so a click does exactly what the key does.
func (m *Model) pressKey(key string) (tea.Model, tea.Cmd) {
	return m.handleKey(keyMsgFor(key))
}

// keyMsgFor turns the name of a key into the message of a key press.
func keyMsgFor(key string) tea.KeyMsg {
	types := map[string]tea.KeyType{
		"enter":     tea.KeyEnter,
		"esc":       tea.KeyEsc,
		"tab":       tea.KeyTab,
		"shift+tab": tea.KeyShiftTab,
		"up":        tea.KeyUp,
		"down":      tea.KeyDown,
		"pgup":      tea.KeyPgUp,
		"pgdown":    tea.KeyPgDown,
		"home":      tea.KeyHome,
		"end":       tea.KeyEnd,
		"space":     tea.KeySpace,
		" ":         tea.KeySpace,
		"ctrl+t":    tea.KeyCtrlT,
		"ctrl+v":    tea.KeyCtrlV,
		"ctrl+k":    tea.KeyCtrlK,
		"ctrl+c":    tea.KeyCtrlC,
	}
	if kind, ok := types[key]; ok {
		return tea.KeyMsg{Type: kind}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
}

// cycleTheme switches to the next color scheme: hell, dunkel, automatisch.
func (m *Model) cycleTheme() {
	switch m.theme {
	case ThemeLight:
		m.setTheme(ThemeDark)
	case ThemeDark:
		m.setTheme(ThemeAuto)
	default:
		m.setTheme(ThemeLight)
	}
}

// capitalize writes the first letter of a text in upper case.
func capitalize(text string) string {
	first, size := utf8.DecodeRuneInString(text)
	if first == utf8.RuneError {
		return text
	}
	return string(unicode.ToUpper(first)) + text[size:]
}

// confirmState is a question before a step that cannot be undone.
type confirmState struct {
	title  string
	text   string
	yes    string
	cursor int
	run    func() (tea.Model, tea.Cmd)
}

// askConfirm opens the question. The choice starts on "Abbrechen", so a
// hasty enter does no harm.
func (m *Model) askConfirm(title, text, yes string, run func() (tea.Model, tea.Cmd)) {
	m.confirm = &confirmState{title: title, text: text, yes: yes, cursor: 1, run: run}
}

// handleConfirmKey answers the question.
func (m *Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	state := m.confirm
	switch msg.String() {
	case "up", "k", "left", "h":
		state.cursor = 0
	case "down", "right", "l", "tab":
		state.cursor = 1
	// „j“ für ja, „y“ für yes; x bestätigt, weil x die Frage gestellt hat.
	case "y", "j", "x":
		return m.acceptConfirm()
	case "enter":
		if state.cursor == 0 {
			return m.acceptConfirm()
		}
		m.confirm = nil
	case "n", "esc", "q":
		m.confirm = nil
	}
	return m, nil
}

// acceptConfirm runs the step the question was about.
func (m *Model) acceptConfirm() (tea.Model, tea.Cmd) {
	run := m.confirm.run
	m.confirm = nil
	if run == nil {
		return m, nil
	}
	return run()
}

// confirmEntries are the two answers.
func (m *Model) confirmEntries() []string {
	return []string{m.confirm.yes, "Abbrechen"}
}

// confirmPopup draws the question.
func (m *Model) confirmPopup(width int) []string {
	inner := min(56, max(30, width-16))
	content := []string{}
	for _, line := range wrapText(m.confirm.text, inner) {
		content = append(content, theme.Muted.Render(line))
	}
	content = append(content, "")
	for index, entry := range m.confirmEntries() {
		label := entry
		if index == 0 {
			label = theme.Danger.Render(entry)
		}
		content = append(content, renderWindowRow(theme.Row.Render(label), index == m.confirm.cursor, inner))
	}
	return windowBoxFootColor(inner+4, m.confirm.title, "y bestätigen · n abbrechen", theme.BoxList, content)
}

// confirmFirstRow is the line of the first answer inside the window: the
// text and one empty line stand above it.
func (m *Model) confirmFirstRow(width int) int {
	inner := min(56, max(30, width-16))
	return len(wrapText(m.confirm.text, inner)) + 1
}
