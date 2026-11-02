package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/revunix/orangeflix-dl/internal/app"
	"github.com/revunix/orangeflix-dl/internal/chat"
)

// interactiveChat is a live chat with messages that carry the ids of the
// server, so they can be answered and reacted to.
func interactiveChat() *Model {
	model := liveModel()
	now := time.Now()
	model.chatMessages = nil
	model.appendChatLine(chatLine{from: "Anna", text: "Erste Nachricht", time: now, id: "100-1"})
	model.appendChatLine(chatLine{from: "Ben", text: "Zweite Nachricht", time: now, id: "100-2"})
	model.appendChatLine(chatLine{text: "Chat verbunden", system: true})
	model.appendChatLine(chatLine{from: "Du", text: "Meine", time: now, self: true, id: "100-3"})
	return model
}

func TestSChoosesTheNewestMessageAndArrowsMove(t *testing.T) {
	model := interactiveChat()
	model = press(t, model, "s")
	if model.chatSelectedID != "100-3" {
		t.Fatalf("selected = %q, want the newest", model.chatSelectedID)
	}
	model = press(t, model, "up")
	if model.chatSelectedID != "100-2" {
		t.Errorf("up chose %q, the note of the program must be skipped", model.chatSelectedID)
	}
	view := ansi.Strip(model.View())
	if !strings.Contains(view, "▌") || !strings.Contains(view, "a antworten") {
		t.Errorf("the choice is not shown:\n%s", view)
	}
	model = press(t, model, "esc")
	if model.chatSelectedID != "" || model.screen != ScreenLive {
		t.Errorf("esc left the live screen or kept the choice: %q %v", model.chatSelectedID, model.screen)
	}
}

func TestAnAnswerQuotesTheMessage(t *testing.T) {
	model := interactiveChat()
	var sent, to string
	model.replyChatFunc = func(_ context.Context, text, replyTo string) error {
		sent, to = text, replyTo
		return nil
	}
	model.sendChatFunc = func(context.Context, string) error {
		t.Error("an answer went out as a plain message")
		return nil
	}
	model = press(t, model, "s")
	model = press(t, model, "up")
	model = press(t, model, "a")
	if !model.chatFocused || model.chatReplyTo == nil {
		t.Fatal("a did not open the answer")
	}
	if view := ansi.Strip(model.View()); !strings.Contains(view, "↪ Antwort an Ben · Zweite Nachricht") {
		t.Errorf("the answer is not shown above the input:\n%s", view)
	}
	for _, key := range []string{"J", "a"} {
		model = press(t, model, key)
	}
	updated, cmd := model.Update(keyMsgFor("enter"))
	model = updated.(*Model)
	cmd()
	if sent != "Ja" || to != "100-2" {
		t.Errorf("sent %q to %q", sent, to)
	}
	if model.chatReplyTo != nil {
		t.Error("the answer stays open after sending")
	}
	if view := ansi.Strip(model.View()); !strings.Contains(view, "▎Ben: Zweite Nachricht") {
		t.Errorf("the chat does not quote the answered message:\n%s", view)
	}
}

func TestEscFirstDropsTheAnswer(t *testing.T) {
	model := interactiveChat()
	model.replyChatFunc = func(context.Context, string, string) error { return nil }
	model = press(t, model, "s")
	model = press(t, model, "a")
	model = press(t, model, "esc")
	if model.chatReplyTo != nil || !model.chatFocused {
		t.Fatalf("esc: answer %v, focused %v", model.chatReplyTo != nil, model.chatFocused)
	}
	model = press(t, model, "esc")
	if model.chatFocused {
		t.Error("the second esc did not leave the input")
	}
}

func TestAReactionFromTheMenu(t *testing.T) {
	model := interactiveChat()
	calls := []string{}
	model.reactChatFunc = func(_ context.Context, id, emoji string, active bool) error {
		state := "an"
		if !active {
			state = "aus"
		}
		calls = append(calls, id+" "+emoji+" "+state)
		return nil
	}
	model = press(t, model, "s")
	model = press(t, model, "up")
	model = press(t, model, "enter")
	if model.context == nil {
		t.Fatal("enter did not open the menu of the message")
	}
	if view := ansi.Strip(model.View()); !strings.Contains(view, "👍 Daumen hoch") || !strings.Contains(view, "Antworten") {
		t.Errorf("the menu misses the entries:\n%s", view)
	}
	updated, cmd := model.Update(keyMsgFor("1"))
	model = updated.(*Model)
	cmd()
	line := model.chatMessages[model.chatIndex("100-2")]
	if line.reactions["thumbsup"] != 1 || !line.mine["thumbsup"] {
		t.Errorf("reactions = %v, mine = %v", line.reactions, line.mine)
	}
	if view := ansi.Strip(model.View()); !strings.Contains(view, "👍 1") {
		t.Errorf("the reaction is not shown:\n%s", view)
	}

	// Ein zweites Mal nimmt sie zurück.
	model = press(t, model, ".")
	if !strings.Contains(model.context.items[1].label, "zurücknehmen") {
		t.Errorf("item = %q", model.context.items[1].label)
	}
	updated, cmd = model.Update(keyMsgFor("1"))
	model = updated.(*Model)
	cmd()
	if got := strings.Join(calls, ", "); got != "100-2 thumbsup an, 100-2 thumbsup aus" {
		t.Errorf("calls = %s", got)
	}
	if line := model.chatMessages[model.chatIndex("100-2")]; len(line.reactions) != 0 {
		t.Errorf("reactions after taking back = %v", line.reactions)
	}
}

func TestARefusedReactionGoesBack(t *testing.T) {
	model := interactiveChat()
	model.reactChatFunc = func(context.Context, string, string, bool) error {
		return chat.ReactionError{Code: "rate_limited"}
	}
	updated, cmd := model.toggleReaction("100-1", "heart")
	model = updated.(*Model)
	model = update(t, model, cmd())
	if line := model.chatMessages[model.chatIndex("100-1")]; len(line.reactions) != 0 || line.mine["heart"] {
		t.Errorf("the reaction stayed: %v %v", line.reactions, line.mine)
	}
	if model.message != "Zu viele Reaktionen. Bitte kurz warten." || !model.isError {
		t.Errorf("message = %q", model.message)
	}
}

func TestAnUnchangedReactionReadsTheStateAgain(t *testing.T) {
	model := interactiveChat()
	model.reactChatFunc = func(context.Context, string, string, bool) error { return chat.ErrReactionUnchanged }
	model.chatReactionsFunc = func(_ context.Context, ids []string) ([]app.ChatReaction, map[string]string, error) {
		return []app.ChatReaction{{MessageID: ids[0], Cursor: "100-9", Counts: map[string]int{"laugh": 4}}}, nil, nil
	}
	updated, cmd := model.toggleReaction("100-1", "laugh")
	model = updated.(*Model)
	updated, refresh := model.Update(cmd())
	model = updated.(*Model)
	if refresh == nil {
		t.Fatal("no new read of the state")
	}
	model = update(t, model, refresh())
	line := model.chatMessages[model.chatIndex("100-1")]
	if line.reactions["laugh"] != 4 || line.mine["laugh"] {
		t.Errorf("reactions = %v, mine = %v", line.reactions, line.mine)
	}
}

func TestOwnMessagesTakeNoReaction(t *testing.T) {
	model := interactiveChat()
	model = press(t, model, "s")
	model = press(t, model, ".")
	for _, item := range model.context.items {
		if strings.Contains(item.label, "Daumen") {
			t.Errorf("the menu of an own message offers %q", item.label)
		}
	}
	model = press(t, model, "esc")
	model.reactChatFunc = func(context.Context, string, string, bool) error { return nil }
	updated, cmd := model.toggleReaction("100-3", "heart")
	model = updated.(*Model)
	if cmd != nil || !strings.Contains(model.message, "eigene") {
		t.Errorf("an own message took a reaction: %q", model.message)
	}
}

func TestChangesOfTheServerKeepTheirOrder(t *testing.T) {
	model := interactiveChat()
	model.applyReactionStates([]app.ChatReaction{{MessageID: "100-1", Cursor: "200-5", Counts: map[string]int{"heart": 2}, Mine: []string{"heart"}}},
		map[string]string{"fire": "🔥"})
	model.applyReactionChange(app.ChatReactionChange{MessageID: "100-1", Cursor: "200-4", Emoji: "heart", Count: 9})
	line := model.chatMessages[model.chatIndex("100-1")]
	if line.reactions["heart"] != 2 {
		t.Errorf("an older change won: %v", line.reactions)
	}
	model.applyReactionChange(app.ChatReactionChange{MessageID: "100-1", Cursor: "200-6", Emoji: "heart", Count: 3})
	model.applyReactionChange(app.ChatReactionChange{MessageID: "100-1", Cursor: "200-7", Emoji: "laugh", Count: 1})
	line = model.chatMessages[model.chatIndex("100-1")]
	if line.reactions["heart"] != 3 || line.reactions["laugh"] != 1 || !line.mine["heart"] {
		t.Errorf("reactions = %v, mine = %v", line.reactions, line.mine)
	}
	// Ein älterer Schnappschuss überschreibt nichts.
	model.applyReactionStates([]app.ChatReaction{{MessageID: "100-1", Cursor: "200-1", Counts: map[string]int{}}}, nil)
	if line := model.chatMessages[model.chatIndex("100-1")]; line.reactions["heart"] != 3 {
		t.Errorf("an older state won: %v", line.reactions)
	}
	// Eine Reaktion, die nur der Server kennt, wird gezählt, aber nicht
	// angeboten: der Web-Chat hat genau vier.
	model.applyReactionChange(app.ChatReactionChange{MessageID: "100-1", Cursor: "200-8", Emoji: "fire", Count: 2})
	if view := ansi.Strip(model.View()); !strings.Contains(view, "🔥 2") {
		t.Errorf("the counted reaction of the server is not shown:\n%s", view)
	}
	model.chatSelectedID = "100-1"
	_, items := model.chatMenu()
	offered := []string{}
	for _, item := range items {
		if item.key >= "1" && item.key <= "9" {
			offered = append(offered, item.label)
		}
	}
	if strings.Join(offered, "|") != "👍 Daumen hoch|❤️ Herz zurücknehmen|😂 Lachen|🎉 Feiern" {
		t.Errorf("offered = %v", offered)
	}
}

func TestTheMouseChoosesAndOpensAMessage(t *testing.T) {
	model := interactiveChat()
	model = clickOn(t, model, "Erste Nachricht", tea.MouseButtonLeft)
	if model.chatSelectedID != "100-1" || model.context != nil {
		t.Fatalf("click: selected %q, menu %v", model.chatSelectedID, model.context != nil)
	}
	model = clickOn(t, model, "Erste Nachricht", tea.MouseButtonLeft)
	if model.context == nil || !model.context.anchored {
		t.Fatal("the second click did not open the menu at the pointer")
	}
	model = press(t, model, "esc")
	model = clickOn(t, model, "Zweite Nachricht", tea.MouseButtonRight)
	if model.chatSelectedID != "100-2" || model.context == nil {
		t.Errorf("right click: selected %q, menu %v", model.chatSelectedID, model.context != nil)
	}
}

func TestAReplayOnlyCopies(t *testing.T) {
	model := interactiveChat()
	model.chatConnected = false
	model.chatSelectedID = "100-1"
	_, items := model.chatMenu()
	if len(items) != 1 || items[0].label != "Text kopieren" {
		labels := []string{}
		for _, item := range items {
			labels = append(labels, item.label)
		}
		t.Errorf("items = %v", labels)
	}
}

func TestTheChatStaysInItsBoxWithQuotesAndReactions(t *testing.T) {
	t.Run("ohne Farben", func(t *testing.T) { checkChatBox(t) })
	t.Run("mit Farben", func(t *testing.T) {
		colors(t)
		checkChatBox(t)
	})
}

func checkChatBox(t *testing.T) {
	t.Helper()
	model := interactiveChat()
	model.chatMessages[1].replyTo = "100-1"
	model.chatMessages[1].reactions = map[string]int{"thumbsup": 12, "heart": 3, "laugh": 1, "celebrate": 7}
	model.chatMessages[1].mine = map[string]bool{"heart": true}
	model.chatSelectedID = "100-2"
	lines := strings.Split(model.View(), "\n")
	for index, line := range lines {
		if width := ansi.StringWidth(line); width != model.width {
			t.Errorf("line %d is %d wide, want %d: %q", index, width, model.width, ansi.Strip(line))
		}
	}
	view := ansi.Strip(strings.Join(lines, "\n"))
	for _, want := range []string{"▎Anna: Erste Nachricht", "👍 12", "❤️ 3", "😂 1", "🎉 7"} {
		if !strings.Contains(view, want) {
			t.Errorf("the chat misses %q:\n%s", want, view)
		}
	}
}
