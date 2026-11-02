package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// TestHeaderShowsTheNextLivestream checks the hint in the head line: the
// next show today, a show that is on now, and a line that stays on its width.
func TestHeaderShowsTheNextLivestream(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
	applyTheme(ThemeDark)

	now := time.Date(2026, 11, 2, 16, 13, 0, 0, time.Local)
	start := time.Date(2026, 11, 2, 19, 0, 0, 0, time.Local)
	model := testModel()
	model.calendar = []orangeflix.V3CalendarEntry{
		{
			Type:      "calendar",
			Title:     "Veto",
			Timestamp: time.Date(2026, 11, 2, 17, 0, 0, 0, time.Local).Unix(),
		},
		{
			Type:      "live",
			Title:     "Frisch gepresst",
			Timestamp: start.Unix(),
			Live:      &orangeflix.V3CalendarLive{End: start.Add(2 * time.Hour).Unix()},
		},
		{
			Type:      "live",
			Title:     "Später",
			Timestamp: start.Add(48 * time.Hour).Unix(),
		},
	}

	hint := model.nextLiveHint(now)
	const want = "heute 19:00 · in 2 Std. 47 Min. · Frisch gepresst"
	if hint != want {
		t.Fatalf("hint = %q", hint)
	}

	lines := renderHeader(100, "Formate", "52 Formate", "", hint, "16:13")
	if len(lines) != 2 {
		t.Fatalf("lines = %d", len(lines))
	}
	if !strings.Contains(lines[0], want) {
		t.Errorf("the header has no livestream: %q", lines[0])
	}
	if !strings.Contains(lines[0], "52 Formate") {
		t.Errorf("the header dropped the view context: %q", lines[0])
	}
	if !strings.Contains(lines[0], "16:13") {
		t.Errorf("the header has no clock: %q", lines[0])
	}
	assertHeaderWidth(t, lines, 100)

	// Während der Sendung zählt das Ende aus dem Kalender, nicht nur die
	// Live-Ansicht.
	during := start.Add(30 * time.Minute)
	hint = model.nextLiveHint(during)
	if hint != "● LIVE · Frisch gepresst" {
		t.Fatalf("live hint = %q", hint)
	}
	liveLines := renderHeader(100, "Formate", "52 Formate", "", hint, "19:30")
	if !strings.Contains(liveLines[0], "● LIVE · Frisch gepresst") {
		t.Errorf("the header does not show the running show: %q", liveLines[0])
	}
	assertHeaderWidth(t, liveLines, 100)

	// In einem schmalen Terminal bleibt die Uhr stehen und die Zeile exakt.
	narrow := renderHeader(40, "Formate", "52 Formate", "", want, "16:13")
	if !strings.Contains(narrow[0], "16:13") {
		t.Errorf("the narrow header lost the clock: %q", narrow[0])
	}
	if strings.Contains(narrow[0], "52 Formate") {
		t.Errorf("the narrow header kept the view context: %q", narrow[0])
	}
	assertHeaderWidth(t, narrow, 40)
}

// TestNextLiveHintNamesTomorrowAcrossTheClockChange checks that the day
// after today stays "morgen" when the clocks go back.
func TestNextLiveHintNamesTomorrowAcrossTheClockChange(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 25, 0, 30, 0, 0, loc)
	when := time.Date(2026, 10, 26, 19, 0, 0, 0, loc)
	got := formatNextLive(when, now, "Frisch gepresst")
	if got != "morgen 19:00 · in 1 Tag 19 Std. · Frisch gepresst" {
		t.Fatalf("hint = %q", got)
	}
}

func TestLiveCountdown(t *testing.T) {
	now := time.Date(2026, 11, 2, 16, 0, 0, 0, time.Local)
	cases := []struct {
		after time.Duration
		want  string
	}{
		{0, "gleich"},
		{42 * time.Second, "in 42 Sek."},
		{9*time.Minute + 12*time.Second, "in 9 Min. 12 Sek."},
		{47 * time.Minute, "in 47 Min."},
		{2*time.Hour + 47*time.Minute, "in 2 Std. 47 Min."},
		{2 * time.Hour, "in 2 Std."},
		{26 * time.Hour, "in 1 Tag 2 Std."},
		{48 * time.Hour, "in 2 Tagen"},
	}
	for _, test := range cases {
		if got := liveCountdown(now.Add(test.after), now); got != test.want {
			t.Errorf("after %s = %q, want %q", test.after, got, test.want)
		}
	}
}

// TestAQuietCalendarLoadDoesNotOpenTheCalendar checks that the header load
// fills the entries without switching the screen or showing an error.
func TestAQuietCalendarLoadDoesNotOpenTheCalendar(t *testing.T) {
	model := testModel()
	model.screen = ScreenFormats
	model.message = "bleibt"
	entries := []orangeflix.V3CalendarEntry{{
		Type:      "live",
		Title:     "Frisch gepresst",
		Timestamp: time.Now().Add(time.Hour).Unix(),
	}}

	updated, cmd := model.Update(CalendarMsg{Entries: entries, Background: true})
	if cmd != nil {
		t.Errorf("a quiet load returned a command")
	}
	next, ok := updated.(*Model)
	if !ok {
		t.Fatal("the model has the wrong type")
	}
	if next.screen != ScreenFormats {
		t.Errorf("screen = %v, want the formats", next.screen)
	}
	if len(next.calendar) != 1 || next.calendar[0].Title != "Frisch gepresst" {
		t.Errorf("calendar = %+v", next.calendar)
	}
	if next.message != "bleibt" || next.isError {
		t.Errorf("message = %q, error = %v", next.message, next.isError)
	}

	failed, _ := next.Update(CalendarMsg{Err: errString("weg"), Background: true})
	after, ok := failed.(*Model)
	if !ok {
		t.Fatal("the model has the wrong type")
	}
	if len(after.calendar) != 1 || after.screen != ScreenFormats || after.message != "bleibt" {
		t.Errorf("a failed quiet load changed the screen: screen %v, message %q, entries %d", after.screen, after.message, len(after.calendar))
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func assertHeaderWidth(t *testing.T, lines []string, width int) {
	t.Helper()
	for index, line := range lines {
		if got := ansi.StringWidth(line); got != width {
			t.Errorf("line %d is %d wide, want %d: %q", index, got, width, line)
		}
	}
}
