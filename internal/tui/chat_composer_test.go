package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestTheFieldInvitesToWriteAndAClickFocusesIt(t *testing.T) {
	model := interactiveChat()
	view := ansi.Strip(model.View())
	if !strings.Contains(view, "Nachricht schreiben …") || !strings.Contains(view, "😂") {
		t.Fatalf("the field does not invite to write or misses the emoji:\n%s", view)
	}
	model = clickOn(t, model, "Nachricht schreiben", tea.MouseButtonLeft)
	if !model.chatFocused {
		t.Fatal("a click into the field does not focus it")
	}
	view = ansi.Strip(model.View())
	for _, want := range []string{"Nachricht an den Chat", "0/500", "↵ Chat", "alt+1…9"} {
		if !strings.Contains(view, want) {
			t.Errorf("the focused field misses %q:\n%s", want, view)
		}
	}
}

func TestTheFieldGrowsWithTheText(t *testing.T) {
	model := interactiveChat()
	model = press(t, model, "c")
	if lines := model.composerFieldLines(80); lines != composerMinLines {
		t.Errorf("empty field has %d lines, want %d", lines, composerMinLines)
	}
	model.chatInput.SetValue(strings.Repeat("wort ", 60))
	if lines := model.composerFieldLines(80); lines != composerMaxLines {
		t.Errorf("long text has %d lines, want %d", lines, composerMaxLines)
	}
	model = press(t, model, "esc")
	if lines := model.composerFieldLines(80); lines != 1 {
		t.Errorf("the field without the keyboard has %d lines", lines)
	}
}

func TestTheCounterWarnsBeforeTheLimit(t *testing.T) {
	colors(t)
	model := interactiveChat()
	model = press(t, model, "c")
	model.chatInput.SetValue(strings.Repeat("a", 470))
	field := strings.Join(model.composerField(80, false), "")
	if !strings.Contains(field, "470/500") || !strings.Contains(field, sequenceOf(theme.Warning)) {
		t.Errorf("no warning at 470: %q", field)
	}
	model.chatInput.SetValue(strings.Repeat("a", 520))
	field = strings.Join(model.composerField(80, false), "")
	if !strings.Contains(field, "520/500") || !strings.Contains(field, sequenceOf(theme.Danger)) {
		t.Errorf("no alarm over the limit: %q", field)
	}
	model.sendChatFunc = func(context.Context, string) error {
		t.Error("a message over the limit went out")
		return nil
	}
	model = press(t, model, "enter")
	if !strings.Contains(model.message, "zu lang") || model.chatInput.Value() == "" {
		t.Errorf("message %q, the text was dropped: %v", model.message, model.chatInput.Value() == "")
	}
}

func TestTheEmojiGoIntoTheText(t *testing.T) {
	model := interactiveChat()
	model = press(t, model, "c")
	model = press(t, model, "J")
	model = update(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a"), Alt: false})
	model = update(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2"), Alt: true})
	if got := model.chatInput.Value(); got != "Ja👍" {
		t.Errorf("value = %q after alt+2", got)
	}
	// Ein Klick auf ein Emoji setzt den Cursor ins Feld und fügt es ein.
	model = press(t, model, "esc")
	model.chatInput.SetValue("")
	model = clickOn(t, model, "🔥", tea.MouseButtonLeft)
	if !model.chatFocused || model.chatInput.Value() != "🔥" {
		t.Errorf("click: focused %v, value %q", model.chatFocused, model.chatInput.Value())
	}
}

func TestTabSendsToTheStudio(t *testing.T) {
	model := interactiveChat()
	var studio, chat []string
	model.sendStudioFunc = func(_ context.Context, text string) error {
		studio = append(studio, text)
		return nil
	}
	model.sendChatFunc = func(_ context.Context, text string) error {
		chat = append(chat, text)
		return nil
	}
	model = press(t, model, "c")
	model = update(t, model, tea.KeyMsg{Type: tea.KeyTab})
	if !model.chatToStudio {
		t.Fatal("tab did not switch to the studio")
	}
	view := ansi.Strip(model.View())
	if !strings.Contains(view, "nur das Team liest sie") || !strings.Contains(view, "↵ Studio") || !strings.Contains(view, "tab Chat") {
		t.Errorf("the field does not show the studio:\n%s", view)
	}
	model.chatInput.SetValue("Ton ist weg")
	updated, cmd := model.Update(keyMsgFor("enter"))
	model = updated.(*Model)
	cmd()
	if len(studio) != 1 || studio[0] != "Ton ist weg" || len(chat) != 0 {
		t.Errorf("studio %v, chat %v", studio, chat)
	}
	last := model.chatMessages[len(model.chatMessages)-1]
	if !last.studio || chatSelectable(last) {
		t.Errorf("the message to the studio = %+v", last)
	}
	if view := ansi.Strip(model.View()); !strings.Contains(view, "Du → Studio") {
		t.Errorf("the chat does not mark the message to the studio:\n%s", view)
	}

	// Zurück zum Chat, und /host schickt trotzdem ans Studio.
	model = update(t, model, tea.KeyMsg{Type: tea.KeyTab})
	model.chatInput.SetValue("/host Bild hängt")
	updated, cmd = model.Update(keyMsgFor("enter"))
	model = updated.(*Model)
	cmd()
	if len(studio) != 2 || studio[1] != "Bild hängt" || len(chat) != 0 {
		t.Errorf("/host: studio %v, chat %v", studio, chat)
	}
}

func TestTheAnswerSitsOnTheField(t *testing.T) {
	model := interactiveChat()
	model.replyChatFunc = func(context.Context, string, string) error { return nil }
	model = press(t, model, "s")
	model = press(t, model, "up")
	model = press(t, model, "a")
	rows, _ := model.chatComposer(90, false)
	if len(rows) != model.composerRows() || len(rows) != 2+composerMinLines+1 {
		t.Fatalf("rows = %d, composerRows = %d", len(rows), model.composerRows())
	}
	reply := ansi.Strip(rows[1])
	if !strings.Contains(reply, "↪ Antwort an Ben · Zweite Nachricht") || !strings.Contains(reply, "esc ✕") {
		t.Errorf("reply = %q", reply)
	}
}

func TestTheRuleTellsThatTheChatIsScrolled(t *testing.T) {
	model := longChat()
	model.View()
	model = press(t, model, "up")
	model = press(t, model, "up")
	rule := ansi.Strip(model.composerRule(80))
	if !strings.Contains(rule, "↑ 2 Zeilen zurück · end") || ansi.StringWidth(rule) != 80 {
		t.Errorf("rule = %q (%d)", rule, ansi.StringWidth(rule))
	}
}

func TestTheFieldKeepsTheWidth(t *testing.T) {
	for _, withColors := range []bool{false, true} {
		if withColors {
			colors(t)
		}
		model := interactiveChat()
		model.replyChatFunc = func(context.Context, string, string) error { return nil }
		model.sendStudioFunc = func(context.Context, string) error { return nil }
		for _, width := range []int{30, 60, 97} {
			for _, state := range []string{"idle", "selected", "typing", "long", "reply", "studio"} {
				model.chatFocused, model.chatReplyTo, model.chatSelectedID, model.chatToStudio = false, nil, "", false
				model.chatInput.SetValue("")
				switch state {
				case "selected":
					model.chatSelectedID = "100-1"
				case "typing":
					model.chatFocused = true
					model.chatInput.SetValue("Hallo 👋 zusammen ❤️")
				case "long":
					model.chatFocused = true
					model.chatInput.SetValue(strings.Repeat("ganz lange Nachricht 😂 ", 20))
				case "reply":
					model.chatFocused = true
					target := model.chatMessages[0]
					model.chatReplyTo = &target
				case "studio":
					model.chatFocused, model.chatToStudio = true, true
				}
				rows, _ := model.chatComposer(width, state == "selected")
				for index, row := range rows {
					if got := ansi.StringWidth(row); got != width {
						t.Errorf("colors %v, width %d, %s, row %d is %d wide: %q", withColors, width, state, index, got, ansi.Strip(row))
					}
				}
			}
		}
	}
}
