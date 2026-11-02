package chat

import (
	"encoding/json"
	"testing"
)

func TestParsePollReadsTheVoting(t *testing.T) {
	poll, ok := parsePoll(json.RawMessage(`{"question":"Was kommt als Nächstes?","answers":["Folge 421","Folge 422","Folge 423"],
		"results":[3,5,1],"pollId":"p1","secretPoll":false,"resultsVisible":true,"submissions":9,"closed":false}`))
	if !ok {
		t.Fatal("the voting was not read")
	}
	if poll.Question != "Was kommt als Nächstes?" || len(poll.Answers) != 3 {
		t.Errorf("poll = %+v", poll)
	}
	if poll.Results[1] != 5 || poll.Submissions != 9 || poll.Secret || !poll.ResultsVisible || poll.Closed {
		t.Errorf("poll = %+v", poll)
	}
	if poll.VoteCount() != 9 {
		t.Errorf("votes = %d", poll.VoteCount())
	}
	best, votes := poll.Leading()
	if best != 1 || votes != 5 {
		t.Errorf("leading = %d with %d", best, votes)
	}
}

func TestParsePollHidesTheVotesOfASecretVoting(t *testing.T) {
	poll, ok := parsePoll(json.RawMessage(`{"question":"Was ist dein Favorit?","answers":["A","B"],
		"results":[2,4],"pollId":"p2","secretPoll":true,"resultsVisible":false,"submissions":6}`))
	if !ok {
		t.Fatal("the voting was not read")
	}
	if !poll.Secret || poll.ResultsVisible || poll.Results != nil {
		t.Errorf("poll = %+v", poll)
	}
	if poll.PollID != "p2" || poll.Submissions != 6 {
		t.Errorf("poll = %+v", poll)
	}
}

func TestParsePollRefusesWhatIsNoVoting(t *testing.T) {
	for _, raw := range []string{
		`{"answers":["A"]}`,
		`{"question":"?","answers":[]}`,
		`"text"`,
		`{}`,
	} {
		if _, ok := parsePoll(json.RawMessage(raw)); ok {
			t.Errorf("accepted %s", raw)
		}
	}
	// Falsche Werte im Ergebnis gelten als keine, wie im Web-Chat.
	poll, ok := parsePoll(json.RawMessage(`{"question":"?","answers":["A","B"],"results":[1,2,3]}`))
	if !ok || len(poll.Results) != 2 || poll.Results[0] != 0 || poll.Results[1] != 0 {
		t.Errorf("poll = %+v", poll)
	}
}

func TestParsePollResults(t *testing.T) {
	pollID, results, submissions, ok := parsePollResults(json.RawMessage(`{"pollId":"p1","results":[2,4],"submissions":6}`), 2)
	if !ok || pollID != "p1" || results[1] != 4 || submissions != 6 {
		t.Errorf("pollID %q, results %v, submissions %d, ok %v", pollID, results, submissions, ok)
	}
	if _, _, _, ok := parsePollResults(json.RawMessage(`{"results":[1]}`), 2); ok {
		t.Error("results of the wrong length were accepted")
	}
}

func TestParsePollVisibility(t *testing.T) {
	pollID, visible, results, submissions, ok := parsePollVisibility(json.RawMessage(`{"pollId":"p1","resultsVisible":true,"results":[1,2],"submissions":3}`), 2)
	if !ok || pollID != "p1" || !visible || results[1] != 2 || submissions != 3 {
		t.Errorf("%q %v %v %d %v", pollID, visible, results, submissions, ok)
	}
	pollID, visible, _, _, ok = parsePollVisibility(json.RawMessage(`{"pollId":"p1","resultsVisible":false}`), 2)
	if !ok || pollID != "p1" || visible {
		t.Errorf("%q %v %v", pollID, visible, ok)
	}
	if _, _, _, _, ok := parsePollVisibility(json.RawMessage(`{"resultsVisible":true}`), 2); ok {
		t.Error("a visibility without a poll was accepted")
	}
}

func TestVoteAcceptedReadsTheAnswer(t *testing.T) {
	if !voteAccepted(json.RawMessage(`true`)) {
		t.Error("true was not accepted")
	}
	if voteAccepted(json.RawMessage(`false`)) {
		t.Error("false was accepted")
	}
	if !voteAccepted(json.RawMessage(`{"ok":true}`)) {
		t.Error("ok was not accepted")
	}
	if voteAccepted(json.RawMessage(`{"ok":false}`)) {
		t.Error("a refusal was accepted")
	}
	if voteAccepted(json.RawMessage(`"text"`)) {
		t.Error("text was accepted")
	}
}
