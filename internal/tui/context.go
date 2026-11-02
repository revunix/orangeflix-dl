package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// menuItem is one entry of the context menu.
type menuItem struct {
	label string
	// key does the same without the menu; it also runs the entry while the
	// menu is open.
	key string
	// danger marks a step that removes something.
	danger bool
	run    clickFunc
}

// contextMenu is the menu of one row of a list.
type contextMenu struct {
	title  string
	items  []menuItem
	cursor int
	// anchored places the menu at the pointer, otherwise it is centered.
	anchored bool
	x, y     int
}

// openContextMenu opens the menu of the row under the cursor. The mouse
// opens it at the pointer, the key in the middle of the page.
func (m *Model) openContextMenu(anchored bool) (tea.Model, tea.Cmd) {
	title, items := m.contextItems()
	if len(items) == 0 {
		return m, nil
	}
	m.context = &contextMenu{title: title, items: items, anchored: anchored, x: m.mouseX, y: m.mouseY}
	return m, nil
}

// contextItems returns the title and the entries of the menu of the row
// under the cursor. Every entry works on that row; with marked episodes the
// menu also offers the marked ones.
func (m *Model) contextItems() (string, []menuItem) {
	switch m.screen {
	case ScreenFormats:
		return m.formatMenu()
	case ScreenEpisodes:
		return m.episodeMenu()
	case ScreenClips, ScreenSearch:
		return m.clipMenu()
	case ScreenCalendar:
		if m.calendarCursor < 0 || m.calendarCursor >= len(m.calendar) {
			return "", nil
		}
		entry := m.calendar[m.calendarCursor]
		items := []menuItem{}
		if entry.Type == "live" {
			items = append(items, menuItem{label: "Live-Ansicht öffnen", key: "enter", run: m.keyRun("enter")})
		}
		if entry.URL != "" {
			items = append(items, menuItem{label: "Link im Browser öffnen", key: "o", run: m.keyRun("o")})
		}
		items = append(items, menuItem{label: "Titel kopieren", key: "y", run: m.keyRun("y")})
		return entry.Title, items
	case ScreenLive:
		return m.chatMenu()
	case ScreenAnnouncements:
		if m.announceCursor < 0 || m.announceCursor >= len(m.announcements) {
			return "", nil
		}
		item := m.announcements[m.announceCursor]
		items := []menuItem{{label: "Titel kopieren", run: m.copyRun(item.Title)}}
		if item.URL != "" {
			items = append([]menuItem{{label: "Link im Browser öffnen", key: "o", run: m.keyRun("o")}}, items...)
		}
		return item.Title, items
	}
	return "", nil
}

// formatMenu is the menu of a format.
func (m *Model) formatMenu() (string, []menuItem) {
	formats := m.filteredFormats()
	if len(formats) == 0 || m.formatCursor >= len(formats) {
		return "", nil
	}
	format := formats[m.formatCursor]
	latest := "Neueste Folge"
	if !format.LastEpisodeTime.IsZero() {
		latest += " (" + format.LastEpisodeTime.Local().Format("02.01.") + ")"
	}
	items := []menuItem{
		{label: "Folgen öffnen", key: "enter", run: m.keyRun("enter")},
		{label: latest + " ansehen", key: "w", run: m.keyRun("w")},
		{label: latest + " herunterladen", key: "D", run: m.keyRun("D")},
	}
	if _, ok := m.libraryEntry(format.PID); ok {
		items = append(items,
			menuItem{label: "Neue Folgen herunterladen", key: "d", run: m.keyRun("d")},
			menuItem{label: "Bibliothek nach neuen Folgen durchsuchen", key: "n", run: m.keyRun("n")},
		)
	} else {
		items = append(items, menuItem{label: "Zur Bibliothek hinzufügen", key: "a", run: m.keyRun("a")})
	}
	items = append(items, menuItem{label: "Titel kopieren", run: m.copyRun(format.Title)})
	return format.Title, items
}

// episodeMenu is the menu of an episode of a format.
func (m *Model) episodeMenu() (string, []menuItem) {
	episode, ok := m.currentEpisode()
	if !ok {
		return "", nil
	}
	single := []orangeflix.Episode{episode}
	items := []menuItem{
		{label: "Herunterladen", key: "d", run: func() (tea.Model, tea.Cmd) {
			return m.startDownloadWith([]string{episode.Identifier})
		}},
		{label: capitalize(m.watchLabel(episode.Identifier)) + " mit mpv", key: "w", run: func() (tea.Model, tea.Cmd) {
			return m.watchList(single)
		}},
	}
	if marked := m.markedCount(); marked > 0 {
		items = append(items,
			menuItem{label: fmt.Sprintf("%d markierte herunterladen", marked), run: m.startDownload},
			menuItem{label: fmt.Sprintf("%d markierte ansehen", marked), run: m.watchEpisodes},
			menuItem{label: fmt.Sprintf("%d markierte als gesehen markieren", marked), run: func() (tea.Model, tea.Cmd) {
				return m.setWatched(m.watchedTargets(), true)
			}},
		)
	}
	mark := "Markieren"
	if m.selected[episode.Identifier] {
		mark = "Markierung entfernen"
	}
	items = append(items,
		menuItem{label: mark, key: "space", run: m.keyRun(" ")},
		m.bookmarkItem(episode, m.keyRun("b")),
		m.watchedItem(episode),
		menuItem{label: "Qualität wählen", key: "i", run: m.keyRun("i")},
		menuItem{label: "Ähnliche Folgen", key: "r", run: m.keyRun("r")},
	)
	if len(episode.Participants) > 0 {
		items = append(items, menuItem{label: "Gast öffnen", key: "h", run: m.keyRun("h")})
	}
	items = append(items, menuItem{label: "Titel kopieren", key: "y", run: m.keyRun("y")})
	if m.episodeLoaded(episode.Identifier) {
		items = append(items, menuItem{label: "Datei löschen …", key: "x", danger: true, run: m.keyRun("x")})
	}
	return episodeTitle(episode), items
}

// clipMenu is the menu of a clip of a list or of a hit of the search. The
// search has the keyboard in its field, so the entries call the steps
// directly instead of pressing keys.
func (m *Model) clipMenu() (string, []menuItem) {
	episode, ok := m.currentEpisode()
	if !ok {
		return "", nil
	}
	items := []menuItem{
		{label: capitalize(m.watchLabel(episode.Identifier)) + " mit mpv", key: "w", run: m.watchCurrentClip},
		{label: "Herunterladen", key: "d", run: m.downloadCurrentClip},
		m.bookmarkItem(episode, m.toggleBookmark),
		m.watchedItem(episode),
	}
	if m.screen == ScreenClips {
		items = append(items, menuItem{label: "Qualität wählen", key: "i", run: m.keyRun("i")})
	}
	items = append(items, menuItem{label: "Titel kopieren", run: m.copyRun(episode.Title)})
	if m.screen == ScreenSearch {
		// Im Suchfeld wären die Buchstaben Text; das Menü nennt keine Tasten.
		for index := range items {
			items[index].key = ""
		}
	}
	return episodeTitle(episode), items
}

// bookmarkItem is the entry that puts an episode on the list or takes it
// off.
func (m *Model) bookmarkItem(episode orangeflix.Episode, run clickFunc) menuItem {
	if episode.Bookmarked {
		return menuItem{label: "Von der Merkliste nehmen", key: "b", run: run}
	}
	return menuItem{label: "Auf die Merkliste setzen", key: "b", run: run}
}

// episodeTitle names an episode in the title of a menu.
func episodeTitle(episode orangeflix.Episode) string {
	if episode.Title != "" {
		return episode.Title
	}
	return episode.FormatName
}

// keyRun is an entry that presses a key.
func (m *Model) keyRun(key string) clickFunc {
	return func() (tea.Model, tea.Cmd) { return m.pressKey(key) }
}

// copyRun is an entry that copies a text.
func (m *Model) copyRun(text string) clickFunc {
	return func() (tea.Model, tea.Cmd) {
		m.copyText(text)
		return m, nil
	}
}

// handleContextKey processes the keys of the menu. The key of an entry runs
// it at once.
func (m *Model) handleContextKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	menu := m.context
	key := msg.String()
	switch key {
	case "up", "k":
		menu.cursor = max(0, menu.cursor-1)
		return m, nil
	case "down", "j", "tab":
		menu.cursor = min(len(menu.items)-1, menu.cursor+1)
		return m, nil
	case "enter":
		return m.runContextItem(menu.cursor)
	case "esc", "q", ".":
		m.context = nil
		return m, nil
	}
	if key == " " {
		key = "space"
	}
	for index, item := range menu.items {
		if item.key != "" && item.key == key {
			return m.runContextItem(index)
		}
	}
	return m, nil
}

// runContextItem closes the menu and runs one entry.
func (m *Model) runContextItem(index int) (tea.Model, tea.Cmd) {
	menu := m.context
	m.context = nil
	if menu == nil || index < 0 || index >= len(menu.items) {
		return m, nil
	}
	return menu.items[index].run()
}

// contextPopup draws the menu: the entries with their keys on the right.
func (m *Model) contextPopup() []string {
	menu := m.context
	labelWidth, keyWidth := 0, 0
	for _, item := range menu.items {
		labelWidth = max(labelWidth, ansi.StringWidth(item.label))
		keyWidth = max(keyWidth, ansi.StringWidth(item.key))
	}
	inner := min(48, max(26, rowIndent+labelWidth+3+keyWidth))
	content := make([]string, 0, len(menu.items))
	for index, item := range menu.items {
		label := truncateColumn(item.label, max(4, inner-rowIndent-keyWidth-3))
		style := theme.Row
		if item.danger {
			style = theme.Danger
		}
		gap := max(1, inner-rowIndent-ansi.StringWidth(label)-keyWidth)
		line := style.Render(label) + strings.Repeat(" ", gap) + theme.Subtle.Render(padRight(item.key, keyWidth))
		content = append(content, renderWindowRow(line, index == menu.cursor, inner))
	}
	return windowBoxFootColor(inner+4, fitToWidth(menu.title, max(4, inner-6)), "", theme.BoxList, content)
}

// contextPlace puts the menu below the pointer, or above it when there is
// no room below; it never leaves the page.
func (m *Model) contextPlace(pageHeight, width int, window []string) (top, left int) {
	if !m.context.anchored {
		top, left, _ = popupPlace(pageHeight, width, window)
		return top, left
	}
	windowWidth := 0
	for _, line := range window {
		windowWidth = max(windowWidth, ansi.StringWidth(line))
	}
	left = max(0, min(m.context.x, width-windowWidth))
	top = m.context.y + 1
	if top+len(window) > pageHeight {
		top = m.context.y - len(window)
	}
	top = max(0, min(top, pageHeight-len(window)))
	return top, left
}
