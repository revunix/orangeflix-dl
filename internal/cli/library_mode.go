package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/revunix/orangeflix-dl/internal/config"
	"github.com/revunix/orangeflix-dl/internal/media"
	"github.com/revunix/orangeflix-dl/internal/store"
)

// libraryEntry is one format of the library in the machine readable form.
type libraryEntry struct {
	PID       int    `json:"pid"`
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Enabled   bool   `json:"enabled"`
	NewCount  int    `json:"newCount"`
	Pending   int    `json:"pending"`
	LastError string `json:"lastError,omitempty"`
	CheckedAt string `json:"checkedAt,omitempty"`
}

// runLibraryMode executes the library modes.
func runLibraryMode(ctx context.Context, opts *config.Options, stdout, stderr io.Writer, state *store.Store,
	interactive bool, stdin io.Reader, stdinTTY bool) int {
	if state == nil {
		fmt.Fprintln(stderr, "Die Bibliothek ist nicht verfügbar.")
		return 1
	}
	switch opts.LibraryMode {
	case "list":
		return runLibraryList(opts, stdout, stderr, state)
	case "check":
		return runLibraryCheck(ctx, opts, stdout, stderr, state, stdin, stdinTTY)
	case "download-new":
		return runLibraryDownloadNew(ctx, opts, stdout, stderr, state)
	case "add":
		return runLibraryAdd(ctx, opts, stdout, stderr, state)
	}
	fmt.Fprintln(stderr, "Unbekannter Bibliotheksmodus: "+opts.LibraryMode)
	return 1
}

// runLibraryList prints the stored formats.
func runLibraryList(opts *config.Options, stdout, stderr io.Writer, state *store.Store) int {
	entries, err := state.Formats()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	result := make([]libraryEntry, 0, len(entries))
	for _, entry := range entries {
		pending, _ := state.CountPending(entry.PID)
		item := libraryEntry{
			PID:       entry.PID,
			Slug:      entry.Slug,
			Title:     entry.Title,
			Status:    entry.Status,
			Enabled:   entry.Enabled,
			NewCount:  entry.NewCount,
			Pending:   pending,
			LastError: entry.LastError,
		}
		if entry.CheckedAt != nil {
			item.CheckedAt = entry.CheckedAt.Format("2006-01-02 15:04")
		}
		result = append(result, item)
	}
	if opts.JSON {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	if len(result) == 0 {
		fmt.Fprintln(stdout, "Die Bibliothek ist leer.")
		return 0
	}
	fmt.Fprintf(stdout, "%-4s %-22s %-6s %s\n", "ID", "FORMAT", "NEU", "TITEL")
	for _, entry := range result {
		title := entry.Title
		if title == "" {
			title = entry.Slug
		}
		fmt.Fprintf(stdout, "%-4d %-22s %-6d %s\n", entry.PID, entry.Slug, entry.NewCount, title)
	}
	return 0
}

// runLibraryCheck looks for new episodes in every stored format.
func runLibraryCheck(ctx context.Context, opts *config.Options, stdout, stderr io.Writer, state *store.Store,
	stdin io.Reader, stdinTTY bool) int {
	logs := newLogRouter(stdout)
	application := buildApp(opts, logs, state, layoutOf(opts), nil)
	application.UI = &headlessUI{out: stdout, in: stdin, interactive: stdinTTY}
	application.Logf = nil
	warnExpiry(ctx, application, logs)

	results, err := application.CheckLibrary(ctx)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	failed := false
	total := 0
	for _, result := range results {
		if result.Err != nil {
			failed = true
			fmt.Fprintf(stderr, "FEHLER %s: %s\n", result.Entry.Slug, result.Err.Error())
			continue
		}
		total += len(result.New)
		for _, episode := range result.New {
			fmt.Fprintf(stdout, "%s: %d - %s\n", result.Entry.Slug, episode.Number, episode.Title)
		}
	}
	fmt.Fprintf(stdout, "%d neue Folge(n)\n", total)
	if failed {
		return 1
	}
	return 0
}

// runLibraryDownloadNew downloads the new episodes of every stored format.
func runLibraryDownloadNew(ctx context.Context, opts *config.Options, stdout, stderr io.Writer, state *store.Store) int {
	logs := newLogRouter(stdout)
	application := buildApp(opts, logs, state, layoutOf(opts), nil)
	application.UI = &headlessUI{out: stdout, in: strings.NewReader(""), interactive: false}
	application.Logf = nil
	warnExpiry(ctx, application, logs)

	entries, err := state.Formats()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	failures := 0
	for _, entry := range entries {
		if !entry.Enabled {
			continue
		}
		if err := application.DownloadNew(ctx, entry); err != nil {
			fmt.Fprintf(stderr, "FEHLER %s: %s\n", entry.Slug, err.Error())
			failures++
		}
	}
	if failures > 0 {
		return 1
	}
	return 0
}

// runLibraryAdd stores a format in the library.
func runLibraryAdd(ctx context.Context, opts *config.Options, stdout, stderr io.Writer, state *store.Store) int {
	logs := newLogRouter(stdout)
	application := buildApp(opts, logs, state, layoutOf(opts), nil)
	application.UI = &headlessUI{out: stdout, in: strings.NewReader(""), interactive: false}
	application.Logf = nil
	// The application writes the message about the added format.
	if _, err := application.AddFormat(ctx, opts.AddFormat); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// layoutOf returns the layout of the options.
func layoutOf(opts *config.Options) media.Layout {
	layout, err := media.ParseLayout(opts.Layout)
	if err != nil {
		return media.LayoutCompat
	}
	return layout
}
