package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/revunix/orangeflix-dl/internal/app"
	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// liveModel builds a model on the live screen with a connected chat that
// holds the messages that stress the layout: emoji and CJK runes are two
// cells wide, umlauts are two bytes, and a long word has no place to wrap.
func liveModel() *Model {
	model := testModel()
	model.width = 100
	model.height = 40
	model.screen = ScreenLive
	access := true
	model.live = &orangeflix.V3LiveInfo{
		OK:        true,
		IsLive:    true,
		CanAccess: &access,
		Show:      &orangeflix.V3LiveShow{Title: "Sendung", Chat: true, IsLive: true},
	}
	model.chatConnected = true
	model.chatStatus = "Chat verbunden"
	now := time.Now()
	model.chatMessages = []chatLine{
		{from: "Jürgen", text: "Hallo zusammen! 👍👍👍", time: now},
		{from: "Fan", text: strings.Repeat("🎉", 60), time: now},
		{from: "Link", text: "https://example.com/ein/sehr/langer/pfad/der/nicht/mehr/in/eine_zeile_passt?query=1#fragment", time: now},
		{from: "日本語", text: "日本語のテキスト CJK はセル幅2 🚀 gemischt mit words", time: now},
	}
	return model
}

// TestTheChatKeepsItsBox checks the live screen with wide characters in the
// chat: every line of the page holds the width of the terminal, and the
// right border of the chat box survives. Counting runes instead of cells
// would pad the lines too wide and the frame would cut them.
func TestTheChatKeepsItsBox(t *testing.T) {
	model := liveModel()
	lines := strings.Split(model.View(), "\n")
	if len(lines) != model.height {
		t.Fatalf("the view has %d lines, want %d", len(lines), model.height)
	}
	for index, line := range lines {
		if width := ansi.StringWidth(line); width != model.width {
			t.Errorf("line %d is %d cells wide, want %d: %q", index, width, model.width, ansi.Strip(line))
		}
	}

	// The chat box reaches to the right margin: in its last cell stands the
	// border, not the ellipsis of a truncated line.
	head := len(model.header(model.width))
	body := model.height - head - len(model.footer(model.width))
	_, chat := model.liveBoxes(model.width, body)
	for row := chat.y; row < chat.y+chat.height && head+row < len(lines); row++ {
		line := lines[head+row]
		cell := ansi.Strip(ansi.Cut(line, chat.x+chat.width-1, chat.x+chat.width))
		if cell != "│" && cell != "╮" && cell != "╯" {
			t.Errorf("the chat box ends line %d with %q, want its border: %q", head+row, cell, ansi.Strip(line))
		}
	}
}

// TestTheChatWrapsIntoThePanel checks that every line of the chat body fits
// into the panel: a run of emoji and a long link wrap instead of running
// over the frame.
func TestTheChatWrapsIntoThePanel(t *testing.T) {
	model := liveModel()
	box := rect{x: 1, y: 1, width: 98, height: 20}
	for index, line := range model.chatPanel(box) {
		if width := ansi.StringWidth(line); width != box.width {
			t.Errorf("panel line %d is %d cells wide, want %d: %q", index, width, box.width, ansi.Strip(line))
		}
	}
	body := model.chatBody(box.innerWidth())
	if len(body) < len(model.chatMessages) {
		t.Fatalf("the chat body has %d lines, the wide messages did not wrap", len(body))
	}
	for index, line := range body {
		if width := ansi.StringWidth(line); width > box.innerWidth() {
			t.Errorf("body line %d is %d cells wide, want at most %d: %q", index, width, box.innerWidth(), ansi.Strip(line))
		}
	}
}

// TestCleanChatText checks that control characters and the zero-width
// pieces of emoji sequences leave the text before it reaches the layout.
func TestCleanChatText(t *testing.T) {
	cases := map[string]string{
		"a\x1b[31mb\x07":      "a[31mb",
		"zeile1\nzeile2\t\r3": "zeile1 zeile2 3",
		"👨\u200d👩\u200d👧":     "👨👩👧",
		"❤\ufe0f":             "❤",
		"👍\U0001F3FD":         "👍",
		"  x  ":               "x",
	}
	for input, want := range cases {
		if got := cleanChatText(input); got != want {
			t.Errorf("cleanChatText(%q) = %q, want %q", input, got, want)
		}
	}
}

// TestTheChatSurvivesHostileMessages sends messages with escape sequences,
// line breaks and a very long name through the event path: the page keeps
// its size.
func TestTheChatSurvivesHostileMessages(t *testing.T) {
	model := liveModel()
	model.chatMessages = nil
	for _, event := range []app.Event{
		{Kind: app.EventChat, Label: "Na\nme\x1b[2J" + strings.Repeat("x", 80), Message: "hi\x1b[2J\nzweite Zeile\r\u200b👨\u200d👩\u200d👧\ufe0f"},
		{Kind: app.EventChat, Label: "Fan", Message: "\x1b]0;titel\x07 ende"},
	} {
		model.applyEvent(event)
	}
	lines := strings.Split(model.View(), "\n")
	if len(lines) != model.height {
		t.Fatalf("the view has %d lines, want %d", len(lines), model.height)
	}
	for index, line := range lines {
		if strings.ContainsAny(line, "\r\x07") {
			t.Errorf("line %d holds a control character: %q", index, line)
		}
		if width := ansi.StringWidth(line); width != model.width {
			t.Errorf("line %d is %d cells wide, want %d", index, width, model.width)
		}
	}
}
