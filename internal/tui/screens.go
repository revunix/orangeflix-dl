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
