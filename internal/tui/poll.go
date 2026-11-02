package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/revunix/orangeflix-dl/internal/app"
)

// PollMsg carries the answer of the server to a vote.
type PollMsg struct {
	Answer int
	Err    error
}

// pollAnswers holds the voting of the live show.
type pollAnswers struct {
	question string
	answers  []string
	results  []int
	// visible tells that the votes may be shown; secret is a voting whose
	// votes stay hidden until it ends.
	visible bool
	secret  bool
	// submissions is the number of the people that took part.
	submissions int
	closed      bool
	// choice is the answer of the account.
	choice int
}

// hasPoll tells whether a voting is open.
func (m *Model) hasPoll() bool {
	return m.poll != nil && len(m.poll.answers) > 0
}

// pollVoted tells that the account has voted.
func (m *Model) pollVoted() bool {
	return m.hasPoll() && m.poll.choice >= 0
}

// applyPoll takes over a voting of the server. The choice of the account
// stays when the voting is the same.
func (m *Model) applyPoll(poll *app.ChatPoll, removed bool) {
	if removed {
		m.poll = nil
		return
	}
	if poll == nil {
		return
	}
	choice := -1
	if m.poll != nil && m.poll.question == poll.Question && len(m.poll.answers) == len(poll.Answers) {
		choice = m.poll.choice
	}
	m.poll = &pollAnswers{
		question:    poll.Question,
		answers:     poll.Answers,
		results:     poll.Results,
		visible:     poll.ResultsVisible,
		secret:      poll.Secret,
		submissions: poll.Submissions,
		closed:      poll.Closed,
		choice:      choice,
	}
}

// votePoll answers the voting with the answer of the given place.
func (m *Model) votePoll(answer int) (tea.Model, tea.Cmd) {
	if !m.hasPoll() {
		return m, nil
	}
	if answer < 0 || answer >= len(m.poll.answers) {
		return m, nil
	}
	if m.votePollFunc == nil {
		m.message = "Abstimmen steht in dieser Fassung nicht zur Verfügung."
		m.isError = true
		return m, nil
	}
	if m.poll.closed {
		m.message = "Die Abstimmung ist beendet."
		m.isError = true
		return m, nil
	}
	m.poll.choice = answer
	vote := m.votePollFunc
	ctx := m.ctx
	return m, func() tea.Msg {
		return PollMsg{Answer: answer, Err: vote(ctx, answer)}
	}
}

// handlePoll takes the answer of the server to a vote.
func (m *Model) handlePoll(message PollMsg) (tea.Model, tea.Cmd) {
	if message.Err != nil {
		m.poll.choice = -1
		m.message = pollErrorText(message.Err)
		m.isError = true
		return m, nil
	}
	m.message = "Deine Stimme ist gezählt."
	m.isError = false
	return m, nil
}

// pollErrorText explains a refused vote.
func pollErrorText(err error) string {
	if err == nil {
		return ""
	}
	if strings.Contains(err.Error(), "not counted") {
		return "Die Stimme wurde nicht gezählt."
	}
	return "Abstimmung fehlgeschlagen: " + err.Error()
}

// pollPanel renders the voting of the live show: the question, the answers
// with their votes as a bar, and the share in percent. Without a vote of
// the account the numbers stay hidden when the server hides them.
func (m *Model) pollPanel(box rect) []string {
	if !m.hasPoll() {
		return renderBoxColor(box.width, "Abstimmung", theme.BoxInfo,
			[]string{theme.Muted.Render("Zurzeit läuft keine Abstimmung.")})
	}
	poll := m.poll
	width := box.innerWidth()
	lines := []string{}
	for _, part := range wrapText(poll.question, width) {
		lines = append(lines, theme.Title.Bold(true).Render(part))
	}
	lines = append(lines, "")
	total := 0
	for _, count := range poll.results {
		total += count
	}
	showResults := poll.visible && !poll.secret || poll.closed
	for index, answer := range poll.answers {
		count := 0
		if index < len(poll.results) {
			count = poll.results[index]
		}
		mark := " "
		if poll.choice == index {
			mark = "▸"
		}
		key := strconv.Itoa(index + 1)
		prefix := mark + key + " "
		room := max(6, width-ansi.StringWidth(prefix)-2)
		share := ""
		if showResults {
			if total > 0 {
				share = fmt.Sprintf("%d %%", count*100/total)
			} else {
				share = "0 %%"
			}
			room = max(6, width-ansi.StringWidth(prefix)-ansi.StringWidth(share)-1)
		}
		head := prefix + truncateColumn(answer, room)
		if showResults {
			head = padRight(head, max(6, width-ansi.StringWidth(share)-1)) + theme.Muted.Render(share)
		}
		style := theme.Text
		if poll.choice == index {
			style = theme.AccentBold
		}
		lines = append(lines, style.Render(head))
		if showResults && total > 0 {
			bar := pollBar(count, total, max(4, width-4))
			lines = append(lines, "  "+bar)
		}
	}
	lines = append(lines, "")
	if poll.closed {
		lines = append(lines, theme.Muted.Render("Die Abstimmung ist beendet."))
	} else if poll.secret && !poll.visible {
		lines = append(lines, theme.Muted.Render("Das Ergebnis bleibt verdeckt, bis die Abstimmung endet."))
	} else if !m.pollVoted() {
		lines = append(lines, theme.Accent.Render("1 – "+strconv.Itoa(len(poll.answers))+" wählen"))
	} else {
		lines = append(lines, theme.Muted.Render(fmt.Sprintf("%d haben abgestimmt.", poll.submissions)))
	}
	return renderBoxAt(box.width, box.height, "Abstimmung", theme.BoxInfo, trimContent(lines, box.innerHeight()))
}

// pollBar draws the share of the votes as a bar.
func pollBar(count, total, width int) string {
	if total <= 0 || width <= 0 {
		return ""
	}
	filled := count * width / total
	if count > 0 && filled == 0 {
		filled = 1
	}
	return theme.Accent.Render(strings.Repeat("█", filled)) + theme.Subtle.Render(strings.Repeat("·", max(0, width-filled)))
}
