package tui

import (
	"fmt"
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
	// „/host Text“ schickt wie im Web-Chat ans Studio.
	studio := m.chatToStudio
	if lower := strings.ToLower(text); strings.HasPrefix(lower, "/host ") || lower == "/host" {
		studio = true
		text = strings.TrimSpace(text[len("/host"):])
		if text == "" {
			return m, nil
		}
	}
	if len([]rune(text)) > chatLimit {
		m.message = fmt.Sprintf("Nachricht zu lang (max. %d Zeichen).", chatLimit)
		m.isError = true
		return m, nil
	}
	m.chatInput.SetValue("")
	// Wer schreibt, will seine Nachricht sehen: der Chat springt ans Ende.
	m.chatScroll = 0
	if studio && m.sendStudioFunc != nil {
		m.chatReplyTo = nil
		m.appendChatLine(chatLine{from: "Du", text: text, time: time.Now(), self: true, studio: true})
		send := m.sendStudioFunc
		return m, func() tea.Msg {
			return ChatSentMsg{Err: send(m.ctx, text)}
		}
	}
	replyTo := ""
	if m.chatReplyTo != nil && m.replyChatFunc != nil {
		replyTo = m.chatReplyTo.id
	}
	m.chatReplyTo = nil
	m.appendChatLine(chatLine{
		from:    "Du",
		text:    text,
		time:    time.Now(),
		self:    true,
		pending: true,
		replyTo: replyTo,
	})
	if replyTo != "" {
		reply := m.replyChatFunc
		return m, func() tea.Msg {
			return ChatSentMsg{Err: reply(m.ctx, text, replyTo)}
		}
	}
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
	m.chatAppended = 0
	m.chatAppendedSeen = 0
	m.chatMaxScroll = 0
	m.chatSelectedID = ""
	m.chatReplyTo = nil
	m.chatInput.SetValue("")
	m.chatInput.Blur()
}

// chatHistory is the number of chat lines that are kept.
const chatHistory = 300

// appendChatLine adds a line to the chat and counts it, so the panel can
// keep its place while the user reads further up.
func (m *Model) appendChatLine(line chatLine) {
	m.chatMessages = append(m.chatMessages, line)
	m.chatAppended++
	if len(m.chatMessages) > chatHistory {
		m.chatMessages = m.chatMessages[len(m.chatMessages)-chatHistory:]
	}
}
