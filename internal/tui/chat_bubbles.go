package tui

import (
	"hash/fnv"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// chatGroupGap is the time within which the messages of one sender share a
// bubble head, like in a messenger.
const chatGroupGap = 3 * time.Minute

// chatGrouped tells whether a message follows the one before from the same
// sender: it then needs no head and no gap.
func (m *Model) chatGrouped(index int) bool {
	if index <= 0 || index >= len(m.chatMessages) {
		return false
	}
	previous, current := m.chatMessages[index-1], m.chatMessages[index]
	if previous.system || current.system || current.replyTo != "" {
		return false
	}
	if previous.from != current.from || previous.self != current.self || previous.studio != current.studio {
		return false
	}
	if previous.time.IsZero() || current.time.IsZero() {
		return true
	}
	gap := current.time.Sub(previous.time)
	return gap >= 0 && gap < chatGroupGap
}

// chatMessageLines draws one message as a bubble on the left, with the
// avatar of the sender beside it; the own ones stand in the warm color. A
// gap stands above the first bubble of a sender; an empty string is such a
// gap.
func (m *Model) chatMessageLines(index, width int) []string {
	message := m.chatMessages[index]
	lines := []string{}
	if index > 0 && !m.chatGrouped(index) {
		lines = append(lines, "")
	}
	if message.system {
		for _, part := range wrapText(message.text, max(4, width-4)) {
			lines = append(lines, centerLine(theme.Subtle.Render("· "+part+" ·"), width))
		}
		return lines
	}

	// Jede Nachricht trägt links den Avatar, auch die eigene; die Blase
	// rückt dafür ein.
	avatar := m.avatarsShown()
	room := 0
	if avatar {
		room = avatarRoom
	}
	// Die Blase nimmt höchstens drei Viertel der Breite; der Rand innen ist
	// ein Zeichen auf jeder Seite.
	maxText := max(10, (width-room)*3/4-2)
	rows := []string{}
	if !m.chatGrouped(index) {
		rows = append(rows, m.bubbleHead(message))
	}
	if message.replyTo != "" {
		rows = append(rows, m.bubbleQuote(message, maxText))
	}
	style := theme.Text
	if message.pending {
		style = theme.Muted
	}
	for _, part := range wrapText(message.text, maxText) {
		rows = append(rows, style.Render(part))
	}
	if len(rows) == 0 {
		rows = append(rows, "")
	}
	inner := 0
	for _, row := range rows {
		inner = max(inner, ansi.StringWidth(row))
	}
	// Die Kopfzeile steht über die ganze Breite der Blase: der Name links,
	// die Uhrzeit rechts.
	if !m.chatGrouped(index) {
		rows[0] = m.bubbleHeadFit(message, inner)
		inner = max(inner, ansi.StringWidth(rows[0]))
	}

	surface := m.bubbleSurface(message)
	// Der Avatar steht neben den ersten Zeilen der ersten Blase einer
	// Person; die folgenden Zeilen und Blasen rücken bündig ein.
	var face []string
	if avatar && !m.chatGrouped(index) {
		face = m.avatarRows(m.avatarName(message))
	}
	for number, row := range rows {
		lead := ""
		if avatar {
			lead = strings.Repeat(" ", avatarRoom)
			if number < len(face) {
				lead = face[number] + " "
			}
		}
		lines = append(lines, lead+bubbleRow(row, inner, surface))
	}
	if chips := m.reactionChips(message, width-2-room); chips != "" {
		lines = append(lines, strings.Repeat(" ", 1+room)+chips)
	}
	return lines
}

// bubbleSurface is the color of the bubble of a message.
func (m *Model) bubbleSurface(message chatLine) string {
	switch {
	case message.id != "" && message.id == m.chatSelectedID:
		return sequenceOf(theme.BubbleChosen)
	case message.self:
		return sequenceOf(theme.BubbleSelf)
	}
	return sequenceOf(theme.Bubble)
}

// bubbleRow pads one row of a bubble to its width and paints the surface
// behind it. Without colors a thin edge stands on its left.
func bubbleRow(row string, inner int, surface string) string {
	padded := row + strings.Repeat(" ", max(0, inner-ansi.StringWidth(row)))
	if surface == "" {
		return "│" + padded + " "
	}
	return withBackground(" "+padded+" ", surface) + "\x1b[0m"
}

// chatName is the shown name of a sender.
func chatName(message chatLine) string {
	switch {
	case message.self:
		return "Du"
	case message.from == "":
		return "?"
	}
	return message.from
}

// chatClock is the time of a message, or the mark of a pending one.
func chatClock(message chatLine) string {
	switch {
	case message.pending:
		return "…"
	case message.time.IsZero():
		return ""
	}
	return message.time.Local().Format("15:04")
}

// bubbleHead is the head of a bubble at its smallest width; it is drawn
// again over the whole bubble once the width is known.
func (m *Model) bubbleHead(message chatLine) string {
	return m.bubbleHeadFit(message, 0)
}

// bubbleHeadFit draws the head of a bubble: the name in its color on the
// left, the time on the right, over the given width.
func (m *Model) bubbleHeadFit(message chatLine, width int) string {
	name := chatName(message)
	if message.admin {
		name += " ★"
	}
	if message.studio {
		name += " → Studio"
	}
	clock := chatClock(message)
	nameWidth := max(4, min(24, ansi.StringWidth(name)))
	name = truncateColumn(name, nameWidth)
	gap := max(2, width-ansi.StringWidth(name)-ansi.StringWidth(clock))
	return chatNameStyle(message).Render(name) + strings.Repeat(" ", gap) + theme.Subtle.Render(clock)
}

// chatNameStyle colors a sender: the account in the brand color, the admins
// in the warning color, everybody else in a color of their own.
func chatNameStyle(message chatLine) lipgloss.Style {
	switch {
	case message.studio:
		return lipgloss.NewStyle().Foreground(colorOf(colorWarning)).Bold(true)
	case message.self:
		return theme.AccentBold
	case message.admin:
		return lipgloss.NewStyle().Foreground(colorOf(colorWarning)).Bold(true)
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(message.from))
	color := chatNameColors[hash.Sum32()%uint32(len(chatNameColors))]
	return lipgloss.NewStyle().Foreground(colorOf(color)).Bold(true)
}

// bubbleQuote is the quote of the answered message inside the bubble.
func (m *Model) bubbleQuote(message chatLine, maxText int) string {
	bar := theme.Accent.Render("▎")
	index := m.chatIndex(message.replyTo)
	if index < 0 {
		return bar + theme.Muted.Render(truncateColumn("Antwort auf eine ältere Nachricht", maxText-1))
	}
	target := m.chatMessages[index]
	name := truncateColumn(chatName(target), max(4, maxText/3))
	text := truncateColumn(target.text, max(4, maxText-1-ansi.StringWidth(name)-2))
	return bar + chatNameStyle(target).Render(name) + theme.Muted.Render(": "+text)
}

// reactionChips draws the reactions below a bubble, every reaction a chip
// of its own; the account's stand out. Chips that do not fit are left out.
func (m *Model) reactionChips(message chatLine, width int) string {
	keys := m.reactionOrder(message.reactions)
	if len(keys) == 0 {
		return ""
	}
	chip := sequenceOf(theme.Reaction)
	mineChip := sequenceOf(theme.ReactionMine)
	parts := []string{}
	used := 0
	for _, key := range keys {
		label := m.reactionEmoji(key) + " " + strconv.Itoa(message.reactions[key])
		cellWidth := ansi.StringWidth(label) + 2
		if used+cellWidth+1 > width {
			break
		}
		used += cellWidth + 1
		mine := message.mine[key]
		switch {
		case chip == "":
			// Ohne Farben zeigen Klammern die Chips, Sterne die eigenen.
			if mine {
				parts = append(parts, "*"+label+"*")
			} else {
				parts = append(parts, "["+label+"]")
			}
		case mine:
			parts = append(parts, withBackground(" "+theme.AccentBold.Render(label)+" ", mineChip)+"\x1b[0m")
		default:
			parts = append(parts, withBackground(" "+theme.Muted.Render(label)+" ", chip)+"\x1b[0m")
		}
	}
	return strings.Join(parts, " ")
}
