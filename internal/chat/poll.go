package chat

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/maldikhan/go.socket.io/socket.io/v5/client/emit"
)

// Poll is a voting of the live show.
type Poll struct {
	// Question is what is asked.
	Question string
	// Answers are the choices in the order of the server.
	Answers []string
	// Results holds the votes per answer; nil while they are hidden.
	Results []int
	// PollID names the voting; only a secret one has one.
	PollID string
	// Secret tells that the votes stay hidden until the voting ends.
	Secret bool
	// ResultsVisible tells that the votes may be shown.
	ResultsVisible bool
	// Submissions is the number of people that took part.
	Submissions int
	// Closed tells that the voting is over.
	Closed bool
}

// VoteCount is the number of the votes that were counted.
func (p Poll) VoteCount() int {
	total := 0
	for _, count := range p.Results {
		total += count
	}
	return total
}

// Leading is the answer with the most votes; with a tie it is the first
// one. The number of votes stands beside it.
func (p Poll) Leading() (int, int) {
	best, votes := -1, 0
	for index, count := range p.Results {
		if count > votes {
			best, votes = index, count
		}
	}
	return best, votes
}

// VoteMessage carries the answer of the server to a vote.
type VoteMessage struct {
	// PollID names the voting the answer belongs to.
	PollID string
	// Accepted tells that the vote was counted.
	Accepted bool
}

// ErrVoteRejected tells that the server did not count the vote.
var ErrVoteRejected = errors.New("the vote was not counted")

// Vote answers a poll with the answer of the given place.
func (s *Session) Vote(answer int) error {
	accepted := make(chan bool, 1)
	err := s.client.Emit("vote", answer,
		emit.WithAck(func(data json.RawMessage) {
			accepted <- voteAccepted(data)
		}),
		emit.WithTimeout(8*time.Second, func() {}),
	)
	if err != nil {
		return err
	}
	select {
	case ok := <-accepted:
		if !ok {
			return ErrVoteRejected
		}
		return nil
	case <-s.done:
		return errors.New("the chat is closed")
	case <-time.After(8 * time.Second):
		return errors.New("the chat did not answer the vote")
	}
}

// voteAccepted reads the answer of the server to a vote.
func voteAccepted(raw json.RawMessage) bool {
	var answer bool
	if err := json.Unmarshal(raw, &answer); err == nil {
		return answer
	}
	var object struct {
		OK       *bool `json:"ok"`
		Accepted *bool `json:"accepted"`
	}
	if err := json.Unmarshal(raw, &object); err != nil {
		return false
	}
	if object.Accepted != nil {
		return *object.Accepted
	}
	if object.OK != nil {
		return *object.OK
	}
	return false
}

// parsePoll reads a voting of the server. A voting without a question or
// without answers is not one.
func parsePoll(raw json.RawMessage) (Poll, bool) {
	var wire struct {
		Question       string          `json:"question"`
		Answers        json.RawMessage `json:"answers"`
		Results        json.RawMessage `json:"results"`
		PollID         string          `json:"pollId"`
		SecretPoll     bool            `json:"secretPoll"`
		ResultsVisible *bool           `json:"resultsVisible"`
		Submissions    json.RawMessage `json:"submissions"`
		Closed         bool            `json:"closed"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return Poll{}, false
	}
	if wire.Question == "" {
		return Poll{}, false
	}
	answers := []string{}
	if err := json.Unmarshal(wire.Answers, &answers); err != nil || len(answers) == 0 {
		return Poll{}, false
	}
	for index := range answers {
		answers[index] = cleanText(answers[index])
	}
	secret := wire.SecretPoll && wire.PollID != ""
	visible := !secret || (wire.ResultsVisible != nil && *wire.ResultsVisible)
	poll := Poll{
		Question:       cleanText(wire.Question),
		Answers:        answers,
		PollID:         wire.PollID,
		Secret:         secret,
		ResultsVisible: visible,
		Submissions:    countOf(wire.Submissions),
		Closed:         wire.Closed,
	}
	if visible {
		// Falsche Werte im Ergebnis gelten als keine: der Web-Chat zeigt
		// dann Nullen.
		poll.Results = countsOf(wire.Results, len(answers))
		if poll.Results == nil {
			poll.Results = make([]int, len(answers))
		}
	}
	return poll, true
}

// parsePollResults reads the new votes of a voting.
func parsePollResults(raw json.RawMessage, answers int) (pollID string, results []int, submissions int, ok bool) {
	var wire struct {
		PollID      string          `json:"pollId"`
		Results     json.RawMessage `json:"results"`
		Submissions json.RawMessage `json:"submissions"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return "", nil, 0, false
	}
	counts := countsOf(wire.Results, answers)
	if counts == nil {
		return "", nil, 0, false
	}
	return wire.PollID, counts, countOf(wire.Submissions), true
}

// parsePollVisibility reads the change of the visibility of the votes.
func parsePollVisibility(raw json.RawMessage, answers int) (pollID string, visible bool, results []int, submissions int, ok bool) {
	var wire struct {
		PollID         string          `json:"pollId"`
		ResultsVisible bool            `json:"resultsVisible"`
		Results        json.RawMessage `json:"results"`
		Submissions    json.RawMessage `json:"submissions"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return "", false, nil, 0, false
	}
	if wire.PollID == "" {
		return "", false, nil, 0, false
	}
	counts := countsOf(wire.Results, answers)
	if wire.ResultsVisible && counts == nil {
		return "", false, nil, 0, false
	}
	return wire.PollID, wire.ResultsVisible, counts, countOf(wire.Submissions), true
}

// countsOf reads the votes of the answers: a list of the same length with
// numbers that are not negative.
func countsOf(raw json.RawMessage, answers int) []int {
	if len(raw) == 0 {
		return nil
	}
	var list []float64
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil
	}
	if len(list) != answers {
		return nil
	}
	counts := make([]int, 0, len(list))
	for _, value := range list {
		if value < 0 || value != float64(int(value)) {
			return nil
		}
		counts = append(counts, int(value))
	}
	return counts
}

// countOf reads the number of the people that took part.
func countOf(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var number float64
	if err := json.Unmarshal(raw, &number); err != nil {
		return 0
	}
	if number < 0 {
		return 0
	}
	return int(number)
}
