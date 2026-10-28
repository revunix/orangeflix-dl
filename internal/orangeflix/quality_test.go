package orangeflix

import "testing"

func TestSelectFilePicksTheExactQuality(t *testing.T) {
	files := []File{
		{URL: "a.mp4", Desc: "HD 1080p", Dimensions: "1920x1080", Kind: "film"},
		{URL: "b.mp4", Desc: "HD 720p", Dimensions: "1280x720", Kind: "film"},
		{URL: "c.mp4", Desc: "SD 432p", Dimensions: "768x432", Kind: "film"},
		{URL: "d.m4a", Desc: "Audio m4a", Kind: "music"},
	}
	cases := map[string]string{
		QualityHD:     "HD 1080p",
		QualityMobile: "HD 720p",
		QualitySD:     "SD 432p",
		QualityAudio:  "Audio m4a",
	}
	for quality, want := range cases {
		file, exact, err := SelectFile(files, quality)
		if err != nil {
			t.Errorf("SelectFile(%q): %v", quality, err)
			continue
		}
		if !exact {
			t.Errorf("SelectFile(%q) did not find the quality", quality)
		}
		if file.Desc != want {
			t.Errorf("SelectFile(%q) = %q, want %q", quality, file.Desc, want)
		}
	}
}

func TestSelectFilePicksTheHighestForBest(t *testing.T) {
	files := []File{
		{URL: "a.mp4", Desc: "SD 432p", Dimensions: "768x432"},
		{URL: "b.mp4", Desc: "HD 1080p", Dimensions: "1920x1080"},
		{URL: "c.m4a", Desc: "Audio m4a", Kind: "music"},
	}
	file, exact, err := SelectFile(files, QualityBest)
	if err != nil {
		t.Fatalf("SelectFile: %v", err)
	}
	if !exact {
		t.Error("best did not find a file")
	}
	if file.Desc != "HD 1080p" {
		t.Errorf("file = %q", file.Desc)
	}
}

func TestSelectFileFallsBack(t *testing.T) {
	files := []File{
		{URL: "a.mp4", Desc: "SD 432p", Dimensions: "768x432"},
	}
	file, exact, err := SelectFile(files, QualityHD)
	if err != nil {
		t.Fatalf("SelectFile: %v", err)
	}
	if exact {
		t.Error("the missing quality was reported as available")
	}
	if file.Desc != "SD 432p" {
		t.Errorf("file = %q", file.Desc)
	}
}

func TestSelectFileWithoutFiles(t *testing.T) {
	if _, _, err := SelectFile(nil, QualityBest); err == nil {
		t.Fatal("want an error for the empty list")
	}
}

func TestResolutionLabel(t *testing.T) {
	cases := map[string]string{
		"1920x1080": "1080p",
		"1280x720":  "720p",
		"768x432":   "432p",
		"":          "Unknown",
	}
	for dimensions, want := range cases {
		file := File{Dimensions: dimensions}
		if got := ResolutionLabel(file); got != want {
			t.Errorf("ResolutionLabel(%q) = %q, want %q", dimensions, got, want)
		}
	}
}

func TestExtension(t *testing.T) {
	cases := []struct {
		file File
		want string
	}{
		{File{URL: "x/mt420FULLHD.mp4", Desc: "HD 1080p"}, ".mp4"},
		{File{URL: "x/mt420.m4a", Desc: "Audio m4a", Kind: "music"}, ".m4a"},
		{File{URL: "x/mt420.mp3", Desc: "Audio mp3", Kind: "music"}, ".mp3"},
		{File{URL: "x/mt420", Desc: "HD 1080p"}, ".mp4"},
	}
	for _, testCase := range cases {
		if got := Extension(testCase.file); got != testCase.want {
			t.Errorf("Extension(%q) = %q, want %q", testCase.file.URL, got, testCase.want)
		}
	}
}

func TestValidQuality(t *testing.T) {
	for _, quality := range Qualities {
		if !ValidQuality(quality) {
			t.Errorf("the quality %q was rejected", quality)
		}
	}
	if ValidQuality("fhd") {
		t.Error("the unknown quality was accepted")
	}
}

func TestFormatsByStatus(t *testing.T) {
	order, grouped := FormatsByStatus()
	if len(order) != 3 {
		t.Fatalf("got %d groups, want 3", len(order))
	}
	total := 0
	for _, status := range order {
		total += len(grouped[status])
	}
	if total != len(Catalog) {
		t.Errorf("the groups hold %d formats, the catalog has %d", total, len(Catalog))
	}
	if len(grouped[StatusActive]) == 0 {
		t.Error("no active format")
	}
}

func TestResolutionRank(t *testing.T) {
	cases := map[string]int{
		"HD 1080p":  1080,
		"HD 720p":   720,
		"SD 576p":   576,
		"SD 432p":   432,
		"Audio m4a": 0,
		"":          0,
	}
	for desc, want := range cases {
		if got := resolutionRank(desc); got != want {
			t.Errorf("resolutionRank(%q) = %d, want %d", desc, got, want)
		}
	}
}

func TestResolutionLabelWithoutDimensions(t *testing.T) {
	// Some clips report the quality but no pixel size.
	file := File{Desc: "HD 720p"}
	if got := ResolutionLabel(file); got != "720p" {
		t.Errorf("ResolutionLabel = %q, want 720p", got)
	}
}

func TestSelectFileWithoutDimensions(t *testing.T) {
	files := []File{
		{URL: "a.mp4", Desc: "HD 720p"},
		{URL: "b.mp4", Desc: "SD 576p"},
	}
	file, exact, err := SelectFile(files, QualityBest)
	if err != nil {
		t.Fatalf("SelectFile: %v", err)
	}
	if !exact {
		t.Error("best did not find a file")
	}
	if file.Desc != "HD 720p" {
		t.Errorf("file = %q, want the higher resolution", file.Desc)
	}
}

func TestSelectFilePicksTheLowestForSD(t *testing.T) {
	files := []File{
		{URL: "a.mp4", Desc: "HD 720p"},
		{URL: "b.mp4", Desc: "SD 576p"},
	}
	file, exact, err := SelectFile(files, QualitySD)
	if err != nil {
		t.Fatalf("SelectFile: %v", err)
	}
	if exact {
		t.Error("the missing quality was reported as available")
	}
	if file.Desc != "SD 576p" {
		t.Errorf("file = %q, want the lower resolution", file.Desc)
	}
}

func TestResolutionLabelOfAnAudioFile(t *testing.T) {
	file := File{URL: "x.m4a", Desc: "Audio m4a", Kind: "music"}
	if got := ResolutionLabel(file); got != "Audio" {
		t.Errorf("ResolutionLabel = %q, want Audio", got)
	}
}
