package chat

import (
	"encoding/json"
	"testing"
	"time"
)

func TestWireMessageConvertsTextAndTime(t *testing.T) {
	message := wireMessage{
		ID:   "1",
		From: "Gast",
		Text: "a &gt; b",
		H:    true,
		Time: json.RawMessage(`"2026-11-01T19:24:36.991Z"`),
	}.message("Gast")
	if message.Text != "a > b" {
		t.Errorf("text = %q", message.Text)
	}
	if !message.Admin || !message.Self {
		t.Errorf("message = %+v", message)
	}
	if message.Time.UTC().Format(time.RFC3339Nano) != "2026-11-01T19:24:36.991Z" {
		t.Errorf("time = %v", message.Time)
	}
}

func TestAFullQueueKeepsTheMessage(t *testing.T) {
	session := &Session{events: make(chan Event, 1), done: make(chan struct{})}
	session.send(Event{Kind: EventStatus, Status: "alt"})
	session.send(Event{Kind: EventMessage, Message: Message{Text: "neu"}})
	session.send(Event{Kind: EventStatus, Status: "weg"})
	got := <-session.events
	if got.Kind != EventMessage || got.Message.Text != "neu" {
		t.Fatalf("event = %+v, want the message", got)
	}
}

func TestParseTimeAcceptsNumbers(t *testing.T) {
	got := parseTime(json.RawMessage(`1793561076991`))
	if got.Unix() != 1793561076 {
		t.Errorf("time = %v", got)
	}
	if !parseTime(nil).IsZero() {
		t.Error("nil time is not zero")
	}
}

func TestSendRejectsEmptyAndLongMessages(t *testing.T) {
	session := &Session{}
	if err := session.Send("   "); err == nil {
		t.Error("empty message was accepted")
	}
	long := ""
	for range 501 {
		long += "a"
	}
	if err := session.Send(long); err == nil {
		t.Error("long message was accepted")
	}
}
