// Package app orchestrates the Orangeflix workflows: listing the formats,
// downloading episodes and keeping the library up to date.
package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/revunix/orangeflix-dl/internal/config"
	"github.com/revunix/orangeflix-dl/internal/download"
	"github.com/revunix/orangeflix-dl/internal/media"
	"github.com/revunix/orangeflix-dl/internal/orangeflix"
	"github.com/revunix/orangeflix-dl/internal/store"
)

// EventKind classifies progress events for the user interface.
type EventKind int

// Event kinds.
const (
	EventLog EventKind = iota
	EventQueued
	EventProgress
	EventSkipped
	EventDone
	EventFailed
)

// Event is one user visible state change.
type Event struct {
	Kind    EventKind
	Label   string
	Path    string
	Message string
	Done    int64
	Total   int64
	// EpisodeID names the episode the event belongs to.
	EpisodeID string
	// PID names the format of the episode.
	PID int
}

// UI is the front end the app talks to.
type UI interface {
	// SelectEpisodes asks which episodes of a format have to be downloaded.
	// The automatic mode never asks.
	SelectEpisodes(ctx context.Context, format orangeflix.Format, episodes []orangeflix.Episode) ([]string, error)
	// Event reports a state change.
	Event(Event)
}

// App runs the download workflows.
type App struct {
	Config     *config.Options
	API        *orangeflix.Client
	Downloader *download.Downloader
	Manager    *download.Manager
	Store      *store.Store
	UI         UI
	Logf       func(format string, args ...any)

	// Layout is the directory layout of the downloads.
	Layout media.Layout

	// Requested holds the identifiers of the episodes that were marked in the
	// interface. It is empty when the command line decides what to load.
	Requested []string

	// V3 is the client of the version 3 API. When it is set, the program uses
	// it for the catalog, the episodes and the download files.
	V3 *V3

	// Credentials are the access data of the legacy API.
	Credentials download.Credentials

	episodeFailures int
	firstFailure    string
	failMu          sync.Mutex
	logFile         *os.File
}

// automatic reports whether the flow must run without asking the user.
func (a *App) automatic() bool {
	return a.Config != nil && a.Config.Automatic
}

func (a *App) logf(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	if a.Logf != nil {
		a.Logf("%s", line)
	}
	if a.logFile != nil {
		_, _ = a.logFile.WriteString(line + "\n")
	}
	if a.UI != nil {
		a.UI.Event(Event{Kind: EventLog, Message: line})
	}
}

func (a *App) fail(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

func (a *App) event(event Event) {
	if a.UI != nil {
		a.UI.Event(event)
	}
}

func (a *App) recordFailure(format string, args ...any) {
	a.failMu.Lock()
	defer a.failMu.Unlock()
	reason := fmt.Sprintf(format, args...)
	a.episodeFailures++
	if a.firstFailure == "" {
		a.firstFailure = reason
	}
	a.logf("FEHLER: %s", reason)
	a.event(Event{Kind: EventFailed, Message: reason})
}

// SetQuality takes over the quality for the next download.
func (a *App) SetQuality(quality string) {
	if a.Config != nil {
		a.Config.Quality = quality
	}
}

// SetDownloadsDir changes the root of the downloads for the next run.
func (a *App) SetDownloadsDir(dir string) {
	if dir == "" {
		return
	}
	if a.Config != nil {
		a.Config.DownloadsDir = dir
	}
	if a.Manager != nil {
		a.Manager.DownloadsDir = dir
	}
}

// SetCredentials hands new credentials to the client and the downloader.
// The access token of the old account is dropped: it belongs to another
// account and would answer for it.
func (a *App) SetCredentials(user, password string) {
	if a.API != nil {
		a.API.Username = user
		a.API.Password = password
	}
	a.Credentials.Username = user
	a.Credentials.Password = password
	if a.V3 != nil && a.V3.Client != nil {
		// Der Client der Version 3 meldet sich mit eigenen Zugangsdaten an;
		// ohne sie holte er den Token wieder für das alte Konto.
		a.V3.Client.Username = user
		a.V3.Client.Password = password
		a.V3.Client.AccessToken = ""
		a.V3.Client.RefreshToken = ""
		a.V3.Client.ExpiresAt = time.Time{}
	}
}

// Episodes returns the episodes of a format.
func (a *App) Episodes(ctx context.Context, pid int) ([]orangeflix.Episode, error) {
	format, ok := orangeflix.FormatByPID(pid)
	if !ok {
		format = orangeflix.Format{PID: pid, Slug: fmt.Sprintf("format-%d", pid), Title: fmt.Sprintf("Format %d", pid)}
	}
	// Der Benutzer war in dem Format: die Liste merkt sich das.
	a.MarkOpened(format)
	return a.episodes(ctx, format)
}

// MarkOpened remembers that the user has been in a format.
func (a *App) MarkOpened(format orangeflix.Format) {
	if a.Store == nil {
		return
	}
	if err := a.Store.MarkOpened(format.PID); err != nil {
		a.logf("Der Besuch konnte nicht gemerkt werden: %s", err.Error())
	}
}

// Run executes the configured mode.
func (a *App) Run(ctx context.Context) error {
	if a.Config == nil {
		return a.fail("no configuration")
	}
	if a.Batch() {
		return a.runBatch(ctx)
	}
	format, err := orangeflix.LookupFormat(a.Config.Format)
	if err != nil {
		return a.fail("%s", err.Error())
	}
	return a.runFormat(ctx, format)
}

// Batch reports whether the links-file mode is active.
func (a *App) Batch() bool {
	return a.Config != nil && a.Config.Batch()
}

// runBatch processes every format of the links file.
func (a *App) runBatch(ctx context.Context) error {
	links, err := readLinks(a.Config.LinksFile)
	if err != nil {
		return err
	}
	stamp := time.Now().Format("2006-01-02_15-04-05")
	logsDir := filepath.Join(filepath.Dir(a.Config.LinksFile), "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return a.fail("Das Logs-Verzeichnis ist nicht beschreibbar: %s", logsDir)
	}
	successLog := filepath.Join(logsDir, "successful_links_"+stamp+".txt")
	failedLog := filepath.Join(logsDir, "failed_links_"+stamp+".txt")
	downloadLog := filepath.Join(logsDir, "download_log_"+stamp+".txt")
	for _, path := range []string{successLog, failedLog, downloadLog} {
		file, createErr := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if createErr != nil {
			return a.fail("Das Logs-Verzeichnis ist nicht beschreibbar: %s", logsDir)
		}
		file.Close()
	}
	logFile, err := os.OpenFile(downloadLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return a.fail("Das Logs-Verzeichnis ist nicht beschreibbar: %s", logsDir)
	}
	defer func() {
		logFile.Close()
		a.logFile = nil
	}()
	a.logFile = logFile

	a.logf("Stapelverarbeitung gestartet: %s", stamp)
	failures := 0
	for _, link := range links {
		format, lookupErr := orangeflix.LookupFormat(link)
		if lookupErr != nil {
			a.logf("FEHLER: %s", lookupErr.Error())
			appendLine(failedLog, link+"\t"+lookupErr.Error())
			failures++
			continue
		}
		a.episodeFailures, a.firstFailure = 0, ""
		if runErr := a.runFormat(ctx, format); runErr != nil {
			appendLine(failedLog, link+"\t"+runErr.Error())
			failures++
			continue
		}
		appendLine(successLog, link)
	}
	a.logf("Stapelverarbeitung beendet, Fehler: %d", failures)
	if failures > 0 {
		return a.fail("%d Format(e) fehlgeschlagen", failures)
	}
	return nil
}

// readLinks reads the formats of the links file.
func readLinks(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Die Link-Datei fehlt: %s", path)
	}
	var links []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		links = append(links, line)
	}
	return links, nil
}

func appendLine(path, line string) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.WriteString(line + "\n")
}
