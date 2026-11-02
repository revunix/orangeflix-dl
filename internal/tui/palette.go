package tui

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// paletteItem is one command of the palette.
type paletteItem struct {
	// label says what the command does.
	label string
	// key is the shortcut that does the same, shown beside the label.
	key string
	// group names where the command belongs.
	group string
	run   func() (tea.Model, tea.Cmd)
}

// openPalette shows the command palette. A filter that is being typed is
// taken over first, so the commands work on the list that is visible.
func (m *Model) openPalette() (tea.Model, tea.Cmd) {
	if m.filtering {
		m.filter = m.filterInput.Value()
		m.filtering = false
		m.filterInput.Blur()
	}
	if m.yearFiltering {
		m.episodeYear = m.yearInput.Value()
		m.yearFiltering = false
		m.yearInput.Blur()
	}
	m.showPalette = true
	m.paletteCursor = 0
	m.paletteScroll = 0
	m.paletteInput.SetValue("")
	return m, m.paletteInput.Focus()
}

// closePalette hides the palette.
func (m *Model) closePalette() {
	m.showPalette = false
	m.paletteInput.Blur()
	m.paletteInput.SetValue("")
	m.paletteCursor = 0
	m.paletteScroll = 0
	// Die Suche behält ihr Feld.
	if m.screen == ScreenSearch {
		m.searchInput.Focus()
	}
}

// paletteItems returns every command: first what the open screen offers,
// then the places to go and the commands of the whole program.
func (m *Model) paletteItems() []paletteItem {
	items := []paletteItem{}
	seen := map[string]bool{}
	add := func(item paletteItem) {
		if seen[item.label] {
			return
		}
		seen[item.label] = true
		items = append(items, item)
	}

	group := m.helpGroup()
	for _, item := range m.screenHints() {
		switch item.Run {
		case "", "?", "q", "ctrl+k":
			continue
		}
		label := item.Long
		if label == "" {
			label = item.Label
		}
		run := item.Run
		add(paletteItem{label: capitalize(label), key: item.Key, group: group, run: func() (tea.Model, tea.Cmd) {
			return m.pressKey(run)
		}})
	}

	for _, tab := range m.tabs() {
		key := tab.key
		add(paletteItem{label: tab.label, key: key, group: "Gehe zu", run: func() (tea.Model, tea.Cmd) {
			return m.switchTab(key)
		}})
	}
	add(paletteItem{label: "Ankündigungen", key: "!", group: "Gehe zu", run: m.openAnnouncements})

	general := []paletteItem{
		{label: "Volltextsuche in allen Folgen", key: "f", run: m.openSearch},
		{label: "Speicherort ändern", key: "p", run: m.openPathPopup},
		{label: "Bibliothek nach neuen Folgen durchsuchen", key: "n", run: m.checkLibrary},
		{label: "Erscheinungsbild wechseln", key: "ctrl+t", run: func() (tea.Model, tea.Cmd) {
			m.cycleTheme()
			m.message = "Erscheinungsbild: " + string(m.theme)
			m.isError = false
			return m, nil
		}},
		{label: "Protokoll ein- oder ausblenden", key: "l", run: func() (tea.Model, tea.Cmd) {
			m.showLogBox = !m.showLogBox
			m.logOffset = 0
			return m, nil
		}},
		{label: "Zugangsdaten ändern", key: "s", run: func() (tea.Model, tea.Cmd) {
			m.closeChat()
			m.screen = ScreenStart
			m.field = 0
			return m, m.focusField()
		}},
		{label: "Alle Tasten anzeigen", key: "?", run: func() (tea.Model, tea.Cmd) {
			m.showHelp = true
			return m, nil
		}},
		{label: "Programm beenden", key: "q", run: func() (tea.Model, tea.Cmd) {
			m.closeChat()
			m.quitting = true
			return m, tea.Quit
		}},
	}
	for _, item := range general {
		item.group = "Allgemein"
		add(item)
	}
	return items
}

// filteredPaletteItems returns the commands that match the text of the
// field. Every word has to appear; commands whose label starts with the
// text come first.
func (m *Model) filteredPaletteItems() []paletteItem {
	items := m.paletteItems()
	query := strings.ToLower(strings.TrimSpace(m.paletteInput.Value()))
	if query == "" {
		return items
	}
	words := strings.Fields(query)
	kept := []paletteItem{}
	for _, item := range items {
		haystack := strings.ToLower(item.label + " " + item.group + " " + item.key)
		match := true
		for _, word := range words {
			if !strings.Contains(haystack, word) {
				match = false
				break
			}
		}
		if match {
			kept = append(kept, item)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool {
		a := strings.HasPrefix(strings.ToLower(kept[i].label), query)
		b := strings.HasPrefix(strings.ToLower(kept[j].label), query)
		return a && !b
	})
	return kept
}

// handlePaletteKey processes the keys of the palette. Everything that is
// not a move or a choice goes into the field.
func (m *Model) handlePaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.filteredPaletteItems()
	switch msg.String() {
	case "esc", "ctrl+k":
		m.closePalette()
		return m, nil
	case "enter":
		return m.runPaletteItem(m.paletteCursor)
	case "up", "ctrl+p":
		m.paletteCursor = max(0, m.paletteCursor-1)
		return m, nil
	case "down", "ctrl+n", "tab":
		m.paletteCursor = min(max(0, len(items)-1), m.paletteCursor+1)
		return m, nil
	case "pgup":
		m.paletteCursor = max(0, m.paletteCursor-m.paletteRows())
		return m, nil
	case "pgdown":
		m.paletteCursor = min(max(0, len(items)-1), m.paletteCursor+m.paletteRows())
		return m, nil
	}
	before := m.paletteInput.Value()
	var cmd tea.Cmd
	m.paletteInput, cmd = m.paletteInput.Update(msg)
	if m.paletteInput.Value() != before {
		m.paletteCursor = 0
		m.paletteScroll = 0
	}
	return m, cmd
}

// runPaletteItem closes the palette and runs one command.
func (m *Model) runPaletteItem(index int) (tea.Model, tea.Cmd) {
	items := m.filteredPaletteItems()
	if index < 0 || index >= len(items) {
		return m, nil
	}
	m.closePalette()
	return items[index].run()
}

// paletteRows is the number of commands the window shows at once.
func (m *Model) paletteRows() int {
	rows := 12
	if m.height > 0 {
		rows = min(rows, m.height-14)
	}
	return max(rows, 4)
}

// paletteInnerWidth is the room for the text inside the palette.
func paletteInnerWidth(width int) int {
	return min(64, max(36, width-16))
}

// paletteFirstRow is the line of the first command inside the window: the
// field and an empty line stand above it.
const paletteFirstRow = 2

// palettePopup draws the palette: the field on top, the commands below with
// their place and their key on the right.
func (m *Model) palettePopup(width int) []string {
	inner := paletteInnerWidth(width)
	m.paletteInput.Width = max(10, inner-4)
	m.paletteInput.PromptStyle = theme.AccentBold
	m.paletteInput.TextStyle = theme.Text

	items := m.filteredPaletteItems()
	if m.paletteCursor >= len(items) {
		m.paletteCursor = max(0, len(items)-1)
	}
	content := []string{m.paletteInput.View(), ""}
	if len(items) == 0 {
		content = append(content, theme.Muted.Render("Kein Befehl gefunden."))
		return windowBoxFootColor(inner+4, "Befehle", "esc schließen", theme.BoxList, content)
	}
	start, end := fitListWindow(&m.paletteScroll, m.paletteCursor, m.paletteRows(), len(items))
	// Die Tasten stehen in einer Spalte; der Bereich steht nur an der
	// ersten Zeile, die zu ihm gehört.
	keyWidth, groupWidth := 0, 0
	for _, item := range items[start:end] {
		keyWidth = max(keyWidth, ansi.StringWidth(item.key))
		groupWidth = max(groupWidth, ansi.StringWidth(item.group))
	}
	for index := start; index < end; index++ {
		item := items[index]
		group := ""
		if index == start || items[index-1].group != item.group {
			group = item.group
		}
		right := theme.Subtle.Render(padRight(group, groupWidth)) + "  " + theme.AccentBold.Render(padRight(item.key, keyWidth))
		rightWidth := groupWidth + 2 + keyWidth
		room := max(4, inner-rowIndent-rightWidth-2)
		label := truncateColumn(item.label, room)
		gap := max(1, inner-rowIndent-ansi.StringWidth(label)-rightWidth)
		line := theme.Row.Render(label) + strings.Repeat(" ", gap) + right
		content = append(content, renderWindowRow(line, index == m.paletteCursor, inner))
	}
	foot := "↑↓ wählen · enter ausführen · esc schließen"
	if len(items) > end-start {
		foot = itoa(int64(m.paletteCursor+1)) + "/" + itoa(int64(len(items))) + " · " + foot
	}
	return windowBoxFootColor(inner+4, "Befehle", foot, theme.BoxList, content)
}

// openSearch opens the full text search with the last query.
func (m *Model) openSearch() (tea.Model, tea.Cmd) {
	if m.screen == ScreenLive {
		m.closeChat()
	}
	m.screen = ScreenSearch
	m.searchInput.SetValue(m.searchQuery)
	m.searchInput.CursorEnd()
	return m, m.searchInput.Focus()
}
