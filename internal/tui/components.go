package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// keyHint is one shortcut of the footer and of the command palette.
type keyHint struct {
	// Key is the key as the footer shows it ("↑↓", "enter").
	Key string
	// Label is the short text of the footer.
	Label string
	// Run is the key that is pressed when the hint is clicked or picked in
	// the palette; empty for hints that only explain, like the arrows.
	Run string
	// Long is the text of the palette; empty means Label.
	Long string
	// Prio orders the hints when the footer is too narrow: the higher the
	// number, the earlier the hint gives way.
	Prio int
}

// The priorities of the hints.
const (
	// prioAlways never leaves the footer.
	prioAlways = 0
	// prioMain is what the screen is for.
	prioMain = 1
	// prioUseful is shown when there is room.
	prioUseful = 2
	// prioExtra is shown on wide terminals.
	prioExtra = 3
	// prioPalette is only in the palette and in the help.
	prioPalette = 9
)

// hint builds a hint whose key is also the key that runs it.
func hint(key, label string, prio int) keyHint {
	return keyHint{Key: key, Label: label, Run: key, Prio: prio}
}

// hintLong builds a hint with a longer text for the palette.
func hintLong(key, label, long string, prio int) keyHint {
	return keyHint{Key: key, Label: label, Run: key, Long: long, Prio: prio}
}

// hintShow builds a hint that only explains and runs nothing.
func hintShow(key, label string, prio int) keyHint {
	return keyHint{Key: key, Label: label, Prio: prio}
}

// hintSpot is the place of a hint in the footer line.
type hintSpot struct {
	x0, x1 int
	run    string
}

// The characters of the boxes: thin lines with round corners, the title
// sits in the upper line.
const (
	boxHorizontal  = "─"
	boxVertical    = "│"
	boxTopLeft     = "╭"
	boxTopRight    = "╮"
	boxBottomLeft  = "╰"
	boxBottomRight = "╯"
	// meterSymbol fills the progress bar.
	meterSymbol = "■"
	// meterBackground is the empty part of the progress bar.
	meterBackground = "·"
)

// renderBox draws a box around the content. The title stands in the upper
// line, like the panels of btop.
func renderBox(width int, title string, content []string) []string {
	return renderBoxColor(width, title, theme.BoxList, content)
}

// renderBoxColor draws a box around the content with a frame of its own
// color.
func renderBoxColor(width int, title string, frame lipgloss.Style, content []string) []string {
	return renderBoxFootColor(width, title, "", frame, content)
}

// renderBoxAt draws a box of the given height. Empty lines fill the box up to
// its height, like the panels of btop.
func renderBoxAt(width, height int, title string, frame lipgloss.Style, content []string) []string {
	lines := renderBoxFootColor(width, title, "", frame, content)
	if height <= len(lines) {
		return lines
	}
	empty := frame.Render(boxVertical) +
		withBackground(strings.Repeat(" ", max(0, width-2)), sequenceOf(theme.Panel)) +
		frame.Render(boxVertical)
	filled := append([]string{}, lines[:len(lines)-1]...)
	for len(filled) < height-1 {
		filled = append(filled, empty)
	}
	return append(filled, lines[len(lines)-1])
}

// renderBoxFootColor draws a box around the content with a frame of its own
// color and a foot in the lower line.
func renderBoxFootColor(width int, title, foot string, frame lipgloss.Style, content []string) []string {
	if width <= 0 {
		return nil
	}
	// Der Rahmen braucht zwei Zeichen, dazu kommt je ein Leerzeichen
	// zwischen dem Rahmen und dem Text.
	inner := width - 4
	if inner < 1 {
		inner = 1
	}

	// Die obere Linie trägt den Titel: der Name hell, die Zahl dahinter
	// gedämpft, wie die Karten moderner Oberflächen.
	top := frame.Render(boxTopLeft + boxHorizontal)
	if title != "" {
		label := " " + boxTitle(title) + " "
		room := width - 4
		if ansi.StringWidth(label) > room {
			label = fitToWidth(label, max(1, room))
		}
		top += label
	}
	remaining := width - ansi.StringWidth(top) - 1
	if remaining < 0 {
		remaining = 0
	}
	top += frame.Render(strings.Repeat(boxHorizontal, remaining) + boxTopRight)

	lines := []string{top}
	for _, line := range content {
		text := fitToWidth(line, inner)
		padding := inner - ansi.StringWidth(text)
		if padding < 0 {
			padding = 0
		}
		middle := " " + text + strings.Repeat(" ", padding) + " "
		lines = append(lines, frame.Render(boxVertical)+
			withBackground(middle, sequenceOf(theme.Panel))+
			frame.Render(boxVertical))
	}
	bottom := frame.Render(boxBottomLeft + boxHorizontal)
	if foot != "" {
		label := " " + foot + " "
		if ansi.StringWidth(label) > width-4 {
			label = fitToWidth(label, max(1, width-4))
		}
		bottom += theme.Muted.Render(label)
	}
	rest := width - ansi.StringWidth(bottom) - 1
	if rest < 0 {
		rest = 0
	}
	bottom += frame.Render(strings.Repeat(boxHorizontal, rest) + boxBottomRight)
	lines = append(lines, bottom)
	return lines
}

// boxTitle renders the title of a box. A number in brackets at the end, like
// "Formate (52)", stands behind the name in the muted color.
func boxTitle(title string) string {
	if open := strings.LastIndex(title, " ("); open > 0 && strings.HasSuffix(title, ")") {
		count := title[open+2 : len(title)-1]
		if _, err := strconv.Atoi(count); err == nil {
			return theme.Title.Render(title[:open]) + " " + theme.Muted.Render(count)
		}
	}
	return theme.Title.Render(title)
}

// renderMeter draws a progress bar in the style of btop: the filled part
// carries the gradient of the brand color, the empty part is dotted.
func renderMeter(width int, percent int, label string) string {
	if width <= 0 {
		return ""
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	filled := percent * width / 100

	var b strings.Builder
	for index := 0; index < width; index++ {
		if index < filled {
			// Die Farbe läuft über die Breite von hell nach dunkel.
			position := (index + 1) * 100 / width
			b.WriteString(meterGradient(position).Render(meterSymbol))
			continue
		}
		b.WriteString(theme.Border.Render(meterBackground))
	}
	if label != "" {
		b.WriteString(" " + label)
	}
	return b.String()
}

// meterGradient returns the color of the meter at a position. The gradient
// runs from the light brand color to the dark one.
func meterGradient(position int) lipgloss.Style {
	start, middle, end := gradientEndpoints()
	if position > 66 {
		position = 66
	}
	if position < 1 {
		position = 1
	}
	color := interpolateHex(start, middle, end, position-1, 66)
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color))
}

// renderHeader returns the head line: on the left the mark and the name of
// the brand, then the path to the current view; on the right the next
// livestream, the context and the clock. A rule closes the head.
func renderHeader(width int, title, context, status, liveHint, clock string) []string {
	if width <= 0 {
		return []string{"", ""}
	}
	separator := theme.Divider.Render("  │  ")

	// Links das Zeichen und der Name der Marke, wie oben links auf der
	// Website; in schmalen Terminals bleibt nur der Name.
	head := theme.AccentBold.Render("orangeflix")
	if width >= 72 {
		head = irisMiniMark() + " " + head + theme.Muted.Render("-dl")
	}
	if title != "" {
		// Der Pfad: die Stufen davor gedämpft, die aktuelle hervorgehoben.
		steps := strings.Split(title, " › ")
		parts := make([]string, len(steps))
		for index, step := range steps {
			if index == len(steps)-1 {
				parts[index] = theme.Title.Bold(true).Render(step)
				continue
			}
			parts[index] = theme.Muted.Render(step)
		}
		head += separator + strings.Join(parts, theme.Subtle.Render(" › "))
	}
	if status != "" {
		head += "   " + theme.Accent.Render(status)
	}

	// Rechts der nächste Livestream, der Zusammenhang der Ansicht und die
	// Uhrzeit. Die Uhr bleibt stehen. Der Zusammenhang weicht zuerst, der
	// Hinweis wird gekürzt, wenn der Platz nicht reicht.
	clockStyled := ""
	if clock != "" {
		clockStyled = theme.Text.Render(clock)
	}
	hintStyled := ""
	if liveHint != "" {
		hintStyled = theme.Accent.Render(liveHint)
	}
	contextStyled := ""
	if context != "" {
		contextStyled = theme.Muted.Render(context)
	}
	sepWidth := ansi.StringWidth(separator)
	// Links bleiben mindestens ein paar Zeichen der Marke sichtbar.
	budget := width - 13
	if budget < ansi.StringWidth(clockStyled) {
		budget = max(0, width-1)
	}
	remain := budget
	if clockStyled != "" {
		remain -= ansi.StringWidth(clockStyled)
	}
	if hintStyled != "" {
		room := remain
		if clockStyled != "" {
			room -= sepWidth
		}
		if ansi.StringWidth(hintStyled) > room {
			if room < 8 {
				hintStyled = ""
			} else {
				hintStyled = fitToWidth(hintStyled, room)
			}
		}
		if hintStyled != "" {
			remain -= ansi.StringWidth(hintStyled)
			if clockStyled != "" {
				remain -= sepWidth
			}
		}
	}
	if contextStyled != "" {
		room := remain
		if hintStyled != "" || clockStyled != "" {
			room -= sepWidth
		}
		if ansi.StringWidth(contextStyled) > room {
			contextStyled = ""
		}
	}
	right := ""
	if hintStyled != "" {
		right = hintStyled
	}
	if contextStyled != "" {
		if right != "" {
			right += separator
		}
		right += contextStyled
	}
	if clockStyled != "" {
		if right != "" {
			right += separator
		}
		right += clockStyled
	}

	// Der linke Teil weicht, wenn der Platz nicht reicht; die Uhr bleibt.
	room := width - ansi.StringWidth(right) - 2
	if ansi.StringWidth(head) > room {
		head = fitToWidth(head, max(1, room))
	}
	gap := max(1, width-ansi.StringWidth(head)-ansi.StringWidth(right))
	line := head + strings.Repeat(" ", gap) + right
	return []string{fitToWidth(line, width), renderRule(width)}
}

// renderRule returns a horizontal rule of exactly the width.
func renderRule(width int) string {
	if width <= 0 {
		return ""
	}
	return theme.Divider.Render(strings.Repeat(boxHorizontal, width))
}

// renderFooter renders the key hints as keycaps in one line.
func renderFooter(width int, hints []keyHint) []string {
	line, _ := layoutFooter(width, hints)
	return []string{line}
}

// footerGap separates two hints of the footer.
const footerGap = "  ·  "

// footerKeepsMain tells whether the main hints fit into the width.
func footerKeepsMain(width int, hints []keyHint) bool {
	total := 0
	for _, item := range hints {
		if item.Prio > prioMain {
			continue
		}
		if total > 0 {
			total += ansi.StringWidth(footerGap)
		}
		total += ansi.StringWidth(item.Key) + 1 + ansi.StringWidth(item.Label)
	}
	return total <= width
}

// layoutFooter fits the hints into one line. When the line is too narrow
// the least important hints give way, the most important keep their order;
// the help key then says that there is more. The spots tell where each
// hint stands, so a click can run it.
func layoutFooter(width int, hints []keyHint) (string, []hintSpot) {
	if width <= 0 {
		return "", nil
	}
	shown := make([]keyHint, 0, len(hints))
	for _, item := range hints {
		if item.Prio < prioPalette {
			shown = append(shown, item)
		}
	}
	if len(shown) == 0 {
		return "", nil
	}
	plainWidth := func(items []keyHint) int {
		total := 0
		for index, item := range items {
			if index > 0 {
				total += ansi.StringWidth(footerGap)
			}
			total += ansi.StringWidth(item.Key) + 1 + ansi.StringWidth(item.Label)
		}
		return total
	}
	dropped := false
	for plainWidth(shown) > width {
		worst := -1
		for index, item := range shown {
			if item.Prio == prioAlways {
				continue
			}
			if worst < 0 || item.Prio >= shown[worst].Prio {
				worst = index
			}
		}
		if worst < 0 {
			break
		}
		shown = append(shown[:worst:worst], shown[worst+1:]...)
		dropped = true
	}
	if dropped {
		for index := range shown {
			if shown[index].Key == "?" {
				shown[index].Label = "mehr"
			}
		}
	}

	// Die Taste hell und fett, die Beschreibung gedämpft, ein Punkt trennt
	// die Paare.
	gap := theme.Divider.Render(footerGap)
	var b strings.Builder
	spots := make([]hintSpot, 0, len(shown))
	column := 0
	for index, item := range shown {
		if index > 0 {
			b.WriteString(gap)
			column += ansi.StringWidth(footerGap)
		}
		cell := theme.Title.Render(item.Key) + " " + theme.KeyLabel.Render(item.Label)
		cellWidth := ansi.StringWidth(item.Key) + 1 + ansi.StringWidth(item.Label)
		b.WriteString(cell)
		spots = append(spots, hintSpot{x0: column, x1: column + cellWidth, run: item.Run})
		column += cellWidth
	}
	return fitToWidth(b.String(), width), spots
}

// padStyled puts a styled text into a field of the given width. The padding
// is counted on the visible text: a colour in the text would otherwise shift
// the columns.
func padStyled(style lipgloss.Style, text string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(text) > width {
		text = ansi.Truncate(text, width, "…")
	}
	padding := width - ansi.StringWidth(text)
	return style.Render(text) + strings.Repeat(" ", padding)
}

// detailRow renders one line of the detail column: a fixed width label and
// the value behind it.
func detailRow(label, value string) string {
	if value == "" {
		return ""
	}
	return theme.Muted.Render(padRight(label, 13)) + " " + value
}

// rowIndent is the room in front of every line of a list: the marked line
// carries its bar there.
const rowIndent = 2

// renderRow renders a line of a list. The marked line has a bar in the brand
// color on the left and a background over its whole width.
func renderRow(text string, selected bool) string {
	return renderRowWidth(text, selected, 0)
}

// renderRowWidth renders a line of a list; the background of the marked line
// reaches the given width.
func renderRowWidth(text string, selected bool, width int) string {
	return renderRowOn(text, selected, width, theme.Selected)
}

// renderWindowRow renders a line of a list in a window. The marked line has
// a background of its own that stands out from the raised surface.
func renderWindowRow(text string, selected bool, width int) string {
	return renderRowOn(text, selected, width, theme.WindowSelected)
}

// renderRowOn renders a line of a list; the marked line gets the bar in the
// brand color and the given background over the width.
func renderRowOn(text string, selected bool, width int, background lipgloss.Style) string {
	if !selected {
		return strings.Repeat(" ", rowIndent) + text
	}
	line := theme.SelectedFg.Render(text)
	if padding := width - rowIndent - ansi.StringWidth(text); padding > 0 {
		line += strings.Repeat(" ", padding)
	}
	return withBackground(theme.Accent.Render("▌")+" "+line, sequenceOf(background))
}

// statusBadge renders the state of an episode.
func statusBadge(state string) string {
	switch state {
	case "heruntergeladen", "geladen":
		return theme.Success.Render("✓ heruntergeladen")
	case "gesehen":
		return theme.Accent.Render("✓ gesehen")
	case "neu":
		return theme.Warning.Render("● neu")
	case "offen":
		return theme.Muted.Render("○ offen")
	}
	if strings.HasPrefix(state, "▶ ") {
		return theme.Accent.Render(state)
	}
	return theme.Muted.Render(state)
}

// humanBytes formats a number of bytes.
func humanBytes(size int64) string {
	const unit = 1024
	if size <= 0 {
		return "-"
	}
	value := float64(size)
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	for _, name := range units {
		value /= unit
		if value < unit {
			if value >= 100 {
				return fmt.Sprintf("%.0f %s", value, name)
			}
			return fmt.Sprintf("%.1f %s", value, name)
		}
	}
	return fmt.Sprintf("%.1f PiB", value)
}
