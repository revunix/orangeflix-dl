package tui

import (
	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// Screen identifies the visible view.
type Screen int

// The screens.
const (
	ScreenSplash Screen = iota
	ScreenStart
	ScreenFormats
	ScreenEpisodes
	ScreenQueue
	ScreenSummary
	ScreenSearch
	ScreenClips
	ScreenCalendar
	ScreenLive
	ScreenAnnouncements
)

// breadcrumb returns the path that is shown in the head line.
func (m *Model) breadcrumb() string {
	switch m.screen {
	case ScreenStart:
		return "Anmeldung"
	case ScreenFormats:
		if m.libraryOnly {
			return "Bibliothek"
		}
		return "Formate"
	case ScreenEpisodes:
		if m.currentFormat != nil {
			return "Formate › " + m.currentFormat.Title
		}
		return "Formate"
	case ScreenQueue:
		return "Downloads"
	case ScreenSummary:
		return "Fertig"
	case ScreenSearch:
		return "Suche"
	case ScreenClips:
		if m.clipList.title != "" {
			return m.clipList.title
		}
		return "Folgen"
	case ScreenCalendar:
		return "Kalender"
	case ScreenLive:
		return "Live"
	case ScreenAnnouncements:
		return "Ankündigungen"
	}
	return ""
}

// statusChip returns the text of the chip on the right of the head line. It
// names the context of the current view.
func (m *Model) statusChip() string {
	switch m.screen {
	case ScreenFormats:
		if m.libraryOnly {
			return "nur die Bibliothek"
		}
		return plural(int64(len(m.visibleFormats())), "Format", "Formate")
	case ScreenEpisodes:
		if m.episodeYear != "" {
			return "Jahr " + m.episodeYear
		}
		return plural(int64(len(m.filteredEpisodes())), "Folge", "Folgen")
	case ScreenQueue:
		return plural(int64(len(m.jobs)), "Datei", "Dateien")
	case ScreenSummary:
		return "v" + m.version
	case ScreenSearch:
		return "Volltext"
	case ScreenClips:
		return plural(int64(len(m.clipList.episodes)), "Folge", "Folgen")
	case ScreenCalendar:
		return plural(int64(len(m.calendar)), "Termin", "Termine")
	case ScreenLive:
		return "Live"
	case ScreenAnnouncements:
		return plural(int64(len(m.announcements)), "Hinweis", "Hinweise")
	}
	return ""
}

// statusLine returns the text behind the title of the head line. It shows
// what the program is doing right now.
func (m *Model) statusLine() string {
	if m.loading {
		return m.spinner.View() + " " + m.loadingLabel
	}
	if m.playback != nil {
		text := "▶ " + truncateColumn(m.playback.label, 28) + " " + playbackClock(m.playback.position)
		if m.playback.duration > 0 {
			text += " / " + playbackClock(m.playback.duration)
		}
		return text
	}
	return ""
}

// hints returns the keys of the current screen.
func (m *Model) hints() []keyHint {
	// Ein offenes Fenster zeigt seine eigenen Tasten.
	if m.showHelp {
		return []keyHint{{"esc", "schließen"}}
	}
	if m.showQuality {
		return []keyHint{{"↑/↓", "wählen"}, {"enter", "übernehmen"}, {"esc", "abbrechen"}}
	}
	if m.showAction {
		return []keyHint{{"↑/↓", "wählen"}, {"enter", "bestätigen"}, {"esc", "abbrechen"}}
	}
	if m.showParticipants {
		return []keyHint{{"↑/↓", "wählen"}, {"enter", "öffnen"}, {"esc", "abbrechen"}}
	}
	if m.showPath {
		return []keyHint{
			{"↑/↓", "wählen"}, {"tab", "öffnen"}, {"shift+tab", "hoch"},
			{"ctrl+v", "einfügen"}, {"enter", "übernehmen"}, {"esc", "abbrechen"},
		}
	}
	if m.screen == ScreenLive && m.chatFocused {
		return []keyHint{{"enter", "senden"}, {"esc", "Eingabe verlassen"}, {"↑/↓", "blättern"}}
	}
	switch m.screen {
	case ScreenStart:
		return []keyHint{
			{"tab", "Feld"}, {"ctrl+t", "Erscheinungsbild"},
			{"enter", "anmelden"}, {"esc", "beenden"},
		}
	case ScreenFormats:
		return []keyHint{
			{"↑/↓", "wählen"}, {"enter", "öffnen"}, {"d", "laden"}, {"/", "filtern"},
			{"f", "suchen"}, {"a", "merken"}, {"p", "Ordner"}, {"?", "Hilfe"}, {"q", "beenden"},
		}
	case ScreenEpisodes:
		return []keyHint{
			{"↑/↓", "wählen"}, {"space", "markieren"}, {"enter", "Menü"},
			{"d", "laden"}, {"w", "ansehen"}, {"J", "Jahr"}, {"?", "Hilfe"},
		}
	case ScreenQueue:
		return []keyHint{
			{"l", "Protokoll"}, {"y", "kopieren"}, {"o", "öffnen"}, {"x", "löschen"}, {"q", "beenden"},
		}
	case ScreenSummary:
		return []keyHint{
			{"enter", "zurück"}, {"?", "Hilfe"}, {"q", "beenden"},
		}
	case ScreenSearch:
		return []keyHint{
			{"enter", "suchen"}, {"↑/↓", "wählen"}, {"esc", "zurück"}, {"q", "beenden"},
		}
	case ScreenClips:
		return []keyHint{
			{"↑/↓", "wählen"}, {"enter", "Menü"}, {"w", "ansehen"}, {"d", "laden"},
			{"b", "merken"}, {"esc", "zurück"}, {"?", "Hilfe"}, {"q", "beenden"},
		}
	case ScreenCalendar:
		return []keyHint{
			{"↑/↓", "wählen"}, {"enter", "Live"}, {"o", "Link"}, {"y", "kopieren"},
			{"esc", "zurück"}, {"?", "Hilfe"}, {"q", "beenden"},
		}
	case ScreenLive:
		return []keyHint{
			{"enter", "ansehen"}, {"r", "Replay"}, {"c", "Chat"},
			{"R", "neu laden"}, {"esc", "zurück"}, {"?", "Hilfe"}, {"q", "beenden"},
		}
	case ScreenAnnouncements:
		return []keyHint{
			{"↑/↓", "wählen"}, {"o", "Link"}, {"esc", "zurück"}, {"?", "Hilfe"}, {"q", "beenden"},
		}
	}
	return nil
}

// statusLabel returns the German label of a status.
func statusLabel(status string) string {
	switch status {
	case orangeflix.StatusActive:
		return "AKTIV"
	case orangeflix.StatusSeasonal:
		return "SAISONAL"
	case orangeflix.StatusArchived:
		return "ARCHIVIERT"
	}
	return status
}
