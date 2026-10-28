package tui

import (
	"strings"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// qualityEntry is one quality of an episode.
type qualityEntry struct {
	// desc is the name of the quality ("HD 1080p").
	desc string
	// dimensions is the pixel size of the file.
	dimensions string
	// size is the size of the file in bytes.
	size int64
	// kind tells whether the file carries the picture or only the sound.
	kind string
}

// qualityPopup builds the window that shows the qualities of the marked
// episode. The window floats over the page and is centered in both
// directions.
func (m *Model) qualityPopup(width int) []string {
	entries := m.qualityEntries()
	inner := qualityInnerWidth(width)

	content := []string{}
	if len(entries) == 0 {
		content = append(content, theme.Muted.Render("Für diese Folge sind keine Dateien bekannt."))
		content = append(content, theme.Muted.Render("Mit 'i' werden die Qualitäten geladen."))
	} else {
		content = append(content, strings.Repeat(" ", rowIndent)+theme.Subtle.Render(padRight("QUALITÄT", 13)+
			padRight("AUFLÖSUNG", 13)+"GRÖSSE"))
		for index, entry := range entries {
			dimensions := entry.dimensions
			if dimensions == "" {
				dimensions = "-"
			}
			size := humanBytes(entry.size)
			if entry.kind == "music" {
				size += " (nur Ton)"
			}
			line := padRight(entry.desc, 13) + padRight(dimensions, 13) + size
			content = append(content, renderWindowRow(theme.Row.Render(line), index == m.qualityCursor, inner))
		}
	}

	// Das Fenster hat eine eigene Farbe, wie das Menü von btop.
	return windowBoxFootColor(inner+4, "Qualität wählen", "", theme.BoxList, content)
}

// qualityEntries returns the qualities of the marked episode.
func (m *Model) qualityEntries() []qualityEntry {
	identifier := episodeIdentifier(m)
	files := m.clips[identifier]
	entries := make([]qualityEntry, 0, len(files))
	for _, file := range files {
		entries = append(entries, qualityEntry{
			desc:       file.Desc,
			dimensions: file.Dimensions,
			size:       file.Size,
			kind:       file.Kind,
		})
	}
	return entries
}

// qualityInnerWidth returns the width of the text inside the quality window.
func qualityInnerWidth(width int) int {
	inner := width - 16
	if inner > 50 {
		inner = 50
	}
	if inner < 26 {
		inner = 26
	}
	return inner
}

// currentQualityIndex returns the position of the quality that is in use. It
// is zero when the quality is not known.
func (m *Model) currentQualityIndex() int {
	if m.config == nil || m.config.Quality == "" {
		return 0
	}
	for index, entry := range m.qualityEntries() {
		if qualityName(entry.desc) == strings.ToLower(m.config.Quality) {
			return index
		}
	}
	return 0
}

// qualityName returns the name of a quality in the list of the program.
func qualityName(desc string) string {
	switch {
	case strings.HasPrefix(desc, "HD 1080"):
		return orangeflix.QualityHD
	case strings.HasPrefix(desc, "HD 720"):
		return orangeflix.QualityMobile
	case strings.HasPrefix(desc, "SD"):
		return orangeflix.QualitySD
	case strings.HasPrefix(desc, "Audio"):
		return orangeflix.QualityAudio
	}
	return desc
}
