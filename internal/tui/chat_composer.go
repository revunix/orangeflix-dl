package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// chatLimit is the longest message the chat takes.
const chatLimit = 500

// The height of the field while it has the keyboard: it grows with the
// text from two to four lines.
const (
	composerMinLines = 2
	composerMaxLines = 4
)

// quickEmojis are the emoji below the field; alt with a digit or a click
// puts one into the text.
var quickEmojis = []string{"😂", "👍", "❤️", "🔥", "😮", "😢", "👏", "🙏", "🎉"}

// composerRight is the room for the counter and the target on the right of
// the field.
const composerRight = 8

// newChatInput builds the field of the chat: several lines that wrap the
// text, no line numbers; enter sends instead of breaking the line.
func newChatInput() textarea.Model {
	input := textarea.New()
	input.ShowLineNumbers = false
	input.Prompt = ""
	input.Placeholder = "Nachricht an den Chat"
	// Über die Grenze hinaus darf man tippen; der Zähler wird rot, und das
	// Senden sagt, was zu viel ist.
	input.CharLimit = chatLimit + 200
	input.MaxHeight = composerMaxLines
	input.EndOfBufferCharacter = ' '
	input.KeyMap.InsertNewline.SetEnabled(false)
	input.SetHeight(composerMinLines)
	input.SetWidth(40)
	return input
}

// composerFieldLines is the height of the field: one line without the
// keyboard, with it as many as the text needs, two to four.
func (m *Model) composerFieldLines(width int) int {
	if !m.chatFocused {
		return 1
	}
	room := max(1, composerInputWidth(width)-1)
	needed := ansi.StringWidth(m.chatInput.Value())/room + 1
	return max(composerMinLines, min(composerMaxLines, needed))
}

// composerInputWidth is the room for the text in the field.
func composerInputWidth(width int) int {
	return max(4, width-2-composerRight-1)
}

// composerRows is the height of the whole field to write: the rule, the
// answer when there is one, the field and the row of the emoji.
func (m *Model) composerRows() int {
	width := m.chatComposerWidth
	if width <= 0 {
		width = 60
	}
	rows := 2 + m.composerFieldLines(width)
	if m.chatFocused && m.chatReplyTo != nil {
		rows++
	}
	return rows
}

// chatComposer draws the field to write, like the one of a messenger: a
// rule parts it from the messages, the field has a surface of its own with
// a bar in the color of its target, a counter of the characters and the
// key that sends; below it stand the emoji to pick. The spots tell where
// the emoji are in the last row.
func (m *Model) chatComposer(width int, selected bool) ([]string, []hintSpot) {
	m.chatComposerWidth = width
	rows := []string{m.composerRule(width)}
	surface := sequenceOf(theme.Bubble)
	if m.chatFocused && m.chatReplyTo != nil {
		rows = append(rows, composerRow(m.replyBar(width), width, surface))
	}
	for _, row := range m.composerField(width, selected) {
		rows = append(rows, composerRow(row, width, surface))
	}
	emoji, spots := m.emojiRow(width)
	rows = append(rows, composerRow(emoji, width, surface))
	return rows, spots
}

// composerRule is the line above the field. While the chat is scrolled
// back it says so on the right.
func (m *Model) composerRule(width int) string {
	if m.chatScroll <= 0 || m.chatFocused {
		return theme.Divider.Render(strings.Repeat(boxHorizontal, width))
	}
	label := fmt.Sprintf(" ↑ %d Zeilen zurück · end ", m.chatScroll)
	label = truncateColumn(label, max(0, width-4))
	left := max(0, width-ansi.StringWidth(label)-2)
	return theme.Divider.Render(strings.Repeat(boxHorizontal, left)) +
		theme.Accent.Render(label) +
		theme.Divider.Render(strings.Repeat(boxHorizontal, max(0, width-left-ansi.StringWidth(label))))
}

// targetStyle is the color of the target: the brand for the chat, the
// warning color for the studio.
func (m *Model) targetStyle() lipgloss.Style {
	if m.chatToStudio {
		return theme.Warning
	}
	return theme.Accent
}

// composerField is the field. Without the keyboard it is one row that says
// what a key does: c to write, or the keys of the chosen message.
func (m *Model) composerField(width int, selected bool) []string {
	if !m.chatFocused {
		bar := theme.Subtle.Render("▏")
		text, key := "Nachricht schreiben …", "c"
		if m.chatToStudio {
			text = "Nachricht an das Studio schreiben …"
		}
		if selected {
			text, key = "a antworten · enter Reaktionen · ↑↓ wählen · esc fertig", "esc"
		}
		room := max(4, width-2-ansi.StringWidth(key)-2)
		text = truncateColumn(text, room)
		style := theme.Muted
		if selected {
			style = theme.Accent
		}
		gap := max(1, width-2-ansi.StringWidth(text)-ansi.StringWidth(key)-1)
		return []string{bar + " " + style.Render(text) + strings.Repeat(" ", gap) + theme.Subtle.Render(key) + " "}
	}

	inputWidth := composerInputWidth(width)
	lines := m.composerFieldLines(width)
	plain := lipgloss.NewStyle()
	style := m.chatInput.FocusedStyle
	style.Base, style.CursorLine, style.EndOfBuffer = plain, plain, plain
	style.Text, style.Placeholder, style.Prompt = theme.Text, theme.Subtle, plain
	m.chatInput.FocusedStyle, m.chatInput.BlurredStyle = style, style
	m.chatInput.Cursor.Style = m.targetStyle()
	m.chatInput.Placeholder = "Nachricht an den Chat"
	if m.chatToStudio {
		m.chatInput.Placeholder = "Nachricht an das Studio · nur das Team liest sie"
	}
	m.chatInput.SetWidth(inputWidth)
	m.chatInput.SetHeight(lines)
	view := strings.Split(m.chatInput.View(), "\n")

	count := len([]rune(m.chatInput.Value()))
	counterStyle := theme.Subtle
	switch {
	case count > chatLimit:
		counterStyle = theme.Danger
	case count > chatLimit-50:
		counterStyle = theme.Warning
	}
	target := "↵ Chat"
	if m.chatToStudio {
		target = "↵ Studio"
	}
	bar := m.targetStyle().Render("▌")
	rows := make([]string, lines)
	for index := range rows {
		text := ""
		if index < len(view) {
			text = view[index]
		}
		text = fitToWidth(text, inputWidth)
		text += strings.Repeat(" ", max(0, inputWidth-ansi.StringWidth(text)))
		right := ""
		switch index {
		case 0:
			right = counterStyle.Render(fmt.Sprintf("%d/%d", count, chatLimit))
		case lines - 1:
			right = m.targetStyle().Bold(true).Render(target)
		}
		right = strings.Repeat(" ", max(0, composerRight-ansi.StringWidth(right))) + right
		rows[index] = bar + " " + text + " " + right
	}
	return rows
}

// emojiRow is the row of the emoji to pick below the field; while the
// field has the keyboard it names the keys on the right.
func (m *Model) emojiRow(width int) (string, []hintSpot) {
	var b strings.Builder
	spots := []hintSpot{}
	column := 2
	b.WriteString("  ")
	hint := ""
	if m.chatFocused {
		hint = "alt+1…9"
		if m.sendStudioFunc != nil {
			if m.chatToStudio {
				hint += " · tab Chat"
			} else {
				hint += " · tab Studio"
			}
		}
	}
	room := width - ansi.StringWidth(hint) - 2
	for _, emoji := range quickEmojis {
		cell := ansi.StringWidth(emoji)
		if column+cell+2 > room {
			break
		}
		b.WriteString(emoji)
		spots = append(spots, hintSpot{x0: column, x1: column + cell, run: emoji})
		b.WriteString("  ")
		column += cell + 2
	}
	row := b.String()
	if hint != "" {
		row += strings.Repeat(" ", max(1, width-column-ansi.StringWidth(hint)-1)) + theme.Subtle.Render(hint)
	}
	return row, spots
}

// insertEmoji puts an emoji into the field at the cursor and gives the
// field the keyboard.
func (m *Model) insertEmoji(emoji string) {
	if !m.chatFocused {
		m.chatFocused = true
		m.chatSelectedID = ""
		m.chatInput.Focus()
	}
	m.chatInput.InsertString(emoji)
}

// replyBar is the row of the answer above the field: whom it answers and
// what, and the key that drops it.
func (m *Model) replyBar(width int) string {
	target := *m.chatReplyTo
	close := "esc ✕"
	name := truncateColumn(chatName(target), max(4, width/4))
	head := "↪ Antwort an "
	room := max(4, width-2-ansi.StringWidth(head)-ansi.StringWidth(name)-3-ansi.StringWidth(close)-1)
	text := truncateColumn(target.text, room)
	line := theme.Accent.Render("▎") + " " + theme.Muted.Render(head) + chatNameStyle(target).Render(name) +
		theme.Muted.Render(" · "+text)
	gap := max(1, width-ansi.StringWidth(line)-ansi.StringWidth(close)-1)
	return line + strings.Repeat(" ", gap) + theme.Subtle.Render(close) + " "
}

// composerRow puts a row of the field on its surface over the whole width.
func composerRow(row string, width int, surface string) string {
	row = fitToWidth(row, width)
	row += strings.Repeat(" ", max(0, width-ansi.StringWidth(row)))
	if surface == "" {
		return row
	}
	return withBackground(row, surface) + "\x1b[0m"
}
