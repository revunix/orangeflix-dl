package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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

// moreApp builds an application with a version 3 client and a legacy client
// against one test server.
func moreApp(t *testing.T, handler http.HandlerFunc) (*App, *httptest.Server) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	state, err := store.Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { state.Close() })
	application := &App{
		Config:  &config.Options{Quality: orangeflix.QualityHD, Submodule: 1},
		API:     &orangeflix.Client{HTTP: server.Client(), BaseURL: server.URL + "/api/v1/"},
		Manager: &download.Manager{DownloadsDir: filepath.Join(t.TempDir(), "downloads")},
		Store:   state,
		Layout:  media.LayoutCompat,
		V3: &V3{Client: &orangeflix.V3Client{
			HTTP:        server.Client(),
			BaseURL:     server.URL + "/api/v3/",
			AccessToken: "token-1",
		}},
	}
	application.Downloader = &download.Downloader{HTTP: server.Client()}
	return application, server
}

func TestRecentsMapsProgress(t *testing.T) {
	application, _ := moreApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/content/recents" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"ok":true,"clips":[{"id":"mt420","magazineId":23,"seqNr":420,
			"title":"Folge 420","projectTitle":"Die Mediatheke","durationSeconds":1200,
			"isBookmarked":true,"progress":{"percentage":39}}]}`))
	})
	episodes, err := application.Recents(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(episodes) != 1 {
		t.Fatalf("episodes = %d", len(episodes))
	}
	if episodes[0].Identifier != "mt420" || episodes[0].ProgressPercent != 39 || !episodes[0].Bookmarked {
		t.Errorf("episode = %+v", episodes[0])
	}
}

func TestBookmarksMapsClips(t *testing.T) {
	application, _ := moreApp(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/bookmarks" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"ok":true,"clips":[{"id":"mt419","title":"Folge 419",
			"duration":"00:20:00","durationSeconds":1200,"isBookmarked":true}]}`))
	})
	episodes, err := application.Bookmarks(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(episodes) != 1 || episodes[0].Identifier != "mt419" || episodes[0].DurationSeconds != 1200 {
		t.Fatalf("episodes = %+v", episodes)
	}
}

func TestSetBookmarkCallsPostAndDelete(t *testing.T) {
	var methods []string
	application, _ := moreApp(t, func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	if err := application.SetBookmark(context.Background(), "mt420", true); err != nil {
		t.Fatal(err)
	}
	if err := application.SetBookmark(context.Background(), "mt420", false); err != nil {
		t.Fatal(err)
	}
	if len(methods) != 2 || methods[0] != http.MethodPost || methods[1] != http.MethodDelete {
		t.Errorf("methods = %v", methods)
	}
}

func TestDownloadUsesOfficialEndpoint(t *testing.T) {
	var official, direct bool
	var server *httptest.Server
	application, srv := moreApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/content/clips/mt420":
			_, _ = w.Write([]byte(`{"ok":true,"clip":{"id":"mt420"}}`))
		case r.URL.Path == "/api/v3/playback/mt420":
			_, _ = w.Write([]byte(`{"ok":true,"clip":{"id":"mt420","durationSeconds":1200},
				"stream":{"media":{"files":[{"url":"` + server.URL + `/direct.mp4","desc":"HD 1080p","size":4,"file":"mt420FULLHD.mp4"}]}}}`))
		case r.URL.Path == "/api/v3/playback/mt420/download":
			official = true
			if r.URL.Query().Get("file") != "mt420FULLHD.mp4" {
				t.Errorf("file = %q", r.URL.Query().Get("file"))
			}
			_, _ = w.Write([]byte(`{"ok":true,"url":"` + server.URL + `/official.mp4"}`))
		case r.URL.Path == "/official.mp4":
			_, _ = w.Write([]byte("data"))
		case r.URL.Path == "/direct.mp4":
			direct = true
			_, _ = w.Write([]byte("data"))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	server = srv
	err := application.DownloadEpisode(context.Background(), orangeflix.Episode{
		Identifier: "mt420", PID: 23, FormatName: "Die Mediatheke",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !official {
		t.Error("the official download endpoint was not used")
	}
	if direct {
		t.Error("the direct URL was used")
	}
}

func TestLiveURLSelection(t *testing.T) {
	info := &orangeflix.V3LiveInfo{URL: map[string]string{
		"application/dash+xml":  "https://example.org/dash.mpd",
		"application/x-mpegURL": "https://example.org/hls.m3u8",
	}}
	got, err := liveURL(info, "live")
	if err != nil || got != "https://example.org/hls.m3u8" {
		t.Fatalf("liveURL = %q, %v", got, err)
	}
	if _, err := liveURL(&orangeflix.V3LiveInfo{}, "live"); err == nil {
		t.Fatal("want an error without a URL")
	}
}

func TestMPDStartTime(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<MPD availabilityStartTime="2026-11-01T17:58:59.245Z"></MPD>`))
	}))
	defer server.Close()
	got, err := mpdStartTime(context.Background(), &V3{Client: &orangeflix.V3Client{HTTP: server.Client()}}, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if got.UTC().Format(time.RFC3339Nano) != "2026-11-01T17:58:59.245Z" {
		t.Errorf("start = %v", got)
	}
}

func TestLiveRunnerReport(t *testing.T) {
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		_, _ = w.Write([]byte(`{"ok":true,"recorded":true}`))
	}))
	defer server.Close()
	application := &App{V3: &V3{Client: &orangeflix.V3Client{
		HTTP:        server.Client(),
		BaseURL:     server.URL + "/api/v3/",
		AccessToken: "token-1",
	}}}
	replay := &liveRunner{app: application, mode: "replay", start: time.Unix(1000, 0)}
	replay.reportLive(context.Background(), 12)
	if !strings.Contains(body, `"mode":"replay"`) || !strings.Contains(body, `"watchedAt":1012`) {
		t.Errorf("body = %q", body)
	}
	live := &liveRunner{app: application, mode: "live"}
	live.reportLive(context.Background(), 12)
	if !strings.Contains(body, `"mode":"live"`) || strings.Contains(body, "watchedAt") {
		t.Errorf("body = %q", body)
	}
}
