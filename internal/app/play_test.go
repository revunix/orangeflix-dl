package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/revunix/orangeflix-dl/internal/config"
	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// TestPreparePlaybackNeedsMPV checks that the program says it plainly when
// mpv is not installed.
func TestPreparePlaybackNeedsMPV(t *testing.T) {
	// Ein leerer Pfad bedeutet, dass das Programm nicht gefunden wird.
	t.Setenv("PATH", t.TempDir())
	application := &App{Config: &config.Options{Quality: "best"}}
	_, err := application.preparePlayback(context.Background(), []orangeflix.Episode{{Identifier: "x"}})
	if err == nil {
		t.Fatal("want an error without mpv")
	}
	if err != ErrNoPlayer {
		t.Errorf("err = %v, want ErrNoPlayer", err)
	}
}

// TestPreparePlaybackBuildsTheCommand checks the command of the player: the
// addresses of the files stand behind the separator.
func TestPreparePlaybackBuildsTheCommand(t *testing.T) {
	// Ein falsches mpv wird in einen echten gelegt, damit der Test ohne
	// Netz und ohne mpv läuft.
	dir := t.TempDir()
	binary := filepath.Join(dir, "mpv")
	script := "#!/bin/sh\necho \"$@\"\n"
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	application, _, _ := testApp(t, handler(t, []byte("video")))
	application.Config.Quality = orangeflix.QualityHD

	items, err := application.preparePlayback(context.Background(), []orangeflix.Episode{{Identifier: "mt420"}})
	if err != nil {
		t.Fatalf("preparePlayback: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	player := items[0].player
	if player.Binary != binary {
		t.Errorf("binary = %q, want %q", player.Binary, binary)
	}
	// Die Optionen stehen vor dem Doppelpunkt, die Adressen danach.
	separator := -1
	for index, arg := range player.Args {
		if arg == "--" {
			separator = index
			break
		}
	}
	if separator < 0 {
		t.Fatalf("args = %v, want a separator", player.Args)
	}
	if separator+1 >= len(player.Args) {
		t.Errorf("args = %v, want a file", player.Args)
	}
	// Die alte API braucht die Zugangsdaten als Kopfzeile.
	if separator == 0 {
		t.Errorf("args = %v, want the header before the separator", player.Args)
	}
}

// TestPreparePlaybackResumes checks that a clip the account stopped in the
// middle starts at the saved position.
func TestPreparePlaybackResumes(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "mpv")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/content/clips/mt420":
			_, _ = w.Write([]byte(`{"ok":true,"clip":{"id":"mt420"}}`))
		case "/api/v3/playback/mt420":
			_, _ = w.Write([]byte(`{"ok":true,"clip":{"id":"mt420","durationSeconds":1200},
				"progress":{"hasStarted":true,"percentage":25,"lastPosition":300},
				"stream":{"media":{"files":[{"url":"https://cdn.example.org/mt420FULLHD.mp4","desc":"HD 1080p"}]}}}`))
		default:
			t.Errorf("path = %q", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	application := &App{
		Config: &config.Options{Quality: orangeflix.QualityBest},
		API:    &orangeflix.Client{BaseURL: server.URL + "/api/v1/"},
		V3: &V3{Client: &orangeflix.V3Client{
			HTTP:        server.Client(),
			BaseURL:     server.URL + "/api/v3/",
			AccessToken: "token",
		}},
	}

	items, err := application.preparePlayback(context.Background(), []orangeflix.Episode{
		{Identifier: "mt420", DurationSeconds: 1200},
	})
	if err != nil {
		t.Fatalf("preparePlayback: %v", err)
	}
	if items[0].resume != 300 {
		t.Errorf("resume = %d, want 300", items[0].resume)
	}
	want := "--start=0:05:00"
	found := false
	for _, arg := range items[0].player.Args {
		if arg == want {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("args = %v, want %q", items[0].player.Args, want)
	}
}

// TestShutdownRefusesLatePlayback checks that a playback prepared while the
// interface closes cannot start a player afterwards.
func TestShutdownRefusesLatePlayback(t *testing.T) {
	application := &App{}
	application.ShutdownPlayback()
	application.startRunner(context.Background(), func(context.Context) error { return nil })
	application.playbackMu.Lock()
	defer application.playbackMu.Unlock()
	if application.playbackDone != nil || application.playbackCancel != nil {
		t.Error("the playback started after the shutdown")
	}
}

// TestClockArgument checks the time format of the player.
func TestClockArgument(t *testing.T) {
	cases := map[int]string{0: "0:00:00", 65: "0:01:05", 3725: "1:02:05"}
	for seconds, want := range cases {
		if got := clockArgument(seconds); got != want {
			t.Errorf("clockArgument(%d) = %q, want %q", seconds, got, want)
		}
	}
}
