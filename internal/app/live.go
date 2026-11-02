package app

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// liveReportInterval is the time between two live progress pings.
const liveReportInterval = 30 * time.Second

// liveRunner plays a livestream or its replay and reports the position.
type liveRunner struct {
	app      *App
	binary   string
	url      string
	mode     string
	title    string
	resume   int
	start    time.Time
	duration int
	clipID   string
	chat     []replayChatLine
	chatNext int

	reportMu          sync.Mutex
	reportErrorLogged bool
}

// PlayLive starts the active stream or its replay in the background.
func (a *App) PlayLive(ctx context.Context, info *orangeflix.V3LiveInfo, mode string) error {
	if info == nil {
		return errors.New("keine Live-Angaben")
	}
	if err := a.EnsureV3(ctx); err != nil {
		return err
	}
	if mode != "live" && mode != "replay" {
		return errors.New("unbekannter Live-Modus")
	}
	streamURL, err := liveURL(info, mode)
	if err != nil {
		return err
	}
	binary, err := exec.LookPath("mpv")
	if err != nil {
		return ErrNoPlayer
	}

	runner := &liveRunner{app: a, binary: binary, url: streamURL, mode: mode}
	if mode == "replay" {
		if info.Replay == nil {
			return errors.New("keine Replay-Angaben")
		}
		runner.title = info.Replay.Title
		runner.duration = int(info.Replay.End - info.Replay.Begin)
		runner.clipID = info.Replay.ClipID
		start, startErr := mpdStartTime(ctx, a.V3, streamURL)
		if startErr != nil {
			a.logf("Die Startzeit des Replays ist nicht lesbar (%s), der Fortschritt wird nicht gemeldet.", startErr.Error())
		} else {
			runner.start = start
			if info.LastWatchedAt > 0 {
				offset := info.LastWatchedAt - start.Unix()
				if offset > 5 && runner.duration > int(offset) {
					runner.resume = int(offset)
				}
			}
		}
	} else {
		if info.Show != nil {
			runner.title = info.Show.Title
		} else {
			runner.title = "Live"
		}
	}

	a.logf("Starte %s für %s.", runner.mode, runner.title)
	a.startRunner(ctx, runner.run)
	return nil
}

// liveURL picks a stream URL. HLS is preferred because it is supported by
// every mpv build; DASH is the fallback.
func liveURL(info *orangeflix.V3LiveInfo, mode string) (string, error) {
	var streams map[string]string
	if mode == "replay" {
		if info.Replay == nil {
			return "", errors.New("keine Replay-Adresse verfügbar")
		}
		streams = info.Replay.URL
	} else {
		streams = info.URL
	}
	order := []string{"application/x-mpegURL", "application/dash+xml"}
	if mode == "replay" {
		// Der Replay-Fortschritt braucht availabilityStartTime aus dem
		// DASH-Manifest, deshalb steht DASH hier vorn.
		order = []string{"application/dash+xml", "application/x-mpegURL"}
	}
	for _, mime := range order {
		if value := strings.TrimSpace(streams[mime]); value != "" {
			return value, nil
		}
	}
	keys := make([]string, 0, len(streams))
	for key, value := range streams {
		if strings.TrimSpace(value) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		if mode == "replay" {
			return "", errors.New("keine Replay-Adresse verfügbar")
		}
		return "", errors.New("keine Live-Adresse verfügbar")
	}
	return streams[keys[0]], nil
}

// mpdStartTime reads availabilityStartTime from a DASH manifest.
func mpdStartTime(ctx context.Context, v3 *V3, manifestURL string) (time.Time, error) {
	client := http.DefaultClient
	if v3 != nil && v3.Client != nil && v3.Client.HTTP != nil {
		client = v3.Client.HTTP
	}
	requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return time.Time{}, err
	}
	request.Header.Set("User-Agent", orangeflix.DefaultUA)
	response, err := client.Do(request)
	if err != nil {
		return time.Time{}, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode >= 400 {
		return time.Time{}, fmt.Errorf("HTTP %d", response.StatusCode)
	}
	var document struct {
		AvailabilityStartTime string `xml:"availabilityStartTime,attr"`
	}
	if err := xml.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&document); err != nil {
		return time.Time{}, err
	}
	if document.AvailabilityStartTime == "" {
		return time.Time{}, errors.New("availabilityStartTime fehlt")
	}
	return time.Parse(time.RFC3339Nano, document.AvailabilityStartTime)
}

// run plays the stream and reports setup errors to the interface.
func (r *liveRunner) run(ctx context.Context) error {
	err := r.play(ctx)
	if err != nil && ctx.Err() == nil {
		r.app.event(Event{Kind: EventPlayed, Message: err.Error(), Error: true})
	}
	return err
}

// play plays the stream until the player ends.
func (r *liveRunner) play(ctx context.Context) error {
	dir, err := os.MkdirTemp("", "orangeflix-mpv-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	socket := mpvSocketPath(dir)
	args := []string{"--no-terminal", "--idle=no", "--keep-open=no", "--input-ipc-server=" + socket}
	if r.resume > 0 {
		args = append(args, "--start="+clockArgument(r.resume))
	}
	args = append(args, "--", r.url)

	command := exec.CommandContext(ctx, r.binary, args...)
	command.Stdout = io.Discard
	var stderr limitedBuffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		return err
	}
	exited := make(chan error, 1)
	go func() { exited <- command.Wait() }()

	ipc, ipcErr := dialMPV(ctx, socket, exited)
	if ipcErr != nil {
		if !errors.Is(ipcErr, errPlayerExited) {
			_ = command.Process.Kill()
		}
		return fmt.Errorf("mpv konnte nicht erreicht werden: %w", ipcErr)
	}
	defer ipc.Close()

	r.loadReplayChat(ctx)
	r.emitReplayChat(r.resume)
	position := r.resume
	lastStatus := -1
	lastReport := time.Now()
	r.app.event(Event{Kind: EventPlaying, Label: "Live: " + r.title, Done: int64(position)})

	ticker := time.NewTicker(playbackPollInterval)
	defer ticker.Stop()
	var waitErr error
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
			if position != lastStatus {
				lastStatus = position
				r.app.event(Event{Kind: EventPlaying, Label: "Live: " + r.title, Done: int64(position)})
				r.emitReplayChat(position)
			}
			if time.Since(lastReport) >= liveReportInterval {
				go r.reportLive(ctx, position)
				lastReport = time.Now()
			}
		}
	}

	r.reportLiveFinal(position)
	if ctx.Err() == nil {
		if waitErr != nil {
			message := strings.TrimSpace(stderr.String())
			if message == "" {
				message = waitErr.Error()
			}
			return fmt.Errorf("mpv ist fehlgeschlagen: %s", message)
		}
		r.app.event(Event{Kind: EventPlayed, Message: "Wiedergabe beendet."})
	}
	return nil
}

// replayChatLine is one message of a chat replay, placed on the timeline
// of the video.
type replayChatLine struct {
	at   float64
	from string
	text string
	unix int64
}

// loadReplayChat fetches the chat of a replay. A failure only leaves the
// panel empty: the video still plays.
func (r *liveRunner) loadReplayChat(ctx context.Context) {
	if r.mode != "replay" || r.clipID == "" || r.app.V3 == nil || r.app.V3.Client == nil {
		return
	}
	end := float64(r.duration)
	if end <= 0 {
		end = 1
	}
	replay, err := r.app.V3.Client.ChatReplay(ctx, r.clipID, 0, end)
	if err != nil {
		r.app.logf("Der Chat-Mitschnitt konnte nicht geladen werden: %s", err.Error())
		r.app.event(Event{Kind: EventChatStatus, Message: "Chat-Mitschnitt nicht verfügbar", Error: true})
		return
	}
	lines := make([]replayChatLine, 0, len(replay.Messages))
	for _, message := range replay.Messages {
		text := strings.TrimSpace(message.Message)
		if text == "" {
			continue
		}
		lines = append(lines, replayChatLine{
			at:   replayChatOffset(message, r.start.Unix(), r.duration),
			from: message.Username,
			text: text,
			unix: message.Time,
		})
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].at < lines[j].at })
	r.chat = lines
	if len(lines) == 0 {
		r.app.event(Event{Kind: EventChatStatus, Message: "Kein Chat-Mitschnitt"})
		return
	}
	r.app.event(Event{Kind: EventChatStatus, Message: fmt.Sprintf("Chat-Mitschnitt: %d Nachrichten", len(lines))})
}

// emitReplayChat sends the messages that belong up to the position.
func (r *liveRunner) emitReplayChat(position int) {
	for r.chatNext < len(r.chat) && r.chat[r.chatNext].at <= float64(position) {
		line := r.chat[r.chatNext]
		r.chatNext++
		r.app.event(Event{
			Kind:    EventChat,
			Label:   line.from,
			Message: line.text,
			Done:    line.unix,
		})
	}
}

// replayChatOffset places one replay message on the video timeline, in
// seconds. videoTime is already an offset when it fits into the duration;
// otherwise the wall clock is measured from the start of the replay.
func replayChatOffset(message orangeflix.V3ChatMessage, begin int64, duration int) float64 {
	limit := float64(duration) + 120
	if limit < 120 {
		limit = 24 * 3600
	}
	video := message.VideoTime
	if video > 1_000_000_000_000 {
		video /= 1000
	}
	if video >= 0 && video <= limit && (video > 0 || message.Time == 0) {
		return video
	}
	moment := message.Time
	if moment > 1_000_000_000_000 {
		moment /= 1000
	}
	if video > 1_000_000_000 {
		moment = int64(video)
	}
	if begin > 0 && moment > 0 {
		return float64(moment - begin)
	}
	if video > 0 {
		return video
	}
	return 0
}

// reportLive sends one position ping.
func (r *liveRunner) reportLive(ctx context.Context, position int) {
	reportCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	r.reportMu.Lock()
	defer r.reportMu.Unlock()
	r.sendLiveReport(reportCtx, position)
}

// reportLiveFinal sends the last position with its own context.
func (r *liveRunner) reportLiveFinal(position int) {
	reportCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r.reportMu.Lock()
	defer r.reportMu.Unlock()
	r.sendLiveReport(reportCtx, position)
}

// sendLiveReport stores the live or replay position.
func (r *liveRunner) sendLiveReport(ctx context.Context, position int) {
	if r.app.V3 == nil || r.app.V3.Client == nil || r.app.V3.Client.AccessToken == "" {
		return
	}
	request := orangeflix.V3LiveProgressRequest{Mode: r.mode}
	if r.mode == "replay" {
		if r.start.IsZero() {
			return
		}
		request.WatchedAt = r.start.Unix() + int64(position)
	}
	if _, err := r.app.V3.Client.RecordLiveProgress(ctx, request); err != nil {
		if !r.reportErrorLogged {
			r.reportErrorLogged = true
			r.app.logf("Der Live-Fortschritt konnte nicht gespeichert werden: %s", err.Error())
		}
	}
}
