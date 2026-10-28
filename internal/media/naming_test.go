package media

import (
	"strings"
	"testing"
	"time"
)

func TestNamingCompatLayout(t *testing.T) {
	naming := Naming{
		Title:      "Die Mediatheke",
		Year:       2026,
		Number:     420,
		Resolution: "1080p",
		Layout:     LayoutCompat,
	}

	wantDir := "Die.Mediatheke.S2026.German.AAC.DOKU.1080p.Web-DL.x264/" +
		"Die.Mediatheke.S2026E420.German.AAC.DOKU.1080p.Web-DL.x264"
	if got := naming.RelativeDir(); got != wantDir {
		t.Errorf("RelativeDir = %q, want %q", got, wantDir)
	}

	wantFile := wantDir + "/Die.Mediatheke.S2026E420.German.AAC.DOKU.1080p.Web-DL.x264.mp4"
	if got := naming.RelativeFile(".mp4"); got != wantFile {
		t.Errorf("RelativeFile = %q, want %q", got, wantFile)
	}
}

func TestNamingFlatLayout(t *testing.T) {
	naming := Naming{
		Title:      "Die Mediatheke",
		Year:       2026,
		Number:     420,
		Resolution: "1080p",
		Layout:     LayoutFlat,
	}

	wantDir := "Die.Mediatheke.S2026"
	if got := naming.RelativeDir(); got != wantDir {
		t.Errorf("RelativeDir = %q, want %q", got, wantDir)
	}
	// The flat layout takes only the prefix.
	wantFile := wantDir + "/Die.Mediatheke.S2026E420.mp4"
	if got := naming.RelativeFile(".mp4"); got != wantFile {
		t.Errorf("RelativeFile = %q, want %q", got, wantFile)
	}
}

func TestNamingPadsTheEpisodeNumber(t *testing.T) {
	naming := Naming{Title: "Pantoffelkino", Year: 2026, Number: 7, Resolution: "1080p"}
	if !strings.Contains(naming.EpisodeName(), "S2026E007") {
		t.Errorf("EpisodeName = %q", naming.EpisodeName())
	}
}

func TestNamingGlobsCoverBothLayouts(t *testing.T) {
	naming := Naming{Title: "Die Mediatheke", Year: 2026, Number: 420}
	globs := naming.Globs()
	if len(globs) != 2 {
		t.Fatalf("got %d globs, want 2", len(globs))
	}
	joined := strings.Join(globs, "|")
	if !strings.Contains(joined, "Die.Mediatheke.S2026.*/Die.Mediatheke.S2026E420.*/Die.Mediatheke.S2026E420.*") {
		t.Errorf("the globs do not cover the compatible layout: %v", globs)
	}
	if !strings.Contains(joined, "Die.Mediatheke.S2026/Die.Mediatheke.S2026E420.*") {
		t.Errorf("the globs do not cover the flat layout: %v", globs)
	}
}

func TestSanitizeTitle(t *testing.T) {
	cases := map[string]string{
		"Die Mediatheke":   "Die.Mediatheke",
		"Olli vs James":    "Olli.vs.James",
		"Pressesch(l)au":   "Pressesch.l.au",
		"Youtube & Twitch": "Youtube.Twitch",
		"":                 "_",
		"../../etc/passwd": "etc.passwd",
		"MAXimal Musik":    "MAXimal.Musik",
		"Zurückgespult":    "Zurückgespult",
	}
	for in, want := range cases {
		if got := SanitizeTitle(in); got != want {
			t.Errorf("SanitizeTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitizeTitleKeepsThePathSafe(t *testing.T) {
	got := SanitizeTitle("../..")
	if strings.Contains(got, "/") || strings.Contains(got, "..") {
		t.Errorf("SanitizeTitle = %q, it can leave the directory", got)
	}
}

func TestParseLayout(t *testing.T) {
	if _, err := ParseLayout("compat"); err != nil {
		t.Errorf("compat: %v", err)
	}
	if _, err := ParseLayout("flat"); err != nil {
		t.Errorf("flat: %v", err)
	}
	if _, err := ParseLayout(""); err != nil {
		t.Errorf("the empty name: %v", err)
	}
	if _, err := ParseLayout("unsinn"); err == nil {
		t.Error("want an error for the unknown layout")
	}
}

func TestSuffixUsesTheDefaultTag(t *testing.T) {
	naming := Naming{Title: "X", Year: 2026, Number: 1, Resolution: "720p"}
	if got := naming.Suffix(); got != "German.AAC.DOKU.720p.Web-DL.x264" {
		t.Errorf("Suffix = %q", got)
	}
}

func TestValidateTitle(t *testing.T) {
	if err := ValidateTitle("Die Mediatheke"); err != nil {
		t.Errorf("ValidateTitle: %v", err)
	}
	if err := ValidateTitle("   "); err == nil {
		t.Error("want an error for the empty title")
	}
}

func TestValidateYear(t *testing.T) {
	if err := ValidateYear(2026); err != nil {
		t.Errorf("ValidateYear: %v", err)
	}
	if err := ValidateYear(1999); err == nil {
		t.Error("want an error for the year before 2000")
	}
	if err := ValidateYear(time.Now().Year() + 2); err == nil {
		t.Error("want an error for the year in the far future")
	}
}

func TestValidateEpisodeNumber(t *testing.T) {
	if err := ValidateEpisodeNumber(420); err != nil {
		t.Errorf("ValidateEpisodeNumber: %v", err)
	}
	if err := ValidateEpisodeNumber(0); err == nil {
		t.Error("want an error for the number zero")
	}
	if err := ValidateEpisodeNumber(1234567); err == nil {
		t.Error("want an error for the long number")
	}
}

func TestNamingValidate(t *testing.T) {
	naming := Naming{Title: "Die Mediatheke", Year: 2026, Number: 420}
	if err := naming.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
	naming.Year = 1990
	if err := naming.Validate(); err == nil {
		t.Error("want an error for the old year")
	}
}
