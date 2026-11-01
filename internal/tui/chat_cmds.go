package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// startChat connects to the live chat.
func (m *Model) startChat() tea.Cmd {
	if m.openChatFunc == nil {
		return nil
	}
	return func() tea.Msg {
		return ChatStartedMsg{Err: m.openChatFunc(m.ctx)}
	}
}

// sendChat sends the text in the input field.
func (m *Model) sendChat() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.chatInput.Value())
	if text == "" {
		return m, nil
	}
	if !m.chatConnected || m.sendChatFunc == nil {
		m.chatStatus = "Der Chat ist nicht verbunden."
		return m, nil
	}
	m.chatInput.SetValue("")
	m.chatMessages = append(m.chatMessages, chatLine{
		from:    "Du",
		text:    text,
		time:    time.Now(),
		self:    true,
		pending: true,
	})
	return m, func() tea.Msg {
		return ChatSentMsg{Err: m.sendChatFunc(m.ctx, text)}
	}
}

// closeChat disconnects the chat and clears the visible state.
func (m *Model) closeChat() {
	if m.closeChatFunc != nil {
		m.closeChatFunc()
	}
	m.chatFocused = false
	m.chatConnected = false
	m.chatLoading = false
	m.chatStatus = ""
	m.chatMessages = nil
	m.chatScroll = 0
	m.chatInput.SetValue("")
	m.chatInput.Blur()
}
