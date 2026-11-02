package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/revunix/orangeflix-dl/internal/app"
)

// pollModel is a live screen with a voting of the show.
func pollModel() *Model {
	model := interactiveChat()
	model.width, model.height = 130, 40
	model.applyPoll(&app.ChatPoll{
		Question:       "Was kommt als Nächstes?",
		Answers:        []string{"Folge 421", "Folge 422", "Folge 423"},
		Results:        []int{3, 5, 1},
		ResultsVisible: true,
		Submissions:    9,
	}, false)
	return model
}

func TestTheVotingStandsUnderTheLiveBox(t *testing.T) {
	model := pollModel()
	info, poll, chat := model.liveBoxes(model.width, 34)
	if poll.height <= 0 {
		t.Fatal("the voting has no box")
	}
	if poll.x != info.x || poll.width != info.width {
		t.Errorf("poll %+v, info %+v: the voting stands under the live box", poll, info)
	}
	if poll.y <= info.y+info.height {
		t.Errorf("poll %+v, info %+v", poll, info)
	}
	if chat.height != 32 {
		t.Errorf("the chat has %d lines, it keeps the whole height", chat.height)
	}
	view := ansi.Strip(model.View())
	for _, want := range []string{"╭─ Abstimmung", "Was kommt als Nächstes?", "1 Folge 421", "2 Folge 422", "3 Folge 423", "55 %"} {
		if !strings.Contains(view, want) {
			t.Errorf("the voting misses %q:\n%s", want, view)
		}
	}
	for index, line := range strings.Split(model.View(), "\n") {
		if width := ansi.StringWidth(line); width != model.width {
			t.Errorf("line %d is %d wide, want %d", index, width, model.width)
		}
	}
}

func TestWithoutAVotingTheBoxIsGone(t *testing.T) {
	model := interactiveChat()
	model.width, model.height = 130, 40
	_, poll, _ := model.liveBoxes(model.width, 34)
	if poll.height != 0 {
		t.Errorf("the voting has a box without a voting: %+v", poll)
	}
	if view := ansi.Strip(model.View()); strings.Contains(view, "╭─ Abstimmung") {
		t.Errorf("the box of the voting is shown:\n%s", view)
	}
}

func TestTheDigitVotes(t *testing.T) {
	model := pollModel()
	votes := []int{}
	model.votePollFunc = func(_ context.Context, answer int) error {
		votes = append(votes, answer)
		return nil
	}
	model = pressAndRun(t, model, "2")
	if len(votes) != 1 || votes[0] != 1 {
		t.Fatalf("votes = %v", votes)
	}
	if model.poll.choice != 1 {
		t.Errorf("choice = %d", model.poll.choice)
	}
	view := ansi.Strip(model.View())
	if !strings.Contains(view, "▸2 Folge 422") {
		t.Errorf("the own vote is not marked:\n%s", view)
	}
	if !strings.Contains(view, "Deine Stimme ist gezählt.") {
		t.Errorf("no answer of the server:\n%s", view)
	}
}

func TestARefusedVoteGoesBack(t *testing.T) {
	model := pollModel()
	model.votePollFunc = func(context.Context, int) error { return errors.New("the vote was not counted") }
	model = pressAndRun(t, model, "1")
	if model.poll.choice != -1 {
		t.Errorf("choice = %d, want it taken back", model.poll.choice)
	}
	if !model.isError || !strings.Contains(model.message, "nicht gezählt") {
		t.Errorf("message = %q", model.message)
	}
}

func TestASecretVotingHidesTheVotes(t *testing.T) {
	model := interactiveChat()
	model.width, model.height = 130, 40
	model.applyPoll(&app.ChatPoll{
		Question:       "Was ist dein Favorit?",
		Answers:        []string{"A", "B"},
		Results:        nil,
		PollID:         "p2",
		Secret:         true,
		ResultsVisible: false,
		Submissions:    6,
	}, false)
	view := ansi.Strip(model.View())
	if !strings.Contains(view, "verdeckt") {
		t.Errorf("the hidden result is not said:\n%s", view)
	}
	if strings.Contains(view, "50 %") {
		t.Errorf("the votes are shown although they are hidden:\n%s", view)
	}
}

func TestTheVotingKeepsItsChoiceWhenTheVotesChange(t *testing.T) {
	model := pollModel()
	model.votePollFunc = func(context.Context, int) error { return nil }
	model = pressAndRun(t, model, "3")
	// Der Server meldet die neuen Stimmen.
	model.applyPoll(&app.ChatPoll{
		Question:       "Was kommt als Nächstes?",
		Answers:        []string{"Folge 421", "Folge 422", "Folge 423"},
		Results:        []int{1, 2, 8},
		ResultsVisible: true,
		Submissions:    11,
	}, false)
	if model.poll.choice != 2 {
		t.Errorf("choice = %d, want it kept", model.poll.choice)
	}
	if model.poll.results[2] != 8 {
		t.Errorf("results = %v", model.poll.results)
	}
}

func TestAGoneVotingDisappears(t *testing.T) {
	model := pollModel()
	model.applyPoll(nil, true)
	if model.hasPoll() {
		t.Error("the voting is still there")
	}
	view := ansi.Strip(model.View())
	if strings.Contains(view, "╭─ Abstimmung") {
		t.Errorf("the box is still shown:\n%s", view)
	}
}

func TestTheVotingReachesTheInterface(t *testing.T) {
	model := interactiveChat()
	model.screen = ScreenLive
	updated, _ := model.Update(EventMsg{Event: app.Event{
		Kind: app.EventChatPoll,
		Poll: &app.ChatPoll{Question: "Was gilt?", Answers: []string{"Ja", "Nein"}},
	}})
	model = updated.(*Model)
	if !model.hasPoll() || model.poll.question != "Was gilt?" {
		t.Errorf("poll = %+v", model.poll)
	}
}
