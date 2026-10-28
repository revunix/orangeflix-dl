package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// helpEntry is one group of the help window.
type helpEntry struct {
	// title names the screen the keys belong to.
	title string
	// keys are the shortcuts of the screen.
	keys [][2]string
}

// helpEntries returns the shortcuts of the program. The group of the screen
// that is open comes first.
func (m *Model) helpEntries() []helpEntry {
	entries := []helpEntry{
		{"Formate", [][2]string{
			{"enter", "öffnen"},
			{"/", "filtern"},
			{"f", "Volltextsuche"},
			{"b", "nur die Bibliothek"},
			{"a", "in Bibliothek merken"},
			{"n", "neue Folgen suchen"},
			{"d", "neue Folgen laden"},
			{"l", "Protokoll ein/aus"},
			{"s", "Zugangsdaten"},
			{"p", "Speicherort"},
		}},
		{"Folgen", [][2]string{
			{"enter", "Menü: laden/ansehen"},
			{"space", "markieren"},
			{"a", "alle markieren"},
			{"d", "laden"},
			{"w", "mit mpv ansehen"},
			{"J", "nach Jahr filtern"},
			{"i", "Qualität wählen"},
			{"p", "Speicherort"},
			{"y", "Titel kopieren"},
			{"x", "Datei löschen"},
		}},
		{"Suche", [][2]string{
			{"enter", "suchen"},
		}},
		{"Downloads", [][2]string{
			{"l", "Protokoll"},
			{"y", "Pfad kopieren"},
			{"o", "Ordner öffnen"},
			{"x", "Datei löschen"},
		}},
		{"Anmeldung", [][2]string{
			{"tab", "nächstes Feld"},
			{"enter", "anmelden"},
			{"ctrl+t", "Erscheinungsbild"},
		}},
		{"Allgemein", [][2]string{
			{"↑ ↓", "wählen"},
			{"pgup pgdn", "seitenweise"},
			{"esc", "zurück"},
			{"?", "diese Hilfe"},
			{"q", "beenden"},
			{"ctrl+c", "Abbruch"},
		}},
	}
	current := m.helpGroup()
	for index, entry := range entries {
		if entry.title == current && index > 0 {
			entries = append([]helpEntry{entry}, append(entries[:index:index], entries[index+1:]...)...)
			break
		}
	}
	return entries
}

// helpGroup names the group of the help that belongs to the open screen.
func (m *Model) helpGroup() string {
	switch m.screen {
	case ScreenEpisodes:
		return "Folgen"
	case ScreenSearch:
		return "Suche"
	case ScreenQueue, ScreenSummary:
		return "Downloads"
	case ScreenStart:
		return "Anmeldung"
	}
	return "Formate"
}

// helpColumnGap is the space between two columns of the help.
const helpColumnGap = 3

// helpCurrent marks the group of the open screen.
const helpCurrent = " · hier"

// helpColumnWidth returns the width of the widest row of the groups.
func helpColumnWidth(entries []helpEntry) int {
	width := 0
	for _, entry := range entries {
		width = max(width, ansi.StringWidth(entry.title+helpCurrent)+2)
		keyWidth := helpKeyWidth(entry)
		for _, key := range entry.keys {
			width = max(width, keyWidth+2+ansi.StringWidth(key[1]))
		}
	}
	return width
}

// helpKeyWidth returns the width of the widest key of a group.
func helpKeyWidth(entry helpEntry) int {
	width := 0
	for _, key := range entry.keys {
		width = max(width, ansi.StringWidth(key[0]))
	}
	return width
}

// helpBlock renders one group: the title with a line up to the end of the
// column, then the keys in the accent color and the descriptions beside them.
func helpBlock(entry helpEntry, width int, current bool) []string {
	label := theme.Title.Bold(true).Render(entry.title)
	plain := entry.title
	if current {
		label = theme.AccentBold.Render(entry.title) + theme.Muted.Render(helpCurrent)
		plain += helpCurrent
	}
	rule := max(0, width-ansi.StringWidth(plain)-1)
	lines := []string{label + " " + theme.Divider.Render(strings.Repeat("─", rule))}
	keyWidth := helpKeyWidth(entry)
	for _, key := range entry.keys {
		lines = append(lines, theme.AccentBold.Render(padRight(key[0], keyWidth))+"  "+theme.Text.Render(key[1]))
	}
	return lines
}

// helpLines lays the groups out in as many columns as the width allows. The
// group of the open screen stands at the top left, the others fill up the
// shortest column, so the columns end at about the same height.
func (m *Model) helpLines(inner int) []string {
	entries := m.helpEntries()
	width := helpColumnWidth(entries)
	columns := max(1, (inner+helpColumnGap)/(width+helpColumnGap))
	columns = min(columns, len(entries))

	stacks := make([][]string, columns)
	place := func(column int, block []string) {
		if len(stacks[column]) > 0 {
			stacks[column] = append(stacks[column], "")
		}
		stacks[column] = append(stacks[column], block...)
	}
	place(0, helpBlock(entries[0], width, true))
	rest := append([]helpEntry{}, entries[1:]...)
	sort.SliceStable(rest, func(i, j int) bool { return len(rest[i].keys) > len(rest[j].keys) })
	for _, entry := range rest {
		shortest := 0
		for column := range stacks {
			if len(stacks[column]) < len(stacks[shortest]) {
				shortest = column
			}
		}
		place(shortest, helpBlock(entry, width, false))
	}

	height := 0
	for _, stack := range stacks {
		height = max(height, len(stack))
	}
	lines := make([]string, height)
	gap := strings.Repeat(" ", helpColumnGap)
	for row := range lines {
		parts := make([]string, 0, columns)
		for _, stack := range stacks {
			cell := ""
			if row < len(stack) {
				cell = stack[row]
			}
			parts = append(parts, padRight(cell, width))
		}
		lines[row] = strings.TrimRight(strings.Join(parts, gap), " ")
	}
	return lines
}

// helpPopup builds the window of the help. The groups stand in columns with
// the keys in the accent color; when the window is too low for all of them,
// the arrows scroll.
func (m *Model) helpPopup(width int) []string {
	lines := m.helpLines(helpInnerWidth(width))
	inner := 0
	for _, line := range lines {
		inner = max(inner, ansi.StringWidth(line))
	}

	foot := ""
	if m.height > 0 {
		// Die Kanten, die Kopfzeile und die Abstände brauchen sieben Zeilen,
		// über und unter dem Fenster bleibt ein Rand von zwei Zeilen.
		room := max(3, m.height-12)
		if total := len(lines); total > room {
			m.helpScroll = max(0, min(m.helpScroll, total-room))
			lines = lines[m.helpScroll : m.helpScroll+room]
			foot = fmt.Sprintf("↑ ↓ blättern · %d/%d", m.helpScroll+room, total)
		} else {
			m.helpScroll = 0
		}
	}
	return windowBoxFootColor(inner+4, "Hilfe", foot, theme.BoxList, lines)
}

// helpInnerWidth returns the room for the text inside the help window.
func helpInnerWidth(width int) int {
	// Das Fenster ist eine Karte über der Seite: es lässt links und rechts
	// Platz, damit die Kästen dahinter sichtbar bleiben.
	return max(20, width-10)
}

// dimText lays the scrim over a part of the page: the colors fall away and
// the text stands in one quiet color on the page. The page stays readable
// behind an open window, but steps back.
func dimText(text string) string {
	if text == "" {
		return ""
	}
	return withBackground(theme.Scrim.Render(ansi.Strip(text)), backgroundSequence())
}

// windowPadding is the room between the edge of a window and its text.
const windowPadding = 2

// windowBoxFootColor draws a window: a card with its own raised surface and
// sharp edges out of half blocks, a thin strip in the given color on top.
// The title stands in the first line with the key that closes the window on
// the right; the foot, when there is one, stands at the bottom in the muted
// color. The width includes the padding on both sides.
func windowBoxFootColor(width int, title, foot string, frame lipgloss.Style, content []string) []string {
	if width <= 0 {
		return nil
	}
	inner := max(1, width-2*windowPadding)
	page := backgroundSequence()
	background := windowBackgroundSequence()
	pad := strings.Repeat(" ", windowPadding)

	row := func(text string) string {
		text = fitToWidth(text, inner)
		text += strings.Repeat(" ", max(0, inner-ansi.StringWidth(text)))
		return pad + text + pad
	}
	body := []string{}
	// Kopfzeile: der Titel links, die Taste zum Schließen rechts.
	close := theme.Subtle.Render("esc")
	label := fitToWidth(theme.Title.Render(title), max(1, inner-5))
	body = append(body, row(label+strings.Repeat(" ", max(1, inner-ansi.StringWidth(label)-3))+close))
	body = append(body, row(""))
	for _, line := range content {
		body = append(body, row(line))
	}
	if foot != "" {
		body = append(body, row(""), row(theme.Subtle.Render(foot)))
	}
	body = append(body, row(""))

	// Der Inhalt der Kästen trägt den Hintergrund der Seite, im Fenster
	// gehört die gehobene Fläche hinein; nach jedem Reset wird sie neu
	// gesetzt, damit die Seite nicht durch die Lücken scheint.
	panel := sequenceOf(theme.Panel)
	for index, line := range body {
		if panel != "" {
			line = strings.ReplaceAll(line, panel, background)
		}
		body[index] = withBackground(line, background)
	}

	// Oben ein schmaler Streifen in der Farbe des Fensters, unten die Kante
	// der Fläche: Halbblöcke geben dem Fenster scharfe Ränder.
	top := withBackground(frame.Render(strings.Repeat("▄", width)), page)
	bottom := withBackground(theme.WindowEdge.Render(strings.Repeat("▀", width)), page)
	return append(append([]string{top}, body...), bottom)
}

// overlayCentered puts a window in the middle of the page. The window keeps
// its own background, the rest of the page stays as it is.
func overlayCentered(page, window []string, width int) []string {
	if len(window) == 0 {
		return page
	}
	result := append([]string{}, page...)

	windowWidth := 0
	for _, line := range window {
		if lineWidth := ansi.StringWidth(line); lineWidth > windowWidth {
			windowWidth = lineWidth
		}
	}
	left := (width - windowWidth) / 2
	if left < 0 {
		left = 0
	}
	top := (len(result) - len(window)) / 2
	if top < 0 {
		top = 0
	}

	for index, line := range window {
		position := top + index
		if position < 0 || position >= len(result) {
			continue
		}
		// Die Seite bleibt links und rechts vom Fenster stehen, unter dem
		// Schleier wie der Rest der Seite.
		row := result[position]
		leftPart := dimText(ansi.Cut(row, 0, left))
		rightPart := dimText(ansi.Cut(row, left+windowWidth, width))
		result[position] = leftPart + line + rightPart
	}
	return result
}
