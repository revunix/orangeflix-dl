package tui

import (
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// TestTheColumnsStayInTheirPlace checks that the marks with a colour do
// not shift the columns: the padding is counted on the visible text.
func TestTheColumnsStayInTheirPlace(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	applyTheme(ThemeDark)

	model := testModel()
	model.screen = ScreenFormats
	model.formatWidth = 80

	plain := model.formatLine(orangeflix.Format{
		PID: 19, Slug: "veto", Title: "Veto", Status: orangeflix.StatusActive,
	})
	marked := model.formatLine(orangeflix.Format{
		PID: 19, Slug: "veto", Title: "Veto", Status: orangeflix.StatusActive,
		NewEpisode: true, Opened: true,
	})
	if ansi.StringWidth(plain) != ansi.StringWidth(marked) {
		t.Errorf("die Zeilen haben verschiedene Breite: %d und %d",
			ansi.StringWidth(plain), ansi.StringWidth(marked))
	}
	// Der Titel steht in beiden Zeilen an derselben Stelle. Die Farben und
	// die mehrbyteigen Zeichen zählen nicht, darum wird die sichtbare
	// Spalte gemessen.
	title := visibleColumn(plain, "Veto")
	if title < 0 {
		t.Fatalf("der Titel fehlt: %q", plain)
	}
	if index := visibleColumn(marked, "Veto"); index != title {
		t.Errorf("der Titel steht an %d, in der markierten Zeile an %d", title, index)
	}
}

// visibleColumn returns the column of a text in a line, ohne die Farben und
// ohne die mehrbyteigen Zeichen zu zählen.
func visibleColumn(line, text string) int {
	runes := []rune(ansi.Strip(line))
	for index := 0; index+len([]rune(text)) <= len(runes); index++ {
		if string(runes[index:index+len([]rune(text))]) == text {
			return index
		}
	}
	return -1
}

// TestPlayableEpisodes checks the episodes that are played: the marked
// ones, otherwise the one under the cursor.
func TestPlayableEpisodes(t *testing.T) {
	model := testModel()
	model.screen = ScreenEpisodes
	model.episodes = []orangeflix.Episode{
		{Identifier: "a", Number: 1, Title: "Eins", Date: time.Now()},
		{Identifier: "b", Number: 2, Title: "Zwei", Date: time.Now()},
		{Identifier: "c", Number: 3, Title: "Drei", Date: time.Now()},
	}
	model.episodeCursor = 1
	episodes := model.playableEpisodes()
	if len(episodes) != 1 || episodes[0].Identifier != "b" {
		t.Fatalf("episodes = %+v, want the one under the cursor", episodes)
	}

	// Markierte Folgen haben Vorrang, sie kommen in der Reihenfolge der
	// Liste.
	model.selected = map[string]bool{"a": true, "c": true}
	episodes = model.playableEpisodes()
	if len(episodes) != 2 {
		t.Fatalf("episodes = %+v, want the marked ones", episodes)
	}
	marked := map[string]bool{}
	for _, episode := range episodes {
		marked[episode.Identifier] = true
	}
	if !marked["a"] || !marked["c"] {
		t.Errorf("episodes = %+v, want a and c", episodes)
	}
}
