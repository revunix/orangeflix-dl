// Package cli is the entry point: argument handling, wiring and exit codes.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/revunix/orangeflix-dl/internal/app"
	"github.com/revunix/orangeflix-dl/internal/config"
	"github.com/revunix/orangeflix-dl/internal/download"
	"github.com/revunix/orangeflix-dl/internal/media"
	"github.com/revunix/orangeflix-dl/internal/orangeflix"
	"github.com/revunix/orangeflix-dl/internal/secret"
	"github.com/revunix/orangeflix-dl/internal/store"
	"github.com/revunix/orangeflix-dl/internal/tui"
)

// Version is the build version, injected through -ldflags.
var Version = "dev"

// Usage is printed for --help.
const Usage = `Usage: orangeflix-dl [options]
       orangeflix-dl [options] --format <format> [--episode <n> | --range <a-b> | --year <jjjj> | --since <datum> | --title <text> | --all]
       orangeflix-dl --library | --check | --download-new | --add <format>
       orangeflix-dl --links-file <datei>

Options:
  --list                     alle Formate auflisten
  --format <format>          Format wählen (Slug, Name oder ID)
  --episode <n>              einzelne Folge
  --range <a-b>              Folgenbereich
  --year <jjjj>              alle Folgen eines Jahres
  --since <datum>            alle Folgen ab dem Tag (YYYY-MM-DD)
  --title <text>             alle Folgen, deren Titel den Text enthält
  --all                      alle Folgen des Formats
  --quality <qualitaet>      best, hd, sd, mobile, audio
  --layout <layout>          compat (Standard) oder flat
  --parallel <n>             bis zu n Downloads gleichzeitig (1-8)
  --delay <sekunden>         Pause zwischen zwei Downloads
  --thumb                    Vorschaubild (.jpg) neben das Video legen
  --meta                     Metadaten (.json) neben das Video legen
  --totp <code>              aktueller Code des zweiten Faktors
  --library                  gespeicherte Formate auflisten
  --json                     Ausgabe als JSON (mit --library oder --list)
  --check                    nach neuen Folgen suchen
  --download-new             die neuen Folgen laden
  --add <format>             Format in die Bibliothek aufnehmen
  --automatic                ohne Rueckfragen laufen
  --links-file <datei>       Formate aus einer Datei lesen
  --force-redownload         vorhandene Dateien erneut laden
  --watch                    die Folgen mit mpv ansehen statt laden
  --completions <shell>      Kompletion ausgeben: bash, zsh oder fish
  --man                      die Manpage ausgeben
  -h, --help                 diese Hilfe anzeigen
  -v, --version              die Version anzeigen

Umgebung: ORANGEFLIX_USER, ORANGEFLIX_PASSWORD, ORANGEFLIX_QUALITY,
          ORANGEFLIX_LAYOUT, ORANGEFLIX_SUBMODULE, ORANGEFLIX_DOWNLOADS_DIR,
          ORANGEFLIX_LIBRARY, ORANGEFLIX_CREDENTIALS_FILE, ORANGEFLIX_TOTP,
          ORANGEFLIX_PARALLEL, ORANGEFLIX_DELAY, ORANGEFLIX_THUMB,
          ORANGEFLIX_META, ORANGEFLIX_CONFIG_FILE, XDG_STATE_HOME, XDG_CONFIG_HOME

Konfigurationsdatei (~/.config/orangeflix-dl/config, Vorgaben wie quality=hd,
layout=flat, thumb=1, parallel=2): die Kommandozeile und die Umgebung haben Vorrang.`

// Run executes the command line and returns the process exit code.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, err := config.ParseArgs(args)
	missing := errors.Is(err, config.ErrNoTarget)
	switch {
	case errors.Is(err, config.ErrHelp):
		fmt.Fprintln(stdout, Usage)
		return 0
	case errors.Is(err, config.ErrVersion):
		fmt.Fprintf(stdout, "orangeflix-dl %s\n", Version)
		return 0
	case err != nil && !missing:
		fmt.Fprintln(stderr, err)
		return 1
	case missing:
		opts = &config.Options{}
	}
	if opts.Completions != "" {
		return printCompletions(opts.Completions, stdout)
	}
	if opts.Man {
		return printManPage(stdout)
	}

	env := config.OSEnv
	if err := opts.Resolve(env, stderr); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := opts.Validate(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := config.ValidateQuality(opts.Quality); err != nil {
		fmt.Fprintf(stderr, "Ungültige Qualität: %s (erlaubt: %s)\n", opts.Quality, strings.Join(orangeflix.Qualities, ", "))
		return 1
	}
	if opts.Format != "" {
		if err := config.ValidateFormat(opts.Format); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	layout, err := media.ParseLayout(opts.Layout)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	stdinFile, stdinIsFile := stdin.(*os.File)
	stdinTTY := stdinIsFile && term.IsTerminal(int(stdinFile.Fd()))
	stdoutFile, stdoutIsFile := stdout.(*os.File)
	stdoutTTY := stdoutIsFile && term.IsTerminal(int(stdoutFile.Fd()))
	interactive := useTUI(opts, stdinTTY, stdoutTTY)

	opts.WorkDir, err = resolveWorkDir(opts)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	opts.LibraryPath = config.LibraryPath(env, opts.WorkDir)
	opts.DownloadsDir = config.DownloadsDir(env, opts.WorkDir)

	state, storeErr := store.Open(opts.LibraryPath)
	if storeErr != nil {
		fmt.Fprintf(stderr, "Warnung: Die Bibliothek ist nicht verfügbar (%v)\n", storeErr)
		state = nil
	} else {
		defer state.Close()
	}
	applyStoredDownloadsDir(opts, state, env)

	// Das Passwort liegt im Schlüsselbund des Systems, der Benutzername in
	// der Bibliothek. So fragen beide nach dem ersten Mal nicht mehr.
	secrets := secret.Open(opts.CredentialsFile)
	hasToken, err := resolveCredentials(opts, env, secrets, state)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	if opts.List {
		return runList(ctx, opts, stdout, stderr, secrets)
	}
	if opts.Search != "" {
		return runSearch(ctx, opts, stdout, stderr, secrets)
	}
	if opts.Profile {
		return runProfile(ctx, opts, stdout, stderr, secrets)
	}
	if opts.LibraryMode != "" {
		return runLibraryMode(ctx, opts, stdout, stderr, state, interactive, stdin, stdinTTY)
	}

	if opts.Batch() {
		if !fileExists(opts.LinksFile) {
			fmt.Fprintf(stderr, "Link-Datei fehlt: %s\n", opts.LinksFile)
			return 1
		}
	} else if opts.Format == "" {
		if !interactive {
			fmt.Fprintln(stderr, config.ErrNoTarget)
			return 1
		}
	}

	if err := config.EnsureWritableDir(opts.DownloadsDir); err != nil {
		fmt.Fprintf(stderr, "Das Download-Verzeichnis ist nicht beschreibbar: %s (%v)\n", opts.DownloadsDir, err)
		return 1
	}

	if opts.Username == "" || opts.Password == "" {
		if interactive {
			// The start screen of the TUI asks for the credentials.
		} else {
			fmt.Fprintln(stderr, "Zugangsdaten fehlen: ORANGEFLIX_USER und ORANGEFLIX_PASSWORD setzen oder die Zugangsdaten-Datei verwenden.")
			return 1
		}
	}

	logs := newLogRouter(stdout)
	// The diagnostic files collect the requests of the run, when the feature
	// is switched on.
	if debugDir := resolveDebugDir(env); debugDir != "" {
		file, openErr := os.OpenFile(filepath.Join(debugDir, "requests.log"),
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if openErr != nil {
			fmt.Fprintf(stderr, "Warnung: Die Diagnose-Datei ist nicht schreibbar: %v\n", openErr)
		} else {
			logs.SetFile(file)
			defer logs.CloseFile()
			logs.Logf("Diagnose-Dateien landen in: %s", debugDir)
		}
	}
	application := buildApp(opts, logs, state, layout, secrets)

	if interactive {
		ui := tui.NewUI(tui.Options{
			Context:  ctx,
			Config:   opts,
			Store:    state,
			Version:  Version,
			HasToken: hasToken,
			Run: func(runCtx context.Context) error {
				return application.Run(runCtx)
			},
			Check: func(checkCtx context.Context) ([]app.CheckResult, error) {
				return application.CheckLibrary(checkCtx)
			},
			DownloadNew: func(newCtx context.Context, entry store.FormatEntry) error {
				return application.DownloadNew(newCtx, entry)
			},
			AddFormat: func(addCtx context.Context, value string) (store.FormatEntry, error) {
				return application.AddFormat(addCtx, value)
			},
			Episodes: func(episodeCtx context.Context, pid int) ([]orangeflix.Episode, error) {
				return application.Episodes(episodeCtx, pid)
			},
			ClipInfo: func(clipCtx context.Context, identifier string) ([]orangeflix.File, error) {
				return application.ClipInfo(clipCtx, identifier)
			},
			Formats: func(formatsCtx context.Context) ([]orangeflix.Format, error) {
				return application.Formats(formatsCtx)
			},
			Profile: func(profileCtx context.Context) (*app.ProfileSummary, error) {
				return application.ProfileSummary(profileCtx)
			},
			Search: func(searchCtx context.Context, query string, limit int) ([]orangeflix.Episode, error) {
				return application.Search(searchCtx, query, limit)
			},
			Watch: func(watchCtx context.Context, episodes []orangeflix.Episode) (*app.Player, error) {
				return application.PlayerFor(watchCtx, episodes)
			},
			SetQuality: func(quality string) {
				application.SetQuality(quality)
			},
			SetDownloadsDir: func(dir string) {
				application.SetDownloadsDir(dir)
			},
			SetSelection: func(identifiers []string) {
				application.Requested = identifiers
			},
			SetCredentials: func(user, password string) {
				application.SetCredentials(user, password)
				if password != "" {
					if err := secrets.Set(password); err != nil {
						fmt.Fprintf(stderr, "Warnung: Das Passwort konnte nicht gespeichert werden (%v)\n", err)
					}
				}
				// Der Token gehört zum alten Konto, er wird entfernt.
				if err := secrets.ClearToken(); err != nil {
					fmt.Fprintf(stderr, "Warnung: Der alte Zugriffstoken konnte nicht entfernt werden (%v)\n", err)
				}
				if state != nil {
					_ = state.SetSetting(store.SettingUsername, user)
				}
			},
			SetTotp: func(code string) {
				application.SetTotp(code)
			},
		})
		logs.SetSink(func(line string) {
			ui.Event(app.Event{Kind: app.EventLog, Message: line})
		})
		application.Logf = nil
		application.UI = ui
		final, runErr := ui.Program().Run()
		model, isModel := final.(*tui.Model)
		if isModel {
			model.Close()
		}
		if runErr != nil {
			fmt.Fprintln(stderr, runErr)
			return 1
		}
		if isModel {
			return model.ExitCode()
		}
		return 0
	}

	application.UI = &headlessUI{out: stdout, in: stdin, interactive: stdinTTY}
	// The interface writes the log lines, so the router must not print them a
	// second time.
	application.Logf = nil
	// Ohne Menü kann der Code des zweiten Faktors an der Konsole abgefragt
	// werden, sobald die Anmeldung ihn verlangt.
	application.V3.TotpPrompt = totpPrompt(stdout, stdin)
	warnExpiry(ctx, application, logs)
	if err := application.Run(ctx); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// applyStoredDownloadsDir uses the folder that was chosen in the interface
// first, then the one of the settings file, when the environment does not
// name one.
func applyStoredDownloadsDir(opts *config.Options, state *store.Store, env config.Env) {
	if env("ORANGEFLIX_DOWNLOADS_DIR") != "" {
		return
	}
	if state != nil {
		if value, err := state.Setting(store.SettingDownloadsDir); err == nil {
			if value = strings.TrimSpace(value); value != "" {
				opts.DownloadsDir = config.ExpandPath(value)
				return
			}
		}
	}
	if value := strings.TrimSpace(opts.FileDownloadsDir); value != "" {
		opts.DownloadsDir = config.ExpandPath(value)
	}
}

// useTUI reports whether the interactive interface should be started.
func useTUI(opts *config.Options, stdinTTY, stdoutTTY bool) bool {
	if opts.LibraryMode != "" || opts.List || opts.Batch() {
		return false
	}
	return stdinTTY && stdoutTTY
}

// buildApp wires the components of the application.
func buildApp(opts *config.Options, logs *logRouter, state *store.Store, layout media.Layout, secrets secret.Store) *app.App {
	transport := httpTransport()
	client := &orangeflix.Client{
		HTTP:     &http.Client{Timeout: 120 * time.Second, Transport: transport},
		Username: opts.Username,
		Password: opts.Password,
		Logf:     logs.Logf,
	}
	downloader := &download.Downloader{
		HTTP: &http.Client{Transport: transport},
		UA:   orangeflix.DefaultUA,
		Logf: logs.Logf,
	}
	manager := &download.Manager{
		DownloadsDir: opts.DownloadsDir,
		Force:        opts.ForceRedownload,
		Logf:         logs.Logf,
	}
	application := &app.App{
		Config:     opts,
		API:        client,
		Downloader: downloader,
		Manager:    manager,
		Store:      state,
		Layout:     layout,
		Credentials: download.Credentials{
			Username: opts.Username,
			Password: opts.Password,
		},
	}
	// Die Version-3-API liefert mehr Daten und braucht nur einen Token. Sie
	// wird benutzt, wenn die Zugangsdaten stimmen; sonst bleibt das alte
	// API der Rückfall.
	v3client := &orangeflix.V3Client{
		HTTP:     &http.Client{Timeout: 120 * time.Second, Transport: transport},
		Username: opts.Username,
		Password: opts.Password,
		Logf:     logs.Logf,
	}
	application.V3 = &app.V3{Client: v3client}
	if secrets != nil {
		application.V3.TokenStore = secrets
	}
	application.Logf = logs.Logf
	return application
}

// httpTransport returns the shared transport.
func httpTransport() *http.Transport {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil
	}
	clone := base.Clone()
	clone.MaxIdleConns = 32
	clone.MaxIdleConnsPerHost = 8
	clone.IdleConnTimeout = 90 * time.Second
	return clone
}

// runList prints the catalog of the formats. The live catalog of the API is
// preferred, the catalog of the program is the fallback.
func runList(ctx context.Context, opts *config.Options, stdout, stderr io.Writer, secrets secret.Store) int {
	application := buildApp(opts, newLogRouter(stdout), nil, media.LayoutCompat, secrets)
	formats, err := application.Formats(ctx)
	if err != nil {
		fmt.Fprintln(stderr, "Der Katalog konnte nicht geladen werden:", err)
		return 1
	}
	type entry struct {
		PID         int    `json:"pid"`
		Slug        string `json:"slug"`
		Title       string `json:"title"`
		Handle      string `json:"handle,omitempty"`
		Status      string `json:"status"`
		ClipCount   int    `json:"clipCount,omitempty"`
		LastEpisode string `json:"lastEpisode,omitempty"`
	}
	var entries []entry
	for _, format := range formats {
		entries = append(entries, entry{
			PID:         format.PID,
			Slug:        format.Slug,
			Title:       format.Title,
			Handle:      format.Handle,
			Status:      format.Status,
			ClipCount:   format.ClipCount,
			LastEpisode: format.LastEpisode,
		})
	}
	if opts.JSON {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(entries); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	// Die Formate stehen nach ihrem Status gruppiert.
	groups := map[string][]orangeflix.Format{}
	for _, format := range formats {
		groups[format.Status] = append(groups[format.Status], format)
	}
	for _, status := range []string{orangeflix.StatusActive, orangeflix.StatusSeasonal, orangeflix.StatusArchived} {
		group := groups[status]
		if len(group) == 0 {
			continue
		}
		label := "aktiv"
		switch status {
		case orangeflix.StatusSeasonal:
			label = "saisonal"
		case orangeflix.StatusArchived:
			label = "archiviert"
		}
		fmt.Fprintf(stdout, "\n%s (%d)\n", strings.ToUpper(label), len(group))
		for _, format := range group {
			count := ""
			if format.ClipCount > 0 {
				count = fmt.Sprintf("  %d Folgen", format.ClipCount)
			}
			handle := format.Handle
			if len(handle) > 6 {
				handle = handle[:6]
			}
			fmt.Fprintf(stdout, "  %-4d %-20s %-26s %-6s %s\n", format.PID, format.Slug, format.Title, handle, count)
		}
	}
	fmt.Fprintf(stdout, "\n%d Formate\n", len(entries))
	return 0
}
