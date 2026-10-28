package app

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/revunix/orangeflix-dl/internal/download"
	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// Player is the command that plays the files of the episodes. It is usually
// mpv, das die Dateien direkt vom Auslieferungsnetz abholt.
type Player struct {
	// Binary is the program, usually "mpv".
	Binary string
	// Args are the arguments of the program, the files at the end.
	Args []string
}

// ErrNoPlayer is returned when no player is installed.
var ErrNoPlayer = errors.New("mpv ist nicht installiert")

// PlayerFor builds the command that plays the episodes. The files of the
// delivery network carry a token in their address, so they are resolved
// right before the start and never kept.
func (a *App) PlayerFor(ctx context.Context, episodes []orangeflix.Episode) (*Player, error) {
	if len(episodes) == 0 {
		return nil, errors.New("keine Folge ausgewählt")
	}
	binary, err := exec.LookPath("mpv")
	if err != nil {
		return nil, ErrNoPlayer
	}

	options := []string{}
	urls := []string{}
	var credentials *download.Credentials
	for _, episode := range episodes {
		files, _, creds, err := a.episodeFiles(ctx, episode)
		if err != nil {
			return nil, fmt.Errorf("die Metadaten von %s konnten nicht geladen werden: %w", episode.Identifier, err)
		}
		file, exact, err := orangeflix.SelectFile(files, a.quality())
		if err != nil {
			return nil, fmt.Errorf("für %s gibt es keine Datei: %w", episode.Identifier, err)
		}
		if !exact {
			a.logf("Qualität %s ist nicht verfügbar, nehme %s.", a.quality(), file.Desc)
		}
		rawURL, err := a.API.ToHTTPS(file.URL)
		if err != nil {
			return nil, fmt.Errorf("ungültige Adresse für %s: %w", episode.Identifier, err)
		}
		if creds != nil {
			credentials = creds
		}
		urls = append(urls, rawURL)
	}

	// Die alte API braucht die Zugangsdaten im Kopf der Anfrage, mpv
	// bekommt sie als Kopfzeile.
	if credentials != nil && (credentials.Username != "" || credentials.Password != "") {
		token := base64.StdEncoding.EncodeToString([]byte(credentials.Username + ":" + credentials.Password))
		options = append(options, "--http-header-fields=Authorization: Basic "+token)
	}
	// Der Doppelpunkt trennt die Optionen von den Adressen.
	args := append(options, "--")
	args = append(args, urls...)
	return &Player{Binary: binary, Args: args}, nil
}

// Watch plays the episodes and waits for the player. It is used by the runs
// without a terminal.
func (a *App) Watch(ctx context.Context, episodes []orangeflix.Episode) error {
	player, err := a.PlayerFor(ctx, episodes)
	if err != nil {
		return err
	}
	a.logf("Starte %s mit %d Folge(n).", player.Binary, len(episodes))
	command := exec.CommandContext(ctx, player.Binary, player.Args...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("%s ist fehlgeschlagen: %w", player.Binary, err)
	}
	return nil
}

// PlayerName returns the name of the player, for the messages of the
// interface.
func PlayerName() string {
	if path, err := exec.LookPath("mpv"); err == nil {
		return strings.TrimSpace(path)
	}
	return "mpv"
}
