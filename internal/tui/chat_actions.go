package tui

import (
	"errors"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/revunix/orangeflix-dl/internal/app"
	"github.com/revunix/orangeflix-dl/internal/chat"
)

// ChatReactMsg carries the answer to a reaction.
type ChatReactMsg struct {
	ID     string
	Key    string
	Active bool
	Err    error
}

// ChatReactionsMsg carries the reactions of messages that were read again.
type ChatReactionsMsg struct {
	States []app.ChatReaction
	Emojis map[string]string
	Err    error
}

// chatInteractive tells whether the chat takes answers and reactions: only
// a connected live chat does, not the transcript of a replay.
func (m *Model) chatInteractive() bool {
	return m.chatConnected && m.live != nil && m.live.IsLive
}

// chatIndex returns the place of a message in the chat, -1 when it is gone.
func (m *Model) chatIndex(id string) int {
	if id == "" {
		return -1
	}
	for index := len(m.chatMessages) - 1; index >= 0; index-- {
		if m.chatMessages[index].id == id {
			return index
		}
	}
	return -1
}

// chatSelectable tells whether a line can be chosen: it needs the id of the
// server, so a pending message or a note of the program cannot.
func chatSelectable(line chatLine) bool {
	return line.id != "" && !line.system && !line.pending
}

// selectedChat returns the chosen message.
func (m *Model) selectedChat() (*chatLine, bool) {
	index := m.chatIndex(m.chatSelectedID)
	if index < 0 {
		m.chatSelectedID = ""
		return nil, false
	}
	return &m.chatMessages[index], true
}

// startChatSelect chooses the newest message that can be chosen.
func (m *Model) startChatSelect() {
	for index := len(m.chatMessages) - 1; index >= 0; index-- {
		if chatSelectable(m.chatMessages[index]) {
			m.chatSelectedID = m.chatMessages[index].id
			m.chatFollow = true
			return
		}
	}
	m.message = "Keine Nachricht, die sich wählen lässt."
	m.isError = false
}

// moveChatSelect moves the choice to an older (negative) or newer message.
func (m *Model) moveChatSelect(delta int) {
	current := m.chatIndex(m.chatSelectedID)
	if current < 0 {
		m.startChatSelect()
		return
	}
	for index := current + delta; index >= 0 && index < len(m.chatMessages); index += delta {
		if chatSelectable(m.chatMessages[index]) {
			m.chatSelectedID = m.chatMessages[index].id
			m.chatFollow = true
			return
		}
	}
}

// startReply opens the input as an answer to a message.
func (m *Model) startReply(line chatLine) (tea.Model, tea.Cmd) {
	if !m.chatInteractive() || m.replyChatFunc == nil {
		m.message = "Antworten geht nur im verbundenen Live-Chat."
		m.isError = true
		return m, nil
	}
	reply := line
	m.chatReplyTo = &reply
	m.chatSelectedID = ""
	m.chatFocused = true
	return m, m.chatInput.Focus()
}

// reactionEmoji returns the emoji of a reaction key.
func (m *Model) reactionEmoji(key string) string {
	if emoji, ok := m.chatEmojis[key]; ok {
		return emoji
	}
	if emoji, ok := chat.ReactionEmojis[key]; ok {
		return emoji
	}
	return key
}

// reactionKeys are the reactions the menu offers: the four of the web
// chat. Others that the server reports are counted, not offered.
func (m *Model) reactionKeys() []string {
	return chat.ReactionKeys
}

// reactionOrder sorts the reactions of a message for the display.
func (m *Model) reactionOrder(counts map[string]int) []string {
	order := map[string]int{}
	for index, key := range append(m.reactionKeys(), "surprised") {
		if _, ok := order[key]; !ok {
			order[key] = index
		}
	}
	keys := make([]string, 0, len(counts))
	for key, count := range counts {
		if count > 0 {
			keys = append(keys, key)
		}
	}
	sort.SliceStable(keys, func(i, j int) bool {
		a, aok := order[keys[i]]
		b, bok := order[keys[j]]
		switch {
		case aok && bok:
			return a < b
		case aok != bok:
			return aok
		}
		return keys[i] < keys[j]
	})
	return keys
}

// toggleReaction sets or removes a reaction of the account. The chat shows
// it at once and takes it back when the server refuses.
func (m *Model) toggleReaction(id, key string) (tea.Model, tea.Cmd) {
	index := m.chatIndex(id)
	if index < 0 {
		m.message = "Nachricht nicht mehr verfügbar."
		m.isError = true
		return m, nil
	}
	line := &m.chatMessages[index]
	if line.self {
		m.message = "Auf eigene Nachrichten kann man nicht reagieren."
		m.isError = true
		return m, nil
	}
	if !m.chatInteractive() || m.reactChatFunc == nil {
		m.message = "Reagieren geht nur im verbundenen Live-Chat."
		m.isError = true
		return m, nil
	}
	active := !line.mine[key]
	adjustReaction(line, key, active)
	react := m.reactChatFunc
	ctx := m.ctx
	return m, func() tea.Msg {
		return ChatReactMsg{ID: id, Key: key, Active: active, Err: react(ctx, id, key, active)}
	}
}

// adjustReaction counts the reaction of the account in or out.
func adjustReaction(line *chatLine, key string, active bool) {
	if line.reactions == nil {
		line.reactions = map[string]int{}
	}
	if line.mine == nil {
		line.mine = map[string]bool{}
	}
	if line.mine[key] == active {
		return
	}
	line.mine[key] = active
	if active {
		line.reactions[key]++
	} else if line.reactions[key] > 0 {
		line.reactions[key]--
	}
	if line.reactions[key] == 0 {
		delete(line.reactions, key)
	}
}

// handleChatReact takes the answer of the server.
func (m *Model) handleChatReact(message ChatReactMsg) (tea.Model, tea.Cmd) {
	if message.Err == nil {
		return m, nil
	}
	if errors.Is(message.Err, chat.ErrReactionUnchanged) {
		// Der Server hat anders entschieden: der Stand wird neu gelesen.
		return m, m.refreshReactions([]string{message.ID})
	}
	if index := m.chatIndex(message.ID); index >= 0 {
		adjustReaction(&m.chatMessages[index], message.Key, !message.Active)
	}
	var refusal chat.ReactionError
	if errors.As(message.Err, &refusal) {
		m.message = refusal.Error()
	} else {
		m.message = "Reaktion fehlgeschlagen: " + message.Err.Error()
	}
	m.isError = true
	return m, nil
}

// refreshReactions reads the reactions of messages again.
func (m *Model) refreshReactions(ids []string) tea.Cmd {
	if m.chatReactionsFunc == nil || len(ids) == 0 {
		return nil
	}
	fetch := m.chatReactionsFunc
	ctx := m.ctx
	return func() tea.Msg {
		states, emojis, err := fetch(ctx, ids)
		return ChatReactionsMsg{States: states, Emojis: emojis, Err: err}
	}
}

// applyReactionStates takes over the state of messages. A state that is
// older than the one the message has is left out.
func (m *Model) applyReactionStates(states []app.ChatReaction, emojis map[string]string) {
	for key, emoji := range emojis {
		if m.chatEmojis == nil {
			m.chatEmojis = map[string]string{}
		}
		if _, known := chat.ReactionEmojis[key]; !known {
			m.chatEmojis[key] = strings.TrimSpace(emoji)
		}
	}
	for _, state := range states {
		index := m.chatIndex(state.MessageID)
		if index < 0 {
			continue
		}
		line := &m.chatMessages[index]
		if line.cursor != "" && chat.CompareCursor(state.Cursor, line.cursor) < 0 {
			continue
		}
		line.reactions = map[string]int{}
		for key, count := range state.Counts {
			if count > 0 {
				line.reactions[key] = count
			}
		}
		line.mine = map[string]bool{}
		for _, key := range state.Mine {
			line.mine[key] = true
		}
		line.cursor = state.Cursor
	}
}

// applyReactionChange takes over one change of the server. The count is
// the total, so a change that arrives twice does no harm.
func (m *Model) applyReactionChange(change app.ChatReactionChange) {
	index := m.chatIndex(change.MessageID)
	if index < 0 {
		return
	}
	line := &m.chatMessages[index]
	if line.cursor != "" && chat.CompareCursor(change.Cursor, line.cursor) <= 0 {
		return
	}
	if line.reactions == nil {
		line.reactions = map[string]int{}
	}
	if change.Count > 0 {
		line.reactions[change.Emoji] = change.Count
	} else {
		delete(line.reactions, change.Emoji)
	}
	if change.Self {
		if line.mine == nil {
			line.mine = map[string]bool{}
		}
		line.mine[change.Emoji] = change.Active
	}
	line.cursor = change.Cursor
}

// chatMenu is the menu of the chosen message, or of the newest one.
func (m *Model) chatMenu() (string, []menuItem) {
	line, ok := m.selectedChat()
	if !ok {
		m.startChatSelect()
		if line, ok = m.selectedChat(); !ok {
			return "", nil
		}
	}
	target := *line
	items := []menuItem{}
	if m.chatInteractive() {
		items = append(items, menuItem{label: "Antworten", key: "a", run: func() (tea.Model, tea.Cmd) {
			return m.startReply(target)
		}})
		if !target.self {
			for index, key := range m.reactionKeys() {
				reaction := key
				label := m.reactionEmoji(key) + " " + reactionName(key)
				if target.mine[key] {
					label += " zurücknehmen"
				}
				shortcut := ""
				if index < 9 {
					shortcut = strconv.Itoa(index + 1)
				}
				items = append(items, menuItem{label: label, key: shortcut, run: func() (tea.Model, tea.Cmd) {
					return m.toggleReaction(target.id, reaction)
				}})
			}
		}
	}
	items = append(items, menuItem{label: "Text kopieren", key: "y", run: m.copyRun(target.text)})
	return target.from + ": " + target.text, items
}

// reactionName is the German name of a reaction.
func reactionName(key string) string {
	if name, ok := chat.ReactionNames[key]; ok {
		return name
	}
	return key
}
