package app

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// runFormat downloads the selected episodes of one format.
func (a *App) runFormat(ctx context.Context, format orangeflix.Format) error {
	a.episodeFailures = 0
	a.firstFailure = ""

	episodes, err := a.episodes(ctx, format)
	if err != nil {
		return a.fail("Die Episoden konnten nicht geladen werden: %s", err.Error())
	}
	if len(episodes) == 0 {
		return a.fail("Das Format %s hat keine Episoden", format.Title)
	}
	a.logf("%s: %d Episoden", format.Title, len(episodes))

	// The format joins the library and the history is written, so a later
	// check only reports the episodes that are really new.
	a.Remember(format)
	a.recordEpisodes(format, episodes)

	selected, err := a.selectEpisodes(ctx, format, episodes)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		a.logf("Keine Folge ausgewählt.")
		return nil
	}
	a.logf("%d Folge(n) ausgewählt.", len(selected))

	// Mit --watch werden die Folgen nicht geladen, sondern mit mpv
	// abgespielt.
	if a.Config != nil && a.Config.Watch {
		return a.Watch(ctx, selected)
	}

	if err := a.downloadEpisodes(ctx, format, selected); err != nil {
		return err
	}
	if a.episodeFailures > 0 {
		return a.fail("%d Download(s) fehlgeschlagen (erster Fehler: %s)", a.episodeFailures, a.firstFailure)
	}
	return nil
}

// downloadEpisodes loads the selected episodes. When more than one parallel
// download is requested, the episodes run in a small worker pool; the
// failures are counted like in the sequential run.
func (a *App) downloadEpisodes(ctx context.Context, format orangeflix.Format, selected []orangeflix.Episode) error {
	parallel := 1
	delay := 0.0
	if a.Config != nil {
		if a.Config.Parallel > 0 {
			parallel = a.Config.Parallel
		}
		delay = a.Config.Delay
	}
	if parallel <= 1 {
		for index, episode := range selected {
			if err := ctx.Err(); err != nil {
				return err
			}
			if index > 0 && delay > 0 {
				a.pause(ctx, delay)
			}
			if err := a.downloadEpisode(ctx, format, episode); err != nil {
				a.recordFailure("%s", err.Error())
			}
		}
		return nil
	}

	a.logf("Lade %d Folge(n) gleichzeitig.", parallel)
	sem := make(chan struct{}, parallel)
	var collected sync.WaitGroup
	for _, episode := range selected {
		if ctx.Err() != nil {
			break
		}
		sem <- struct{}{}
		collected.Add(1)
		go func(episode orangeflix.Episode) {
			defer collected.Done()
			defer func() { <-sem }()
			if delay > 0 {
				a.pause(ctx, delay)
			}
			if err := a.downloadEpisode(ctx, format, episode); err != nil {
				a.recordFailure("%s", err.Error())
			}
		}(episode)
	}
	collected.Wait()
	return nil
}

// pause waits the span between two downloads.
func (a *App) pause(ctx context.Context, seconds float64) {
	span := time.Duration(seconds * float64(time.Second))
	timer := time.NewTimer(span)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// episodes loads the episodes of a format. The version 3 API is preferred:
// it delivers fifty episodes per request, the legacy feed only ten. The RSS
// feed is the fallback of the legacy API.
func (a *App) episodes(ctx context.Context, format orangeflix.Format) ([]orangeflix.Episode, error) {
	if a.V3 != nil {
		episodes, err := a.EpisodesV3(ctx, format.PID)
		if err == nil && len(episodes) > 0 {
			a.logf("Episoden via API v3 geladen.")
			return episodes, nil
		}
		if err != nil {
			a.logf("Die API v3 ist nicht erreichbar (%s), nutze den RSS-Feed.", err.Error())
		}
	}

	submodule := 1
	if a.Config != nil && a.Config.Submodule > 0 {
		submodule = a.Config.Submodule
	}
	episodes, err := a.API.EpisodesViaRSS(ctx, format.PID, submodule, "hd")
	if err == nil && len(episodes) > 0 {
		a.logf("Episoden via RSS-Feed geladen.")
		return episodes, nil
	}
	if err != nil {
		a.logf("RSS-Feed nicht verfügbar (%s), nutze die JSON-API.", err.Error())
	}
	return a.API.Episodes(ctx, format.PID)
}

// episodesPage loads one page of the episodes of a format.
func (a *App) episodesPage(ctx context.Context, format orangeflix.Format, page int) ([]orangeflix.Episode, error) {
	if a.V3 != nil {
		if err := a.EnsureV3(ctx); err == nil {
			clips, err := a.V3.Client.Feed(ctx, format.PID, (page-1)*50, 50)
			if err == nil {
				episodes := make([]orangeflix.Episode, 0, len(clips))
				for _, clip := range clips {
					episodes = append(episodes, clip.Episode())
				}
				return episodes, nil
			}
		}
	}
	submodule := 1
	if a.Config != nil && a.Config.Submodule > 0 {
		submodule = a.Config.Submodule
	}
	episodes, err := a.API.RSSPage(ctx, format.PID, submodule, page, "hd")
	if err == nil {
		return episodes, nil
	}
	return a.API.Feed(ctx, format.PID, page)
}

// selectEpisodes returns the episodes that have to be downloaded. The
// command line decides; without a selection the user is asked.
func (a *App) selectEpisodes(ctx context.Context, format orangeflix.Format, episodes []orangeflix.Episode) ([]orangeflix.Episode, error) {
	selection := a.Config

	if len(a.Requested) > 0 {
		wanted := map[string]bool{}
		for _, identifier := range a.Requested {
			wanted[identifier] = true
		}
		return pickEpisodes(episodes, func(episode orangeflix.Episode) bool {
			return wanted[episode.Identifier]
		}), nil
	}

	if selection.Episode != "" {
		number, err := strconv.Atoi(selection.Episode)
		if err != nil {
			return nil, a.fail("Ungültige Episodennummer: %s", selection.Episode)
		}
		return pickEpisodes(episodes, func(episode orangeflix.Episode) bool {
			return episode.Number == number
		}), nil
	}

	if selection.EpisodeRange != "" {
		from, to, found := strings.Cut(selection.EpisodeRange, "-")
		if !found {
			return nil, a.fail("Ungültiger Bereich: %s", selection.EpisodeRange)
		}
		start, err := strconv.Atoi(from)
		if err != nil {
			return nil, a.fail("Ungültiger Bereich: %s", selection.EpisodeRange)
		}
		end, err := strconv.Atoi(to)
		if err != nil {
			return nil, a.fail("Ungültiger Bereich: %s", selection.EpisodeRange)
		}
		if end < start {
			start, end = end, start
		}
		return pickEpisodes(episodes, func(episode orangeflix.Episode) bool {
			return episode.Number >= start && episode.Number <= end
		}), nil
	}

	if selection.Year != "" {
		year, err := strconv.Atoi(selection.Year)
		if err != nil {
			return nil, a.fail("Ungültiges Jahr: %s", selection.Year)
		}
		return pickEpisodes(episodes, func(episode orangeflix.Episode) bool {
			return !episode.Date.IsZero() && episode.Date.Year() == year
		}), nil
	}

	if selection.Since != "" {
		since, err := time.Parse("2006-01-02", selection.Since)
		if err != nil {
			return nil, a.fail("Ungültiges Datum: %s", selection.Since)
		}
		return a.pickSince(episodes, since), nil
	}

	if selection.Title != "" {
		return a.pickTitle(episodes, selection.Title), nil
	}

	if selection.All || a.automatic() || a.UI == nil {
		return episodes, nil
	}

	identifiers, err := a.UI.SelectEpisodes(ctx, format, episodes)
	if err != nil {
		return nil, err
	}
	wanted := map[string]bool{}
	for _, identifier := range identifiers {
		wanted[identifier] = true
	}
	return pickEpisodes(episodes, func(episode orangeflix.Episode) bool {
		return wanted[episode.Identifier]
	}), nil
}

// pickEpisodes returns the episodes that match the predicate.
func pickEpisodes(episodes []orangeflix.Episode, match func(orangeflix.Episode) bool) []orangeflix.Episode {
	var selected []orangeflix.Episode
	for _, episode := range episodes {
		if match(episode) {
			selected = append(selected, episode)
		}
	}
	return selected
}

// pickSince returns the episodes that were released from the date on
// including the date itself. An episode without a date is not selected.
func (a *App) pickSince(episodes []orangeflix.Episode, since time.Time) []orangeflix.Episode {
	since = time.Date(since.Year(), since.Month(), since.Day(), 0, 0, 0, 0, since.Location())
	return pickEpisodes(episodes, func(episode orangeflix.Episode) bool {
		day := time.Date(episode.Date.Year(), episode.Date.Month(), episode.Date.Day(), 0, 0, 0, 0, since.Location())
		return !episode.Date.IsZero() && !day.Before(since)
	})
}

// pickTitle returns the episodes whose title contains the text without
// regard to the case.
func (a *App) pickTitle(episodes []orangeflix.Episode, text string) []orangeflix.Episode {
	text = strings.ToLower(strings.TrimSpace(text))
	return pickEpisodes(episodes, func(episode orangeflix.Episode) bool {
		return strings.Contains(strings.ToLower(episode.Title), text)
	})
}
