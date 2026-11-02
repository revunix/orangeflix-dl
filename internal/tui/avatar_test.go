package tui

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/revunix/orangeflix-dl/internal/app"
)

// quarterPicture is a picture of four colored quarters, like a small
// avatar of the server.
func quarterPicture(t *testing.T, size int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	colors := [2][2]color.RGBA{
		{{255, 0, 0, 255}, {0, 255, 0, 255}},
		{{0, 0, 255, 255}, {255, 255, 255, 255}},
	}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, colors[y*2/size][x*2/size])
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestDecodeAvatarScalesThePicture(t *testing.T) {
	picture, err := decodeAvatar(quarterPicture(t, 50))
	if err != nil {
		t.Fatal(err)
	}
	want := map[[2]int]color.RGBA{
		{0, 0}: {255, 0, 0, 255}, {0, 3}: {0, 255, 0, 255},
		{3, 0}: {0, 0, 255, 255}, {3, 3}: {255, 255, 255, 255},
	}
	for at, color := range want {
		if got := picture.pixels[at[0]][at[1]]; got != color {
			t.Errorf("pixel %v = %v, want %v", at, got, color)
		}
	}
	if _, err := decodeAvatar([]byte("kein Bild")); err == nil {
		t.Error("text was read as a picture")
	}
	// Das Standardbild des Servers wird zum Punkt in der Farbe des Namens.
	if _, err := decodeAvatar(quarterPicture(t, 45)); !errors.Is(err, errDefaultAvatar) {
		t.Errorf("the default picture was kept: %v", err)
	}
}

func TestTheAvatarIsRoundAndFourCellsWide(t *testing.T) {
	colors(t)
	picture, _ := decodeAvatar(quarterPicture(t, 8))
	rows := picture.rows()
	if len(rows) != avatarRows {
		t.Fatalf("rows = %d", len(rows))
	}
	for index, row := range rows {
		if width := ansi.StringWidth(row); width != avatarCols {
			t.Errorf("row %d is %d wide", index, width)
		}
	}
	// Die Ecken sind ausgespart: oben ein unterer, unten ein oberer Halbblock.
	if plain := ansi.Strip(rows[0]); !strings.HasPrefix(plain, "▄") || !strings.HasSuffix(plain, "▄") {
		t.Errorf("the upper corners are not round: %q", plain)
	}
	if plain := ansi.Strip(rows[1]); !strings.HasPrefix(plain, "▀") || !strings.HasSuffix(plain, "▀") {
		t.Errorf("the lower corners are not round: %q", plain)
	}
}

func TestAvatarsLoadOncePerSender(t *testing.T) {
	model := interactiveChat()
	asked := []string{}
	model.chatAvatarFunc = func(_ context.Context, name string) ([]byte, error) {
		asked = append(asked, name)
		if name == "Ben" {
			return nil, errors.New("offline")
		}
		return quarterPicture(t, 50), nil
	}
	cmd := model.avatarCmds()
	if cmd == nil {
		t.Fatal("no loading")
	}
	for _, msg := range runBatch(cmd) {
		model = update(t, model, msg)
	}
	if strings.Join(asked, ",") != "Ben,Anna" {
		t.Errorf("asked = %v, want the senders without the account and the notes", asked)
	}
	if model.avatars["Anna"] == nil || model.avatars["Ben"] != nil {
		t.Errorf("avatars: Anna %v, Ben %v", model.avatars["Anna"] != nil, model.avatars["Ben"] != nil)
	}
	// Ein zweites Mal wird nicht geladen, auch nicht nach einem Fehler.
	if cmd := model.avatarCmds(); cmd != nil && len(runBatch(cmd)) > 0 {
		t.Error("the pictures load again")
	}
}

func TestANewSenderLoadsWithTheMessage(t *testing.T) {
	model := interactiveChat()
	asked := 0
	model.chatAvatarFunc = func(context.Context, string) ([]byte, error) {
		asked++
		return nil, errors.New("offline")
	}
	model.avatars = map[string]*avatarPicture{"Anna": nil, "Ben": nil}
	updated, cmd := model.Update(EventMsg{Event: appChatEvent("Clara", "Hallo")})
	model = updated.(*Model)
	if cmd == nil {
		t.Fatal("the new sender loads no picture")
	}
	runBatch(cmd)
	if asked != 1 {
		t.Errorf("asked = %d", asked)
	}
}

func TestTheAvatarStandsBesideTheFirstBubble(t *testing.T) {
	colors(t)
	model := interactiveChat()
	model.chatAvatarFunc = func(context.Context, string) ([]byte, error) { return nil, errors.New("offline") }
	model.appendChatLine(chatLine{from: "Ben", text: "Noch eine", time: model.chatMessages[1].time, id: "100-4"})
	model.appendChatLine(chatLine{from: "Ben", text: "Und noch eine", time: model.chatMessages[1].time.Add(time.Second), id: "100-5"})
	lines := strings.Split(model.View(), "\n")
	for index, line := range lines {
		if width := ansi.StringWidth(line); width != model.width {
			t.Errorf("line %d is %d wide, want %d: %q", index, width, model.width, ansi.Strip(line))
		}
	}
	width := 96
	head := ansi.Strip(model.chatMessageLines(4, width)[1])
	if !strings.HasPrefix(head, "▄") {
		t.Errorf("the first bubble of Ben has no avatar: %q", head)
	}
	grouped := ansi.Strip(model.chatMessageLines(5, width)[0])
	if !strings.HasPrefix(grouped, strings.Repeat(" ", avatarRoom)) {
		t.Errorf("the grouped bubble is not lined up under the first: %q", grouped)
	}
	// Die eigene Nachricht steht links wie die anderen, mit Avatar.
	own := ansi.Strip(model.chatMessageLines(3, width)[1])
	if !strings.HasPrefix(own, "▄") || !strings.Contains(own, "Du") {
		t.Errorf("the own message has no avatar on the left: %q", own)
	}
}

func TestTheOwnAvatarUsesTheChatName(t *testing.T) {
	model := interactiveChat()
	asked := []string{}
	model.chatAvatarFunc = func(_ context.Context, name string) ([]byte, error) {
		asked = append(asked, name)
		return nil, errors.New("offline")
	}
	model.avatars = map[string]*avatarPicture{"Anna": nil, "Ben": nil}
	// Ohne bekannten Namen lädt die eigene Nachricht nichts.
	if cmd := model.avatarCmds(); cmd != nil && len(runBatch(cmd)) > 0 {
		t.Fatalf("asked = %v before the name is known", asked)
	}
	event := appChatEvent("revunix", "Hallo")
	event.Self = true
	updated, cmd := model.Update(EventMsg{Event: event})
	model = updated.(*Model)
	runBatch(cmd)
	if model.chatSelfName != "revunix" || strings.Join(asked, ",") != "revunix" {
		t.Errorf("name %q, asked %v", model.chatSelfName, asked)
	}
	// Eine Nachricht, die noch unterwegs ist, trägt denselben Avatar.
	pending := chatLine{from: "Du", text: "gleich", self: true, pending: true}
	if got := model.avatarName(pending); got != "revunix" {
		t.Errorf("avatar of a pending message = %q", got)
	}
}

func TestNoAvatarsWithoutColors(t *testing.T) {
	model := interactiveChat()
	model.chatAvatarFunc = func(context.Context, string) ([]byte, error) { return nil, nil }
	if model.avatarsShown() {
		t.Error("avatars without colors would be empty boxes")
	}
}

// runBatch runs a command and the commands of a batch, and returns their
// messages.
func runBatch(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		if msg == nil {
			return nil
		}
		return []tea.Msg{msg}
	}
	out := []tea.Msg{}
	for _, inner := range batch {
		out = append(out, runBatch(inner)...)
	}
	return out
}

// appChatEvent is a chat message as the application sends it.
func appChatEvent(from, text string) app.Event {
	return app.Event{Kind: app.EventChat, Label: from, Message: text, ChatID: from + "-1", Done: time.Now().Unix()}
}

// appStatus is a state of the chat as the application sends it.
func appStatus(text string) app.Event {
	return app.Event{Kind: app.EventChatStatus, Message: text}
}
