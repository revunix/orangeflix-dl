package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/maldikhan/go.socket.io/socket.io/v5/client/emit"
)

// Room is the room of the public chat.
const Room = "mainroom"

// ReactionKeys are the reactions the web chat offers, in its order. The
// server can name more in its answers.
var ReactionKeys = []string{"thumbsup", "heart", "laugh", "celebrate"}

// ReactionEmojis maps the keys of the reactions to their emoji. The heart
// carries the variation selector: without it the terminal draws the text
// glyph of the font instead of the emoji. With it the heart is two cells
// wide, like the other emoji, and so the layout counts it.
var ReactionEmojis = map[string]string{
	"thumbsup":  "👍",
	"heart":     "❤️",
	"laugh":     "😂",
	"celebrate": "🎉",
	"surprised": "😮",
}

// ReactionNames are the German names of the reactions.
var ReactionNames = map[string]string{
	"thumbsup":  "Daumen hoch",
	"heart":     "Herz",
	"laugh":     "Lachen",
	"celebrate": "Feiern",
	"surprised": "Überrascht",
}

// ReactionState is the reactions of one message at a cursor of the server.
type ReactionState struct {
	MessageID string
	Cursor    string
	Counts    map[string]int
	// Mine are the reactions of the account.
	Mine []string
}

// ReactionUpdate is one change of a reaction, sent by the server to all.
type ReactionUpdate struct {
	MessageID string
	Cursor    string
	Emoji     string
	// Count is the new number of the reaction.
	Count  int
	Active bool
	// Self tells that the account made the change.
	Self bool
}

// ErrReactionUnchanged tells that the server kept the old state; the
// caller should read the state of the message again.
var ErrReactionUnchanged = errors.New("the reaction did not change")

// ReactionError is a refusal of the server.
type ReactionError struct{ Code string }

// Error returns the text of the web chat for the refusal.
func (e ReactionError) Error() string {
	switch e.Code {
	case "unauthenticated":
		return "Zum Reagieren bitte anmelden."
	case "rate_limited":
		return "Zu viele Reaktionen. Bitte kurz warten."
	case "not_found":
		return "Nachricht nicht mehr verfügbar."
	case "forbidden":
		return "Keine Berechtigung für diesen Raum."
	case "invalid_payload":
		return "Reaktion konnte nicht gesendet werden."
	}
	return "Reaktion fehlgeschlagen."
}

// SendReply writes a message to the main room as an answer to another
// message. An empty replyTo writes a plain message.
func (s *Session) SendReply(text, replyTo string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("empty chat message")
	}
	if len([]rune(text)) > 500 {
		return errors.New("the chat message is too long")
	}
	payload := map[string]any{
		"to":       Room,
		"txt":      text,
		"clientId": fmt.Sprintf("tui-%d", time.Now().UnixNano()),
	}
	if replyTo != "" {
		payload["replyTo"] = replyTo
	}
	return s.client.Emit("message", payload)
}

// SendHost writes a message to the studio. Only the team of the show reads
// it; the chat does not show it to the others.
func (s *Session) SendHost(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("empty chat message")
	}
	if len([]rune(text)) > 500 {
		return errors.New("the chat message is too long")
	}
	return s.client.Emit("message", map[string]any{"to": "HOST", "txt": text})
}

// React sets or removes a reaction of the account on a message.
func (s *Session) React(messageID, emoji string, active bool) error {
	answer, err := s.request("reaction", map[string]any{
		"room":      Room,
		"messageId": messageID,
		"emoji":     emoji,
		"active":    active,
	}, 8*time.Second)
	if err != nil {
		return err
	}
	return parseReactionAnswer(answer)
}

// ReactionStates reads the reactions of messages. The server answers for
// at most a hundred messages at once.
func (s *Session) ReactionStates(messageIDs []string) ([]ReactionState, map[string]string, error) {
	states := []ReactionState{}
	emojis := map[string]string{}
	for start := 0; start < len(messageIDs); start += 100 {
		batch := messageIDs[start:min(start+100, len(messageIDs))]
		answer, err := s.request("metadataState", map[string]any{"room": Room, "messageIds": batch}, 10*time.Second)
		if err != nil {
			return states, emojis, err
		}
		part, names, err := parseMetadataState(answer)
		if err != nil {
			return states, emojis, err
		}
		states = append(states, part...)
		for key, emoji := range names {
			emojis[key] = emoji
		}
	}
	return states, emojis, nil
}

// request sends an event and waits for the answer of the server.
func (s *Session) request(event string, payload any, timeout time.Duration) (json.RawMessage, error) {
	answer := make(chan json.RawMessage, 1)
	expired := make(chan struct{}, 1)
	err := s.client.Emit(event, payload,
		emit.WithAck(func(data json.RawMessage) {
			select {
			case answer <- data:
			default:
			}
		}),
		emit.WithTimeout(timeout, func() {
			select {
			case expired <- struct{}{}:
			default:
			}
		}),
	)
	if err != nil {
		return nil, err
	}
	select {
	case data := <-answer:
		return data, nil
	case <-expired:
		return nil, fmt.Errorf("the chat did not answer %s", event)
	case <-s.done:
		return nil, errors.New("the chat is closed")
	case <-time.After(timeout + 2*time.Second):
		return nil, fmt.Errorf("the chat did not answer %s", event)
	}
}

// fetchReactions reads the reactions of the history and hands them to the
// interface.
func (s *Session) fetchReactions(messageIDs []string) {
	if len(messageIDs) == 0 {
		return
	}
	states, emojis, err := s.ReactionStates(messageIDs)
	if err != nil || len(states) == 0 {
		return
	}
	s.send(Event{Kind: EventReaction, Reactions: states, Emojis: emojis})
}

// parseReactionAnswer reads the answer to a reaction.
func parseReactionAnswer(raw json.RawMessage) error {
	var answer struct {
		OK        bool   `json:"ok"`
		Error     string `json:"error"`
		Discarded bool   `json:"discarded"`
		Changed   *bool  `json:"changed"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		return ReactionError{}
	}
	if !answer.OK {
		return ReactionError{Code: answer.Error}
	}
	if answer.Discarded || (answer.Changed != nil && !*answer.Changed) {
		return ErrReactionUnchanged
	}
	return nil
}

// parseMetadataState reads the answer of metadataState.
func parseMetadataState(raw json.RawMessage) ([]ReactionState, map[string]string, error) {
	var answer struct {
		OK        bool              `json:"ok"`
		Cursor    json.RawMessage   `json:"cursor"`
		Emojis    map[string]string `json:"emojis"`
		Reactions []struct {
			MessageID string             `json:"messageId"`
			Counts    map[string]float64 `json:"counts"`
			Mine      []string           `json:"mine"`
		} `json:"reactions"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		return nil, nil, err
	}
	if !answer.OK {
		return nil, nil, errors.New("invalid reaction state")
	}
	cursor := cursorText(answer.Cursor)
	states := make([]ReactionState, 0, len(answer.Reactions))
	for _, entry := range answer.Reactions {
		if entry.MessageID == "" {
			continue
		}
		counts := map[string]int{}
		for key, value := range entry.Counts {
			if count := int(value); count > 0 {
				counts[key] = count
			}
		}
		states = append(states, ReactionState{MessageID: entry.MessageID, Cursor: cursor, Counts: counts, Mine: entry.Mine})
	}
	emojis := map[string]string{}
	for key, emoji := range answer.Emojis {
		if clean := cleanEmoji(emoji); clean != "" {
			emojis[key] = clean
		}
	}
	return states, emojis, nil
}

// parseMetadataEvent reads one change that the server sends to all.
func parseMetadataEvent(raw json.RawMessage, username string) (ReactionUpdate, bool) {
	var event struct {
		Version   int    `json:"version"`
		Type      string `json:"type"`
		ID        string `json:"id"`
		MessageID string `json:"messageId"`
		Room      string `json:"room"`
		Actor     string `json:"actor"`
		Data      *struct {
			Emoji  string   `json:"emoji"`
			Count  *float64 `json:"count"`
			Active bool     `json:"active"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		return ReactionUpdate{}, false
	}
	if event.Version != 1 || event.Type != "reaction.set" || event.ID == "" || event.MessageID == "" ||
		event.Room != Room || event.Data == nil || event.Data.Emoji == "" || event.Data.Count == nil {
		return ReactionUpdate{}, false
	}
	return ReactionUpdate{
		MessageID: event.MessageID,
		Cursor:    event.ID,
		Emoji:     event.Data.Emoji,
		Count:     max(0, int(*event.Data.Count)),
		Active:    event.Data.Active,
		Self:      username != "" && event.Actor == username,
	}, true
}

// cursorText reads a cursor that comes as a text or as a number.
func cursorText(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		return number.String()
	}
	return ""
}

// CompareCursor orders two cursors of the server ("1730000000000-3"): the
// time first, then the sequence. -1 means a is older.
func CompareCursor(a, b string) int {
	at, as := splitCursor(a)
	bt, bs := splitCursor(b)
	switch {
	case at != bt:
		if at < bt {
			return -1
		}
		return 1
	case as != bs:
		if as < bs {
			return -1
		}
		return 1
	}
	return 0
}

func splitCursor(value string) (int64, int64) {
	head, tail, found := strings.Cut(value, "-")
	first, _ := strconv.ParseInt(head, 10, 64)
	if !found {
		return first, 0
	}
	second, _ := strconv.ParseInt(tail, 10, 64)
	return first, second
}

// cleanEmoji trims an emoji of the server. The variation selector stays:
// it makes the terminal draw the emoji and not the text glyph.
func cleanEmoji(value string) string {
	return strings.TrimSpace(value)
}
