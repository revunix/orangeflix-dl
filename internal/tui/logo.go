package tui

import (
	"strings"
)

// logoLine is the logo of the program. It stands in the head of the program
// like the banner of btop.
const logoLine = "orangeflix"

// gradientText colors a text with the brand gradient. The position shifts the
// place in the gradient, so several lines of a logo form a gradient.
func gradientText(text string, position int) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return ""
	}
	start, middle, end := gradientEndpoints()
	var builder strings.Builder
	for index, value := range runes {
		color := interpolateHex(start, middle, end, index+position, len(runes)+position)
		builder.WriteString(colorStyle(color).Render(string(value)))
	}
	return builder.String()
}
