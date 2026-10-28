package download

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/revunix/orangeflix-dl/internal/media"
)

// Manager decides what has to be downloaded and where the file ends up.
type Manager struct {
	// DownloadsDir is the root of the downloads.
	DownloadsDir string
	// Force removes existing files instead of skipping them.
	Force bool

	Logf func(format string, args ...any)
}

func (m *Manager) logf(format string, args ...any) {
	if m.Logf != nil {
		m.Logf(format, args...)
	}
}

// Target returns the absolute path of the episode file.
func (m *Manager) Target(naming media.Naming, extension string) string {
	return filepath.Join(m.DownloadsDir, filepath.FromSlash(naming.RelativeFile(extension)))
}

// Existing returns the file of an already downloaded episode, or an empty
// string when the episode has to be downloaded.
func (m *Manager) Existing(naming media.Naming) string {
	for _, pattern := range naming.Globs() {
		matches, err := filepath.Glob(filepath.Join(m.DownloadsDir, filepath.FromSlash(pattern)))
		if err != nil {
			continue
		}
		for _, match := range matches {
			// The partial file of an interrupted download is not a finished
			// episode: it belongs to the next run.
			if strings.HasSuffix(match, PartSuffix) {
				continue
			}
			info, err := os.Stat(match)
			if err == nil && info.Mode().IsRegular() && info.Size() > 0 {
				return match
			}
		}
	}
	return ""
}

// Prepare decides whether the episode has to be downloaded. It returns the
// file of the episode when it is already there and must be skipped, and an
// empty string when it has to be downloaded.
func (m *Manager) Prepare(naming media.Naming) (string, error) {
	if !m.Force {
		return m.Existing(naming), nil
	}
	for _, pattern := range naming.Globs() {
		matches, err := filepath.Glob(filepath.Join(m.DownloadsDir, filepath.FromSlash(pattern)))
		if err != nil {
			continue
		}
		for _, match := range matches {
			if strings.HasSuffix(match, PartSuffix) {
				// The partial file is removed with the other leftovers.
				continue
			}
			if info, err := os.Stat(match); err == nil && info.Mode().IsRegular() {
				m.remove(match)
			}
		}
	}
	// A forced download starts from the beginning, so the partial file of an
	// interrupted run is removed as well.
	for _, pattern := range m.partGlobs(naming) {
		for _, match := range m.glob(pattern) {
			m.remove(match)
		}
	}
	return "", nil
}

// remove deletes a leftover of a forced download.
func (m *Manager) remove(path string) {
	m.logf("Erneuter Download, entferne: %s", path)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		m.logf("Die Datei konnte nicht entfernt werden: %s", err.Error())
	}
}

// partGlobs returns the patterns of the partial files of an episode.
func (m *Manager) partGlobs(naming media.Naming) []string {
	globs := []string{}
	for _, layout := range []media.Layout{media.LayoutCompat, media.LayoutFlat} {
		candidate := naming
		candidate.Layout = layout
		if layout == media.LayoutFlat {
			globs = append(globs, candidate.SeriesPrefix()+"/"+candidate.EpisodePrefix()+".*"+PartSuffix)
			continue
		}
		globs = append(globs, candidate.SeriesPrefix()+".*/"+candidate.EpisodePrefix()+".*/"+candidate.EpisodePrefix()+".*"+PartSuffix)
	}
	return globs
}

// glob returns the files that match the pattern, relative to the download
// directory.
func (m *Manager) glob(pattern string) []string {
	if pattern == "" {
		return nil
	}
	matches, err := filepath.Glob(filepath.Join(m.DownloadsDir, filepath.FromSlash(pattern)))
	if err != nil {
		return nil
	}
	return matches
}

// EnsureDir creates the directory of the target file.
func (m *Manager) EnsureDir(naming media.Naming) error {
	dir := filepath.Dir(m.Target(naming, ".mp4"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("the directory is not writable: %s", dir)
	}
	return nil
}
