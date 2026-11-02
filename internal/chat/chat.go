// Package chat connects to the live chat of the streaming platform.
package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"regexp"
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
	// EventReaction carries reactions: the state of the history or one
	// change.
	EventReaction
	// EventPoll carries a voting of the live show: the new one, the changed
	// votes or the end.
	EventPoll
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
	// Reactions is the state of messages, Update one change; Emojis names
	// reactions the client does not know yet.
	Reactions []ReactionState
	Update    *ReactionUpdate
	Emojis    map[string]string
	// Poll is the voting, Answers the number of its choices; PollRemoved
	// tells that the voting is over and gone.
	Poll        *Poll
	PollAnswers int
	PollRemoved bool
}

// Session is one connection to the live chat.
type Session struct {
	client    *socketio.Client
	events    chan Event
	done      chan struct{}
	closeOnce sync.Once
	// poll is the running voting; pollMu guards it.
	poll   *Poll
	pollMu sync.Mutex
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
		// Die Bibliothek schreibt sonst jedes Paket über das Standard-log
		// ins Terminal, mitten in die Oberfläche.
		socketio.WithLogger(quietLogger{}),
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
		ids := []string{}
		for _, message := range messages {
			if message.To == "HOST" {
				continue
			}
			session.send(Event{Kind: EventMessage, Message: message.message(username)})
			if message.ID != "" {
				ids = append(ids, message.ID)
			}
		}
		// Die Reaktionen der Vorgeschichte stehen nicht in den Nachrichten.
		go session.fetchReactions(ids)
	})
	client.On("newVoting", func(data json.RawMessage) {
		if poll, ok := parsePoll(data); ok {
			session.send(Event{Kind: EventPoll, Poll: &poll})
		}
	})
	client.On("voteOccured", func(data json.RawMessage) {
		session.updatePoll(data)
	})
	client.On("votingClosed", func(data json.RawMessage) {
		session.updatePoll(data)
	})
	client.On("resultsVisibilityChanged", func(data json.RawMessage) {
		session.updatePoll(data)
	})
	client.On("submissionCount", func(data json.RawMessage) {
		session.updatePoll(data)
	})
	client.On("deleteVoting", func(data json.RawMessage) {
		session.send(Event{Kind: EventPoll, PollRemoved: true})
	})
	client.On("metadata", func(data json.RawMessage) {
		if update, ok := parseMetadataEvent(data, username); ok {
			session.send(Event{Kind: EventReaction, Update: &update})
		}
	})
	client.On("pinnedmsg", func(data json.RawMessage) {
		if text := pinnedText(data); text != "" {
			session.send(Event{Kind: EventMessage, Message: Message{From: "Angeheftet", Text: text, Admin: true, Time: time.Now()}})
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
	return s.SendReply(text, "")
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
	if event.Kind != EventMessage && event.Kind != EventReaction {
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
		Text:    cleanText(w.Text),
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

// quietLogger drops the diagnostics of the Socket.IO library. Failures that
// matter reach the session as errors of Connect, Emit and the disconnect.
type quietLogger struct{}

func (quietLogger) Debugf(string, ...any) {}
func (quietLogger) Infof(string, ...any)  {}
func (quietLogger) Warnf(string, ...any)  {}
func (quietLogger) Errorf(string, ...any) {}

var (
	// breakTag is a line break of the web chat.
	breakTag = regexp.MustCompile(`(?i)<br\s*/?>`)
	// markupTag is any other tag of the web chat; the text between the tags
	// stays.
	markupTag = regexp.MustCompile(`</?[a-zA-Z][^<>]*>`)
)

// cleanText turns the HTML of a chat message into plain text. The tags are
// taken out before the entities are decoded: what a user typed arrives
// escaped ("&lt;3") and must stay. A line break that was escaped twice is
// caught after decoding.
func cleanText(raw string) string {
	text := breakTag.ReplaceAllString(raw, " ")
	text = markupTag.ReplaceAllString(text, "")
	text = html.UnescapeString(text)
	text = breakTag.ReplaceAllString(text, " ")
	return strings.Join(strings.Fields(text), " ")
}

// pinnedText reads the pinned message. The server sends it as a message
// object or as a plain text; an empty answer means nothing is pinned.
func pinnedText(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return cleanText(text)
	}
	var object struct {
		Text    string `json:"text"`
		Txt     string `json:"txt"`
		Msg     string `json:"msg"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &object); err == nil {
		for _, candidate := range []string{object.Text, object.Txt, object.Msg, object.Message} {
			if candidate != "" {
				return cleanText(candidate)
			}
		}
	}
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err == nil && len(list) > 0 {
		return pinnedText(list[0])
	}
	return ""
}

// updatePoll takes over the new votes of the running voting.
func (s *Session) updatePoll(data json.RawMessage) {
	poll := s.currentPoll()
	if poll == nil {
		return
	}
	if pollID, results, submissions, ok := parsePollResults(data, len(poll.Answers)); ok {
		if pollID != "" && poll.PollID != "" && pollID != poll.PollID {
			return
		}
		poll.Results = results
		poll.ResultsVisible = true
		if submissions > 0 {
			poll.Submissions = submissions
		}
	}
	if pollID, visible, results, submissions, ok := parsePollVisibility(data, len(poll.Answers)); ok {
		if poll.PollID != "" && pollID != poll.PollID {
			return
		}
		poll.ResultsVisible = visible
		if visible && results != nil {
			poll.Results = results
		}
		if submissions > 0 {
			poll.Submissions = submissions
		}
	}
	if closed := closedPoll(data); closed {
		poll.Closed = true
		poll.ResultsVisible = true
	}
	copy := *poll
	s.send(Event{Kind: EventPoll, Poll: &copy})
}

// currentPoll returns the running voting.
func (s *Session) currentPoll() *Poll {
	s.pollMu.Lock()
	defer s.pollMu.Unlock()
	return s.poll
}

// closedPoll tells that the server ended the voting.
func closedPoll(raw json.RawMessage) bool {
	var wire struct {
		Closed bool `json:"closed"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return false
	}
	return wire.Closed
}
