package tui

import (
	"sort"
	"strings"
)

// rect describes the place of a box on the screen.
type rect struct {
	x      int
	y      int
	width  int
	height int
}

// The shares of the screen that the boxes take. They follow the boxes of
// btop: the list and the details share the upper part, the log and the
// account the lower one.
const (
	// margin is the free border around the boxes.
	margin = 1
	// gap is the space between two boxes.
	gap = 1
	// listShare is the part of the width the list takes.
	listShare = 0.62
	// topShare is the part of the height the upper boxes take.
	topShare = 0.66
	// minBoxHeight is the smallest height a box may have.
	minBoxHeight = 5
	// minListHeight is the smallest height of the list.
	minListHeight = 8
	// twoColumnWidth is the width from which the screen shows two columns.
	twoColumnWidth = 100
)

// layoutRects calculates the places of the four boxes. On a narrow terminal
// the screen shows the list alone.
func layoutRects(width, height int, twoColumns bool) (list, detail, logBox, info rect) {
	inner := width - 2*margin
	if inner < 20 {
		inner = max(20, width)
	}
	available := height - 2*margin
	if available < 8 {
		available = max(8, height)
	}

	if !twoColumns {
		// Nur eine Spalte: die Liste füllt den Bildschirm.
		list = rect{x: margin, y: margin, width: inner, height: available}
		return list, rect{}, rect{}, rect{}
	}

	listWidth := int(float64(inner-gap) * listShare)
	detailWidth := inner - gap - listWidth
	topHeight := int(float64(available) * topShare)
	// Der Abstand zwischen den Kästen zählt von der Höhe ab.
	bottomHeight := available - gap - topHeight
	if bottomHeight < minBoxHeight {
		bottomHeight = minBoxHeight
		topHeight = available - gap - bottomHeight
	}

	list = rect{x: margin, y: margin, width: listWidth, height: max(minListHeight, topHeight)}
	detail = rect{x: margin + listWidth + gap, y: margin, width: detailWidth, height: max(minListHeight, topHeight)}
	logBox = rect{x: margin, y: margin + topHeight + gap, width: listWidth, height: max(minBoxHeight, bottomHeight)}
	info = rect{x: margin + listWidth + gap, y: margin + topHeight + gap, width: detailWidth, height: max(minBoxHeight, bottomHeight)}
	return list, detail, logBox, info
}

// innerWidth returns the width of the text inside a box.
func (r rect) innerWidth() int {
	return max(1, r.width-4)
}

// innerHeight returns the number of lines that fit into a box. The frame
// takes two lines, the rest is the space for the content.
func (r rect) innerHeight() int {
	return max(1, r.height-2)
}

// piece is one part of a row of the screen. The text keeps its colors, so a
// box that is placed later does not take the colors of the earlier ones away.
type piece struct {
	col  int
	text string
}

// screenRow holds the pieces of one row of the screen.
type screenRow struct {
	pieces []piece
}

// add puts a piece at a column.
func (r *screenRow) add(col int, text string) {
	if text == "" {
		return
	}
	r.pieces = append(r.pieces, piece{col: col, text: text})
}

// render builds the row. The gaps between the pieces are filled with spaces.
func (r screenRow) render(width int) string {
	if len(r.pieces) == 0 {
		return strings.Repeat(" ", width)
	}
	// Die Stücke stehen der Reihe nach; die Kästen überlappen sich nicht.
	ordered := make([]piece, len(r.pieces))
	copy(ordered, r.pieces)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].col < ordered[j].col })

	var b strings.Builder
	column := 0
	for _, item := range ordered {
		if item.col > column {
			b.WriteString(strings.Repeat(" ", item.col-column))
			column = item.col
		}
		b.WriteString(item.text)
		column += visibleWidth(item.text)
	}
	if column < width {
		b.WriteString(strings.Repeat(" ", width-column))
	}
	return b.String()
}

// place puts the lines of a box at its place in the screen.
func place(screen []screenRow, box rect, lines []string) []screenRow {
	for index, line := range lines {
		row := box.y + index
		if row < 0 || row >= len(screen) {
			continue
		}
		// Der Inhalt, der über die Höhe der Box hinausgeht, fällt weg.
		if index >= box.height {
			break
		}
		screen[row].add(box.x, line)
	}
	return screen
}
