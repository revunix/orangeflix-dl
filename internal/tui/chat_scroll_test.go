package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/revunix/orangeflix-dl/internal/app"
)

// longChat fills the chat with messages that wrap onto several lines, so
// the lines outnumber the messages.
func longChat() *Model {
	model := liveModel()
	model.chatMessages = nil
	for index := 0; index < 40; index++ {
		model.appendChatLine(chatLine{
			from: "Fan",
			text: fmt.Sprintf("Nachricht %02d %s", index, strings.Repeat("lang ", 30)),
			time: time.Now(),
		})
	}
	return model
}

// chatRows draws the page and returns the text of the rows inside the chat
// box, without the frame and without the field to write below them.
func chatRows(t *testing.T, model *Model) []string {
	t.Helper()
	model.View()
	head := len(model.header(model.width))
	body := model.height - head - len(model.footer(model.width))
	_, box := model.liveBoxes(model.width, body)
	panel := model.chatPanel(box)
	rows := []string{}
	for _, line := range panel[1 : len(panel)-1-model.composerRows()] {
		plain := ansi.Strip(line)
		rows = append(rows, strings.TrimSpace(strings.Trim(plain, "│")))
	}
	return rows
}

func TestTheTopOfTheChatKeepsTheWindowFull(t *testing.T) {
	model := longChat()
	model.View()
	if model.chatMaxScroll <= len(model.chatMessages) {
		t.Fatalf("the lines (%d) should outnumber the messages (%d)", model.chatMaxScroll, len(model.chatMessages))
	}
	// So weit hoch, wie es geht, mit der Pfeiltaste.
	for step := 0; step < 1000; step++ {
		model = press(t, model, "up")
	}
	if model.chatScroll != model.chatMaxScroll {
		t.Fatalf("scroll = %d, want the top %d", model.chatScroll, model.chatMaxScroll)
	}
	rows := chatRows(t, model)
	// Oben steht der Kopf der ersten Blase, darunter ihr Text.
	if !strings.Contains(rows[0], "Fan") || !strings.Contains(rows[1], "Nachricht 00") {
		t.Errorf("the oldest message is not at the top: %q / %q", rows[0], rows[1])
	}
	for index, row := range rows {
		if row == "" {
			t.Errorf("row %d of the chat is empty at the top:\n%s", index, strings.Join(rows, "\n"))
			break
		}
	}
}

func TestAScrolledChatStaysWhileMessagesComeIn(t *testing.T) {
	model := longChat()
	model.View()
	for step := 0; step < 5; step++ {
		model = press(t, model, "pgup")
	}
	before := chatRows(t, model)
	for index := 0; index < 3; index++ {
		model.applyEvent(app.Event{Kind: app.EventChat, Label: "Neu", Message: fmt.Sprintf("frisch %d %s", index, strings.Repeat("wort ", 20))})
	}
	after := chatRows(t, model)
	if before[0] != after[0] || before[len(before)-1] != after[len(after)-1] {
		t.Errorf("the window moved:\nbefore %q … %q\nafter  %q … %q", before[0], before[len(before)-1], after[0], after[len(after)-1])
	}

	// end springt ans Ende, dort stehen die neuen Nachrichten.
	model = press(t, model, "end")
	rows := chatRows(t, model)
	if model.chatScroll != 0 || !strings.Contains(strings.Join(rows, " "), "frisch 2") {
		t.Errorf("end did not reach the newest message: scroll %d", model.chatScroll)
	}
}

func TestTheChatShowsThatItIsScrolled(t *testing.T) {
	model := longChat()
	model.View()
	model = press(t, model, "up")
	view := ansi.Strip(model.View())
	if !strings.Contains(view, "Zeilen zurück") || !strings.Contains(view, "end neueste") {
		t.Errorf("the scrolled state is not shown:\n%s", view)
	}
}

func TestTheWheelScrollsTheChat(t *testing.T) {
	model := longChat()
	model.View()
	model = update(t, model, tea.MouseMsg{X: 20, Y: 25, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp})
	if model.chatScroll != 3 {
		t.Errorf("scroll = %d, want 3", model.chatScroll)
	}
	model = update(t, model, tea.MouseMsg{X: 20, Y: 25, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})
	if model.chatScroll != 0 {
		t.Errorf("scroll = %d, want 0", model.chatScroll)
	}
}

func TestTheHistoryIsCapped(t *testing.T) {
	model := liveModel()
	for index := 0; index < chatHistory+50; index++ {
		model.appendChatLine(chatLine{from: "a", text: "b"})
	}
	if len(model.chatMessages) != chatHistory {
		t.Errorf("kept %d lines, want %d", len(model.chatMessages), chatHistory)
	}
}
