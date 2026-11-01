package app

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

type recordingUI struct {
	events chan Event
}

func (u *recordingUI) SelectEpisodes(context.Context, orangeflix.Format, []orangeflix.Episode) ([]string, error) {
	return nil, nil
}
func (u *recordingUI) Event(event Event) { u.events <- event }

// TestQuitIsDetectedAtOnce checks that quitting mpv ends the status right
// away instead of waiting for the next report or the end of the file.
func TestQuitIsDetectedAtOnce(t *testing.T) {
	if testing.Short() {
		t.Skip("needs mpv")
	}
	if _, err := exec.LookPath("mpv"); err != nil {
		t.Skip("mpv is not installed")
	}
	before, _ := filepath.Glob(filepath.Join(os.TempDir(), "orangeflix-mpv-*", "ipc.sock"))
	existing := map[string]bool{}
	for _, path := range before {
		existing[path] = true
	}

	ui := &recordingUI{events: make(chan Event, 100)}
	application := &App{UI: ui}
	item := playbackItem{
		episode: orangeflix.Episode{Identifier: "quit-smoke", Title: "Quit", DurationSeconds: 60},
		player: &player{Binary: "mpv", Args: []string{
			"--vo=null", "--ao=null", "--really-quiet", "--",
			"av://lavfi:sine=frequency=440:duration=60",
		}},
	}
	runner := &playbackRunner{app: application, items: []playbackItem{item}}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- runner.run(ctx) }()

	// Wait for the player to run.
	for {
		select {
		case event := <-ui.events:
			if event.Kind == EventPlaying {
				goto running
			}
		case <-time.After(10 * time.Second):
			t.Fatal("the playback did not start")
		}
	}
running:
	// Find the IPC socket of this player and quit it like the user would.
	var socket string
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline) && socket == ""; {
		matches, _ := filepath.Glob(filepath.Join(os.TempDir(), "orangeflix-mpv-*", "ipc.sock"))
		for _, path := range matches {
			if !existing[path] {
				socket = path
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if socket == "" {
		t.Fatal("no IPC socket")
	}
	conn, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	payload, _ := json.Marshal(map[string]any{"command": []any{"quit"}})
	_, _ = conn.Write(append(payload, '\n'))
	_ = conn.Close()
	quitAt := time.Now()

	// The status must be cleared at once, before the final report.
	for {
		select {
		case event := <-ui.events:
			if event.Kind == EventPlayed && event.Message == "" {
				if elapsed := time.Since(quitAt); elapsed > 2*time.Second {
					t.Fatalf("the quit was detected after %v", elapsed)
				}
				goto cleared
			}
		case <-time.After(3 * time.Second):
			t.Fatal("the quit was not detected")
		}
	}
cleared:
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("the runner failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the runner did not end after the quit")
	}
}
