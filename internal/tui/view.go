package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

	// Die Klickflächen entstehen beim Zeichnen neu.
	m.zones = m.zones[:0]
	header := m.header(width)
	m.bodyTop = len(header)
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

	// Die Tasten der Fußzeile lassen sich anklicken.
	footerY := len(lines) - 1
	for _, spot := range m.footerSpots {
		if spot.run == "" {
			continue
		}
		key := spot.run
		m.addButton(footerY, spot.x0, spot.x1, func() (tea.Model, tea.Cmd) { return m.pressKey(key) })
	}
	if m.screen == ScreenSplash {
		for y := range lines {
			m.addZone(y, 0, width, func() (tea.Model, tea.Cmd) { return m.pressKey(" ") })
		}
	}

	// Die Seite wird zuerst gemalt, dann liegen die Fenster darüber. So behält
	// das Fenster seinen eigenen Hintergrund. Die Seite bleibt unverändert:
	// sie neu zu tönen würde bei jedem Öffnen den ganzen Bildschirm neu
	// zeichnen und flackern.
	lines = m.paintRows(lines)
	// Solange ein Fenster offen ist, liegt ein Schleier über der Seite: sie
	// bleibt lesbar, das Fenster steht davor.
	if m.windowOpen() {
		for index := range lines {
			lines[index] = dimText(lines[index])
		}
	}
	// Die Fenster liegen in der Reihenfolge übereinander, in der sie die
	// Tasten bekommen: das oberste hat die Tastatur und die Maus.
	page := len(lines)
	overlayTo := func(window []string, top, left int, rows map[int]popupRow) {
		lines = overlayAt(lines, window, top, left, width)
		m.popupZones(page, width, top, left, window, rows)
	}
	overlay := func(window []string, rows map[int]popupRow) {
		top, left, _ := popupPlace(page, width, window)
		overlayTo(window, top, left, rows)
	}
	if m.showPath {
		window := m.pathPopup(width)
		rows := map[int]popupRow{}
		end := min(m.pathOffset+m.pathVisibleRows(), len(m.pathDirs))
		for index := m.pathOffset; index < end; index++ {
			folder := index
			rows[pathFirstRow+index-m.pathOffset] = popupRow{
				click: m.pick(&m.pathCursor, folder, "tab"),
				hover: func() { m.pathCursor = folder },
			}
		}
		overlay(window, rows)
	}
	if m.showParticipants {
		window := m.participantPopup(width)
		rows := map[int]popupRow{}
		if episode, ok := m.currentEpisode(); ok {
			for index := range episode.Participants {
				rows[2+index] = m.entryRow(&m.participantCursor, index, "enter")
			}
		}
		overlay(window, rows)
	}
	if m.showAction {
		window := m.actionPopup(width)
		rows := map[int]popupRow{}
		for index := range actionEntries() {
			rows[2+index] = m.entryRow(&m.actionCursor, index, "enter")
		}
		overlay(window, rows)
	}
	if m.showQuality {
		window := m.qualityPopup(width)
		rows := map[int]popupRow{}
		for index := range m.qualityEntries() {
			rows[1+index] = m.entryRow(&m.qualityCursor, index, "enter")
		}
		overlay(window, rows)
	}
	if m.showHelp {
		overlay(m.helpPopup(width), nil)
	}
	if m.showPalette {
		window := m.palettePopup(width)
		rows := map[int]popupRow{}
		end := min(m.paletteScroll+m.paletteRows(), len(m.filteredPaletteItems()))
		for index := m.paletteScroll; index < end; index++ {
			item := index
			rows[paletteFirstRow+index-m.paletteScroll] = popupRow{
				click: func() (tea.Model, tea.Cmd) { return m.runPaletteItem(item) },
				hover: func() { m.paletteCursor = item },
			}
		}
		overlay(window, rows)
	}
	if m.context != nil {
		window := m.contextPopup()
		top, left := m.contextPlace(page, width, window)
		rows := map[int]popupRow{}
		for index := range m.context.items {
			item := index
			rows[index] = popupRow{
				click: func() (tea.Model, tea.Cmd) { return m.runContextItem(item) },
				hover: func() { m.context.cursor = item },
			}
		}
		overlayTo(window, top, left, rows)
	}
	if m.confirm != nil {
		window := m.confirmPopup(width)
		first := m.confirmFirstRow(width)
		rows := map[int]popupRow{
			first:     m.entryRow(&m.confirm.cursor, 0, "enter"),
			first + 1: m.entryRow(&m.confirm.cursor, 1, "enter"),
		}
		overlay(window, rows)
	}
	// Die Schaltfläche unter dem Zeiger leuchtet.
	m.highlightHover(lines, width)
	return strings.Join(lines, "\n")
}

// windowOpen tells whether a window lies over the page.
func (m *Model) windowOpen() bool {
	return m.showHelp || m.showQuality || m.showPath || m.showAction || m.showParticipants ||
		m.showPalette || m.confirm != nil || m.context != nil
}

// header builds the head line of the screen: the brand, the path, the next
// livestream, the status and the clock.
func (m *Model) header(width int) []string {
	now := time.Now()
	lines := renderHeader(width, m.breadcrumb(), m.statusChip(), m.statusLine(), m.nextLiveHint(now), now.Format("15:04"))
	// Die Linie unter dem Kopf trägt die Reiter.
	if m.showsTabs() && len(lines) > 1 {
		bar, spots := m.tabBar(width)
		lines[1] = bar
		m.headerZones(lines, spots, now)
	}
	return lines
}

// tabBar draws the tabs into the rule below the head line. The open tab
// stands in the accent color on the surface of a marked row. When the
// terminal is narrow the labels get shorter, at last only the digits stay.
func (m *Model) tabBar(width int) (string, []hintSpot) {
	tabs := m.tabs()
	short := map[string]string{"1": "Formate", "2": "Weiter", "3": "Merkliste", "4": "Kalender", "5": "Live", "6": "Downloads"}
	labels := func(mode int) []string {
		out := make([]string, len(tabs))
		for index, tab := range tabs {
			switch mode {
			case 0:
				out[index] = " " + tab.key + " " + tab.label + " "
			case 1:
				out[index] = " " + tab.key + " " + short[tab.key] + " "
			default:
				out[index] = " " + tab.key + " "
			}
		}
		return out
	}
	var cells []string
	for mode := 0; mode < 3; mode++ {
		cells = labels(mode)
		total := margin + 1
		for _, cell := range cells {
			total += ansi.StringWidth(cell) + 1
		}
		if total <= width {
			break
		}
	}

	active := m.activeTab()
	var b strings.Builder
	spots := []hintSpot{}
	column := margin + 1
	b.WriteString(theme.Divider.Render(strings.Repeat(boxHorizontal, column)))
	for index, tab := range tabs {
		cell := cells[index]
		cellWidth := ansi.StringWidth(cell)
		if column+cellWidth > width {
			break
		}
		if tab.key == active {
			if background := sequenceOf(theme.Selected); background != "" {
				b.WriteString(withBackground(theme.AccentBold.Render(cell), background) + "\x1b[0m")
			} else {
				// Ohne Farben zeigen Klammern den offenen Reiter.
				b.WriteString("[" + cell[1:len(cell)-1] + "]")
			}
		} else {
			key := strings.Index(cell, tab.key)
			b.WriteString(theme.Subtle.Render(cell[:key+len(tab.key)]) + theme.Muted.Render(cell[key+len(tab.key):]))
		}
		spots = append(spots, hintSpot{x0: column, x1: column + cellWidth, run: tab.key})
		column += cellWidth
		if column < width {
			b.WriteString(theme.Divider.Render(boxHorizontal))
			column++
		}
	}
	if column < width {
		b.WriteString(theme.Divider.Render(strings.Repeat(boxHorizontal, width-column)))
	}
	return b.String(), spots
}

// footer builds the foot line of the screen. The message of the Programm
// belongs into the bar: it stands on the right of the keys, and when there
// is no room it gets a bar of its own above them.
func (m *Model) footer(width int) []string {
	// Die Fußzeile hält denselben Rand wie die Kästen. Eine Meldung steht
	// rechts daneben, solange dafür nur weniger wichtige Tasten weichen.
	hints := m.hints()
	room := width - 2*margin
	if m.message != "" {
		reduced := room - ansi.StringWidth("● "+m.message) - 2
		if footerKeepsMain(reduced, hints) {
			room = reduced
		}
	}
	line, spots := layoutFooter(room, hints)
	m.footerSpots = m.footerSpots[:0]
	for _, spot := range spots {
		m.footerSpots = append(m.footerSpots, hintSpot{x0: spot.x0 + margin, x1: spot.x1 + margin, run: spot.run})
	}
	lines := []string{strings.Repeat(" ", margin) + line}
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
		// Ein Klick auf ein Feld setzt den Cursor hinein.
		for field := 0; field < startFields; field++ {
			index := field
			m.addRowZone(box, 2+field, func() (tea.Model, tea.Cmd) {
				m.field = index
				return m, m.focusField()
			}, nil)
		}
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
	case ScreenCalendar:
		box, logBox, info := m.wideBoxes(width, height)
		screen = place(screen, box, m.calendarPanel(box))
		screen = m.placeLowerBoxes(screen, logBox, info)
		return m.render(screen, width)
	case ScreenLive:
		infoBox, pollBox, chatBox := m.liveBoxes(width, height)
		screen = place(screen, infoBox, m.livePanel(infoBox))
		if pollBox.height > 0 {
			screen = place(screen, pollBox, m.pollPanel(pollBox))
		}
		screen = place(screen, chatBox, m.chatPanel(chatBox))
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
	case ScreenClips:
		screen = place(screen, list, m.clipPanel(list))
		screen = place(screen, detail, m.clipDetailPanel(detail))
	case ScreenAnnouncements:
		screen = place(screen, list, m.announcementPanel(list))
		screen = place(screen, detail, m.announcementDetail(detail))
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

// liveBoxes places the chat, the stream information and the voting. On a
// wide terminal the chat takes the left two thirds over the whole height;
// the information stands beside it on the right, the voting under it. On a
// narrow one the information stands small above the chat, and the voting
// between them.
func (m *Model) liveBoxes(width, height int) (info, poll, chat rect) {
	inner := width - 2*margin
	available := height - 2*margin
	if width >= twoColumnWidth {
		infoWidth := max(30, min(44, inner*32/100))
		chatWidth := inner - gap - infoWidth
		chat = rect{x: margin, y: margin, width: chatWidth, height: available}
		info = rect{x: margin + chatWidth + gap, y: margin, width: infoWidth, height: available}
		if m.hasPoll() {
			// Die Live-Box braucht nicht die ganze Höhe, das Feld darunter
			// gehört der Abstimmung.
			infoHeight := max(9, (available-gap)*3/5)
			info.height = infoHeight
			poll = rect{x: margin + chatWidth + gap, y: margin + infoHeight + gap, width: infoWidth, height: available - infoHeight - gap}
		}
		return info, poll, chat
	}
	infoHeight := max(6, min(9, available/4))
	pollHeight := 0
	if m.hasPoll() {
		pollHeight = max(6, min(11, available/4))
		if infoHeight+pollHeight+2*gap+8 > available {
			pollHeight = max(5, (available-infoHeight-gap)*2/3)
		}
	}
	chatHeight := available - infoHeight - pollHeight - gap
	if pollHeight > 0 {
		chatHeight -= gap
	}
	if chatHeight < 8 {
		infoHeight = max(5, available/3)
		if pollHeight > 0 {
			pollHeight = max(5, (available-infoHeight-gap)*2/3)
		}
		chatHeight = available - infoHeight - pollHeight - gap
		if pollHeight > 0 {
			chatHeight -= gap
		}
	}
	if chatHeight < 4 {
		pollHeight = 0
		chatHeight = available - infoHeight - gap
	}
	info = rect{x: margin, y: margin, width: inner, height: infoHeight}
	if pollHeight > 0 {
		poll = rect{x: margin, y: margin + infoHeight + gap, width: inner, height: pollHeight}
		chat = rect{x: margin, y: margin + infoHeight + pollHeight + 2*gap, width: inner, height: chatHeight}
		return info, poll, chat
	}
	chat = rect{x: margin, y: margin + infoHeight + gap, width: inner, height: chatHeight}
	return info, poll, chat
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
