package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/revunix/orangeflix-dl/internal/config"
	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// TestPlayerForNeedsMPV checks that the program says it plainly when mpv is
// not installed.
func TestPlayerForNeedsMPV(t *testing.T) {
	// Ein leerer Pfad bedeutet, dass das Programm nicht gefunden wird.
	t.Setenv("PATH", t.TempDir())
	application := &App{Config: &config.Options{Quality: "best"}}
	_, err := application.PlayerFor(context.Background(), []orangeflix.Episode{{Identifier: "x"}})
	if err == nil {
		t.Fatal("want an error without mpv")
	}
	if err != ErrNoPlayer {
		t.Errorf("err = %v, want ErrNoPlayer", err)
	}
}

// TestPlayerForBuildsTheCommand checks the command of the player: the
// addresses of the files stehen hinter dem Doppelpunkt.
func TestPlayerForBuildsTheCommand(t *testing.T) {
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

	player, err := application.PlayerFor(context.Background(), []orangeflix.Episode{{Identifier: "mt420"}})
	if err != nil {
		t.Fatalf("PlayerFor: %v", err)
	}
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
