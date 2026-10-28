package app

import (
	"context"
	"fmt"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
	"github.com/revunix/orangeflix-dl/internal/store"
)

// CheckResult is the outcome of the check of one format.
type CheckResult struct {
	Entry store.FormatEntry
	// New are the episodes that were not known before.
	New []store.EpisodeRef
	Err error
}

// CheckLibrary compares the episodes of every stored format with the history
// and returns the new ones.
func (a *App) CheckLibrary(ctx context.Context) ([]CheckResult, error) {
	if a.Store == nil {
		return nil, fmt.Errorf("die Bibliothek ist nicht verfügbar")
	}
	entries, err := a.Store.Formats()
	if err != nil {
		return nil, err
	}
	results := make([]CheckResult, 0, len(entries))
	for _, entry := range entries {
		if !entry.Enabled {
			continue
		}
		results = append(results, a.CheckEntry(ctx, entry))
	}
	return results, nil
}

// newEpisodes loads the episodes that are not known yet. The feed lists the
// newest episodes first, so the search stops at the first known episode and
// one request is enough in the normal case.
func (a *App) newEpisodes(ctx context.Context, format orangeflix.Format, known map[string]bool) ([]orangeflix.Episode, error) {
	var episodes []orangeflix.Episode
	for page := 1; page <= 5; page++ {
		listed, err := a.episodesPage(ctx, format, page)
		if err != nil {
			if len(episodes) == 0 {
				return nil, err
			}
			break
		}
		if len(listed) == 0 {
			break
		}
		stop := false
		for _, episode := range listed {
			if known[episode.Identifier] {
				stop = true
				break
			}
			episodes = append(episodes, episode)
		}
		if stop {
			break
		}
	}
	return episodes, nil
}

// CheckEntry checks one format of the library for new episodes.
func (a *App) CheckEntry(ctx context.Context, entry store.FormatEntry) CheckResult {
	result := CheckResult{Entry: entry}
	format, err := orangeflix.LookupFormat(entry.Slug)
	if err != nil {
		format = orangeflix.Format{PID: entry.PID, Slug: entry.Slug, Title: entry.Title, Status: entry.Status}
	}
	known, err := a.Store.KnownIdentifiers(entry.PID)
	if err != nil {
		result.Err = err
		return result
	}

	episodes, err := a.newEpisodes(ctx, format, known)
	if err != nil {
		result.Err = err
		_ = a.Store.RecordCheck(entry.PID, 0, store.TrimError(err))
		return result
	}

	refs := make([]store.EpisodeRef, 0, len(episodes))
	for _, episode := range episodes {
		refs = append(refs, store.EpisodeRef{
			Identifier: episode.Identifier,
			Number:     episode.Number,
			Title:      episode.Title,
			Date:       episode.Date,
		})
	}
	fresh, err := a.Store.SyncEpisodes(entry.PID, refs)
	if err != nil {
		result.Err = err
		return result
	}
	result.New = fresh
	_ = a.Store.RecordCheck(entry.PID, len(fresh), "")
	return result
}

// DownloadNew downloads the new episodes of one format.
func (a *App) DownloadNew(ctx context.Context, entry store.FormatEntry) error {
	format, err := orangeflix.LookupFormat(entry.Slug)
	if err != nil {
		format = orangeflix.Format{PID: entry.PID, Slug: entry.Slug, Title: entry.Title, Status: entry.Status}
	}
	known, err := a.Store.KnownIdentifiers(entry.PID)
	if err != nil {
		return err
	}
	episodes, err := a.newEpisodes(ctx, format, known)
	if err != nil {
		return fmt.Errorf("die Episoden konnten nicht geladen werden: %w", err)
	}
	refs := make([]store.EpisodeRef, 0, len(episodes))
	for _, episode := range episodes {
		refs = append(refs, store.EpisodeRef{
			Identifier: episode.Identifier,
			Number:     episode.Number,
			Title:      episode.Title,
			Date:       episode.Date,
		})
	}
	fresh, err := a.Store.SyncEpisodes(entry.PID, refs)
	if err != nil {
		return err
	}
	if len(fresh) == 0 {
		a.logf("%s: keine neuen Folgen.", format.Title)
		return nil
	}
	wanted := map[string]bool{}
	for _, ref := range fresh {
		wanted[ref.Identifier] = true
	}
	selected := pickEpisodes(episodes, func(episode orangeflix.Episode) bool {
		return wanted[episode.Identifier]
	})
	a.logf("%s: %d neue Folge(n).", format.Title, len(selected))

	a.episodeFailures = 0
	a.firstFailure = ""
	if err := a.downloadEpisodes(ctx, format, selected); err != nil {
		return err
	}
	if a.episodeFailures > 0 {
		return a.fail("%d Download(s) fehlgeschlagen (erster Fehler: %s)", a.episodeFailures, a.firstFailure)
	}
	return nil
}

// AddFormat stores a format in the library without downloading anything. The
// episodes of the format are written into the history, so a later check only
// reports the episodes that are really new.
func (a *App) AddFormat(ctx context.Context, value string) (store.FormatEntry, error) {
	format, err := orangeflix.LookupFormat(value)
	if err != nil {
		return store.FormatEntry{}, err
	}
	if a.Store == nil {
		return store.FormatEntry{}, fmt.Errorf("die Bibliothek ist nicht verfügbar")
	}
	if err := a.Store.UpsertFormat(format.PID, format.Slug, format.Title, format.Status); err != nil {
		return store.FormatEntry{}, err
	}
	a.logf("%s wurde der Bibliothek hinzugefügt.", format.Title)

	// The history of the format is filled, so the episodes that exist now do
	// not count as new later on.
	if episodes, loadErr := a.episodes(ctx, format); loadErr != nil {
		a.logf("Die Episodenliste konnte nicht geladen werden (%s), der Check holt sie später nach.", loadErr.Error())
	} else {
		a.recordEpisodes(format, episodes)
	}

	entry, err := a.Store.FormatByPID(format.PID)
	if err != nil || entry == nil {
		return store.FormatEntry{PID: format.PID, Slug: format.Slug, Title: format.Title, Status: format.Status}, nil
	}
	return *entry, nil
}

// Remember stores a format in the library.
func (a *App) Remember(format orangeflix.Format) {
	if a.Store == nil {
		return
	}
	_ = a.Store.UpsertFormat(format.PID, format.Slug, format.Title, format.Status)
}

// recordEpisodes writes the episodes of a format into the history.
func (a *App) recordEpisodes(format orangeflix.Format, episodes []orangeflix.Episode) {
	if a.Store == nil {
		return
	}
	refs := make([]store.EpisodeRef, 0, len(episodes))
	for _, episode := range episodes {
		refs = append(refs, store.EpisodeRef{
			Identifier: episode.Identifier,
			Number:     episode.Number,
			Title:      episode.Title,
			Date:       episode.Date,
		})
	}
	_, _ = a.Store.SyncEpisodes(format.PID, refs)
}
