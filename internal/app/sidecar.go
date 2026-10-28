package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// saveSidecars stores the requested extra files next to a freshly
// downloaded video: the thumbnail as a JPEG and the metadata of the episode
// as JSON.
func (a *App) saveSidecars(ctx context.Context, episode orangeflix.Episode, videoPath string) {
	base := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	if a.Config != nil && a.Config.Thumb {
		a.saveThumbnail(ctx, episode, base+".jpg")
	}
	if a.Config != nil && a.Config.Meta {
		a.saveMeta(episode, videoPath, base+".json")
	}
}

// saveThumbnailSidecar fetches the thumbnail of an episode that was
// downloaded before the feature existed.
func (a *App) saveThumbnailSidecar(ctx context.Context, episode orangeflix.Episode, videoPath string) {
	if a.Config == nil || !a.Config.Thumb {
		return
	}
	base := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	a.saveThumbnail(ctx, episode, base+".jpg")
}

// saveMetaSidecar writes the metadata of an episode that was downloaded
// before the feature existed.
func (a *App) saveMetaSidecar(episode orangeflix.Episode, videoPath string) {
	if a.Config == nil || !a.Config.Meta {
		return
	}
	base := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	a.saveMeta(episode, videoPath, base+".json")
}

// saveThumbnail fetches the preview image of the episode when it is not
// there yet.
func (a *App) saveThumbnail(ctx context.Context, episode orangeflix.Episode, target string) {
	if episode.Thumb == "" {
		a.logf("Kein Vorschaubild für diese Folge verfügbar.")
		return
	}
	if _, err := os.Stat(target); err == nil {
		a.logf("Vorschaubild vorhanden: %s", target)
		return
	}
	if _, err := a.Downloader.Download(ctx, episode.Thumb, target, nil, nil); err != nil {
		a.logf("Das Vorschaubild konnte nicht gespeichert werden: %s", err.Error())
		return
	}
	a.logf("Vorschaubild: %s", target)
}

// sidecarMeta is the content of the JSON file of an episode. The keys are
// stable, so other tools can read it.
type sidecarMeta struct {
	Schema       string   `json:"schema"`
	Identifier   string   `json:"identifier"`
	PID          int      `json:"pid"`
	Number       int      `json:"episode"`
	Title        string   `json:"title"`
	Format       string   `json:"format"`
	Description  string   `json:"description,omitempty"`
	Date         string   `json:"date,omitempty"`
	Duration     string   `json:"duration,omitempty"`
	DurationSec  int      `json:"durationSeconds,omitempty"`
	Thumb        string   `json:"thumbURL,omitempty"`
	Subtitle     string   `json:"subtitle,omitempty"`
	Participants []string `json:"participants,omitempty"`
}

// saveMeta writes the metadata of the episode next to the video file. The
// name of the subtitle file is filled in when it lies there.
func (a *App) saveMeta(episode orangeflix.Episode, videoPath, target string) {
	meta := sidecarMeta{
		Schema:      "orangeflix-dl/episode/1",
		Identifier:  episode.Identifier,
		PID:         episode.PID,
		Number:      episode.Number,
		Title:       episode.Title,
		Format:      episode.FormatName,
		Description: episode.Description,
		Duration:    episode.Duration,
		DurationSec: episode.DurationSeconds,
		Thumb:       episode.Thumb,
	}
	if !episode.Date.IsZero() {
		meta.Date = episode.Date.Format("2006-01-02")
	}
	if subtitlePresent(videoPath) {
		meta.Subtitle = subtitleName(videoPath, 0)
	}
	for _, participant := range episode.Participants {
		meta.Participants = append(meta.Participants, participant.Name)
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		a.logf("Die Metadaten konnten nicht erzeugt werden: %s", err.Error())
		return
	}
	data = append(data, '\n')
	if err := os.WriteFile(target, data, 0o644); err != nil {
		a.logf("Die Metadaten konnten nicht geschrieben werden: %s", err.Error())
		return
	}
	a.logf("Metadaten: %s", target)
}
