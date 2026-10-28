package tui

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

func TestLayoutRectsFillTheScreen(t *testing.T) {
	list, detail, logBox, info := layoutRects(120, 30, true)

	if list.width <= 0 || list.height <= 0 {
		t.Fatalf("the list has no size: %+v", list)
	}
	if detail.width <= 0 || detail.height <= 0 {
		t.Fatalf("the details have no size: %+v", detail)
	}
	if logBox.width <= 0 || logBox.height <= 0 {
		t.Fatalf("the log has no size: %+v", logBox)
	}
	if info.width <= 0 || info.height <= 0 {
		t.Fatalf("the account has no size: %+v", info)
	}

	// Die Kästen überlappen sich nicht und liegen innerhalb des Randes.
	boxes := []rect{list, detail, logBox, info}
	for i, first := range boxes {
		if first.x < margin || first.y < margin {
			t.Errorf("the box %d lies outside the margin: %+v", i, first)
		}
		if first.x+first.width > 120-margin {
			t.Errorf("the box %d reaches over the right edge: %+v", i, first)
		}
		if first.y+first.height > 30-margin {
			t.Errorf("the box %d reaches over the lower edge: %+v", i, first)
		}
		for j, second := range boxes {
			if i >= j {
				continue
			}
			if overlap(first, second) {
				t.Errorf("the boxes %d and %d overlap: %+v %+v", i, j, first, second)
			}
		}
	}
}

func TestLayoutRectsUseOneColumnOnANarrowTerminal(t *testing.T) {
	list, detail, logBox, info := layoutRects(80, 30, false)
	if detail.width != 0 || logBox.width != 0 || info.width != 0 {
		t.Errorf("the narrow terminal shows more than one column: %+v %+v %+v", detail, logBox, info)
	}
	if list.width <= 0 {
		t.Errorf("the list has no size: %+v", list)
	}
}

// overlap reports whether two boxes share a place.
func overlap(first, second rect) bool {
	return first.x < second.x+second.width &&
		second.x < first.x+first.width &&
		first.y < second.y+second.height &&
		second.y < first.y+first.height
}

// TestTheListStandsOut checks that the list, the box the keys work in,
// carries the brand color and the other boxes stand back in one quiet grey.
func TestTheListStandsOut(t *testing.T) {
	applyTheme(ThemeDark)
	list := fmt.Sprintf("%v", theme.BoxList.GetForeground())
	quiet := fmt.Sprintf("%v", theme.BoxDetail.GetForeground())
	if list == quiet {
		t.Errorf("the list has the color of the other boxes: %s", list)
	}
	for _, box := range []lipgloss.Style{theme.BoxLog, theme.BoxInfo} {
		if color := fmt.Sprintf("%v", box.GetForeground()); color != quiet {
			t.Errorf("a quiet box has the color %s, want %s", color, quiet)
		}
	}
}

// TestTheGreysAreSolidColors checks that no color of the theme carries an
// opacity: terminals do not know it and draw such a color in the default
// color of the text.
func TestTheGreysAreSolidColors(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	for _, scheme := range []Theme{ThemeDark, ThemeLight} {
		applyTheme(scheme)
		for name, style := range map[string]lipgloss.Style{
			"Divider": theme.Divider, "Border": theme.Border, "BorderStrong": theme.BorderStrong,
			"BoxDetail": theme.BoxDetail, "Selected": theme.Selected,
		} {
			if sequenceOf(style) == "" {
				t.Errorf("%s has no color in the %s scheme", name, scheme)
			}
		}
	}
	applyTheme(ThemeDark)
}

func TestSelectedRowHasABackground(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	applyTheme(ThemeDark)

	marked := renderRow("Pantoffelkino", true)
	if !strings.Contains(marked, "\x1b[48;2;") {
		t.Errorf("the marked line has no background: %q", marked)
	}
	plain := renderRow("Veto", false)
	if strings.Contains(plain, "\x1b[48;2;") {
		t.Errorf("an unmarked line has a background: %q", plain)
	}
	if !strings.Contains(marked, "▌") {
		t.Errorf("the marked line has no cursor: %q", marked)
	}
}

func TestTheBoxesReachTheBottomOfTheScreen(t *testing.T) {
	model := testModel()
	model.width = 120
	model.height = 30
	view := model.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 30 {
		t.Fatalf("the view has %d lines, want 30", len(lines))
	}
	// Das Konto-Kasten steht rechts unten.
	found := false
	for _, line := range lines {
		if strings.Contains(line, "Konto") {
			found = true
		}
	}
	if !found {
		t.Errorf("the view does not hold the account box:\n%s", view)
	}
}

func TestStartScreenShowsTheForm(t *testing.T) {
	model := testModel()
	model.width = 120
	model.height = 30
	model.credentials = false
	model.screen = ScreenStart
	view := model.View()
	if !strings.Contains(view, "Zugangsdaten") {
		t.Errorf("the start screen does not show the form:\n%s", view)
	}
	if !strings.Contains(view, "E-Mail") {
		t.Errorf("the start screen does not show the fields:\n%s", view)
	}
}

func TestQueueUsesOneBox(t *testing.T) {
	model := testModel()
	model.width = 120
	model.height = 30
	model.screen = ScreenQueue
	model.jobs = []job{{Label: "Die.Mediatheke.S2026E420", Path: "/tmp/x.mp4", Done: 50, Total: 100, State: "lädt"}}
	view := model.View()
	if !strings.Contains(view, "Downloads") {
		t.Errorf("the queue does not show its box:\n%s", view)
	}
	if !strings.Contains(view, meterSymbol) {
		t.Errorf("the queue does not show the meter:\n%s", view)
	}
}

func TestTheLogBoxCanBeHidden(t *testing.T) {
	model := testModel()
	model.width = 120
	model.height = 30
	model.showLogBox = true
	if !strings.Contains(model.View(), "Protokoll") {
		t.Error("the log box is missing")
	}
	model.showLogBox = false
	if strings.Contains(model.View(), "Protokoll") {
		t.Error("the log box is still shown")
	}
}

func TestThePanelBackgroundSurvivesStyledText(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	applyTheme(ThemeDark)

	panel := sequenceOf(theme.Panel)
	if panel == "" {
		t.Fatal("the panel has no background")
	}
	// Der Text der Zeile enthält einen eigenen Reset: danach muss die Fläche
	// des Kastens wieder stehen.
	line := theme.Muted.Render("Text")
	rows := renderBoxAt(40, 4, "Test", theme.BoxList, []string{line, "ohne Farbe"})
	body := rows[1]
	if !strings.Contains(body, panel) {
		t.Errorf("the row has no panel background: %q", body)
	}
	// Nach dem Reset des Textes steht die Fläche wieder.
	reset := strings.Index(body, "\x1b[0m")
	if reset < 0 {
		t.Fatalf("the row has no reset: %q", body)
	}
	rest := body[reset:]
	if !strings.Contains(rest, panel) {
		t.Errorf("the panel background ends at the reset: %q", body)
	}
}

func TestTheSelectedRowKeepsItsBackground(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	applyTheme(ThemeDark)

	selected := sequenceOf(theme.Selected)
	if selected == "" {
		t.Fatal("the selected row has no background")
	}
	row := renderRow(theme.Muted.Render("Pantoffelkino"), true)
	if !strings.Contains(row, selected) {
		t.Errorf("the row has no background: %q", row)
	}
	// Nach dem Reset des Textes steht der Hintergrund wieder.
	reset := strings.Index(row, "\x1b[0m")
	if reset < 0 {
		t.Fatalf("the row has no reset: %q", row)
	}
	if !strings.Contains(row[reset:], selected) {
		t.Errorf("the background ends at the reset: %q", row)
	}
}

func TestTheSchemasHaveTheirOwnColors(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

	applyTheme(ThemeDark)
	darkPage := sequenceOf(theme.Background)
	darkPanel := sequenceOf(theme.Panel)
	darkText := fmt.Sprintf("%v", theme.Text.GetForeground())
	applyTheme(ThemeLight)
	lightPage := sequenceOf(theme.Background)
	lightPanel := sequenceOf(theme.Panel)
	lightText := fmt.Sprintf("%v", theme.Text.GetForeground())

	if darkPage == lightPage {
		t.Error("the page background is the same in both schemas")
	}
	if darkPanel == lightPanel {
		t.Error("the panel background is the same in both schemas")
	}
	if darkText == lightText {
		t.Error("the text color is the same in both schemas")
	}
	// Der Kasten bringt seinen eigenen Vordergrund mit, sonst hängt der Text
	// von der Terminalfarbe ab.
	if theme.Panel.GetForeground() == nil {
		t.Error("the panel has no foreground color")
	}
	if theme.Window.GetForeground() == nil {
		t.Error("the window has no foreground color")
	}
}

// rgbOf reads the red, green and blue values of a color of the form
// "#rrggbb".
func rgbOf(value string) (int, int, int, bool) {
	if !strings.HasPrefix(value, "#") || len(value) != 7 {
		return 0, 0, 0, false
	}
	parsed, err := strconv.ParseUint(value[1:], 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return int(parsed>>16) & 0xFF, int(parsed>>8) & 0xFF, int(parsed) & 0xFF, true
}

func TestEveryBoxKeepsItsColor(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	applyTheme(ThemeDark)

	model := testModel()
	model.width = 120
	model.height = 30
	view := model.View()

	// Jeder Kasten bringt seine Rahmenfarbe mit, auch wenn er neben einem
	// anderen steht.
	// Die Farben stehen als RGB-Sequenz in der Zeile.
	frames := map[string]bool{}
	for _, style := range []lipgloss.Style{
		theme.BoxList, theme.BoxDetail, theme.BoxLog, theme.BoxInfo,
	} {
		red, green, blue, ok := rgbOf(fmt.Sprintf("%v", style.GetForeground()))
		if !ok {
			t.Fatalf("the color %v could not be read", style.GetForeground())
		}
		frames[fmt.Sprintf("38;2;%d;%d;%d", red, green, blue)] = true
	}
	for frame := range frames {
		if !strings.Contains(view, frame) {
			t.Errorf("the view does not hold the frame color %q", frame)
		}
	}
}

func TestTheHeaderShowsTheBrandThePathAndTheClock(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	applyTheme(ThemeDark)

	model := testModel()
	model.width = 120
	model.height = 30
	model.screen = ScreenEpisodes
	model.currentFormat = &orangeflix.Format{PID: 1, Title: "Die Mediatheke"}
	header := strings.Split(model.View(), "\n")[0]
	plain := stripANSI(header)
	for _, part := range []string{"orangeflix-dl", "Formate › Die Mediatheke", time.Now().Format("15:04")} {
		if !strings.Contains(plain, part) {
			t.Errorf("the header does not hold %q: %q", part, plain)
		}
	}
	// Die Titel der Kästen stehen in den Kästen, nicht im Kopf.
	if strings.Contains(plain, "Protokoll") {
		t.Errorf("the header repeats the titles of the boxes: %q", plain)
	}
	// Der Name der Marke steht in der Akzentfarbe.
	if accent := sequenceOf(theme.AccentBold); accent == "" || !strings.Contains(header, accent+"orangeflix") {
		t.Errorf("the brand is not in the accent color: %q", header)
	}
	// Der Kopf darf die Breite nicht sprengen.
	if visibleWidth(header) != 120 {
		t.Errorf("the header is %d wide, want 120", visibleWidth(header))
	}
}
