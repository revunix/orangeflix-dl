package tui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// The limits of the layout.
const (
	// defaultFrameWidth is the width that is used before the terminal size is
	// known.
	defaultFrameWidth = 80
)

// frameWidth returns the width of the frame: the whole terminal. The frame
// carries the head line, the rules and the background of the interface.
func (m *Model) frameWidth() int {
	if m.width > 0 {
		return m.width
	}
	return defaultFrameWidth
}

// cleanChatText makes text from the chat safe for the layout. Control
// characters (an escape sequence or a line break smuggled in through an HTML
// entity), joiners, variation selectors and skin tones are removed: the
// terminal and the width table disagree about their width, and one wrong
// cell pushes the box apart. All white space becomes a single space.
func cleanChatText(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for _, r := range value {
		switch {
		case unicode.IsSpace(r):
			builder.WriteRune(' ')
		case unicode.IsControl(r), unicode.Is(unicode.Cf, r):
		case r >= 0xFE00 && r <= 0xFE0F:
		case r >= 0x1F3FB && r <= 0x1F3FF:
		case r == 0x20E3:
		case r >= 0xE0000 && r <= 0xE007F:
		case r == 0xFFFD:
		default:
			builder.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

// wrapText breaks a text into lines of at most the width. The width is
// counted in terminal cells, and a word that does not fit on a line of its
// own is split: an emoji or a long link must not push the frame apart.
func wrapText(value string, width int) []string {
	if width <= 0 {
		return nil
	}
	words := strings.Fields(value)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	current := ""
	currentWidth := 0
	for _, word := range words {
		wordWidth := ansi.StringWidth(word)
		for wordWidth > width {
			if current != "" {
				lines = append(lines, current)
				current, currentWidth = "", 0
			}
			head := ansi.Truncate(word, width, "")
			if head == "" {
				// A single character wider than the line: it goes out on a
				// line of its own and the frame cuts it.
				lines = append(lines, word)
				word, wordWidth = "", 0
				break
			}
			lines = append(lines, head)
			word = ansi.TruncateLeft(word, ansi.StringWidth(head), "")
			wordWidth = ansi.StringWidth(word)
		}
		switch {
		case current == "":
			current, currentWidth = word, wordWidth
		case currentWidth+1+wordWidth <= width:
			current += " " + word
			currentWidth += 1 + wordWidth
		default:
			lines = append(lines, current)
			current, currentWidth = word, wordWidth
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

// fitListWindow computes the visible window of a list panel. The budget is
// the number of the rows that fit beside the fixed lines of the panel (the
// header and an input). The hints "… N weitere" above and below the window
// take their own lines, so the window shrinks by the number of the hints -
// and only once, so the list keeps its height.
func fitListWindow(offset *int, cursorRow, budget, total int) (start, end int) {
	if budget < 1 {
		budget = 1
	}
	belegt := 0
	for range 4 {
		s, e, top, bottom := adjustScroll(offset, cursorRow, budget, total)
		hinweise := 0
		if top {
			hinweise++
		}
		if bottom {
			hinweise++
		}
		if hinweise == belegt {
			// Die Zahl der Hinweise ist stabil, das Fenster passt.
			return s, e
		}
		belegt = hinweise
		zeilen := budget - belegt
		if zeilen < 1 {
			zeilen = 1
		}
		// Der Cursor bleibt im Fenster, ohne die Position zu verwerfen.
		start = s
		if start+zeilen <= cursorRow {
			start = cursorRow - zeilen + 1
		}
		if maxStart := total - zeilen; maxStart > 0 && start > maxStart {
			start = maxStart
		}
		if start < 0 {
			start = 0
		}
		*offset = start
		budget = zeilen
	}
	return start, start + budget
}

// adjustScroll moves the visible window of a list minimally along the
// cursor: it only scrolls when the cursor leaves the window, so repeated
// arrow keys move the list step by step instead of centering it under the
// cursor over and over. The offset carries the position between two renders.
func adjustScroll(offset *int, cursorRow, budget, total int) (start, end int, top, bottom bool) {
	if total <= 0 || budget <= 0 {
		if offset != nil {
			*offset = 0
		}
		return 0, 0, false, false
	}
	if cursorRow < 0 {
		cursorRow = 0
	}
	if cursorRow >= total {
		cursorRow = total - 1
	}
	if budget > total {
		budget = total
	}
	if cursorRow < *offset {
		*offset = cursorRow
	}
	if cursorRow >= *offset+budget {
		*offset = cursorRow - budget + 1
	}
	start = *offset
	if start < 0 {
		start = 0
	}
	maxStart := total - budget
	if start > maxStart {
		start = maxStart
	}
	*offset = start
	end = start + budget
	if end > total {
		end = total
	}
	top = start > 0
	bottom = end < total
	return start, end, top, bottom
}

// splitWidths returns the widths of the two columns.
func splitWidths(width int) (int, int) {
	left := int(float64(width) * listShare)
	right := width - left - gap
	if right < 24 {
		return width, 0
	}
	return left, right
}

// padRight fills a text with spaces up to the width.
func padRight(value string, width int) string {
	gap := width - visibleWidth(value)
	if gap <= 0 {
		return value
	}
	return value + strings.Repeat(" ", gap)
}

// visibleWidth returns the width of a text in terminal cells. ANSI sequences
// carry no width, and a wide character (an emoji, a CJK rune) takes two cells:
// counting runes would let a box drift out of its frame.
func visibleWidth(value string) int {
	return ansi.StringWidth(value)
}

// stripANSI removes the ANSI sequences of a text.
func stripANSI(value string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range value {
		switch {
		case r == 0x1b:
			inEscape = true
		case inEscape && (r == 'm' || r == 'K'):
			inEscape = false
		case inEscape:
			// skip the rest of the sequence
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// itoa formats a number without the format package.
func itoa(value int64) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var digits []byte
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}

// padNumber fills a number with leading zeros.
func padNumber(value, width int) string {
	text := itoa(int64(value))
	for len(text) < width {
		text = "0" + text
	}
	return text
}

// playbackClock formats a position of the player.
func playbackClock(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	if seconds >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", seconds/3600, (seconds%3600)/60, seconds%60)
	}
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

// humanDuration formats a duration in seconds.
func humanDuration(seconds int64) string {
	if seconds < 0 {
		return "-"
	}
	if seconds < 60 {
		return "weniger als einer Minute"
	}
	minutes := seconds / 60
	if minutes < 60 {
		return plural(minutes, "Minute", "Minuten")
	}
	hours := minutes / 60
	remainder := minutes % 60
	if hours < 24 {
		if remainder == 0 {
			return plural(hours, "Stunde", "Stunden")
		}
		return plural(hours, "Stunde", "Stunden") + " " + plural(remainder, "Minute", "Minuten")
	}
	return plural(hours/24, "Tag", "Tagen")
}

// plural returns the singular or the plural of a word.
func plural(count int64, singular, pluralForm string) string {
	if count == 1 {
		return "1 " + singular
	}
	return itoa(count) + " " + pluralForm
}

// formatDate formats the date of an episode.
func formatDate(episode orangeflix.Episode) string {
	if episode.Date.IsZero() {
		return "-"
	}
	return episode.Date.Local().Format("02.01.2006")
}
