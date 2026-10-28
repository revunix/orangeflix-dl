package orangeflix

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// testV3Client builds a client that answers with the handler.
func testV3Client(t *testing.T, handler http.HandlerFunc) *V3Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &V3Client{
		HTTP:     server.Client(),
		BaseURL:  server.URL + "/api/v3/",
		Username: "user@example.com",
		Password: "secret",
		Attempts: 1,
		Sleep:    func(context.Context, time.Duration) {},
	}
}

func TestV3Login(t *testing.T) {
	var body string
	client := testV3Client(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/auth/login" {
			t.Errorf("path = %q", r.URL.Path)
		}
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		_, _ = w.Write([]byte(`{"ok":true,"auth":{"accessToken":"token-1","refreshToken":"refresh-1",
			"expiresIn":3600,"tokenType":"Bearer"}}`))
	})

	totp, err := client.Login(context.Background())
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if totp {
		t.Error("the login asks for a second factor")
	}
	if client.AccessToken != "token-1" {
		t.Errorf("token = %q", client.AccessToken)
	}
	if !strings.Contains(body, "user@example.com") {
		t.Errorf("body = %q", body)
	}
	if !client.TokenValid() {
		t.Error("the token is not valid")
	}
}

func TestV3LoginWithTotp(t *testing.T) {
	client := testV3Client(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"requiresTotp":true,"totpToken":"totp-1"}`))
	})

	totp, err := client.Login(context.Background())
	if !totp {
		t.Error("the second factor was not reported")
	}
	var required *TotpRequiredError
	if !errorsAs(err, &required) {
		t.Fatalf("err = %v, want TotpRequiredError", err)
	}
	if required.Token != "totp-1" {
		t.Errorf("token = %q", required.Token)
	}
}

func TestV3Magazines(t *testing.T) {
	client := testV3Client(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/content/magazines" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("the catalog was asked with a token")
		}
		_, _ = w.Write([]byte(`{"ok":true,"magazines":[
			{"pid":23,"title":"Die Mediatheke","handle":"MT","active":false,"original":true,"clipCount":420,
			 "artwork":{"logo":"https://example.org/logo.jpg"}},
			{"pid":2,"title":"Pantoffelkino","handle":"PK","active":true,"clipCount":548}
		]}`))
	})

	magazines, err := client.Magazines(context.Background())
	if err != nil {
		t.Fatalf("Magazines: %v", err)
	}
	if len(magazines) != 2 {
		t.Fatalf("got %d magazines, want 2", len(magazines))
	}
	if magazines[0].PID != 23 || magazines[0].ClipCount != 420 {
		t.Errorf("magazine = %+v", magazines[0])
	}
	if magazines[0].Artwork.Logo == "" {
		t.Error("the artwork is missing")
	}
}

func TestV3Feed(t *testing.T) {
	var query string
	client := testV3Client(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		if r.Header.Get("Authorization") != "Bearer token-1" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"ok":true,"clips":[
			{"id":"mt420","magazineId":23,"title":"Folge 420","projectTitle":"Die Mediatheke",
			 "description":"Text","duration":"00:31:52","durationSeconds":1912,"time":1791602710,
			 "seqNr":420,"image":"https://example.org/mt420.jpg","canAccess":true,"hasDownload":true,
			 "participants":[{"id":"1","name":"Holger Kreymeier"}]}
		],"pagination":{"offset":0,"count":50,"limit":50}}`))
	})
	client.AccessToken = "token-1"

	clips, err := client.Feed(context.Background(), 23, 0, 50)
	if err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if !strings.Contains(query, "magazineId=23") || !strings.Contains(query, "count=50") {
		t.Errorf("query = %q", query)
	}
	if len(clips) != 1 {
		t.Fatalf("got %d clips, want 1", len(clips))
	}
	episode := clips[0].Episode()
	if episode.Identifier != "mt420" || episode.Number != 420 {
		t.Errorf("episode = %+v", episode)
	}
	if episode.FormatName != "Die Mediatheke" {
		t.Errorf("format = %q", episode.FormatName)
	}
	if episode.DurationSeconds != 1912 {
		t.Errorf("duration = %d", episode.DurationSeconds)
	}
	if len(episode.Participants) != 1 || episode.Participants[0].Name != "Holger Kreymeier" {
		t.Errorf("participants = %+v", episode.Participants)
	}
	if episode.Date.Unix() != 1791602710 {
		t.Errorf("date = %v", episode.Date)
	}
}

func TestV3Playback(t *testing.T) {
	client := testV3Client(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/content/clips/mt420":
			_, _ = w.Write([]byte(`{"ok":true,"clip":{"id":"mt420"}}`))
		case "/api/v3/playback/mt420":
			_, _ = w.Write([]byte(`{"ok":true,"clip":{"id":"mt420","title":"Folge 420",
				"subtitles":[{"lang":"de","url":"https://example.org/mt420.webvtt"}]},
				"stream":{"media":{"files":[
					{"url":"https://cdn.example.org/mt420FULLHD.mp4","desc":"HD 1080p","dimensions":"1920x1080","size":826564514},
					{"url":"https://cdn.example.org/mt420.m4a","desc":"Audio m4a","t":"music"}
				]}}}`))
		default:
			t.Errorf("path = %q", r.URL.Path)
		}
	})

	playback, err := client.Playback(context.Background(), "mt420")
	if err != nil {
		t.Fatalf("Playback: %v", err)
	}
	if len(playback.Stream.Media.Files) != 2 {
		t.Fatalf("got %d files, want 2", len(playback.Stream.Media.Files))
	}
	if len(playback.Clip.Subtitles) != 1 {
		t.Fatalf("got %d subtitles, want 1", len(playback.Clip.Subtitles))
	}
	if playback.Clip.Subtitles[0].Lang != "de" {
		t.Errorf("language = %q", playback.Clip.Subtitles[0].Lang)
	}
}

func TestV3Search(t *testing.T) {
	var query string
	client := testV3Client(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"ok":true,"clips":[{"id":"mt200","title":"Folge 200",
			"projectTitle":"Die Mediatheke","duration":"01:24:47","seqNr":200}],"nextOffset":5}`))
	})

	results, err := client.Search(context.Background(), "Mediatheke", 30, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !strings.Contains(query, "q=Mediatheke") {
		t.Errorf("query = %q", query)
	}
	if len(results) != 1 || results[0].ID != "mt200" {
		t.Errorf("results = %+v", results)
	}
}

func TestV3Profile(t *testing.T) {
	client := testV3Client(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"user":{"lastLogin":1792527598},
			"subscription":{"hasAccess":true,"tier":1,"accessType":"subscription","expiresAt":1793122009}}`))
	})
	client.AccessToken = "token-1"

	profile, err := client.Profile(context.Background())
	if err != nil {
		t.Fatalf("Profile: %v", err)
	}
	if !profile.Subscription.HasAccess || profile.Subscription.Tier != 1 {
		t.Errorf("subscription = %+v", profile.Subscription)
	}
	if profile.Subscription.ExpiresAt != 1793122009 {
		t.Errorf("expires = %d", profile.Subscription.ExpiresAt)
	}
}

func TestV3Unauthorized(t *testing.T) {
	client := testV3Client(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	if _, err := client.Magazines(context.Background()); err != ErrV3Unauthorized {
		t.Fatalf("err = %v, want ErrV3Unauthorized", err)
	}
}

// errorsAs is a small helper for the error check of the test.
func errorsAs(err error, target **TotpRequiredError) bool {
	if err == nil {
		return false
	}
	if required, ok := err.(*TotpRequiredError); ok {
		*target = required
		return true
	}
	return false
}

func TestV3RateLimitIsReported(t *testing.T) {
	client := testV3Client(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"ok":false,"error":{"code":429,"message":"Rate limit exceeded.","retryAfter":155}}`))
	})

	_, err := client.Magazines(context.Background())
	if err == nil {
		t.Fatal("want an error for the rate limit")
	}
	if !strings.Contains(err.Error(), "Rate limit exceeded") {
		t.Errorf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "155") {
		t.Errorf("the pause is not mentioned: %v", err)
	}
}
