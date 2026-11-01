package orangeflix

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestV3MoreEndpoints checks method, path, query and body of the extended
// client. Every endpoint has one case.
func TestV3MoreEndpoints(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		query  string
		body   string
		answer string
		call   func(context.Context, *V3Client) error
	}{
		{name: "logout", method: http.MethodPost, path: "/api/v3/auth/logout", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { return c.Logout(ctx) }},
		{name: "handoff", method: http.MethodPost, path: "/api/v3/auth/handoff/redeem", body: `"code":"c"`, answer: `{"ok":true,"auth":{"accessToken":"a"}}`,
			call: func(ctx context.Context, c *V3Client) error { return c.Handoff(ctx, "c", "s") }},
		{name: "bookmark-status", method: http.MethodGet, path: "/api/v3/bookmarks/mt420", answer: `{"ok":true,"isBookmarked":true}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.BookmarkStatus(ctx, "mt420"); return err }},
		{name: "bookmark-add", method: http.MethodPost, path: "/api/v3/bookmarks/mt420", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { return c.AddBookmark(ctx, "mt420") }},
		{name: "bookmark-remove", method: http.MethodDelete, path: "/api/v3/bookmarks/mt420", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { return c.RemoveBookmark(ctx, "mt420") }},
		{name: "bookmark-batch", method: http.MethodPost, path: "/api/v3/bookmarks/batch-check", body: `"clipIds"`, answer: `{"ok":true,"bookmarks":{"a":true}}`,
			call: func(ctx context.Context, c *V3Client) error {
				_, err := c.BookmarkStatuses(ctx, []string{"a"})
				return err
			}},
		{name: "recents", method: http.MethodGet, path: "/api/v3/content/recents", query: "limit=5", answer: `{"ok":true,"clips":[]}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.Recents(ctx, 5); return err }},
		{name: "clip", method: http.MethodGet, path: "/api/v3/content/clips/mt420", query: "share=x", answer: `{"ok":true,"clip":{"id":"mt420"}}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.Clip(ctx, "mt420", "x"); return err }},
		{name: "similar", method: http.MethodGet, path: "/api/v3/content/clips/mt420/similar", query: "limit=7", answer: `{"ok":true,"clips":[]}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.SimilarClips(ctx, "mt420", 7); return err }},
		{name: "magazine", method: http.MethodGet, path: "/api/v3/content/magazines/23", answer: `{"ok":true,"magazine":{"pid":23}}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.Magazine(ctx, 23); return err }},
		{name: "host", method: http.MethodGet, path: "/api/v3/content/hosts/abc", query: "limit=10", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.Host(ctx, "abc", 0, 10); return err }},
		{name: "chapter-categories", method: http.MethodGet, path: "/api/v3/content/chapter-categories", query: "magazineId=23", answer: `{"ok":true,"categories":[]}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.ChapterCategories(ctx, 23); return err }},
		{name: "chapter-context", method: http.MethodGet, path: "/api/v3/content/clips/mt420/chapter-suggestions", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error {
				_, err := c.ChapterSuggestionContext(ctx, "mt420")
				return err
			}},
		{name: "chapter-submit", method: http.MethodPost, path: "/api/v3/content/clips/mt420/chapter-suggestions", body: `"chapters"`,
			call: func(ctx context.Context, c *V3Client) error {
				return c.SubmitChapterSuggestion(ctx, "mt420", []byte(`{"chapters":[]}`))
			}},
		{name: "chat-token", method: http.MethodGet, path: "/api/v3/chat/token", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.ChatToken(ctx); return err }},
		{name: "chat-replay", method: http.MethodGet, path: "/api/v3/chat/mt420/replay.json", query: "end=9", answer: `{"ok":true,"messages":[]}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.ChatReplay(ctx, "mt420", 1, 9); return err }},
		{name: "calendar", method: http.MethodGet, path: "/api/v3/calendar", query: "limit=3", answer: `{"ok":true,"entries":[]}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.Calendar(ctx, 3); return err }},
		{name: "announcements", method: http.MethodGet, path: "/api/v3/announcements", query: "surface=overview", answer: `{"ok":true,"announcements":[]}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.Announcements(ctx, "", 2); return err }},
		{name: "playback-progress", method: http.MethodGet, path: "/api/v3/playback/progress", answer: `{"ok":true,"progress":[]}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.PlaybackProgress(ctx); return err }},
		{name: "playback-download", method: http.MethodGet, path: "/api/v3/playback/mt420/download", query: "file=film.mp4", answer: `{"ok":true,"url":"https://example.org/f"}`,
			call: func(ctx context.Context, c *V3Client) error {
				_, err := c.PlaybackDownloadURL(ctx, "mt420", "film.mp4")
				return err
			}},
		{name: "live", method: http.MethodGet, path: "/api/v3/live/active", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.LiveActive(ctx); return err }},
		{name: "live-progress", method: http.MethodPost, path: "/api/v3/live/progress", body: `"mode":"live"`, answer: `{"ok":true,"recorded":true}`,
			call: func(ctx context.Context, c *V3Client) error {
				_, err := c.RecordLiveProgress(ctx, V3LiveProgressRequest{Mode: "live"})
				return err
			}},
		{name: "notification-events", method: http.MethodGet, path: "/api/v3/notifications/events", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.NotificationEvents(ctx); return err }},
		{name: "notification-preferences", method: http.MethodGet, path: "/api/v3/notifications/preferences", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.NotificationPreferences(ctx); return err }},
		{name: "notification-update", method: http.MethodPatch, path: "/api/v3/notifications/preferences", body: `"email"`, answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error {
				_, err := c.UpdateNotificationPreferences(ctx, V3NotificationPreferencesUpdate{Channels: map[string]V3NotificationChannelPreference{"email": {Enabled: true}}})
				return err
			}},
		{name: "notification-event-update", method: http.MethodPatch, path: "/api/v3/notifications/preferences/events/release", body: `"enabled":true`, answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error {
				on := true
				_, err := c.UpdateNotificationEvent(ctx, "release", V3NotificationEventUpdate{Enabled: &on})
				return err
			}},
		{name: "notification-devices", method: http.MethodGet, path: "/api/v3/notifications/devices", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.NotificationDevices(ctx); return err }},
		{name: "notification-device-delete", method: http.MethodDelete, path: "/api/v3/notifications/devices/d1", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { return c.DeleteNotificationDevice(ctx, "d1") }},
		{name: "notification-resubscribe", method: http.MethodPost, path: "/api/v3/notifications/email/resubscribe", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { return c.ResubscribeNotificationEmail(ctx) }},
		{name: "notification-test", method: http.MethodPost, path: "/api/v3/notifications/test", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.SendTestNotification(ctx); return err }},
		{name: "sharing-limit", method: http.MethodGet, path: "/api/v3/sharing/limit", answer: `{"ok":true,"sharing":{}}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.SharingLimit(ctx); return err }},
		{name: "sharing-create", method: http.MethodPost, path: "/api/v3/sharing/mt420", answer: `{"ok":true,"link":{}}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.CreateShare(ctx, "mt420"); return err }},
		{name: "sharing-validate", method: http.MethodGet, path: "/api/v3/sharing/validate/mt420/abc", answer: `{"ok":true,"valid":true}`,
			call: func(ctx context.Context, c *V3Client) error {
				_, err := c.ValidateShare(ctx, "mt420", "abc")
				return err
			}},
		{name: "sharing-links", method: http.MethodGet, path: "/api/v3/sharing/links", answer: `{"ok":true,"links":[]}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.ShareLinks(ctx); return err }},
		{name: "sharing-revoke", method: http.MethodDelete, path: "/api/v3/sharing/abc", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { return c.RevokeShare(ctx, "abc") }},
		{name: "sharing-resolve", method: http.MethodGet, path: "/api/v3/sharing/resolve/abc", answer: `{"ok":true,"clipId":"mt420"}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.ResolveShare(ctx, "abc"); return err }},
		{name: "subscription", method: http.MethodGet, path: "/api/v3/user/subscription", answer: `{"ok":true,"subscription":{}}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.Subscription(ctx); return err }},
		{name: "feed-seen", method: http.MethodPost, path: "/api/v3/me/feed/seen", answer: `{"ok":true,"feedSeenAt":1}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.FeedSeen(ctx); return err }},
		{name: "user-preferences", method: http.MethodGet, path: "/api/v3/user/preferences", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.UserPreferences(ctx); return err }},
		{name: "user-preferences-update", method: http.MethodPatch, path: "/api/v3/user/preferences", body: `"feedStyle":2`, answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error {
				style := 2
				return c.UpdateUserPreferences(ctx, V3UserPreferencesUpdate{FeedStyle: &style})
			}},
		{name: "feed-preferences-update", method: http.MethodPatch, path: "/api/v3/user/preferences/feed", body: `"feedStyle":2`, answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error {
				style := 2
				return c.UpdateFeedPreferences(ctx, V3FeedPreferencesUpdate{FeedStyle: &style})
			}},
		{name: "address-save", method: http.MethodPut, path: "/api/v3/user/address", body: `"zip":"12345"`, answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error {
				return c.SaveAddress(ctx, V3Address{Name: "N", Address: "A", ZIP: "12345", City: "C", Country: "DE"})
			}},
		{name: "address-delete", method: http.MethodDelete, path: "/api/v3/user/address", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { return c.DeleteAddress(ctx) }},
		{name: "birthday-save", method: http.MethodPut, path: "/api/v3/user/birthday", body: `"day":1`, answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { return c.SaveBirthday(ctx, V3Birthday{Day: 1, Month: 2}) }},
		{name: "birthday-delete", method: http.MethodDelete, path: "/api/v3/user/birthday", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { return c.DeleteBirthday(ctx) }},
		{name: "credits-save", method: http.MethodPut, path: "/api/v3/user/credits", body: `"mag":"MT"`, answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error {
				return c.SaveCredits(ctx, V3Credits{Name: "N", Mag: "MT"})
			}},
		{name: "credits-delete", method: http.MethodDelete, path: "/api/v3/user/credits", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { return c.DeleteCredits(ctx) }},
		{name: "sessions", method: http.MethodGet, path: "/api/v3/auth/session-recovery/sessions", answer: `{"ok":true,"sessions":[]}`,
			call: func(ctx context.Context, c *V3Client) error {
				_, err := c.SessionRecoverySessions(ctx, "recovery")
				return err
			}},
		{name: "session-revoke", method: http.MethodDelete, path: "/api/v3/auth/session-recovery/sessions/s1", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { return c.RevokeSession(ctx, "recovery", "s1") }},
		{name: "meta", method: http.MethodGet, path: "/api/v3/meta", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.Meta(ctx); return err }},
		{name: "total-durations", method: http.MethodGet, path: "/api/v3/meta/totalClipDurations", answer: `{"ok":true,"totalClipDurations":5}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.TotalClipDurations(ctx); return err }},
		{name: "health", method: http.MethodGet, path: "/api/v3/health", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.Health(ctx); return err }},
		{name: "time", method: http.MethodGet, path: "/api/v3/time", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.ServerTime(ctx); return err }},
		{name: "voting-token", method: http.MethodGet, path: "/api/v3/voting/token", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { _, err := c.VotingToken(ctx); return err }},
		{name: "agb", method: http.MethodPost, path: "/api/v3/legal/agb/accept", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { return c.AcceptAGB(ctx) }},
		{name: "privacy", method: http.MethodPost, path: "/api/v3/legal/privacy/accept", answer: `{"ok":true}`,
			call: func(ctx context.Context, c *V3Client) error { return c.AcceptPrivacy(ctx) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotPath, gotQuery, gotBody, gotAuth string
			client := testV3Client(t, func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath = r.Method, r.URL.Path
				gotQuery, gotAuth = r.URL.RawQuery, r.Header.Get("Authorization")
				data, _ := io.ReadAll(r.Body)
				gotBody = string(data)
				answer := tc.answer
				if answer == "" {
					answer = `{"ok":true}`
				}
				_, _ = w.Write([]byte(answer))
			})
			client.AccessToken = "token-1"

			if err := tc.call(context.Background(), client); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if gotMethod != tc.method {
				t.Errorf("method = %q, want %q", gotMethod, tc.method)
			}
			if gotPath != tc.path {
				t.Errorf("path = %q, want %q", gotPath, tc.path)
			}
			if tc.query != "" && !strings.Contains(gotQuery, tc.query) {
				t.Errorf("query = %q, want %q", gotQuery, tc.query)
			}
			if tc.body != "" && !strings.Contains(gotBody, tc.body) {
				t.Errorf("body = %q, want %q", gotBody, tc.body)
			}
			wantAuth := "Bearer token-1"
			if tc.name == "sessions" || tc.name == "session-revoke" {
				wantAuth = "Bearer recovery"
			}
			if gotAuth != wantAuth {
				t.Errorf("authorization = %q, want %q", gotAuth, wantAuth)
			}
		})
	}
}

// TestV3FeedQuery checks the feed filters.
func TestV3FeedQuery(t *testing.T) {
	var query string
	client := testV3Client(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"ok":true,"clips":[]}`))
	})
	client.AccessToken = "token-1"
	_, err := client.Feed(context.Background(), V3FeedQuery{
		Offset: 5, Count: 7, Before: 100, Main: true, WithAccess: true,
		Pinned: true, Order: "random", MagazineID: 23, CategoryID: 2,
		Tags: "a,b", TagMode: "or", TimeFrom: 1, TimeTo: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"offset=5", "count=7", "before=100", "main=true",
		"withAccess=true", "pinned=true", "order=random", "magazineId=23",
		"categoryId=2", "tags=a%2Cb", "tagMode=or", "timeFrom=1", "timeTo=2"} {
		if !strings.Contains(query, want) {
			t.Errorf("query = %q, want %q", query, want)
		}
	}
}

// TestV3BookmarksDecodeClipSummary checks that the bookmark list keeps the
// progress and the bookmark flag.
func TestV3BookmarksDecodeClipSummary(t *testing.T) {
	client := testV3Client(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"clips":[{"id":"mt420","isBookmarked":true,
			"progress":{"percentage":39,"lastAccessed":"2026-01-01 10:00:00"}}]}`))
	})
	client.AccessToken = "token-1"
	clips, err := client.Bookmarks(context.Background(), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(clips) != 1 || !clips[0].IsBookmarked || clips[0].Episode().ProgressPercent != 39 {
		t.Fatalf("clips = %+v", clips)
	}
}

// TestV3SimilarMapsSource checks the mapping of the similar rail.
func TestV3SimilarMapsSource(t *testing.T) {
	client := testV3Client(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"clips":[{"type":"clip","id":"mt1","title":"Folge 1",
			"projectTitle":"Die Mediatheke","shortdesc":"Text","source":"similarity",
			"durationSeconds":120,"seqNr":1,"magazineId":23}]}`))
	})
	client.AccessToken = "token-1"
	clips, err := client.SimilarClips(context.Background(), "mt420", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(clips) != 1 || clips[0].Source != "similarity" {
		t.Fatalf("clips = %+v", clips)
	}
	episode := clips[0].Episode()
	if episode.Identifier != "mt1" || episode.FormatName != "Die Mediatheke" || episode.DurationSeconds != 120 {
		t.Errorf("episode = %+v", episode)
	}
}

// TestV3LiveDecode checks the three branches of the live answer.
func TestV3LiveDecode(t *testing.T) {
	client := testV3Client(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"isLive":true,"lastWatchedAt":100,"canAccess":true,
			"url":{"application/x-mpegURL":"https://example.org/live.m3u8"},
			"show":{"id":"s1","title":"Sendung","isLive":true},
			"replay":{"possible":true,"id":"s1","canAccess":true,"title":"Replay","begin":10,"end":20},
			"scheduled":{"id":"s2","title":"Später","startsIn":60}}`))
	})
	client.AccessToken = "token-1"
	live, err := client.LiveActive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !live.IsLive || !live.HasAccess() || live.Show == nil || live.Show.Title != "Sendung" {
		t.Errorf("live = %+v", live)
	}
	if live.Replay == nil || live.Replay.End != 20 || live.Scheduled == nil || live.Scheduled.StartsIn != 60 {
		t.Errorf("live = %+v", live)
	}
}

// TestV3AnnouncementsDecode checks alert and preview fields.
func TestV3AnnouncementsDecode(t *testing.T) {
	client := testV3Client(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"announcements":[
			{"type":"alert","id":"a1","title":"Warnung","bodyMarkdown":"Text","url":"https://example.org"},
			{"type":"preview","id":"p1","title":"Vorschau","shortDescription":"Kurz","priority":2}]}`))
	})
	client.AccessToken = "token-1"
	items, err := client.Announcements(context.Background(), "overview", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Type != "alert" || items[0].BodyMarkdown != "Text" {
		t.Fatalf("items = %+v", items)
	}
	if items[1].Type != "preview" || items[1].Priority != 2 {
		t.Errorf("items = %+v", items)
	}
}

// TestV3HostDecode checks the host page.
func TestV3HostDecode(t *testing.T) {
	client := testV3Client(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"host":{"id":"h1","name":"Gast","clipCount":2},
			"recents":[{"id":"mt1","title":"Folge 1"}],
			"magazines":[{"pid":23,"title":"Die Mediatheke"}]}`))
	})
	client.AccessToken = "token-1"
	host, err := client.Host(context.Background(), "h1", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if host.Host.Name != "Gast" || len(host.Recents) != 1 || len(host.Magazines) != 1 {
		t.Errorf("host = %+v", host)
	}
}

// TestV3NotificationPreferencesDecode checks the effective preferences.
func TestV3NotificationPreferencesDecode(t *testing.T) {
	client := testV3Client(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"preferences":{"channels":{"email":{"enabled":true}},
			"events":{"release":{"enabled":true,"channels":{"email":true}}},
			"subscriptions":{"shows":[]}}}`))
	})
	client.AccessToken = "token-1"
	preferences, err := client.NotificationPreferences(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !preferences.Preferences.Channels["email"].Enabled {
		t.Errorf("preferences = %+v", preferences)
	}
	if !preferences.Preferences.Events["release"].Enabled {
		t.Errorf("preferences = %+v", preferences)
	}
}
