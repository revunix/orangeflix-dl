// Package chat connects to the live chat of the streaming platform.
package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"strings"
	"sync"
	"time"

	socketio "github.com/maldikhan/go.socket.io/socket.io/v5/client"
)

// DefaultURL is the Socket.IO server of the platform.
const DefaultURL = "https://io.massengeschmack.tv"

// Namespace is the chat namespace of the Socket.IO server.
const Namespace = "/chat"

// EventKind classifies one event of the chat.
type EventKind int

const (
	// EventMessage is one chat message.
	EventMessage EventKind = iota
	// EventStatus reports a connection state.
	EventStatus
	// EventError reports a chat failure.
	EventError
)

// Message is one line of the live chat.
type Message struct {
	ID      string
	From    string
	Text    string
	To      string
	ReplyTo string
	Time    time.Time
	Admin   bool
	Self    bool
}

// Event is one event of the chat session.
type Event struct {
	Kind    EventKind
	Message Message
	Status  string
	Err     error
}

// Session is one connection to the live chat.
type Session struct {
	client    *socketio.Client
	events    chan Event
	done      chan struct{}
	closeOnce sync.Once
}

// Open connects to the chat and logs in with the token.
func Open(ctx context.Context, baseURL, namespace, jwt, username string) (*Session, error) {
	if strings.TrimSpace(jwt) == "" {
		return nil, errors.New("no chat token")
	}
	client, err := socketio.NewClient(
		socketio.WithRawURL(baseURL),
		socketio.WithDefaultNamespace(namespace),
		socketio.WithConnectTimeout(15*time.Second),
	)
	if err != nil {
		return nil, err
	}
	session := &Session{
		client: client,
		events: make(chan Event, 1024),
		done:   make(chan struct{}),
	}
	client.On("msg", func(data json.RawMessage) {
		var message wireMessage
		if err := json.Unmarshal(data, &message); err == nil && message.To != "HOST" {
			session.send(Event{Kind: EventMessage, Message: message.message(username)})
		}
	})
	client.On("shorthistory", func(data json.RawMessage) {
		var messages []wireMessage
		if err := json.Unmarshal(data, &messages); err != nil {
			return
		}
		for _, message := range messages {
			if message.To == "HOST" {
				continue
			}
			session.send(Event{Kind: EventMessage, Message: message.message(username)})
		}
	})
	client.On("userlist", func(data json.RawMessage) {
		var users []string
		if err := json.Unmarshal(data, &users); err == nil {
			session.send(Event{Kind: EventStatus, Status: fmt.Sprintf("%d im Chat", len(users))})
		}
	})
	client.On("connect", func() {
		session.send(Event{Kind: EventStatus, Status: "Chat verbunden"})
	})
	client.On("disconnect", func() {
		session.send(Event{Kind: EventStatus, Status: "Chat getrennt"})
	})
	client.On("reconnect", func() {
		if err := client.Emit("login", jwt); err != nil {
			session.send(Event{Kind: EventError, Err: err})
			return
		}
		session.send(Event{Kind: EventStatus, Status: "Chat verbunden"})
	})

	if err := client.Connect(ctx); err != nil {
		_ = client.Close()
		return nil, err
	}

	// The login is an ordinary event; the client waits until the namespace
	// is connected. A broken server would otherwise block forever.
	login := make(chan error, 1)
	go func() { login <- client.Emit("login", jwt) }()
	select {
	case err := <-login:
		if err != nil {
			_ = client.Close()
			return nil, err
		}
	case <-time.After(15 * time.Second):
		_ = client.Close()
		return nil, errors.New("the chat login timed out")
	}
	return session, nil
}

// Events returns the events of the chat.
func (s *Session) Events() <-chan Event { return s.events }

// Done is closed when the session is closed.
func (s *Session) Done() <-chan struct{} { return s.done }

// Send writes a message to the main room.
func (s *Session) Send(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("empty chat message")
	}
	if len([]rune(text)) > 500 {
		return errors.New("the chat message is too long")
	}
	return s.client.Emit("message", map[string]any{
		"to":  "mainroom",
		"txt": text,
	})
}

// Close disconnects the chat.
func (s *Session) Close() {
	s.closeOnce.Do(func() {
		close(s.done)
		_ = s.client.Close()
	})
}

// send delivers one event without blocking the socket reader. When the
// interface falls behind, the oldest event is dropped.
func (s *Session) send(event Event) {
	select {
	case <-s.done:
		return
	default:
	}
	select {
	case s.events <- event:
		return
	default:
	}
	// A status can be missed. A message takes the place of the oldest
	// event, so a full queue does not swallow the chat.
	if event.Kind != EventMessage {
		return
	}
	select {
	case <-s.events:
	default:
	}
	select {
	case s.events <- event:
	case <-s.done:
	}
}

// wireMessage is the JSON shape of one chat message.
type wireMessage struct {
	ID      string          `json:"id"`
	From    string          `json:"from"`
	Text    string          `json:"text"`
	To      string          `json:"to"`
	ReplyTo string          `json:"replyTo"`
	H       bool            `json:"h"`
	Time    json.RawMessage `json:"time"`
}

// message converts the wire shape into a message.
func (w wireMessage) message(username string) Message {
	return Message{
		ID:      w.ID,
		From:    w.From,
		Text:    html.UnescapeString(w.Text),
		To:      w.To,
		ReplyTo: w.ReplyTo,
		Time:    parseTime(w.Time),
		Admin:   w.H,
		Self:    w.From == username,
	}
}

// parseTime accepts the RFC3339 strings of the history and the numeric
// timestamps of some live events.
func parseTime(raw json.RawMessage) time.Time {
	if len(raw) == 0 || string(raw) == "null" {
		return time.Time{}
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if moment, err := time.Parse(time.RFC3339Nano, text); err == nil {
			return moment
		}
	}
	var number int64
	if err := json.Unmarshal(raw, &number); err == nil {
		if number > 1_000_000_000_000 {
			number /= 1000
		}
		return time.Unix(number, 0)
	}
	return time.Time{}
}
