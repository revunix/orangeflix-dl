package app

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/revunix/orangeflix-dl/internal/chat"
)

// OpenChat connects to the live chat of the current stream.
func (a *App) OpenChat(ctx context.Context) error {
	client, err := a.v3Client(ctx)
	if err != nil {
		return err
	}
	token, err := client.ChatToken(ctx)
	if err != nil {
		return err
	}
	var claims struct {
		Username string `json:"username"`
		Admin    bool   `json:"admin"`
	}
	if len(token.JWTD) > 0 {
		_ = json.Unmarshal(token.JWTD, &claims)
	}
	a.CloseChat()
	session, err := chat.Open(ctx, chat.DefaultURL, chat.Namespace, token.JWT, claims.Username)
	if err != nil {
		return err
	}
	a.chatMu.Lock()
	a.chatSession = session
	a.chatMu.Unlock()
	go a.forwardChat(session)
	return nil
}

// SendChat writes a message to the live chat.
func (a *App) SendChat(ctx context.Context, text string) error {
	a.chatMu.Lock()
	session := a.chatSession
	a.chatMu.Unlock()
	if session == nil {
		return errors.New("the chat is not connected")
	}
	return session.Send(text)
}

// ReplyChat writes a message to the live chat as an answer to another one.
func (a *App) ReplyChat(ctx context.Context, text, replyTo string) error {
	session, err := a.chat()
	if err != nil {
		return err
	}
	return session.SendReply(text, replyTo)
}

// VotePoll answers a voting of the live show with the answer of the given
// place.
func (a *App) VotePoll(ctx context.Context, answer int) error {
	session, err := a.chat()
	if err != nil {
		return err
	}
	return session.Vote(answer)
}

// SendStudio writes a message to the studio of the live show.
func (a *App) SendStudio(ctx context.Context, text string) error {
	session, err := a.chat()
	if err != nil {
		return err
	}
	return session.SendHost(text)
}

// ReactChat sets or removes a reaction of the account on a chat message.
// chat.ErrReactionUnchanged tells that the server kept its state.
func (a *App) ReactChat(ctx context.Context, messageID, emoji string, active bool) error {
	session, err := a.chat()
	if err != nil {
		return err
	}
	return session.React(messageID, emoji, active)
}

// ChatReactions reads the reactions of chat messages.
func (a *App) ChatReactions(ctx context.Context, messageIDs []string) ([]ChatReaction, map[string]string, error) {
	session, err := a.chat()
	if err != nil {
		return nil, nil, err
	}
	states, emojis, err := session.ReactionStates(messageIDs)
	return chatReactions(states), emojis, err
}

// chat returns the open chat session.
func (a *App) chat() (*chat.Session, error) {
	a.chatMu.Lock()
	session := a.chatSession
	a.chatMu.Unlock()
	if session == nil {
		return nil, errors.New("the chat is not connected")
	}
	return session, nil
}

// chatReactions converts the states of the chat package.
func chatReactions(states []chat.ReactionState) []ChatReaction {
	out := make([]ChatReaction, 0, len(states))
	for _, state := range states {
		out = append(out, ChatReaction{MessageID: state.MessageID, Cursor: state.Cursor, Counts: state.Counts, Mine: state.Mine})
	}
	return out
}

// CloseChat disconnects the live chat.
func (a *App) CloseChat() {
	a.chatMu.Lock()
	session := a.chatSession
	a.chatSession = nil
	a.chatMu.Unlock()
	if session != nil {
		session.Close()
	}
}

// forwardChat turns the chat events into application events.
func (a *App) forwardChat(session *chat.Session) {
	for {
		select {
		case event := <-session.Events():
			switch event.Kind {
			case chat.EventMessage:
				a.event(Event{
					Kind:        EventChat,
					Label:       event.Message.From,
					Message:     event.Message.Text,
					Done:        event.Message.Time.Unix(),
					Self:        event.Message.Self,
					Admin:       event.Message.Admin,
					ChatID:      event.Message.ID,
					ChatReplyTo: event.Message.ReplyTo,
				})
			case chat.EventPoll:
				out := Event{Kind: EventChatPoll, PollRemoved: event.PollRemoved}
				if poll := event.Poll; poll != nil {
					out.Poll = &ChatPoll{
						Question:       poll.Question,
						Answers:        poll.Answers,
						Results:        poll.Results,
						PollID:         poll.PollID,
						Secret:         poll.Secret,
						ResultsVisible: poll.ResultsVisible,
						Submissions:    poll.Submissions,
						Closed:         poll.Closed,
					}
				}
				a.event(out)
			case chat.EventReaction:
				out := Event{Kind: EventChatReaction, Reactions: chatReactions(event.Reactions), Emojis: event.Emojis}
				if update := event.Update; update != nil {
					out.Reaction = &ChatReactionChange{
						MessageID: update.MessageID, Cursor: update.Cursor, Emoji: update.Emoji,
						Count: update.Count, Active: update.Active, Self: update.Self,
					}
				}
				a.event(out)
			case chat.EventStatus:
				a.event(Event{Kind: EventChatStatus, Message: event.Status})
			case chat.EventError:
				a.event(Event{Kind: EventChatStatus, Message: event.Err.Error(), Error: true})
			}
		case <-session.Done():
			return
		}
	}
}
