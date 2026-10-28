package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/revunix/orangeflix-dl/internal/config"
	"github.com/revunix/orangeflix-dl/internal/download"
	"github.com/revunix/orangeflix-dl/internal/media"
	"github.com/revunix/orangeflix-dl/internal/orangeflix"
	"github.com/revunix/orangeflix-dl/internal/store"
)

// testApp builds an application that talks to the test server.
func testApp(t *testing.T, handler http.HandlerFunc) (*App, *store.Store, string) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)

	dir := t.TempDir()
	state, err := store.Open(filepath.Join(dir, "library.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { state.Close() })

	client := &orangeflix.Client{
		HTTP:     server.Client(),
		BaseURL:  server.URL + "/api/v1/",
		FeedURL:  server.URL + "/feed/",
		Username: "user",
		Password: "secret",
		Attempts: 1,
		Sleep:    func(context.Context, time.Duration) {},
	}
	application := &App{
		Config:  &config.Options{Format: "mediatheke", Episode: "420", Quality: orangeflix.QualityHD, Submodule: 1},
		API:     client,
		Manager: &download.Manager{DownloadsDir: filepath.Join(dir, "downloads")},
		Store:   state,
		Layout:  media.LayoutCompat,
	}
	application.Downloader = &download.Downloader{HTTP: server.Client()}
	application.Credentials = download.Credentials{Username: "user", Password: "secret"}
	return application, state, filepath.Join(dir, "downloads")
}

// handler answers the feed, the clip and the file.
func handler(t *testing.T, content []byte) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/api/v1/"):
			switch r.URL.Query().Get("action") {
			case "getFeed":
				_, _ = w.Write([]byte(`{"eps":[{"identifier":"mt420","pid":23,"title":"Folge 420",
					"pdesc":"Die Mediatheke","enum":420,"date":1791602710}]}`))
			case "getClip":
				_, _ = w.Write([]byte(`{"identifier":"mt420","pid":23,"enum":420,"files":[
					{"url":"` + "https://" + r.Host + `/file.mp4","desc":"HD 1080p","size":10,"dimensions":"1920x1080"}]}`))
			}
		case r.URL.Path == "/feed/23-1/hd.xml":
			_, _ = w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel>
				<title>Die Mediatheke</title>
				<item><guid>mt420</guid><title>Folge 420</title>
				<pubDate>Sat, 10 Oct 2026 05:25:10 +0200</pubDate>
				<enclosure url="http://example.org/file.mp4" length="10" type="video/mp4" />
				</item></channel></rss>`))
		default:
			w.Header().Set("Content-Length", "10")
			_, _ = w.Write(content)
		}
	}
}

func TestSetDownloadsDirUpdatesConfigAndManager(t *testing.T) {
	opts := &config.Options{DownloadsDir: "/tmp/alt"}
	manager := &download.Manager{DownloadsDir: "/tmp/alt"}
	application := &App{Config: opts, Manager: manager}

	application.SetDownloadsDir("/tmp/neu")
	if opts.DownloadsDir != "/tmp/neu" {
		t.Errorf("config = %q", opts.DownloadsDir)
	}
	if manager.DownloadsDir != "/tmp/neu" {
		t.Errorf("manager = %q", manager.DownloadsDir)
	}
}

func TestRunDownloadsTheEpisode(t *testing.T) {
	application, state, downloads := testApp(t, handler(t, []byte("0123456789")))

	if err := application.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := filepath.Join(downloads,
		"Die.Mediatheke.S2026.German.AAC.DOKU.1080p.Web-DL.x264",
		"Die.Mediatheke.S2026E420.German.AAC.DOKU.1080p.Web-DL.x264",
		"Die.Mediatheke.S2026E420.German.AAC.DOKU.1080p.Web-DL.x264.mp4")
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("the file is missing: %v", err)
	}
	if string(data) != "0123456789" {
		t.Errorf("content = %q", string(data))
	}

	// The format is in the library and the episode is marked as downloaded.
	formats, err := state.Formats()
	if err != nil {
		t.Fatalf("Formats: %v", err)
	}
	if len(formats) != 1 || formats[0].PID != 23 {
		t.Fatalf("formats = %+v", formats)
	}
	pending, err := state.CountPending(23)
	if err != nil {
		t.Fatalf("CountPending: %v", err)
	}
	if pending != 0 {
		t.Errorf("pending = %d, want 0", pending)
	}
}

func TestRunSkipsTheExistingFile(t *testing.T) {
	application, _, downloads := testApp(t, handler(t, []byte("0123456789")))

	if err := application.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// The second run finds the file and does not load it again.
	if err := application.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	entries, err := os.ReadDir(downloads)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("got %d entries in the download directory, want 1", len(entries))
	}
}

func TestRunReportsAMissingFormat(t *testing.T) {
	application, _, _ := testApp(t, handler(t, []byte("0123456789")))
	application.Config.Format = "gibtsnicht"

	if err := application.Run(context.Background()); err == nil {
		t.Fatal("want an error for the unknown format")
	}
}

func TestSelectEpisodesUsesTheMarkedOnes(t *testing.T) {
	application, _, _ := testApp(t, handler(t, []byte("0123456789")))
	application.Requested = []string{"mt420"}

	episodes := []orangeflix.Episode{
		{Identifier: "mt420", Number: 420},
		{Identifier: "mt419", Number: 419},
	}
	selected, err := application.selectEpisodes(context.Background(), orangeflix.Format{PID: 23}, episodes)
	if err != nil {
		t.Fatalf("selectEpisodes: %v", err)
	}
	if len(selected) != 1 || selected[0].Identifier != "mt420" {
		t.Errorf("selected = %+v", selected)
	}
}

func TestSelectEpisodesUsesTheRange(t *testing.T) {
	application, _, _ := testApp(t, handler(t, []byte("0123456789")))
	application.Config.Episode = ""
	application.Config.EpisodeRange = "419-420"

	episodes := []orangeflix.Episode{
		{Identifier: "mt420", Number: 420},
		{Identifier: "mt419", Number: 419},
		{Identifier: "mt418", Number: 418},
	}
	selected, err := application.selectEpisodes(context.Background(), orangeflix.Format{PID: 23}, episodes)
	if err != nil {
		t.Fatalf("selectEpisodes: %v", err)
	}
	if len(selected) != 2 {
		t.Errorf("selected %d episodes, want 2", len(selected))
	}
}

func TestSelectEpisodesUsesTheYear(t *testing.T) {
	application, _, _ := testApp(t, handler(t, []byte("0123456789")))
	application.Config.Episode = ""
	application.Config.Year = "2026"

	episodes := []orangeflix.Episode{
		{Identifier: "mt420", Number: 420, Date: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)},
		{Identifier: "mt300", Number: 300, Date: time.Date(2025, 10, 10, 0, 0, 0, 0, time.UTC)},
	}
	selected, err := application.selectEpisodes(context.Background(), orangeflix.Format{PID: 23}, episodes)
	if err != nil {
		t.Fatalf("selectEpisodes: %v", err)
	}
	if len(selected) != 1 || selected[0].Identifier != "mt420" {
		t.Errorf("selected = %+v", selected)
	}
}

func TestFormatSlugUsesTheKnownName(t *testing.T) {
	// Die Formate aus dem Katalog im Programm behalten ihren sprechenden
	// Namen, auch wenn die API nur das Kürzel kennt.
	format := formatSlug(orangeflix.V3Magazine{PID: 23, Title: "Die Mediatheke", Handle: "MT"})
	if format != "mediatheke" {
		t.Errorf("formatSlug = %q, want mediatheke", format)
	}

	// Ein unbekanntes Format bekommt seinen Namen vom Titel.
	format = formatSlug(orangeflix.V3Magazine{PID: 999, Title: "Neues Format", Handle: "NF"})
	if format != "neues-format" {
		t.Errorf("formatSlug = %q, want neues-format", format)
	}
}

func TestRunRecordsThePathOfTheSkippedFile(t *testing.T) {
	application, state, _ := testApp(t, handler(t, []byte("0123456789")))

	if err := application.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if err := state.ClearDownloaded(23, "mt420"); err != nil {
		t.Fatalf("ClearDownloaded: %v", err)
	}
	if err := application.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	episodes, err := state.Episodes(23)
	if err != nil {
		t.Fatalf("Episodes: %v", err)
	}
	if len(episodes) != 1 || !strings.HasSuffix(episodes[0].Path, ".mp4") {
		t.Errorf("episodes = %+v, want the path of the existing file", episodes)
	}
}

func TestRunKeepsThePartialFileWhenTheDownloadBreaks(t *testing.T) {
	content := []byte("0123456789")
	base := handler(t, content)
	application, _, downloads := testApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/file.mp4" {
			base(w, r)
			return
		}
		// The connection breaks after four bytes.
		w.Header().Set("Content-Length", "10")
		_, _ = w.Write(content[:4])
		w.(http.Flusher).Flush()
		conn, _, _ := w.(http.Hijacker).Hijack()
		conn.Close()
	})
	application.Downloader.Attempts = 1

	if err := application.Run(context.Background()); err == nil {
		t.Fatal("want an error for the broken download")
	}
	parts, _ := filepath.Glob(filepath.Join(downloads, "*", "*", "*"+download.PartSuffix))
	if len(parts) != 1 {
		t.Fatalf("partial files = %v, want one", parts)
	}
	data, err := os.ReadFile(parts[0])
	if err != nil || string(data) != "0123" {
		t.Errorf("partial file = %q, %v; want the received data", data, err)
	}
}

// TestNewCredentialsReachTheV3Client checks that the version 3 client signs
// in with the new account and not with the old one.
func TestNewCredentialsReachTheV3Client(t *testing.T) {
	application := &App{V3: &V3{Client: &orangeflix.V3Client{
		Username:    "alt@example.org",
		Password:    "alt",
		AccessToken: "token-des-alten-kontos",
	}}}
	application.SetCredentials("neu@example.org", "neu")
	client := application.V3.Client
	if client.Username != "neu@example.org" || client.Password != "neu" {
		t.Fatalf("v3 client still has %q/%q", client.Username, client.Password)
	}
	if client.AccessToken != "" {
		t.Fatalf("old token was kept: %q", client.AccessToken)
	}
}
