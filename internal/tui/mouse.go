package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// zone is a place on the screen that answers the mouse. The zones are
// collected while the screen is drawn, so they always match what is seen.
type zone struct {
	y, x0, x1 int
	// left and right run on a click of the button; nil passes the click on
	// to the zones below, unless block is set.
	left  func() (tea.Model, tea.Cmd)
	right func() (tea.Model, tea.Cmd)
	// block keeps a click inside a window from reaching the page behind it.
	block bool
	// hover runs when the pointer moves onto the zone, like marking an
	// entry of a menu.
	hover func()
	// button gets the surface of a marked row while the pointer is on it.
	button bool
}

// popupRow is what one row of a window does with the mouse.
type popupRow struct {
	click clickFunc
	hover func()
}

// clickFunc is what a zone does.
type clickFunc = func() (tea.Model, tea.Cmd)

// addZone registers a place for the left button.
func (m *Model) addZone(y, x0, x1 int, left clickFunc) {
	m.zones = append(m.zones, zone{y: y, x0: x0, x1: x1, left: left})
}

// addButton registers a button: it is lit while the pointer is on it.
func (m *Model) addButton(y, x0, x1 int, left clickFunc) {
	m.zones = append(m.zones, zone{y: y, x0: x0, x1: x1, left: left, button: true})
}

// addRowZone registers one line of the content of a box. The line counts
// from the first line inside the frame.
func (m *Model) addRowZone(box rect, line int, left, right clickFunc) {
	if line < 0 || line >= box.innerHeight() || box.width <= 0 {
		return
	}
	y := m.bodyTop + box.y + 1 + line
	m.zones = append(m.zones, zone{y: y, x0: box.x, x1: box.x + box.width, left: left, right: right})
}

// menuAt is the right click on a row of a list: it marks the row and opens
// its menu at the pointer.
func (m *Model) menuAt(cursor *int, index int) clickFunc {
	return func() (tea.Model, tea.Cmd) {
		*cursor = index
		return m.openContextMenu(true)
	}
}

// pick is the click on a row of a list: the first click marks the row, a
// click on the marked row opens it with the given key.
func (m *Model) pick(cursor *int, index int, activate string) clickFunc {
	return func() (tea.Model, tea.Cmd) {
		if *cursor == index && activate != "" {
			return m.pressKey(activate)
		}
		*cursor = index
		return m, nil
	}
}

// handleMouse answers the mouse: the wheel moves through the list or the
// window under it, a click runs the zone under the pointer.
func (m *Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	m.lastMouse = time.Now()
	m.hoverX, m.hoverY = msg.X, msg.Y
	if msg.Action == tea.MouseActionMotion {
		// Die Bewegung wählt in Fenstern den Eintrag unter dem Zeiger; das
		// Neuzeichnen hebt Schaltflächen und Zeilen hervor.
		if z, ok := m.zoneAt(msg.X, msg.Y, func(z zone) bool { return z.hover != nil }); ok {
			z.hover()
		}
		return m, nil
	}
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return m.wheel("up")
	case tea.MouseButtonWheelDown:
		return m.wheel("down")
	case tea.MouseButtonLeft, tea.MouseButtonRight:
	default:
		return m, nil
	}
	right := msg.Button == tea.MouseButtonRight
	m.mouseX, m.mouseY = msg.X, msg.Y
	for index := len(m.zones) - 1; index >= 0; index-- {
		z := m.zones[index]
		if z.y != msg.Y || msg.X < z.x0 || msg.X >= z.x1 {
			continue
		}
		run := z.left
		if right {
			run = z.right
		}
		if run != nil {
			return run()
		}
		if z.block {
			return m, nil
		}
	}
	return m, nil
}

// zoneAt returns the topmost zone at a place that wants the event. A zone
// that blocks stops the search, so the page behind a window stays quiet.
func (m *Model) zoneAt(x, y int, wants func(zone) bool) (zone, bool) {
	for index := len(m.zones) - 1; index >= 0; index-- {
		z := m.zones[index]
		if z.y != y || x < z.x0 || x >= z.x1 {
			continue
		}
		if wants(z) {
			return z, true
		}
		if z.block {
			return zone{}, false
		}
	}
	return zone{}, false
}

// hovered tells whether the pointer stands on a line of a box.
func (m *Model) hovered(box rect, line int) bool {
	return m.hoverY == m.bodyTop+box.y+1+line && m.hoverX >= box.x && m.hoverX < box.x+box.width
}

// hoverRow marks the row of a list under the pointer with a thin bar in
// the margin. The marked row keeps its own bar.
func (m *Model) hoverRow(box rect, line int, row string, selected bool) string {
	if selected || !m.hovered(box, line) || !strings.HasPrefix(row, strings.Repeat(" ", rowIndent)) {
		return row
	}
	return theme.Subtle.Render("▎") + row[1:]
}

// highlightHover lights the button under the pointer: the cells get the
// surface and the accent of a marked row. Without colors nothing changes.
func (m *Model) highlightHover(lines []string, width int) {
	surface := sequenceOf(theme.Selected)
	if surface == "" || m.hoverY < 0 || m.hoverY >= len(lines) {
		return
	}
	z, ok := m.zoneAt(m.hoverX, m.hoverY, func(z zone) bool { return z.button })
	if !ok {
		return
	}
	line := lines[z.y]
	x1 := min(z.x1, ansi.StringWidth(line))
	if z.x0 >= x1 {
		return
	}
	middle := ansi.Strip(ansi.Cut(line, z.x0, x1))
	lit := withBackground(theme.AccentBold.Render(middle), surface) + "\x1b[0m"
	lines[z.y] = ansi.Cut(line, 0, z.x0) + "\x1b[0m" + lit + ansi.Cut(line, x1, width)
}

// wheel moves one step with the wheel, like the arrow keys.
func (m *Model) wheel(key string) (tea.Model, tea.Cmd) {
	if m.confirm != nil || m.screen == ScreenSplash {
		return m, nil
	}
	if m.screen == ScreenStart && !m.windowOpen() {
		return m, nil
	}
	// Im Chat blättert eine Raste drei Zeilen, wie in anderen Programmen.
	if m.screen == ScreenLive && !m.windowOpen() {
		for step := 0; step < 3; step++ {
			m.scrollChat(key)
		}
		return m, nil
	}
	return m.pressKey(key)
}

// popupPlace returns where overlayCentered puts a window.
func popupPlace(pageHeight, width int, window []string) (top, left, windowWidth int) {
	for _, line := range window {
		windowWidth = max(windowWidth, ansi.StringWidth(line))
	}
	left = max(0, (width-windowWidth)/2)
	top = max(0, (pageHeight-len(window))/2)
	return top, left, windowWidth
}

// windowContentRow is the line of the first content row inside a window:
// the strip on top, the title and an empty line stand above it.
const windowContentRow = 3

// popupZones makes a window answer the mouse: a click beside it closes it,
// a click into it stays inside, the "esc" in its title closes it and the
// rows run their own clicks. The zones of the page are dropped first.
func (m *Model) popupZones(pageHeight, width, top, left int, window []string, rows map[int]popupRow) {
	m.zones = m.zones[:0]
	closeWindow := func() (tea.Model, tea.Cmd) { return m.pressKey("esc") }
	for y := 0; y < pageHeight; y++ {
		m.zones = append(m.zones, zone{y: y, x0: 0, x1: width, left: closeWindow, right: closeWindow})
	}
	windowWidth := 0
	for _, line := range window {
		windowWidth = max(windowWidth, ansi.StringWidth(line))
	}
	for index := range window {
		m.zones = append(m.zones, zone{y: top + index, x0: left, x1: left + windowWidth, block: true})
	}
	// Das „esc“ steht rechts in der Titelzeile.
	m.addButton(top+1, left+windowWidth-windowPadding-3, left+windowWidth-windowPadding, closeWindow)
	for line, row := range rows {
		m.zones = append(m.zones, zone{
			y: top + windowContentRow + line, x0: left, x1: left + windowWidth,
			left: row.click, hover: row.hover, block: true,
		})
	}
}

// selectAndPress marks an entry of a window and presses a key on it.
func (m *Model) selectAndPress(cursor *int, index int, key string) clickFunc {
	return func() (tea.Model, tea.Cmd) {
		*cursor = index
		return m.pressKey(key)
	}
}

// entryRow is a row of a window: the pointer marks the entry, a click
// marks it and presses the key.
func (m *Model) entryRow(cursor *int, index int, key string) popupRow {
	return popupRow{
		click: m.selectAndPress(cursor, index, key),
		hover: func() { *cursor = index },
	}
}

// headerZones makes the tabs and the livestream in the head line clickable.
func (m *Model) headerZones(lines []string, spots []hintSpot, now time.Time) {
	for _, spot := range spots {
		key := spot.run
		m.addButton(1, spot.x0, spot.x1, func() (tea.Model, tea.Cmd) { return m.switchTab(key) })
	}
	hint := m.nextLiveHint(now)
	if hint == "" || len(lines) == 0 {
		return
	}
	plain := ansi.Strip(lines[0])
	probe := []rune(hint)
	if len(probe) > 8 {
		probe = probe[:8]
	}
	index := strings.Index(plain, string(probe))
	if index < 0 {
		return
	}
	x0 := ansi.StringWidth(plain[:index])
	x1 := min(x0+ansi.StringWidth(hint), ansi.StringWidth(plain))
	m.addButton(0, x0, x1, func() (tea.Model, tea.Cmd) { return m.switchTab("5") })
}
