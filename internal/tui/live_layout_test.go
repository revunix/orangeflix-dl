package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

func TestTheChatStandsLeftOfTheLiveBox(t *testing.T) {
	model := interactiveChat()
	model.width, model.height = 140, 40
	info, chat := model.liveBoxes(model.width, 34)
	if chat.x >= info.x || chat.width <= info.width {
		t.Errorf("chat %+v, info %+v: the chat must stand left and wide", chat, info)
	}
	if chat.height != info.height || chat.y != info.y {
		t.Errorf("chat %+v, info %+v: both take the whole height", chat, info)
	}
	if right := info.x + info.width; right != model.width-margin {
		t.Errorf("the live box ends at %d, want %d", right, model.width-margin)
	}
	view := ansi.Strip(model.View())
	lines := strings.Split(view, "\n")
	for _, line := range lines {
		if strings.Contains(line, "╭─ Chat ") && !strings.Contains(line, "╭─ Live ") {
			t.Errorf("the chat and the live box do not start on one line: %q", line)
		}
	}
	for index, line := range strings.Split(model.View(), "\n") {
		if width := ansi.StringWidth(line); width != model.width {
			t.Errorf("line %d is %d wide", index, width)
		}
	}
}

func TestANarrowTerminalStacksTheBoxes(t *testing.T) {
	model := interactiveChat()
	info, chat := model.liveBoxes(80, 34)
	if info.y >= chat.y || info.width != chat.width || chat.height <= info.height {
		t.Errorf("info %+v, chat %+v: the info must stand small above the chat", info, chat)
	}
}

func TestTheLiveBoxWrapsAndCountsThePeople(t *testing.T) {
	model := interactiveChat()
	model.width, model.height = 120, 40
	model.live.Show.Title = "Frisch gepresst mit einem sehr langen Titel, der nicht in die schmale Spalte passt"
	model.live.Scheduled = &orangeflix.V3LiveScheduled{Title: "Betreutes Zocken LIVE", StartsIn: 7200}
	model.applyEvent(appStatus("123 im Chat"))
	model.applyEvent(appStatus("Chat verbunden"))
	view := ansi.Strip(model.View())
	for _, want := range []string{"● LIVE", "schmale Spalte passt", "123 im Chat", "▶ enter ansehen", "Nächste Sendung", "Betreutes Zocken LIVE", "in 2 Std."} {
		if !strings.Contains(view, want) {
			t.Errorf("the live box misses %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "– –") {
		t.Error("a show without a time shows empty dashes")
	}
}
