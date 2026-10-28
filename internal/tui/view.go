package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// View renders the current screen.
func (m *Model) View() string {
	if m.quitting {
		return ""
	}
	// Der Zähler treibt das kleine Logo in der Kopfzeile.
	irisFrameCounter.Add(1)
	if m.width > 0 && (m.width < 40 || (m.height > 0 && m.height < 12)) {
		return fitToWidth(centerLine("Terminal zu klein (mindestens 40x12).", m.width), m.width)
	}
	width := m.frameWidth()
	height := m.height
	if height <= 0 {
		height = 24
	}

	header := m.header(width)
	footer := m.footer(width)

	// Der Inhalt bekommt den Platz zwischen Kopf- und Fußzeile.
	bodyHeight := height - len(header) - len(footer)
	if bodyHeight < 3 {
		bodyHeight = 3
	}
	body := m.body(width, bodyHeight)

	// Der Rand zwischen Kopfzeile, Raster und Fußzeile kommt aus dem Raster.
	lines := append([]string{}, header...)
	lines = append(lines, body...)
	lines = append(lines, footer...)

	// Die Seite wird zuerst gemalt, dann liegen die Fenster darüber. So behält
	// das Fenster seinen eigenen Hintergrund. Die Seite bleibt unverändert:
	// sie neu zu tönen würde bei jedem Öffnen den ganzen Bildschirm neu
	// zeichnen und flackern.
	lines = m.paintRows(lines)
	// Solange ein Fenster offen ist, liegt ein Schleier über der Seite: sie
	// bleibt lesbar, das Fenster steht davor.
	if m.showHelp || m.showQuality || m.showPath || m.showAction {
		for index := range lines {
			lines[index] = dimText(lines[index])
		}
	}
	if m.showHelp {
		lines = overlayCentered(lines, m.helpPopup(width), width)
	}
	if m.showQuality {
		lines = overlayCentered(lines, m.qualityPopup(width), width)
	}
	if m.showAction {
		lines = overlayCentered(lines, m.actionPopup(width), width)
	}
	if m.showPath {
		lines = overlayCentered(lines, m.pathPopup(width), width)
	}
	return strings.Join(lines, "\n")
}

// header builds the head line of the screen: the brand, the path, the status
// and the clock.
func (m *Model) header(width int) []string {
	clock := time.Now().Format("15:04")
	return renderHeader(width, m.breadcrumb(), m.statusChip(), m.statusLine(), clock)
}

// footer builds the foot line of the screen. The message of the Programm
// belongs into the bar: it stands on the right of the keys, and when there
// is no room it gets a bar of its own above them.
func (m *Model) footer(width int) []string {
	// Die Fußzeile hält denselben Rand wie die Kästen.
	lines := renderFooter(width-2*margin, m.hints())
	for index := range lines {
		lines[index] = strings.Repeat(" ", margin) + lines[index]
	}
	if m.message == "" {
		return lines
	}

	style := theme.Accent
	marker := "●"
	if m.isError {
		style = theme.Danger
		marker = "✗"
	}
	text := style.Render(marker + " " + m.message)

	// Passt der Text neben die Tasten, steht er rechts in der Leiste.
	last := lines[len(lines)-1]
	free := width - ansi.StringWidth(last) - ansi.StringWidth(text) - 1
	if free >= 1 {
		lines[len(lines)-1] = last + strings.Repeat(" ", free) + text
		return lines
	}

	// Sonst bekommt der Text eine eigene Leiste über den Tasten: ein
	// farbiger Balken am Rand und der Text auf eigener Fläche.
	return append([]string{m.messageBar(width, m.message, style)}, lines...)
}

// messageBar renders the message as a bar over the full width. The bar has
// its own surface, so it stands clearly above the keys.
func (m *Model) messageBar(width int, message string, style lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	mark := style.Render("▌")
	text := theme.Text.Render(fitToWidth(message, max(0, width-3)))
	line := mark + " " + text
	if padding := width - ansi.StringWidth(line); padding > 0 {
		line += strings.Repeat(" ", padding)
	}
	return withBackground(line, sequenceOf(theme.Bar))
}

// body renders the boxes of the screen. The places come from the layout, the
// boxes are drawn into a blank screen afterwards.
func (m *Model) body(width, height int) []string {
	list, detail, logBox, info := layoutRects(width, height, width >= twoColumnWidth)

	screen := make([]screenRow, height)

	switch m.screen {
	case ScreenSplash:
		// Der Splash nimmt die ganze Höhe unter der Kopfzeile, das Logo
		// bekommt den Platz.
		box := rect{x: margin, y: 2, width: width - 2*margin, height: height - 3}
		if box.height < 8 {
			box.height = 8
		}
		return m.render(place(screen, box, m.splashPanel(box)), width)
	case ScreenStart:
		// Die Anmeldung ist eine schmale Karte in der Mitte.
		cardWidth := min(78, width-2*margin)
		box := rect{x: (width - cardWidth) / 2, y: max(margin+2, (height-16)/2), width: cardWidth, height: 12}
		return m.render(place(screen, box, m.startPanel(box.width)), width)
	case ScreenQueue:
		box, logBox, info := m.wideBoxes(width, height)
		screen = place(screen, box, m.queuePanel(box))
		screen = m.placeLowerBoxes(screen, logBox, info)
		return m.render(screen, width)
	case ScreenSummary:
		box, logBox, info := m.wideBoxes(width, height)
		screen = place(screen, box, m.summaryPanel(box))
		screen = m.placeLowerBoxes(screen, logBox, info)
		return m.render(screen, width)
	case ScreenSearch:
		box, logBox, info := m.wideBoxes(width, height)
		screen = place(screen, box, m.searchPanel(box))
		screen = m.placeLowerBoxes(screen, logBox, info)
		return m.render(screen, width)
	}

	// Die Listen und die Angaben stehen im Raster, darunter das Protokoll
	// und das Konto.
	// Der Rahmen und der Rand nehmen je zwei Zeichen weg.
	m.formatWidth = list.innerWidth() - 2
	m.episodeWidth = list.innerWidth() - 2

	switch m.screen {
	case ScreenFormats:
		screen = place(screen, list, m.formatsPanel(list))
		screen = place(screen, detail, m.formatDetail(detail))
	case ScreenEpisodes:
		screen = place(screen, list, m.episodesPanel(list))
		screen = place(screen, detail, m.episodeDetail(detail))
	default:
		screen = place(screen, list, m.formatsPanel(list))
		screen = place(screen, detail, m.formatDetail(detail))
	}

	if m.showLogBox {
		screen = place(screen, logBox, m.logPanel(logBox))
		screen = place(screen, info, m.infoPanel(info))
	}
	return m.render(screen, width)
}

// render builds the rows of the screen out of their pieces.
func (m *Model) render(screen []screenRow, width int) []string {
	rows := make([]string, 0, len(screen))
	for _, row := range screen {
		rows = append(rows, row.render(width))
	}
	return rows
}

// wideBoxes calculates the places for the screens with one big box on top
// and the protocol and the account below it.
func (m *Model) wideBoxes(width, height int) (box, logBox, info rect) {
	inner := width - 2*margin
	available := height - 2*margin
	lower := 0
	if m.showLogBox && width >= twoColumnWidth {
		lower = available / 4
	}
	box = rect{x: margin, y: margin, width: inner, height: available - lower}
	if lower <= 0 {
		return box, rect{}, rect{}
	}
	left, _ := splitWidths(inner)
	logBox = rect{x: margin, y: margin + box.height + gap, width: left, height: lower - gap}
	info = rect{x: margin + left + gap, y: margin + box.height + gap, width: inner - left - gap, height: lower - gap}
	return box, logBox, info
}

// placeLowerBoxes puts the protocol and the account below the big box.
func (m *Model) placeLowerBoxes(screen []screenRow, logBox, info rect) []screenRow {
	if logBox.width <= 0 {
		return screen
	}
	screen = place(screen, logBox, m.logPanel(logBox))
	screen = place(screen, info, m.infoPanel(info))
	return screen
}

// withBackground puts the surface color behind a rendered line. Every reset
// inside the line starts the surface color again: without that the color would
// end at the first styled part of the line.
func withBackground(line, background string) string {
	if background == "" {
		return line
	}
	return strings.ReplaceAll(background+line, "\x1b[0m", "\x1b[0m"+background)
}

// paintRows fits every line to the width of the terminal and paints the
// surface color of the color scheme behind it.
func (m *Model) paintRows(lines []string) []string {
	width := m.frameWidth()
	background := backgroundSequence()
	painted := make([]string, 0, len(lines))
	for _, line := range lines {
		line = fitToWidth(line, width)
		// The line is filled up to the width of the terminal, so the surface
		// of the interface reaches the edge.
		padding := width - ansi.StringWidth(line)
		if padding > 0 {
			line += strings.Repeat(" ", padding)
		}
		if background != "" {
			line = withBackground(line, background)
		}
		painted = append(painted, line)
	}
	return painted
}

// centerLine puts a text in the middle of a line.
func centerLine(value string, width int) string {
	if width <= 0 {
		return value
	}
	gap := width - ansi.StringWidth(value)
	if gap <= 0 {
		return fitToWidth(value, width)
	}
	left := gap / 2
	return strings.Repeat(" ", left) + value + strings.Repeat(" ", gap-left)
}
