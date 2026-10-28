package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// fitToWidth truncates a styled text to the width, marking the cut with an
// ellipsis.
func fitToWidth(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(value) <= width {
		return value
	}
	return ansi.Truncate(value, width, "…")
}

// colorStyle returns the style of a color of the gradient.
func colorStyle(color string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color))
}

// gradientEndpoints picks the palette entries that stay legible on the
// terminal background. They are the brand gradient of orangeflix.de.
func gradientEndpoints() (string, string, string) {
	if isDark() {
		return colorGradientStartDark, colorGradientMidDark, colorGradientEndDark
	}
	return colorGradientStartLight, colorGradientMidLight, colorGradientEndLight
}

// interpolateHex mixes the three colors of the gradient; the index runs from
// 0 to count-1.
func interpolateHex(start, middle, end string, index, count int) string {
	startR, startG, startB := parseHexColor(start)
	middleR, middleG, middleB := parseHexColor(middle)
	endR, endG, endB := parseHexColor(end)
	if count <= 1 {
		return formatHexColor(startR, startG, startB)
	}
	weight := count - 1
	first := weight / 2
	second := weight - first
	var red, green, blue int
	if index <= first {
		factor := 0
		if first > 0 {
			factor = index * weight / first
		}
		red = mixColor(startR, middleR, factor, weight)
		green = mixColor(startG, middleG, factor, weight)
		blue = mixColor(startB, middleB, factor, weight)
	} else {
		factor := 0
		if second > 0 {
			factor = (index - first) * weight / second
		}
		red = mixColor(middleR, endR, factor, weight)
		green = mixColor(middleG, endG, factor, weight)
		blue = mixColor(middleB, endB, factor, weight)
	}
	return formatHexColor(red, green, blue)
}

// mixColor mixes two color values.
func mixColor(from, to, factor, weight int) int {
	if weight <= 0 {
		return from
	}
	return (from*weight + (to-from)*factor) / weight
}

// parseHexColor reads a color of the form "#rrggbb".
func parseHexColor(value string) (int, int, int) {
	parsed, err := strconv.ParseUint(strings.TrimPrefix(value, "#"), 16, 32)
	if err != nil {
		return 0, 0, 0
	}
	return int(parsed>>16) & 0xFF, int(parsed>>8) & 0xFF, int(parsed) & 0xFF
}

// formatHexColor builds a color of the form "#rrggbb".
func formatHexColor(red, green, blue int) string {
	return fmt.Sprintf("#%02X%02X%02X", red, green, blue)
}
