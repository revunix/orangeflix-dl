// Package orangeflix is the client for the Orangeflix legacy API that is
// served under the old massengeschmack.tv host: the episode feed, the clip
// metadata with the download files and the RSS feeds.
package orangeflix

import (
	"strconv"
	"strings"
	"time"
)

// Format status values.
const (
	StatusActive   = "active"
	StatusSeasonal = "seasonal"
	StatusArchived = "archived"
)

// Episode is one entry of a format feed.
type Episode struct {
	// Identifier is the stable id of the episode ("mt420"). It is the key of
	// the episode in the library and the parameter of the clip request.
	Identifier string
	// PID is the format the episode belongs to.
	PID int
	// Number is the episode number within the format ("enum").
	Number int
	// Title is the episode title ("Folge 420").
	Title string
	// FormatName is the name of the format ("Die Mediatheke").
	FormatName string
	// Description is the long text of the episode.
	Description string
	// Duration is the running time as the API reports it ("00:31:52").
	Duration string
	// Date is the publication date.
	Date time.Time
	// Thumb is the thumbnail URL.
	Thumb string
	// Participants are the guests of the format.
	Participants []Participant
	// DurationSeconds is the running time in seconds.
	DurationSeconds int
	// CategoryTitle is the category of the episode.
	CategoryTitle string
	// ProgressPercent is the share of the episode that was watched.
	ProgressPercent int
	// HasDownload tells whether the episode can be downloaded.
	HasDownload bool
}

// Participant is a guest of a format.
type Participant struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Image       string `json:"image"`
}

// File is one downloadable file of a clip.
type File struct {
	// URL is the download URL. It carries a token and is only valid for a
	// short time, so it is resolved right before the download.
	URL string
	// Desc is the quality description ("HD 1080p", "Audio m4a").
	Desc string
	// Size is the file size in bytes.
	Size int64
	// Dimensions is the pixel size ("1920x1080").
	Dimensions string
	// Kind is the file type ("film", "music").
	Kind string
}

// Height returns the vertical resolution of the file in pixels, 0 when the
// dimensions are unknown.
func (f File) Height() int {
	width, height, ok := strings.Cut(f.Dimensions, "x")
	if !ok {
		return 0
	}
	_ = width
	value, err := strconv.Atoi(strings.TrimSpace(height))
	if err != nil {
		return 0
	}
	return value
}

// Clip is the metadata of one episode together with its files.
type Clip struct {
	Episode
	// Files are the available qualities of the episode.
	Files []File
}

// Format is one show of the catalog.
type Format struct {
	// PID is the numeric id the API expects in the "from" parameter.
	PID int
	// Slug is the short name used on the command line ("mediatheke").
	Slug string
	// Title is the display name ("Die Mediatheke").
	Title string
	// Status is one of active, seasonal or archived.
	Status string
	// ClipCount is the number of the episodes of the format.
	ClipCount int
	// Logo is the URL of the artwork of the format.
	Logo string
	// Original tells whether the format is an own production.
	Original bool
	// Handle is the short name of the format ("PK").
	Handle string
	// LastEpisode is the air date of the most recent episode.
	LastEpisode string
	// LastEpisodeTime is the date of the most recent episode. The list is
	// sorted by it, the newest format comes first.
	LastEpisodeTime time.Time
	// NewEpisode tells that a new episode came out recently. The age of
	// the episode is measured against the current date.
	NewEpisode bool
	// Opened tells that the user has already been in this format.
	Opened bool
	// Description is a one line description of the format.
	Description string
}

// StatusLabel returns the German label of the format status.
func (f Format) StatusLabel() string {
	switch f.Status {
	case StatusActive:
		return "aktiv"
	case StatusSeasonal:
		return "saisonal"
	case StatusArchived:
		return "archiviert"
	}
	return f.Status
}
