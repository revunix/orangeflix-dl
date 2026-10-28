package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/revunix/orangeflix-dl/internal/download"
	"github.com/revunix/orangeflix-dl/internal/media"
	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// downloadEpisode loads one episode: it resolves the download file of the
// requested quality, downloads it and records the result in the library.
func (a *App) downloadEpisode(ctx context.Context, format orangeflix.Format, episode orangeflix.Episode) error {
	naming := a.namingFor(format, episode)

	existing, err := a.Manager.Prepare(naming)
	if err != nil {
		return fmt.Errorf("die Datei konnte nicht vorbereitet werden: %w", err)
	}
	if existing != "" {
		// The resolution is not known before the clip was loaded, so the
		// message names the episode without the media tag.
		a.logf("Übersprungen (bereits vorhanden): %s", naming.EpisodePrefix())
		a.event(Event{Kind: EventSkipped, Label: naming.EpisodePrefix(), Path: existing, EpisodeID: episode.Identifier, PID: format.PID})
		a.markDownloaded(format.PID, episode.Identifier, existing)
		a.loadMissingSubtitles(ctx, episode, existing)
		a.saveThumbnailSidecar(ctx, episode, existing)
		a.saveMetaSidecar(episode, existing)
		return nil
	}

	files, subtitles, creds, err := a.episodeFiles(ctx, episode)
	if err != nil {
		return fmt.Errorf("die Metadaten von %s konnten nicht geladen werden: %w", episode.Identifier, err)
	}
	file, exact, err := orangeflix.SelectFile(files, a.quality())
	if err != nil {
		return fmt.Errorf("für %s gibt es keine Datei: %w", episode.Identifier, err)
	}
	if !exact {
		a.logf("Qualität %s ist nicht verfügbar, nehme %s.", a.quality(), file.Desc)
	}
	// The URL of the file carries a token, so it is resolved right before the
	// download and never kept.
	rawURL, err := a.API.ToHTTPS(file.URL)
	if err != nil {
		return fmt.Errorf("ungültige Download-URL für %s: %w", episode.Identifier, err)
	}

	naming.Resolution = orangeflix.ResolutionLabel(file)
	extension := orangeflix.Extension(file)
	if err := a.Manager.EnsureDir(naming); err != nil {
		return err
	}
	target := a.Manager.Target(naming, extension)

	a.event(Event{Kind: EventQueued, Label: naming.EpisodeName(), Path: target, EpisodeID: episode.Identifier, PID: format.PID})
	a.logf("Lade %s ...", naming.EpisodeName())

	result, err := a.Downloader.Download(ctx, rawURL, target, creds, func(done, total int64) {
		a.event(Event{Kind: EventProgress, Label: naming.EpisodeName(), Done: done, Total: total, EpisodeID: episode.Identifier, PID: format.PID})
	})
	if err != nil {
		// The partial file stays, so the next run continues it. The
		// downloader removes it itself when it does not fit to the file on
		// the server.
		if ctx.Err() != nil {
			a.logf("Abgebrochen, die Teildatei bleibt für den nächsten Lauf erhalten.")
		}
		return fmt.Errorf("der Download von %s ist fehlgeschlagen: %w", episode.Identifier, err)
	}
	a.logf("Fertig: %s (%s)", result.Path, humanSize(result.Size))
	a.event(Event{Kind: EventDone, Label: naming.EpisodeName(), Path: result.Path, EpisodeID: episode.Identifier, PID: format.PID})
	a.markDownloaded(format.PID, episode.Identifier, result.Path)
	a.saveSubtitles(ctx, subtitles, result.Path)
	a.saveSidecars(ctx, episode, result.Path)
	return nil
}

// episodeFiles returns the download files and the subtitles of an episode.
// The version 3 API is preferred: its files lie on the delivery network and
// work without credentials. The legacy API is the fallback.
func (a *App) episodeFiles(ctx context.Context, episode orangeflix.Episode) ([]orangeflix.File, []string, *download.Credentials, error) {
	if a.V3 != nil {
		if err := a.EnsureV3(ctx); err == nil {
			playback, err := a.V3.Client.Playback(ctx, episode.Identifier)
			if err == nil && len(playback.Stream.Media.Files) > 0 {
				// The files of the delivery network carry no credentials.
				return playback.Stream.Media.Files, subtitleURLs(playback), nil, nil
			}
			if err != nil {
				a.logf("Die Version-3-API liefert keine Datei für %s (%s), nutze die alte API.", episode.Identifier, err.Error())
			}
		}
	}

	clip, err := a.API.Clip(ctx, episode.Identifier)
	if err != nil {
		return nil, nil, nil, err
	}
	creds := &download.Credentials{Username: a.Credentials.Username, Password: a.Credentials.Password}
	return clip.Files, nil, creds, nil
}

// subtitleURLs returns the addresses of the subtitles of a clip.
func subtitleURLs(playback *orangeflix.V3Playback) []string {
	subtitles := make([]string, 0, len(playback.Clip.Subtitles))
	for _, subtitle := range playback.Clip.Subtitles {
		if subtitle.URL != "" {
			subtitles = append(subtitles, subtitle.URL)
		}
	}
	return subtitles
}

// loadMissingSubtitles fetches the subtitles of an episode that was
// downloaded before the program stored them. Only the version 3 API knows the
// subtitles, so nothing happens without it.
func (a *App) loadMissingSubtitles(ctx context.Context, episode orangeflix.Episode, videoPath string) {
	if a.V3 == nil || subtitlePresent(videoPath) {
		return
	}
	if err := a.EnsureV3(ctx); err != nil {
		return
	}
	playback, err := a.V3.Client.Playback(ctx, episode.Identifier)
	if err != nil {
		a.logf("Die Untertitel von %s konnten nicht abgefragt werden: %s", episode.Identifier, err.Error())
		return
	}
	a.saveSubtitles(ctx, subtitleURLs(playback), videoPath)
}

// subtitleName returns the file of the subtitle with the index.
func subtitleName(videoPath string, index int) string {
	name := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	if index > 0 {
		name += fmt.Sprintf(".%d", index)
	}
	return name + ".de.vtt"
}

// subtitlePresent reports whether the first subtitle lies next to the video.
func subtitlePresent(videoPath string) bool {
	_, err := os.Stat(subtitleName(videoPath, 0))
	return err == nil
}

// saveSubtitles stores the subtitles of an episode next to the video file.
func (a *App) saveSubtitles(ctx context.Context, subtitles []string, videoPath string) {
	for index, url := range subtitles {
		target := subtitleName(videoPath, index)
		if _, err := a.Downloader.Download(ctx, url, target, nil, nil); err != nil {
			a.logf("Die Untertitel konnten nicht gespeichert werden: %s", err.Error())
			continue
		}
		a.logf("Untertitel: %s", target)
	}
}

// namingFor builds the file names of an episode. The resolution is not known
// before the clip was loaded, so it is filled in later.
func (a *App) namingFor(format orangeflix.Format, episode orangeflix.Episode) media.Naming {
	title := format.Title
	if title == "" {
		title = episode.FormatName
	}
	if title == "" {
		title = format.Slug
	}
	year := 0
	if !episode.Date.IsZero() {
		year = episode.Date.Year()
	}
	return media.Naming{
		Title:  title,
		Year:   year,
		Number: episode.Number,
		Layout: a.Layout,
	}
}

// ClipInfo returns the files of an episode: the available qualities with
// their size.
func (a *App) ClipInfo(ctx context.Context, identifier string) ([]orangeflix.File, error) {
	clip, err := a.API.Clip(ctx, identifier)
	if err != nil {
		return nil, err
	}
	return clip.Files, nil
}

// quality returns the requested download quality.
func (a *App) quality() string {
	if a.Config == nil || a.Config.Quality == "" {
		return orangeflix.QualityBest
	}
	return strings.ToLower(a.Config.Quality)
}

// markDownloaded records a downloaded episode in the library.
func (a *App) markDownloaded(pid int, identifier, path string) {
	if a.Store == nil {
		return
	}
	if err := a.Store.MarkDownloaded(pid, identifier, path); err != nil {
		a.logf("Der Download konnte nicht in der Bibliothek vermerkt werden: %s", err.Error())
	}
}

// humanSize formats a number of bytes.
func humanSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	for _, name := range units {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, name)
		}
	}
	return fmt.Sprintf("%.1f PiB", value)
}
