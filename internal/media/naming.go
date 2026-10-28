// Package media implements the Orangeflix output naming and the path
// sanitizing that keeps the API values inside the download directory.
//
// The default layout is the one of the JavaScript downloader, so that files
// that were downloaded before are recognised and skipped:
//
//	<Series>.S2026.German.AAC.DOKU.1080p.Web-DL.x264/
//	    <Series>.S2026E420.German.AAC.DOKU.1080p.Web-DL.x264/
//	        <Series>.S2026E420.German.AAC.DOKU.1080p.Web-DL.x264.mp4
package media

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Layout names the directory layout of the downloads.
type Layout int

// The supported layouts.
const (
	// LayoutCompat keeps the structure of the JavaScript downloader.
	LayoutCompat Layout = iota
	// LayoutFlat stores the episodes directly in the series folder.
	LayoutFlat
)

// ParseLayout converts a layout name into a Layout.
func ParseLayout(name string) (Layout, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "compat":
		return LayoutCompat, nil
	case "flat":
		return LayoutFlat, nil
	}
	return LayoutCompat, fmt.Errorf("unknown layout: %s (allowed: compat, flat)", name)
}

// Naming describes one episode and the names it gets on disk.
type Naming struct {
	// Title is the name of the format.
	Title string
	// Year is the year of the episode.
	Year int
	// Number is the episode number.
	Number int
	// Resolution is the resolution label of the file ("1080p").
	Resolution string
	// Layout is the directory layout.
	Layout Layout
}

// The rules of the validation. They are the ones of the JavaScript downloader:
// the year of a file name may not lie far in the future and the episode
// number has to stay small enough for the file name.
const (
	// MinYear is the earliest year a file name may carry.
	MinYear = 2000
	// MaxEpisodeDigits is the number of digits an episode number may have.
	MaxEpisodeDigits = 6
)

// ValidateTitle checks the title of a format.
func ValidateTitle(title string) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("ungültiger Serientitel: ein nicht leerer Text wird erwartet")
	}
	return nil
}

// ValidateYear checks the year of an episode. The year comes from the API and
// ends up inside a path segment.
func ValidateYear(year int) error {
	maxYear := time.Now().Year() + 1
	if year < MinYear || year > maxYear {
		return fmt.Errorf("ungültiges Jahr: %d (erwartet wird eine Jahreszahl zwischen %d und %d)", year, MinYear, maxYear)
	}
	return nil
}

// ValidateEpisodeNumber checks the number of an episode.
func ValidateEpisodeNumber(number int) error {
	if number < 1 {
		return fmt.Errorf("ungültige Episodennummer: %d (erwartet wird eine Zahl ab 1)", number)
	}
	digits := len(strconv.Itoa(number))
	if digits > MaxEpisodeDigits {
		return fmt.Errorf("ungültige Episodennummer: %d (erwartet werden höchstens %d Ziffern)", number, MaxEpisodeDigits)
	}
	return nil
}

// Validate checks the naming of an episode.
func (n Naming) Validate() error {
	if err := ValidateTitle(n.Title); err != nil {
		return err
	}
	if err := ValidateYear(n.Year); err != nil {
		return err
	}
	return ValidateEpisodeNumber(n.Number)
}

// Suffix returns the part that follows the title in the names of the
// compatible layout.
func (n Naming) Suffix() string {
	resolution := n.Resolution
	if resolution == "" {
		resolution = "1080p"
	}
	return fmt.Sprintf("German.AAC.DOKU.%s.Web-DL.x264", resolution)
}

// SeriesPrefix returns the name of the series folder; it is the part of the
// name that does not depend on the chosen quality.
func (n Naming) SeriesPrefix() string {
	return fmt.Sprintf("%s.S%d", SanitizeTitle(n.Title), n.Year)
}

// EpisodePrefix returns the name of the episode without the quality.
func (n Naming) EpisodePrefix() string {
	number := strconv.Itoa(n.Number)
	for len(number) < 3 {
		number = "0" + number
	}
	return fmt.Sprintf("%s.S%dE%s", SanitizeTitle(n.Title), n.Year, number)
}

// SeriesFolder returns the name of the folder that holds the episodes of the
// format and the year.
func (n Naming) SeriesFolder() string {
	if n.Layout == LayoutFlat {
		return n.SeriesPrefix()
	}
	return n.SeriesPrefix() + "." + n.Suffix()
}

// EpisodeName returns the name of the episode file without the extension.
// The flat layout takes only the prefix, the compatible layout appends the
// suffix.
func (n Naming) EpisodeName() string {
	if n.Layout == LayoutFlat {
		return n.EpisodePrefix()
	}
	return n.EpisodePrefix() + "." + n.Suffix()
}

// EpisodeFolder returns the name of the folder that holds the episode file.
// The compatible layout puts every episode into its own folder.
func (n Naming) EpisodeFolder() string {
	if n.Layout == LayoutFlat {
		return n.SeriesFolder()
	}
	return n.EpisodeName()
}

// RelativeDir returns the directory of the episode, relative to the download
// directory. The compatible layout puts every episode into its own folder.
func (n Naming) RelativeDir() string {
	if n.Layout == LayoutFlat {
		return n.SeriesFolder()
	}
	return n.SeriesFolder() + "/" + n.EpisodeFolder()
}

// RelativeFile returns the file of the episode, relative to the download
// directory.
func (n Naming) RelativeFile(extension string) string {
	if extension == "" {
		extension = ".mp4"
	}
	if !strings.HasPrefix(extension, ".") {
		extension = "." + extension
	}
	return n.RelativeDir() + "/" + n.EpisodeName() + extension
}

// Globs returns the patterns that match an already downloaded episode. The
// patterns end before the quality, so an episode counts as downloaded when
// it was fetched in another quality. Both layouts are checked, so switching
// the layout never triggers a second download.
func (n Naming) Globs() []string {
	globs := []string{}
	for _, layout := range []Layout{LayoutCompat, LayoutFlat} {
		candidate := n
		candidate.Layout = layout
		if layout == LayoutFlat {
			globs = append(globs, candidate.SeriesPrefix()+"/"+candidate.EpisodePrefix()+".*")
			continue
		}
		// The compatible layout keeps the episode in its own folder.
		globs = append(globs, candidate.SeriesPrefix()+".*/"+candidate.EpisodePrefix()+".*/"+candidate.EpisodePrefix()+".*")
	}
	return globs
}

// SanitizeTitle keeps the letters and digits of a title and replaces every
// other character with a dot. The result is a single path component that
// cannot escape its directory.
func SanitizeTitle(title string) string {
	var b strings.Builder
	for _, r := range title {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteRune('.')
		}
	}
	out := b.String()
	for strings.Contains(out, "..") {
		out = strings.ReplaceAll(out, "..", ".")
	}
	out = strings.Trim(out, ".")
	if out == "" {
		out = "_"
	}
	return CapComponentBytes(170, out)
}

// CapComponentBytes truncates s so its UTF-8 length stays within max bytes,
// always on a rune boundary.
func CapComponentBytes(max int, s string) string {
	if len(s) <= max {
		return s
	}
	var b strings.Builder
	b.Grow(max)
	runes := 0
	for _, r := range s {
		size := utf8.RuneLen(r)
		if size < 0 {
			size = 1
		}
		if b.Len()+size > max {
			break
		}
		b.WriteRune(r)
		runes++
	}
	return b.String()
}
