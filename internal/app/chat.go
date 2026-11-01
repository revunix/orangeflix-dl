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
					Kind:    EventChat,
					Label:   event.Message.From,
					Message: event.Message.Text,
					Done:    event.Message.Time.Unix(),
					Self:    event.Message.Self,
					Admin:   event.Message.Admin,
				})
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
