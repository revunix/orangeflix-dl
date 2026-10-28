package orangeflix

import (
	"fmt"
	"path"
	"strconv"
	"strings"
)

// Quality names the download quality.
const (
	QualityBest   = "best"
	QualityHD     = "hd"
	QualitySD     = "sd"
	QualityMobile = "mobile"
	QualityAudio  = "audio"
)

// Qualities lists the accepted quality names.
var Qualities = []string{QualityBest, QualityHD, QualitySD, QualityMobile, QualityAudio}

// ValidQuality reports whether the name is an accepted quality.
func ValidQuality(name string) bool {
	for _, quality := range Qualities {
		if quality == name {
			return true
		}
	}
	return false
}

// resolutionRank maps a quality description to a comparable number. Higher is
// better. The first number of the description is used, so "HD 720p" ranks
// as 720 and "SD 576p" as 576. Audio files rank lowest.
func resolutionRank(desc string) int {
	// An audio track has no resolution: the number of its description would
	// be the format of the file, not its height.
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(desc)), "audio") {
		return 0
	}
	digits := ""
	for _, r := range desc {
		switch {
		case r >= '0' && r <= '9':
			digits += string(r)
		case digits != "":
			// The number ends at the first character that is not a digit.
			value, err := strconv.Atoi(digits)
			if err != nil {
				return 0
			}
			return value
		}
	}
	if digits == "" {
		return 0
	}
	value, err := strconv.Atoi(digits)
	if err != nil {
		return 0
	}
	return value
}

// isAudio reports whether the file is an audio track.
func isAudio(file File) bool {
	if file.Kind == "music" {
		return true
	}
	return strings.HasPrefix(strings.ToLower(file.Desc), "audio")
}

// SelectFile picks the file for the requested quality. The second return
// value reports whether a file of exactly that quality exists.
func SelectFile(files []File, quality string) (File, bool, error) {
	if len(files) == 0 {
		return File{}, false, fmt.Errorf("no download files available")
	}
	wanted := strings.ToLower(strings.TrimSpace(quality))
	if wanted == "" {
		wanted = QualityBest
	}

	if wanted == QualityBest {
		best, ok := bestVideo(files)
		if !ok {
			return File{}, false, fmt.Errorf("no video file available")
		}
		return best, true, nil
	}

	// The exact descriptions the API reports.
	switch wanted {
	case QualityHD:
		if file, ok := findByDesc(files, "HD 1080p"); ok {
			return file, true, nil
		}
	case QualitySD:
		if file, ok := findByDesc(files, "SD 432p"); ok {
			return file, true, nil
		}
		// The format may offer a different low resolution.
		if file, ok := lowestVideo(files); ok {
			return file, false, nil
		}
	case QualityMobile:
		if file, ok := findByDesc(files, "HD 720p"); ok {
			return file, true, nil
		}
		if file, ok := lowestVideo(files); ok {
			return file, false, nil
		}
	case QualityAudio:
		if file, ok := findByDesc(files, "Audio m4a"); ok {
			return file, true, nil
		}
		if file, ok := findByDesc(files, "Audio mp3"); ok {
			return file, true, nil
		}
	}

	// The requested quality is not available: fall back to the closest one
	// and let the caller report the deviation.
	fallback, ok := closest(files, wanted)
	if !ok {
		return File{}, false, fmt.Errorf("no file for the quality %q available", wanted)
	}
	return fallback, false, nil
}

// findByDesc returns the first file with the given description.
func findByDesc(files []File, desc string) (File, bool) {
	for _, file := range files {
		if file.Desc == desc && file.URL != "" {
			return file, true
		}
	}
	return File{}, false
}

// bestVideo returns the file with the highest resolution.
func bestVideo(files []File) (File, bool) {
	var best File
	found := false
	for _, file := range files {
		if file.URL == "" || isAudio(file) {
			continue
		}
		if !found || fileRank(file) > fileRank(best) {
			best = file
			found = true
		}
	}
	return best, found
}

// lowestVideo returns the file with the lowest resolution.
func lowestVideo(files []File) (File, bool) {
	var lowest File
	found := false
	for _, file := range files {
		if file.URL == "" || isAudio(file) {
			continue
		}
		rank := fileRank(file)
		if rank == 0 {
			continue
		}
		if !found || rank < fileRank(lowest) {
			lowest = file
			found = true
		}
	}
	return lowest, found
}

// fileRank is the comparable number of a file: the vertical resolution.
func fileRank(file File) int {
	if height := file.Height(); height > 0 {
		return height
	}
	return resolutionRank(file.Desc)
}

// closest returns the file that fits the requested quality best.
func closest(files []File, quality string) (File, bool) {
	if quality == QualityAudio {
		for _, file := range files {
			if isAudio(file) && file.URL != "" {
				return file, true
			}
		}
		return File{}, false
	}
	best, ok := bestVideo(files)
	if !ok {
		return File{}, false
	}
	return best, true
}

// ResolutionLabel returns the resolution used in the file name, for example
// "1080p". An audio track has no resolution, it is marked as audio.
func ResolutionLabel(file File) string {
	if isAudio(file) {
		return "Audio"
	}
	height := file.Height()
	if height == 0 {
		height = resolutionRank(file.Desc)
	}
	switch {
	case height >= 2160:
		return "2160p"
	case height >= 1440:
		return "1440p"
	case height >= 1080:
		return "1080p"
	case height >= 720:
		return "720p"
	case height >= 576:
		return "576p"
	case height >= 480:
		return "480p"
	case height > 0:
		return fmt.Sprintf("%dp", height)
	}
	return "Unknown"
}

// Extension returns the file extension of the download, including the dot.
func Extension(file File) string {
	switch {
	case isAudio(file):
		switch strings.ToLower(path.Ext(file.URL)) {
		case ".m4a":
			return ".m4a"
		case ".mp3":
			return ".mp3"
		}
		return ".m4a"
	default:
		if ext := strings.ToLower(path.Ext(file.URL)); ext == ".mp4" || ext == ".mkv" || ext == ".webm" {
			return ext
		}
		return ".mp4"
	}
}
