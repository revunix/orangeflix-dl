package chat

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestParseReactionAnswer(t *testing.T) {
	if err := parseReactionAnswer(json.RawMessage(`{"ok":true,"changed":true}`)); err != nil {
		t.Errorf("ok answer: %v", err)
	}
	if err := parseReactionAnswer(json.RawMessage(`{"ok":true,"changed":false}`)); !errors.Is(err, ErrReactionUnchanged) {
		t.Errorf("unchanged: %v", err)
	}
	if err := parseReactionAnswer(json.RawMessage(`{"ok":true,"discarded":true}`)); !errors.Is(err, ErrReactionUnchanged) {
		t.Errorf("discarded: %v", err)
	}
	err := parseReactionAnswer(json.RawMessage(`{"ok":false,"error":"rate_limited"}`))
	var refusal ReactionError
	if !errors.As(err, &refusal) || err.Error() != "Zu viele Reaktionen. Bitte kurz warten." {
		t.Errorf("rate limit: %v", err)
	}
	if err := parseReactionAnswer(json.RawMessage(`{"ok":false,"error":"neu"}`)); err == nil || err.Error() != "Reaktion fehlgeschlagen." {
		t.Errorf("unknown code: %v", err)
	}
}

func TestParseMetadataState(t *testing.T) {
	states, emojis, err := parseMetadataState(json.RawMessage(`{"ok":true,"cursor":"1730000000000-4",
		"emojis":{"fire":"🔥","heart":"❤️"},
		"reactions":[
			{"messageId":"1730000000000-1","counts":{"thumbsup":3,"heart":0,"laugh":1.9},"mine":["thumbsup"]},
			{"counts":{"thumbsup":1}}
		]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 1 {
		t.Fatalf("states = %+v", states)
	}
	state := states[0]
	if state.MessageID != "1730000000000-1" || state.Cursor != "1730000000000-4" {
		t.Errorf("state = %+v", state)
	}
	if state.Counts["thumbsup"] != 3 || state.Counts["laugh"] != 1 || len(state.Counts) != 2 {
		t.Errorf("counts = %v", state.Counts)
	}
	if len(state.Mine) != 1 || state.Mine[0] != "thumbsup" {
		t.Errorf("mine = %v", state.Mine)
	}
	if emojis["fire"] != "🔥" || emojis["heart"] != "❤️" {
		t.Errorf("emojis = %q", emojis)
	}
	if _, _, err := parseMetadataState(json.RawMessage(`{"ok":false}`)); err == nil {
		t.Error("a refused state was accepted")
	}
}

func TestParseMetadataEvent(t *testing.T) {
	raw := json.RawMessage(`{"version":1,"type":"reaction.set","id":"1730000000000-7","messageId":"1730000000000-1",
		"room":"mainroom","actor":"Ich","data":{"emoji":"heart","count":2,"active":true}}`)
	update, ok := parseMetadataEvent(raw, "Ich")
	if !ok || update.MessageID != "1730000000000-1" || update.Cursor != "1730000000000-7" ||
		update.Emoji != "heart" || update.Count != 2 || !update.Active || !update.Self {
		t.Errorf("update = %+v, ok %v", update, ok)
	}
	if update, _ := parseMetadataEvent(raw, "Andere"); update.Self {
		t.Error("someone else's reaction counts as mine")
	}
	for _, bad := range []string{
		`{"version":2,"type":"reaction.set","id":"1","messageId":"1","room":"mainroom","data":{"emoji":"heart","count":1}}`,
		`{"version":1,"type":"other","id":"1","messageId":"1","room":"mainroom","data":{"emoji":"heart","count":1}}`,
		`{"version":1,"type":"reaction.set","id":"1","messageId":"1","room":"HOST","data":{"emoji":"heart","count":1}}`,
		`{"version":1,"type":"reaction.set","id":"1","messageId":"1","room":"mainroom","data":{"emoji":"heart"}}`,
		`"text"`,
	} {
		if _, ok := parseMetadataEvent(json.RawMessage(bad), "Ich"); ok {
			t.Errorf("accepted %s", bad)
		}
	}
}

func TestCompareCursor(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"100-1", "100-2", -1},
		{"100-2", "100-1", 1},
		{"99-9", "100-0", -1},
		{"100", "100-0", 0},
		{"100-10", "100-9", 1},
		{"", "1-0", -1},
	}
	for _, test := range cases {
		if got := CompareCursor(test.a, test.b); got != test.want {
			t.Errorf("CompareCursor(%q, %q) = %d, want %d", test.a, test.b, got, test.want)
		}
	}
}

func TestAFullQueueKeepsTheReactions(t *testing.T) {
	session := &Session{events: make(chan Event, 1), done: make(chan struct{})}
	session.send(Event{Kind: EventStatus, Status: "alt"})
	session.send(Event{Kind: EventReaction, Update: &ReactionUpdate{MessageID: "1"}})
	event := <-session.events
	if event.Kind != EventReaction {
		t.Errorf("the reaction was dropped: %+v", event)
	}
}
