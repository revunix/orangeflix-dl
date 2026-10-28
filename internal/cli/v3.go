package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/revunix/orangeflix-dl/internal/config"
	"github.com/revunix/orangeflix-dl/internal/media"
	"github.com/revunix/orangeflix-dl/internal/secret"
)

// runSearch searches the whole catalog and prints the hits.
func runSearch(ctx context.Context, opts *config.Options, stdout, stderr io.Writer, secrets secret.Store) int {
	application := buildApp(opts, newLogRouter(stdout), nil, media.LayoutCompat, secrets)
	episodes, err := application.Search(ctx, opts.Search, 50)
	if err != nil {
		fmt.Fprintln(stderr, "Die Suche ist fehlgeschlagen:", err)
		return 1
	}
	if len(episodes) == 0 {
		fmt.Fprintf(stdout, "Keine Treffer für %q\n", opts.Search)
		return 0
	}
	fmt.Fprintf(stdout, "%-8s %-6s %-22s %-10s %s\n", "PID", "NUM", "FORMAT", "LÄNGE", "TITEL")
	for _, episode := range episodes {
		fmt.Fprintf(stdout, "%-8d %-6s %-22s %-10s %s\n",
			episode.PID,
			"E"+padNumber(episode.Number, 3),
			truncate(episode.FormatName, 22),
			episode.Duration,
			episode.Title,
		)
	}
	fmt.Fprintf(stdout, "\n%d Treffer für %q\n", len(episodes), opts.Search)
	return 0
}

// runProfile prints the state of the subscription.
func runProfile(ctx context.Context, opts *config.Options, stdout, stderr io.Writer, secrets secret.Store) int {
	application := buildApp(opts, newLogRouter(stdout), nil, media.LayoutCompat, secrets)
	profile, err := application.ProfileSummary(ctx)
	if err != nil {
		fmt.Fprintln(stderr, "Das Konto konnte nicht gelesen werden:", err)
		return 1
	}
	fmt.Fprintf(stdout, "Zugang:   %s\n", profile.Summarize())
	if profile.ExpiresAt.IsZero() {
		fmt.Fprintln(stdout, "Ablauf:   unbefristet")
	} else {
		fmt.Fprintf(stdout, "Ablauf:   %s\n", profile.ExpiresAt.Local().Format("02.01.2006 15:04"))
	}
	return 0
}

// truncate shortens a text to the width.
func truncate(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}

// padNumber fills a number with leading zeros.
func padNumber(value, width int) string {
	text := fmt.Sprintf("%d", value)
	for len(text) < width {
		text = "0" + text
	}
	return text
}
