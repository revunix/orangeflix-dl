package tui

import (
	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// The actions of the menu that opens when a episode is picked.
const (
	// actionDownload loads the episode.
	actionDownload = "Herunterladen"
	// actionWatch plays the episode with mpv.
	actionWatch = "Mit mpv ansehen"
	// actionCancel closes the menu.
	actionCancel = "Abbrechen"
)

// actionEntries returns the choices of the menu.
func actionEntries() []string {
	return []string{actionDownload, actionWatch, actionCancel}
}

// actionPopup builds the window that asks what should happen with the
// episode. The window floats over the page and is centered in both
// directions.
func (m *Model) actionPopup(width int) []string {
	entries := actionEntries()
	content := []string{}

	title := "Folge"
	if episode, ok := m.actionEpisode(); ok {
		title = episode.Title
		if title == "" {
			title = episode.FormatName
		}
	}
	content = append(content, theme.Muted.Render("Was soll geschehen?"))
	content = append(content, "")

	for index, entry := range entries {
		content = append(content, renderWindowRow(theme.Row.Render(entry), index == m.actionCursor, 32))
	}

	return windowBoxFootColor(36, fitToWidth(title, 28), "", theme.BoxList, content)
}

// actionEpisode returns the episode the menu belongs to.
func (m *Model) actionEpisode() (orangeflix.Episode, bool) {
	episodes := m.filteredEpisodes()
	if len(episodes) == 0 {
		return orangeflix.Episode{}, false
	}
	index := m.actionEpisodeCursor
	if index < 0 || index >= len(episodes) {
		index = m.episodeCursor
	}
	if index < 0 || index >= len(episodes) {
		return orangeflix.Episode{}, false
	}
	return episodes[index], true
}

// openActionMenu shows the menu of the episode under the cursor.
func (m *Model) openActionMenu() {
	m.showAction = true
	m.actionCursor = 0
	m.actionEpisodeCursor = m.episodeCursor
}

// closeActionMenu hides the menu.
func (m *Model) closeActionMenu() {
	m.showAction = false
	m.actionCursor = 0
}

// actionIdentifiers returns the episodes the menu works on: the marked
// ones, otherwise the episode that was picked.
func (m *Model) actionIdentifiers() []string {
	marked := []string{}
	for _, episode := range m.filteredEpisodes() {
		if m.selected[episode.Identifier] {
			marked = append(marked, episode.Identifier)
		}
	}
	if len(marked) > 0 {
		return marked
	}
	if episode, ok := m.actionEpisode(); ok {
		return []string{episode.Identifier}
	}
	return nil
}
