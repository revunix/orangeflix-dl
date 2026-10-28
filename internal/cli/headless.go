package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/revunix/orangeflix-dl/internal/app"
	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// headlessUI is the interface of the runs without a terminal.
type headlessUI struct {
	out         io.Writer
	in          io.Reader
	interactive bool
	reader      *bufio.Reader
}

// SelectEpisodes asks which episodes have to be downloaded. An empty answer
// selects every episode.
func (h *headlessUI) SelectEpisodes(ctx context.Context, format orangeflix.Format, episodes []orangeflix.Episode) ([]string, error) {
	if !h.interactive {
		identifiers := make([]string, 0, len(episodes))
		for _, episode := range episodes {
			identifiers = append(identifiers, episode.Identifier)
		}
		return identifiers, nil
	}
	fmt.Fprintf(h.out, "%s: %d Folgen. Nummern (Komma getrennt, leer = alle): ", format.Title, len(episodes))
	line, err := h.readLine()
	if err != nil {
		return nil, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		identifiers := make([]string, 0, len(episodes))
		for _, episode := range episodes {
			identifiers = append(identifiers, episode.Identifier)
		}
		return identifiers, nil
	}
	wanted := map[int]bool{}
	for _, part := range strings.Split(line, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		number, convErr := strconv.Atoi(part)
		if convErr != nil {
			return nil, fmt.Errorf("ungültige Episodennummer: %s", part)
		}
		wanted[number] = true
	}
	var identifiers []string
	for _, episode := range episodes {
		if wanted[episode.Number] {
			identifiers = append(identifiers, episode.Identifier)
		}
	}
	return identifiers, nil
}

// Event prints a state change.
func (h *headlessUI) Event(event app.Event) {
	switch event.Kind {
	case app.EventLog:
		fmt.Fprintln(h.out, event.Message)
	case app.EventQueued:
		fmt.Fprintf(h.out, "→ %s\n", event.Label)
	case app.EventProgress:
		if event.Total > 0 {
			percent := float64(event.Done) / float64(event.Total) * 100
			fmt.Fprintf(h.out, "\r  %s: %5.1f%%", event.Label, percent)
		}
	case app.EventDone:
		fmt.Fprintf(h.out, "\r✓ %s\n", event.Label)
	case app.EventSkipped:
		fmt.Fprintf(h.out, "· %s (bereits vorhanden)\n", event.Label)
	case app.EventFailed:
		fmt.Fprintf(h.out, "\n✗ %s\n", event.Message)
	}
}

// readLine reads one line from the input.
func (h *headlessUI) readLine() (string, error) {
	if h.reader == nil {
		h.reader = bufio.NewReader(h.in)
	}
	line, err := h.reader.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return line, nil
}
