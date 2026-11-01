package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/revunix/orangeflix-dl/internal/download"
	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// playbackReportInterval is the shortest time between two reports of the
// watch position. The website shows the progress within half a minute.
const playbackReportInterval = 30 * time.Second

// playbackPollInterval is the time between two looks at the position of the
// player.
const playbackPollInterval = time.Second

// ErrNoPlayer is returned when no player is installed.
var ErrNoPlayer = errors.New("mpv ist nicht installiert")

// player is one command of mpv.
type player struct {
	// Binary is the program, usually "mpv".
	Binary string
	// Args are the arguments of the program, the addresses at the end.
	Args []string
}

// playbackItem is one episode prepared for the player.
type playbackItem struct {
	episode orangeflix.Episode
	player  *player
	// resume is the position the playback starts at, 0 for the beginning.
	resume int
}

// playbackTitle returns the name of an episode for the messages of the
// interface.
func playbackTitle(episode orangeflix.Episode) string {
	if episode.Title != "" {
		return episode.Title
	}
	if episode.Number > 0 {
		return fmt.Sprintf("Folge %d", episode.Number)
	}
	return episode.Identifier
}

// preparePlayback resolves the files of the episodes and builds the commands
// of the player. The addresses of the delivery network carry a token, so they
// are resolved right before the start and never kept.
func (a *App) preparePlayback(ctx context.Context, episodes []orangeflix.Episode) ([]playbackItem, error) {
	if len(episodes) == 0 {
		return nil, errors.New("keine Folge ausgewählt")
	}
	binary, err := exec.LookPath("mpv")
	if err != nil {
		return nil, ErrNoPlayer
	}

	items := make([]playbackItem, 0, len(episodes))
	urls := make([]string, 0, len(episodes))
	var credentials *download.Credentials
	for _, episode := range episodes {
		source, err := a.episodeSource(ctx, episode)
		if err != nil {
			return nil, fmt.Errorf("die Metadaten von %s konnten nicht geladen werden: %w", episode.Identifier, err)
		}
		file, exact, err := orangeflix.SelectFile(source.Files, a.quality())
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
		if source.Credentials != nil {
			credentials = source.Credentials
		}

		// Am Ende der Folge oder ganz am Anfang bringt das Weiterspielen
		// nichts; der Anfang ist immer der sichere Fall.
		resume := source.Progress.LastPosition
		if resume <= 5 || (episode.DurationSeconds > 0 && resume >= episode.DurationSeconds-5) {
			resume = 0
		}
		items = append(items, playbackItem{episode: episode, resume: resume})
		urls = append(urls, rawURL)
	}

	options := []string{}
	// Die alte API braucht die Zugangsdaten im Kopf der Anfrage, mpv
	// bekommt sie als Kopfzeile.
	if credentials != nil && (credentials.Username != "" || credentials.Password != "") {
		token := base64.StdEncoding.EncodeToString([]byte(credentials.Username + ":" + credentials.Password))
		options = append(options, "--http-header-fields=Authorization: Basic "+token)
	}
	for index := range items {
		args := append([]string{}, options...)
		if items[index].resume > 0 {
			args = append(args, "--start="+clockArgument(items[index].resume))
		}
		// Der Doppelpunkt trennt die Optionen von der Adresse.
		args = append(args, "--", urls[index])
		items[index].player = &player{Binary: binary, Args: args}
	}
	return items, nil
}

// clockArgument formats a position for the command line of mpv.
func clockArgument(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	return fmt.Sprintf("%d:%02d:%02d", seconds/3600, (seconds%3600)/60, seconds%60)
}

// Play starts the episodes in the background. The player opens its own
// window and the interface stays usable; the progress is reported to the
// account while the playback runs.
func (a *App) Play(ctx context.Context, episodes []orangeflix.Episode) error {
	items, err := a.preparePlayback(ctx, episodes)
	if err != nil {
		return err
	}
	a.logf("Starte %s mit %d Folge(n).", items[0].player.Binary, len(items))
	a.startPlayback(ctx, items)
	return nil
}

// Watch plays the episodes and waits for the player. It is used by the runs
// without a terminal.
func (a *App) Watch(ctx context.Context, episodes []orangeflix.Episode) error {
	items, err := a.preparePlayback(ctx, episodes)
	if err != nil {
		return err
	}
	runner := &playbackRunner{app: a, items: items}
	return runner.run(ctx)
}

// startPlayback starts the runner in the background.
func (a *App) startPlayback(ctx context.Context, items []playbackItem) {
	a.StopPlayback()

	playCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	runner := &playbackRunner{app: a, items: items}

	a.playbackMu.Lock()
	if a.playbackClosed {
		a.playbackMu.Unlock()
		cancel()
		return
	}
	a.playbackID++
	id := a.playbackID
	a.playbackCancel = cancel
	a.playbackDone = done
	a.playbackMu.Unlock()

	go func() {
		defer close(done)
		defer cancel()
		_ = runner.run(playCtx)
		a.playbackMu.Lock()
		if a.playbackID == id {
			a.playbackCancel = nil
			a.playbackDone = nil
		}
		a.playbackMu.Unlock()
	}()
}

// StopPlayback ends the running playback and waits a moment for the final
// report. It does nothing when no playback runs.
func (a *App) StopPlayback() {
	a.stopPlayback(false)
}

// ShutdownPlayback ends the playback and refuses new ones. It is used when
// the interface closes, so a playback that is still being prepared cannot
// start a player after the program is gone.
func (a *App) ShutdownPlayback() {
	a.stopPlayback(true)
}

// stopPlayback cancels the runner and waits for its final report.
func (a *App) stopPlayback(closed bool) {
	a.playbackMu.Lock()
	if closed {
		a.playbackClosed = true
	}
	cancel, done := a.playbackCancel, a.playbackDone
	a.playbackID++
	a.playbackCancel = nil
	a.playbackDone = nil
	a.playbackMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}
}

// playbackRunner plays the episodes one after another.
type playbackRunner struct {
	app   *App
	items []playbackItem

	reportErrorLogged bool
}

// run plays the items until the player is quit or all items are done.
func (r *playbackRunner) run(ctx context.Context) error {
	for _, item := range r.items {
		if err := ctx.Err(); err != nil {
			return err
		}
		finished, err := r.runItem(ctx, item)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			r.app.event(Event{Kind: EventPlayed, Message: err.Error(), Error: true})
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !finished {
			break
		}
	}
	r.app.event(Event{Kind: EventPlayed, Message: "Wiedergabe beendet."})
	return nil
}

// runItem plays one episode. The first result tells whether the episode was
// watched to the end, so the next one follows.
func (r *playbackRunner) runItem(ctx context.Context, item playbackItem) (bool, error) {
	dir, err := os.MkdirTemp("", "orangeflix-mpv-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(dir)

	socket := mpvSocketPath(dir)
	args := []string{"--no-terminal", "--input-ipc-server=" + socket}
	args = append(args, item.player.Args...)

	command := exec.CommandContext(ctx, item.player.Binary, args...)
	command.Stdout = io.Discard
	var stderr limitedBuffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		return false, err
	}

	exited := make(chan error, 1)
	go func() { exited <- command.Wait() }()

	ipc, ipcErr := dialMPV(ctx, socket, exited)
	if ipcErr != nil {
		if !errors.Is(ipcErr, errPlayerExited) {
			_ = command.Process.Kill()
			<-exited
		}
		return false, fmt.Errorf("%s konnte nicht erreicht werden: %w", item.player.Binary, ipcErr)
	}
	defer ipc.Close()
	_ = ipc.observe("eof-reached")

	duration := item.episode.DurationSeconds
	if duration <= 0 {
		if value, ok, err := ipc.number("duration"); err == nil && ok {
			duration = int(value)
		}
	}

	position := item.resume
	lastStatus := -1
	lastReported := 0
	lastReport := time.Now()
	finished := false
	var waitErr error

	r.app.eventPlaying(item, position, duration)

	ticker := time.NewTicker(playbackPollInterval)
	defer ticker.Stop()
loop:
	for {
		select {
		case <-ctx.Done():
			_ = ipc.send("quit")
			waitErr = <-exited
			break loop
		case waitErr = <-exited:
			break loop
		case <-ticker.C:
			value, ok, err := ipc.number("time-pos")
			if err != nil {
				continue
			}
			if ok && int(value) > position {
				position = int(value)
			}
			if ipc.reachedEnd() {
				finished = true
			}
			if duration > 0 && position >= duration-2 {
				finished = true
			}
			if position != lastStatus {
				lastStatus = position
				r.app.eventPlaying(item, position, duration)
			}
			if time.Since(lastReport) >= playbackReportInterval && position > lastReported {
				r.report(ctx, item.episode.Identifier, position)
				lastReported = position
				lastReport = time.Now()
			}
		}
	}

	if ipc.reachedEnd() {
		finished = true
		// Das Ende der Datei ist die sicherste Position: der letzte Abruf
		// kann bis zu einer Sekunde davor liegen.
		if duration > 0 {
			position = duration
		}
	}
	// Der letzte Stand geht auch dann an das Konto, wenn das Programm oder
	// der Player gerade beendet wurde. Nach dem Abbruch bekommt die
	// Oberfläche keine Meldung mehr: sie ist bereits geschlossen.
	r.reportFinal(item.episode.Identifier, position)
	if ctx.Err() == nil {
		r.app.eventPlaying(item, position, duration)
	}

	if waitErr != nil && ctx.Err() == nil && !finished {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = waitErr.Error()
		}
		return finished, fmt.Errorf("%s ist fehlgeschlagen: %s", item.player.Binary, message)
	}
	return finished, nil
}

// eventPlaying reports the position of the playback to the interface.
func (a *App) eventPlaying(item playbackItem, position, duration int) {
	a.event(Event{
		Kind:      EventPlaying,
		Label:     playbackTitle(item.episode),
		EpisodeID: item.episode.Identifier,
		PID:       item.episode.PID,
		Done:      int64(position),
		Total:     int64(duration),
	})
}

// report sends the position to the account.
func (r *playbackRunner) report(ctx context.Context, identifier string, position int) {
	if r.app.V3 == nil || position <= 0 {
		return
	}
	reportCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	r.sendReport(reportCtx, identifier, position)
}

// reportFinal sends the last position with a context of its own: the
// playback context is already gone when the user quits.
func (r *playbackRunner) reportFinal(identifier string, position int) {
	if r.app.V3 == nil || position <= 0 {
		return
	}
	reportCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r.sendReport(reportCtx, identifier, position)
}

// sendReport stores the position and logs the first failure only. Without a
// token the report is skipped: a login in the middle of a playback would
// need the user and would only disturb the flow.
func (r *playbackRunner) sendReport(ctx context.Context, identifier string, position int) {
	if r.app.V3 == nil || r.app.V3.Client == nil || r.app.V3.Client.AccessToken == "" {
		return
	}
	if err := r.app.V3.Client.UpdateProgress(ctx, identifier, position); err != nil {
		if !r.reportErrorLogged {
			r.reportErrorLogged = true
			r.app.logf("Der Fortschritt von %s konnte nicht gespeichert werden: %s", identifier, err.Error())
		}
	}
}

// limitedBuffer keeps the last bytes written to it. The player's error text
// is only needed for the message after a failed start.
type limitedBuffer struct {
	mu    sync.Mutex
	data  []byte
	limit int
}

// Write keeps the newest bytes.
func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.limit <= 0 {
		b.limit = 2048
	}
	b.data = append(b.data, p...)
	if len(b.data) > b.limit {
		b.data = b.data[len(b.data)-b.limit:]
	}
	return len(p), nil
}

// String returns the collected text.
func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(bytes.TrimSpace(b.data))
}
