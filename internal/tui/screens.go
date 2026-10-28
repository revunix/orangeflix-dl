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
	}
	return ""
}

// statusLine returns the text behind the title of the head line. It shows
// what the program is doing right now.
func (m *Model) statusLine() string {
	if m.loading {
		return m.spinner.View() + " " + m.loadingLabel
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
	if m.showPath {
		return []keyHint{
			{"↑/↓", "wählen"}, {"tab", "öffnen"}, {"shift+tab", "hoch"},
			{"ctrl+v", "einfügen"}, {"enter", "übernehmen"}, {"esc", "abbrechen"},
		}
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
