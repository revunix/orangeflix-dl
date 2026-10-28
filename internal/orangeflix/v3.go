package orangeflix

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultV3URL is the base of the version 3 API. It is the API the Orangeflix
// app uses.
const DefaultV3URL = "https://massengeschmack.tv/api/v3/"

// ErrV3Unauthorized is returned when the API rejects the access token.
var ErrV3Unauthorized = errors.New("the access token was rejected (HTTP 401)")

// V3Client talks to the version 3 API. It uses a bearer token instead of the
// basic authentication of the legacy API and delivers more data per request.
type V3Client struct {
	HTTP    *http.Client
	BaseURL string
	UA      string

	// Username and Password are used to fetch a new token.
	Username string
	Password string

	// AccessToken is the bearer token. It is set by Login and refreshed by
	// Refresh.
	AccessToken string
	// RefreshToken is used to fetch a new access token.
	RefreshToken string
	// ExpiresAt is the time the access token runs out.
	ExpiresAt time.Time

	Attempts int
	Sleep    func(context.Context, time.Duration)
	Logf     func(format string, args ...any)
}

// v3LoginResponse is the answer of the login.
type v3LoginResponse struct {
	OK   bool `json:"ok"`
	Auth struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    int64  `json:"expiresIn"`
		ExpiresAt    int64  `json:"expiresAt"`
		TokenType    string `json:"tokenType"`
		SessionID    string `json:"sessionId"`
	} `json:"auth"`
	// RequiresTotp tells that the account asks for a second factor.
	RequiresTotp bool   `json:"requiresTotp"`
	TotpToken    string `json:"totpToken"`
	Error        string `json:"error"`
}

// v3RefreshResponse is the answer of the refresh.
type v3RefreshResponse struct {
	OK   bool `json:"ok"`
	Auth struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    int64  `json:"expiresIn"`
		ExpiresAt    int64  `json:"expiresAt"`
		TokenType    string `json:"tokenType"`
		SessionID    string `json:"sessionId"`
	} `json:"auth"`
	Error string `json:"error"`
}

func (c *V3Client) baseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return DefaultV3URL
}

func (c *V3Client) userAgent() string {
	if c.UA != "" {
		return c.UA
	}
	return DefaultUA
}

func (c *V3Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *V3Client) logf(format string, args ...any) {
	if c.Logf != nil {
		c.Logf(format, args...)
	}
}

// SetToken stores a token that was fetched earlier.
func (c *V3Client) SetToken(access, refresh string, expiresAt time.Time) {
	c.AccessToken = access
	c.RefreshToken = refresh
	c.ExpiresAt = expiresAt
}

// TokenValid reports whether the access token can still be used.
func (c *V3Client) TokenValid() bool {
	return c.AccessToken != "" && (c.ExpiresAt.IsZero() || time.Now().Before(c.ExpiresAt.Add(-time.Minute)))
}

// request sends a request to the API. A request without a token is sent as
// it is, a request with a token carries it in the Authorization header.
//
// A rejected token triggers a single new login: the sessions of the account
// are limited, so a new one is only created when the old one does not work
// any more.
func (c *V3Client) request(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	err := c.do(ctx, method, path, query, body, out)
	if !errors.Is(err, ErrV3Unauthorized) {
		return err
	}
	if c.AccessToken == "" {
		return err
	}
	c.logf("Der Token wurde abgelehnt, hole einen neuen.")
	c.AccessToken = ""
	if _, loginErr := c.Login(ctx); loginErr != nil {
		return loginErr
	}
	return c.do(ctx, method, path, query, body, out)
}

// do sends the request without the recovery of a rejected token.
func (c *V3Client) do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	endpoint, err := url.Parse(c.baseURL() + strings.TrimPrefix(path, "/"))
	if err != nil {
		return err
	}
	if len(query) > 0 {
		endpoint.RawQuery = query.Encode()
	}

	var encoded []byte
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}

	policy := retryPolicy{
		attempts: c.Attempts,
		sleep:    c.Sleep,
		// A repeated login would only extend the lock of the account.
		retryRateLimit: !strings.Contains(path, "auth/login"),
		describe:       apiError,
	}
	if policy.attempts <= 0 {
		policy.attempts = 3
	}
	resp, err := policy.send(ctx, c.httpClient(), func() (*http.Request, error) {
		var payload io.Reader
		if body != nil {
			payload = bytes.NewReader(encoded)
		}
		req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), payload)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", c.userAgent())
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json; charset=utf-8")
		}
		if c.AccessToken != "" {
			req.Header.Set("Authorization", "Bearer "+c.AccessToken)
		}
		return req, nil
	})
	if err != nil {
		return err
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return ErrV3Unauthorized
	case resp.StatusCode >= 400:
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, apiError(resp.Body))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(resp.Body, out); err != nil {
		return fmt.Errorf("could not parse the JSON answer: %w", err)
	}
	return nil
}

// Login fetches a new access token. The answer tells whether the account
// asks for a second factor.
func (c *V3Client) Login(ctx context.Context) (bool, error) {
	body := map[string]any{
		"email":      c.Username,
		"password":   c.Password,
		"remember":   true,
		"deviceName": "orangeflix-dl",
	}
	var answer v3LoginResponse
	if err := c.request(ctx, http.MethodPost, "auth/login", nil, body, &answer); err != nil {
		return false, err
	}
	if answer.RequiresTotp {
		return true, &TotpRequiredError{Token: answer.TotpToken}
	}
	if answer.Auth.AccessToken == "" {
		if answer.Error != "" {
			return false, fmt.Errorf("the login failed: %s", answer.Error)
		}
		return false, errors.New("the login failed: no access token in the answer")
	}
	c.setFromAuth(answer.Auth.AccessToken, answer.Auth.RefreshToken, answer.Auth.ExpiresIn, answer.Auth.ExpiresAt)
	return false, nil
}

// TotpRequiredError tells that the account asks for a second factor.
type TotpRequiredError struct {
	Token string
}

func (e *TotpRequiredError) Error() string {
	return "the account asks for a second factor (TOTP)"
}

// CompleteTotpLogin finishes the login with the code of the second factor.
func (c *V3Client) CompleteTotpLogin(ctx context.Context, totpToken, code string) error {
	body := map[string]any{"totpToken": totpToken, "code": strings.TrimSpace(code)}
	var answer v3LoginResponse
	if err := c.request(ctx, http.MethodPost, "auth/login/totp", nil, body, &answer); err != nil {
		return err
	}
	if answer.Auth.AccessToken == "" {
		return errors.New("the login failed: no access token in the answer")
	}
	c.setFromAuth(answer.Auth.AccessToken, answer.Auth.RefreshToken, answer.Auth.ExpiresIn, answer.Auth.ExpiresAt)
	return nil
}

// Refresh fetches a new access token with the refresh token.
func (c *V3Client) Refresh(ctx context.Context) error {
	if c.RefreshToken == "" {
		return errors.New("no refresh token")
	}
	body := map[string]any{"refreshToken": c.RefreshToken}
	var answer v3RefreshResponse
	if err := c.request(ctx, http.MethodPost, "auth/refresh", nil, body, &answer); err != nil {
		return err
	}
	if answer.Auth.AccessToken == "" {
		if answer.Error != "" {
			return fmt.Errorf("the refresh failed: %s", answer.Error)
		}
		return errors.New("the refresh failed: no access token in the answer")
	}
	c.setFromAuth(answer.Auth.AccessToken, answer.Auth.RefreshToken, answer.Auth.ExpiresIn, answer.Auth.ExpiresAt)
	return nil
}

// setFromAuth stores the tokens of an answer.
func (c *V3Client) setFromAuth(access, refresh string, expiresIn, expiresAt int64) {
	c.AccessToken = access
	if refresh != "" {
		c.RefreshToken = refresh
	}
	switch {
	case expiresAt > 0:
		c.ExpiresAt = time.Unix(expiresAt, 0)
	case expiresIn > 0:
		c.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
	default:
		c.ExpiresAt = time.Time{}
	}
}

// EnsureToken makes sure that a usable access token is there. A token that is
// already known is kept: the account is limited in the number of sessions, so
// a new one is only created when the old one is rejected.
func (c *V3Client) EnsureToken(ctx context.Context) error {
	if c.AccessToken != "" {
		return nil
	}
	if c.RefreshToken != "" {
		if err := c.Refresh(ctx); err == nil {
			return nil
		} else {
			c.logf("Der Token konnte nicht erneuert werden (%s), hole einen neuen.", err.Error())
		}
	}
	_, err := c.Login(ctx)
	return err
}

// apiError reads the reason of a failed request. The answer of the API names
// it in the field "error".
func apiError(body []byte) string {
	var answer struct {
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &answer); err == nil && len(answer.Error) > 0 {
		var message string
		if err := json.Unmarshal(answer.Error, &message); err == nil && message != "" {
			return message
		}
		var detailed struct {
			Message    string `json:"message"`
			Code       int    `json:"code"`
			RetryAfter int    `json:"retryAfter"`
		}
		if err := json.Unmarshal(answer.Error, &detailed); err == nil && detailed.Message != "" {
			if detailed.RetryAfter > 0 {
				return fmt.Sprintf("%s (erneut versuchen in %d Sekunden)", detailed.Message, detailed.RetryAfter)
			}
			return detailed.Message
		}
	}
	return truncateBody(body)
}

// V3Profile is the account of the user.
type V3Profile struct {
	OK   bool `json:"ok"`
	User struct {
		Avatar     string `json:"avatar"`
		Chatname   string `json:"chatname"`
		CreatedAt  int64  `json:"createdAt"`
		LastLogin  int64  `json:"lastLogin"`
		FeedSeenAt any    `json:"feedSeenAt"`
	} `json:"user"`
	Subscription struct {
		HasAccess             bool   `json:"hasAccess"`
		AccessType            string `json:"accessType"`
		Tier                  int    `json:"tier"`
		ExpiresAt             int64  `json:"expiresAt"`
		ExpiresAtFormatted    string `json:"lzkFormatted"`
		Lifetime              bool   `json:"lifetime"`
		ActiveSubDescription  string `json:"activeSubDesc"`
		CaptureAmount         string `json:"captureAmountFormatted"`
		ComputedCaptureAmount int    `json:"computedCaptureAmount"`
	} `json:"subscription"`
	Features struct {
		HasTotp bool `json:"hasTotp"`
	} `json:"features"`
}

// Profile returns the account of the user with the state of the
// subscription.
func (c *V3Client) Profile(ctx context.Context) (*V3Profile, error) {
	var answer V3Profile
	if err := c.request(ctx, http.MethodGet, "user/me", nil, nil, &answer); err != nil {
		return nil, err
	}
	return &answer, nil
}

// V3Magazine is one format of the catalog.
type V3Magazine struct {
	PID       int    `json:"pid"`
	Title     string `json:"title"`
	Handle    string `json:"handle"`
	Active    bool   `json:"active"`
	Original  bool   `json:"original"`
	ClipCount int    `json:"clipCount"`
	TimeRange struct {
		Min int64 `json:"min"`
		Max int64 `json:"max"`
	} `json:"timeRange"`
	Artwork struct {
		Logo       string `json:"logo"`
		LogoSquare string `json:"logoSquare"`
		LogoDark   string `json:"logoDark"`
		Banner     struct {
			One string `json:"1x"`
			Two string `json:"2x"`
		} `json:"banner"`
	} `json:"artwork"`
	// Tags sind je nach Format Strings oder Objekte.
	Tags []any `json:"tags"`
}

// v3MagazinesResponse is the answer of the catalog.
type v3MagazinesResponse struct {
	OK        bool         `json:"ok"`
	Magazines []V3Magazine `json:"magazines"`
}

// Magazines returns the catalog of the formats. The endpoint needs no
// token.
func (c *V3Client) Magazines(ctx context.Context) ([]V3Magazine, error) {
	var answer v3MagazinesResponse
	if err := c.request(ctx, http.MethodGet, "content/magazines", nil, nil, &answer); err != nil {
		return nil, err
	}
	return answer.Magazines, nil
}

// V3Participant is a guest of a format.
type V3Participant struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Image       string `json:"image"`
}

// Episode converts a clip of the version 3 API into an episode.
func (c V3Clip) Episode() Episode {
	episode := Episode{
		Identifier:      c.ID,
		PID:             c.MagazineID,
		Number:          c.SeqNr,
		Title:           c.Title,
		FormatName:      c.ProjectTitle,
		Description:     c.Description,
		Duration:        c.Duration,
		DurationSeconds: c.DurationSeconds,
		CategoryTitle:   c.CategoryTitle,
		ProgressPercent: c.Progress.Percentage,
		HasDownload:     c.HasDownload,
		Thumb:           c.Image,
	}
	if episode.Thumb == "" {
		episode.Thumb = c.Images.Thumbnail
	}
	if c.Time > 0 {
		episode.Date = time.Unix(c.Time, 0).UTC()
	}
	for _, participant := range c.Participants {
		episode.Participants = append(episode.Participants, Participant(participant))
	}
	return episode
}

// V3Clip is one episode of a format.
type V3Clip struct {
	ID               string `json:"id"`
	MagazineID       int    `json:"magazineId"`
	CategoryID       int    `json:"categoryId"`
	CategoryTitle    string `json:"categoryTitle"`
	Title            string `json:"title"`
	ProjectTitle     string `json:"projectTitle"`
	Description      string `json:"description"`
	ShortDescription string `json:"shortDescription"`
	Duration         string `json:"duration"`
	DurationSeconds  int    `json:"durationSeconds"`
	Time             int64  `json:"time"`
	TimeFormatted    string `json:"timeFormatted"`
	SeqNr            int    `json:"seqNr"`
	HideSeqNr        bool   `json:"hideSeqNr"`
	Image            string `json:"image"`
	Images           struct {
		Thumbnail  string `json:"thumbnail"`
		Poster     string `json:"poster"`
		Thumbnail2 string `json:"thumbnail2x"`
	} `json:"images"`
	Participants  []V3Participant `json:"participants"`
	CanAccess     bool            `json:"canAccess"`
	HasDownload   bool            `json:"hasDownload"`
	IsPremium     bool            `json:"isPremium"`
	IsEarlyAccess bool            `json:"isEarlyAccess"`
	IsBookmarked  bool            `json:"isBookmarked"`
	Progress      struct {
		Percentage   int    `json:"percentage"`
		LastAccessed string `json:"lastAccessed"`
	} `json:"progress"`
}

// v3FeedResponse is the answer of the feed.
type v3FeedResponse struct {
	OK         bool     `json:"ok"`
	Clips      []V3Clip `json:"clips"`
	Pagination struct {
		Offset int `json:"offset"`
		Count  int `json:"count"`
		Limit  int `json:"limit"`
	} `json:"pagination"`
}

// Feed returns one page of the episodes of a format. The version 3 API
// delivers fifty episodes per request, the legacy feed only ten.
func (c *V3Client) Feed(ctx context.Context, pid, offset, count int) ([]V3Clip, error) {
	if count <= 0 {
		count = 50
	}
	query := url.Values{}
	query.Set("magazineId", strconv.Itoa(pid))
	query.Set("count", strconv.Itoa(count))
	query.Set("offset", strconv.Itoa(offset))
	query.Set("includePremium", "true")
	query.Set("includeEarlyAccess", "true")

	var answer v3FeedResponse
	if err := c.request(ctx, http.MethodGet, "content/feed", query, nil, &answer); err != nil {
		return nil, err
	}
	return answer.Clips, nil
}

// V3Playback is the answer of the playback request.
type V3Playback struct {
	OK   bool `json:"ok"`
	Clip struct {
		ID              string `json:"id"`
		Title           string `json:"title"`
		Duration        string `json:"duration"`
		DurationSeconds int    `json:"durationSeconds"`
		Subtitles       []struct {
			Lang string `json:"lang"`
			URL  string `json:"url"`
		} `json:"subtitles"`
	} `json:"clip"`
	Stream struct {
		Access bool `json:"access"`
		Media  struct {
			Files   []File            `json:"files"`
			Streams map[string]string `json:"streams"`
		} `json:"media"`
		HLSID string `json:"hlsId"`
	} `json:"stream"`
}

// Playback returns the files of an episode. The URLs of the files work
// without any credentials.
func (c *V3Client) Playback(ctx context.Context, identifier string) (*V3Playback, error) {
	var answer V3Playback
	if err := c.request(ctx, http.MethodGet, "content/clips/"+url.PathEscape(identifier), nil, nil, nil); err != nil {
		return nil, err
	}
	if err := c.request(ctx, http.MethodGet, "playback/"+url.PathEscape(identifier), nil, nil, &answer); err != nil {
		return nil, err
	}
	return &answer, nil
}

// V3SearchResult is one hit of the search.
type V3SearchResult struct {
	ID               string `json:"id"`
	Type             string `json:"type"`
	MagazineID       int    `json:"magazineId"`
	Title            string `json:"title"`
	ProjectTitle     string `json:"projectTitle"`
	Duration         string `json:"duration"`
	SeqNr            int    `json:"seqNr"`
	Image            string `json:"image"`
	ShortDescription string `json:"shortDescription"`
}

// v3SearchResponse is the answer of the search.
type v3SearchResponse struct {
	OK         bool             `json:"ok"`
	Clips      []V3SearchResult `json:"clips"`
	NextOffset int              `json:"nextOffset"`
}

// Search looks for clips. The search covers the titles, the descriptions and
// the transcripts.
func (c *V3Client) Search(ctx context.Context, query string, limit, offset int) ([]V3SearchResult, error) {
	if limit <= 0 {
		limit = 30
	}
	values := url.Values{}
	values.Set("q", query)
	values.Set("limit", strconv.Itoa(limit))
	values.Set("offset", strconv.Itoa(offset))

	var answer v3SearchResponse
	if err := c.request(ctx, http.MethodGet, "search", values, nil, &answer); err != nil {
		return nil, err
	}
	return answer.Clips, nil
}

// V3Bookmark is one bookmark of the account.
type V3Bookmark struct {
	ID           string `json:"id"`
	MagazineID   int    `json:"magazineId"`
	Title        string `json:"title"`
	ProjectTitle string `json:"projectTitle"`
	Duration     string `json:"duration"`
	SeqNr        int    `json:"seqNr"`
	Image        string `json:"image"`
}

// v3BookmarksResponse is the answer of the bookmark list.
type v3BookmarksResponse struct {
	OK    bool         `json:"ok"`
	Clips []V3Bookmark `json:"clips"`
}

// Bookmarks returns the bookmarks of the account.
func (c *V3Client) Bookmarks(ctx context.Context, limit, offset int) ([]V3Bookmark, error) {
	if limit <= 0 {
		limit = 50
	}
	query := url.Values{}
	query.Set("limit", strconv.Itoa(limit))
	query.Set("offset", strconv.Itoa(offset))

	var answer v3BookmarksResponse
	if err := c.request(ctx, http.MethodGet, "bookmarks", query, nil, &answer); err != nil {
		return nil, err
	}
	return answer.Clips, nil
}
