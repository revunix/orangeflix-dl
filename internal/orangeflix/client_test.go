package orangeflix

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// testClient builds a client that answers with the handler.
func testClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := &Client{
		HTTP:     server.Client(),
		BaseURL:  server.URL + "/api/v1/",
		FeedURL:  server.URL + "/feed/",
		Username: "user@example.com",
		Password: "secret",
		Attempts: 1,
		Sleep:    func(context.Context, time.Duration) {},
	}
	return client, server
}

func TestFeedParsesTheEpisodes(t *testing.T) {
	var query string
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"eps":[
			{"identifier":"mt420","pid":23,"title":"Folge 420","pdesc":"Die Mediatheke",
			 "desc":"Text","duration":"00:31:52","enum":420,"date":1791602710,
			 "thumbnail":"https://example.org/bild.jpg"},
			{"identifier":"mt419","pid":23,"title":"Folge 419","enum":419,"date":1791000000}
		]}`))
	})

	episodes, err := client.Feed(context.Background(), 23, 1)
	if err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if len(episodes) != 2 {
		t.Fatalf("got %d episodes, want 2", len(episodes))
	}
	if episodes[0].Identifier != "mt420" || episodes[0].Number != 420 {
		t.Errorf("unexpected episode: %+v", episodes[0])
	}
	if episodes[0].FormatName != "Die Mediatheke" {
		t.Errorf("format name = %q", episodes[0].FormatName)
	}
	if episodes[0].Duration != "00:31:52" {
		t.Errorf("duration = %q", episodes[0].Duration)
	}
	if episodes[0].Date.Unix() != 1791602710 {
		t.Errorf("date = %v", episodes[0].Date)
	}
	if !strings.Contains(query, "action=getFeed") {
		t.Errorf("query = %q, want the action", query)
	}
	if !strings.Contains(query, "from=%5B23%5D") {
		t.Errorf("query = %q, want the format id in brackets", query)
	}
	if !strings.Contains(query, "page=1") {
		t.Errorf("query = %q, want the page", query)
	}
}

func TestFeedSendsTheCredentialsInTheHeader(t *testing.T) {
	var user, password string
	var ok bool
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		user, password, ok = r.BasicAuth()
		_, _ = w.Write([]byte(`{"eps":[]}`))
	})

	if _, err := client.Feed(context.Background(), 1, 1); err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if !ok {
		t.Fatal("the request carried no credentials")
	}
	if user != "user@example.com" || password != "secret" {
		t.Errorf("credentials = %q/%q", user, password)
	}
}

func TestFeedReportsWrongCredentials(t *testing.T) {
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	_, err := client.Feed(context.Background(), 1, 1)
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestFeedReportsTheRateLimit(t *testing.T) {
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("RATE LIMITED"))
	})

	_, err := client.Feed(context.Background(), 1, 1)
	if err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("err = %v, want the rate limit", err)
	}
}

func TestGetRetriesTheRateLimit(t *testing.T) {
	calls := 0
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			_, _ = w.Write([]byte(`{"api_error":"API Rate Limited.","retryAfter":1}`))
			return
		}
		_, _ = w.Write([]byte(`{"eps":[]}`))
	})
	client.Attempts = 3

	if _, err := client.Feed(context.Background(), 1, 1); err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}

func TestEpisodesStopsAtTheEmptyPage(t *testing.T) {
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		switch page {
		case "1":
			_, _ = w.Write([]byte(`{"eps":[{"identifier":"mt2","enum":2},{"identifier":"mt1","enum":1}]}`))
		default:
			_, _ = w.Write([]byte(`{"eps":[]}`))
		}
	})

	episodes, err := client.Episodes(context.Background(), 23)
	if err != nil {
		t.Fatalf("Episodes: %v", err)
	}
	if len(episodes) != 2 {
		t.Fatalf("got %d episodes, want 2", len(episodes))
	}
	// The episodes are sorted by their number, the oldest first.
	if episodes[0].Number != 1 || episodes[1].Number != 2 {
		t.Errorf("wrong order: %d, %d", episodes[0].Number, episodes[1].Number)
	}
}

func TestEpisodesRemovesTheDuplicates(t *testing.T) {
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		switch page {
		case "1":
			_, _ = w.Write([]byte(`{"eps":[{"identifier":"mt2","enum":2}]}`))
		case "2":
			_, _ = w.Write([]byte(`{"eps":[{"identifier":"mt2","enum":2},{"identifier":"mt1","enum":1}]}`))
		default:
			_, _ = w.Write([]byte(`{"eps":[]}`))
		}
	})

	episodes, err := client.Episodes(context.Background(), 23)
	if err != nil {
		t.Fatalf("Episodes: %v", err)
	}
	if len(episodes) != 2 {
		t.Fatalf("got %d episodes, want 2", len(episodes))
	}
}

func TestClipReadsTheFiles(t *testing.T) {
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("action") != "getClip" {
			t.Errorf("action = %q", r.URL.Query().Get("action"))
		}
		if r.URL.Query().Get("identifier") != "mt420" {
			t.Errorf("identifier = %q", r.URL.Query().Get("identifier"))
		}
		_, _ = w.Write([]byte(`{"identifier":"mt420","pid":23,"title":"Folge 420","enum":420,"date":1791602710,
			"files":[
			 {"url":"https://example.org/mt420FULLHD.mp4","desc":"HD 1080p","size":826564514,"dimensions":"1920x1080","t":"film"},
			 {"url":"https://example.org/mt420.mp4","desc":"SD 432p","size":151241243,"dimensions":"768x432","t":"film"},
			 {"url":"https://example.org/mt420.m4a","desc":"Audio m4a","size":76215339,"t":"music"}
			]}`))
	})

	clip, err := client.Clip(context.Background(), "mt420")
	if err != nil {
		t.Fatalf("Clip: %v", err)
	}
	if len(clip.Files) != 3 {
		t.Fatalf("got %d files, want 3", len(clip.Files))
	}
	if clip.Number != 420 {
		t.Errorf("number = %d", clip.Number)
	}
	file, exact, err := SelectFile(clip.Files, QualityHD)
	if err != nil {
		t.Fatalf("SelectFile: %v", err)
	}
	if !exact {
		t.Error("the quality HD 1080p was reported as missing")
	}
	if file.Desc != "HD 1080p" {
		t.Errorf("file = %q", file.Desc)
	}
}

func TestClipWithoutFiles(t *testing.T) {
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"identifier":"mt420","files":[]}`))
	})

	if _, err := client.Clip(context.Background(), "mt420"); err == nil {
		t.Fatal("want an error for the empty file list")
	}
}

func TestToHTTPS(t *testing.T) {
	client := &Client{BaseURL: "https://massengeschmack.tv/api/v1/"}

	cases := []struct {
		in   string
		want string
	}{
		{"https://massengeschmack.tv/dl/mt420/x.mp4", "https://massengeschmack.tv/dl/mt420/x.mp4"},
		{"//massengeschmack.tv/dl/x.mp4", "https://massengeschmack.tv/dl/x.mp4"},
		{"/dl/x.mp4", "https://massengeschmack.tv/dl/x.mp4"},
		{"http://massengeschmack.tv/dl/x.mp4", "https://massengeschmack.tv/dl/x.mp4"},
	}
	for _, testCase := range cases {
		got, err := client.ToHTTPS(testCase.in)
		if err != nil {
			t.Errorf("ToHTTPS(%q): %v", testCase.in, err)
			continue
		}
		if got != testCase.want {
			t.Errorf("ToHTTPS(%q) = %q, want %q", testCase.in, got, testCase.want)
		}
	}

	if _, err := client.ToHTTPS("ftp://example.org/x.mp4"); err == nil {
		t.Error("want an error for the FTP URL")
	}
}

func TestLookupFormat(t *testing.T) {
	format, err := LookupFormat("mediatheke")
	if err != nil {
		t.Fatalf("LookupFormat: %v", err)
	}
	if format.PID != 23 || format.Title != "Die Mediatheke" {
		t.Errorf("format = %+v", format)
	}

	format, err = LookupFormat("23")
	if err != nil {
		t.Fatalf("LookupFormat: %v", err)
	}
	if format.Slug != "mediatheke" {
		t.Errorf("slug = %q", format.Slug)
	}

	// A format that is not in the catalog is accepted with its id.
	format, err = LookupFormat("999")
	if err != nil {
		t.Fatalf("LookupFormat: %v", err)
	}
	if format.PID != 999 {
		t.Errorf("pid = %d", format.PID)
	}

	if _, err := LookupFormat("gibtsnicht"); err == nil {
		t.Error("want an error for the unknown format")
	}
}

func TestNumberFromIdentifier(t *testing.T) {
	cases := map[string]int{"mt420": 420, "fg12": 12, "x": 0, "": 0}
	for identifier, want := range cases {
		if got := numberFromIdentifier(identifier); got != want {
			t.Errorf("numberFromIdentifier(%q) = %d, want %d", identifier, got, want)
		}
	}
}
