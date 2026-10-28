package orangeflix

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Defaults of the client.
const (
	// DefaultBaseURL is the legacy API of Orangeflix. The brand site
	// orangeflix.de serves the new frontend, the API and the download host
	// still live under the old name.
	DefaultBaseURL = "https://massengeschmack.tv/api/v1/"
	// DefaultFeedURL is the base of the RSS feeds.
	DefaultFeedURL = "https://massengeschmack.tv/feed/"
	// DefaultUA is the browser identification the API expects.
	DefaultUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	// maxResponseBytes caps the size of an API response.
	maxResponseBytes = 8 << 20
	// errorBodyLimit is the number of characters of an error body that end up
	// in an error message.
	errorBodyLimit = 200
	// defaultAttempts is the number of tries for a request.
	defaultAttempts = 4
	// maxRateLimitDelay is the longest pause of the own backoff.
	maxRateLimitDelay = 30 * time.Second
	// maxServerDelay is the longest pause that the API may announce.
	maxServerDelay = 5 * time.Minute
)

// ErrUnauthorized is returned when the API rejects the credentials.
var ErrUnauthorized = errors.New("authentication failed (HTTP 401): invalid credentials")

// Client talks to the Orangeflix API.
type Client struct {
	HTTP     *http.Client
	BaseURL  string
	FeedURL  string
	Username string
	Password string
	UA       string

	// Attempts and Sleep are injectable for the tests.
	Attempts int
	Sleep    func(context.Context, time.Duration)

	Logf func(format string, args ...any)
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *Client) baseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return DefaultBaseURL
}

func (c *Client) feedURL() string {
	if c.FeedURL != "" {
		return c.FeedURL
	}
	return DefaultFeedURL
}

func (c *Client) userAgent() string {
	if c.UA != "" {
		return c.UA
	}
	return DefaultUA
}

// applyHeaders sets the request headers. The credentials travel in the
// Authorization header and never in the URL: an e-mail login would be
// percent-encoded twice in a query string.
func (c *Client) applyHeaders(req *http.Request) {
	req.Header.Set("User-Agent", c.userAgent())
	req.Header.Set("Accept", "*/*")
	if c.Username != "" || c.Password != "" {
		req.SetBasicAuth(c.Username, c.Password)
	}
}

// request performs a GET request and decodes the JSON answer into out.
func (c *Client) request(ctx context.Context, rawURL string, out any) error {
	body, err := c.get(ctx, rawURL)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("could not parse the JSON answer: %w", err)
	}
	return nil
}

// get performs a GET request and returns the body. Transient failures and
// the rate limit of the API are retried.
func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	policy := retryPolicy{
		attempts:       c.Attempts,
		sleep:          c.Sleep,
		retryRateLimit: true,
		// The API answers with the plain text "RATE LIMITED" and the status
		// code 200 when the limit is reached.
		rateLimited: isRateLimited,
	}
	if policy.attempts <= 0 {
		policy.attempts = defaultAttempts
	}
	resp, err := policy.send(ctx, c.httpClient(), func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		c.applyHeaders(req)
		return req, nil
	})
	if err != nil {
		return nil, err
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, ErrUnauthorized
	case resp.StatusCode >= 400:
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateBody(resp.Body))
	}
	return resp.Body, nil
}

// truncateBody shortens an error body for the message.
func truncateBody(body []byte) string {
	text := strings.Join(strings.Fields(string(body)), " ")
	if len(text) > errorBodyLimit {
		return text[:errorBodyLimit] + "..."
	}
	return text
}

// feedResponse is the answer of the feed request.
type feedResponse struct {
	EPS []feedEpisode `json:"eps"`
}

// feedEpisode is one episode of the feed answer.
type feedEpisode struct {
	Identifier string `json:"identifier"`
	PID        int    `json:"pid"`
	Title      string `json:"title"`
	PDesc      string `json:"pdesc"`
	Desc       string `json:"desc"`
	Duration   string `json:"duration"`
	Enum       int    `json:"enum"`
	Date       int64  `json:"date"`
	Thumbnail  string `json:"thumbnail"`
	Img        string `json:"img"`
}

func (e feedEpisode) episode() Episode {
	thumb := e.Thumbnail
	if thumb == "" {
		thumb = e.Img
	}
	return Episode{
		Identifier:  e.Identifier,
		PID:         e.PID,
		Number:      e.Enum,
		Title:       e.Title,
		FormatName:  e.PDesc,
		Description: e.Desc,
		Duration:    e.Duration,
		Date:        time.Unix(e.Date, 0).UTC(),
		Thumb:       thumb,
	}
}

// Feed returns one page of the episode list of a format. The API expects the
// format id in brackets: from=[23].
func (c *Client) Feed(ctx context.Context, pid, page int) ([]Episode, error) {
	query := url.Values{}
	query.Set("action", "getFeed")
	query.Set("from", "["+strconv.Itoa(pid)+"]")
	query.Set("page", strconv.Itoa(page))

	endpoint, err := url.Parse(c.baseURL())
	if err != nil {
		return nil, err
	}
	endpoint.RawQuery = query.Encode()

	var answer feedResponse
	if err := c.request(ctx, endpoint.String(), &answer); err != nil {
		return nil, err
	}
	episodes := make([]Episode, 0, len(answer.EPS))
	for _, item := range answer.EPS {
		episode := item.episode()
		if episode.Identifier == "" {
			continue
		}
		if episode.Number == 0 {
			episode.Number = numberFromIdentifier(episode.Identifier)
		}
		episodes = append(episodes, episode)
	}
	return episodes, nil
}

// numberFromIdentifier extracts the episode number from an identifier such as
// "mt420".
func numberFromIdentifier(identifier string) int {
	digits := 0
	for i := len(identifier) - 1; i >= 0; i-- {
		if identifier[i] < '0' || identifier[i] > '9' {
			break
		}
		digits++
	}
	if digits == 0 {
		return 0
	}
	value, err := strconv.Atoi(identifier[len(identifier)-digits:])
	if err != nil {
		return 0
	}
	return value
}

// Episodes returns every episode of a format. The pages are requested until
// the API stops answering with episodes; the same episode can appear on
// several pages and is therefore only kept once.
func (c *Client) Episodes(ctx context.Context, pid int) ([]Episode, error) {
	seen := map[string]bool{}
	var all []Episode
	emptyPages := 0
	const (
		maxPages      = 200
		emptyToleranz = 3
	)
	for page := 1; page <= maxPages; page++ {
		episodes, err := c.Feed(ctx, pid, page)
		if err != nil {
			return nil, err
		}
		if len(episodes) == 0 {
			emptyPages++
			if emptyPages >= emptyToleranz {
				break
			}
			continue
		}
		emptyPages = 0
		added := 0
		for _, episode := range episodes {
			if seen[episode.Identifier] {
				continue
			}
			seen[episode.Identifier] = true
			all = append(all, episode)
			added++
		}
		if added == 0 {
			// The API repeats the last page instead of answering with an
			// empty one.
			break
		}
	}
	sortEpisodes(all)
	return all, nil
}

// clipResponse is the answer of the clip request.
type clipResponse struct {
	Identifier string `json:"identifier"`
	PID        int    `json:"pid"`
	Title      string `json:"title"`
	PDesc      string `json:"pdesc"`
	Desc       string `json:"desc"`
	Duration   string `json:"duration"`
	Enum       int    `json:"enum"`
	Date       int64  `json:"date"`
	Thumbnail  string `json:"thumbnail"`
	Files      []struct {
		URL        string `json:"url"`
		Desc       string `json:"desc"`
		Size       int64  `json:"size"`
		Dimensions string `json:"dimensions"`
		T          string `json:"t"`
	} `json:"files"`
}

// Clip returns the metadata and the download files of one episode.
func (c *Client) Clip(ctx context.Context, identifier string) (*Clip, error) {
	query := url.Values{}
	query.Set("action", "getClip")
	query.Set("identifier", identifier)

	endpoint, err := url.Parse(c.baseURL())
	if err != nil {
		return nil, err
	}
	endpoint.RawQuery = query.Encode()

	var answer clipResponse
	if err := c.request(ctx, endpoint.String(), &answer); err != nil {
		return nil, err
	}

	clip := &Clip{Episode: Episode{
		Identifier:  answer.Identifier,
		PID:         answer.PID,
		Number:      answer.Enum,
		Title:       answer.Title,
		FormatName:  answer.PDesc,
		Description: answer.Desc,
		Duration:    answer.Duration,
		Date:        time.Unix(answer.Date, 0).UTC(),
		Thumb:       answer.Thumbnail,
	}}
	if clip.Number == 0 {
		clip.Number = numberFromIdentifier(clip.Identifier)
	}
	for _, item := range answer.Files {
		if item.URL == "" {
			continue
		}
		clip.Files = append(clip.Files, File{
			URL:        item.URL,
			Desc:       item.Desc,
			Size:       item.Size,
			Dimensions: item.Dimensions,
			Kind:       item.T,
		})
	}
	if len(clip.Files) == 0 {
		return nil, fmt.Errorf("no download files for %s", identifier)
	}
	return clip, nil
}

// ToHTTPS normalizes an API supplied URL to HTTPS. Protocol relative and
// hostless paths are resolved, plain HTTP is upgraded.
func (c *Client) ToHTTPS(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", errors.New("empty URL")
	}
	switch {
	case strings.HasPrefix(value, "//"):
		value = "https:" + value
	case strings.HasPrefix(value, "/"):
		host := c.hostFromBaseURL()
		if host == "" {
			return "", fmt.Errorf("cannot resolve the relative URL %s", raw)
		}
		value = "https://" + host + value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("invalid URL %s: %w", raw, err)
	}
	if parsed.Scheme == "http" {
		parsed.Scheme = "https"
		if parsed.Port() == "80" {
			parsed.Host = parsed.Hostname()
		}
	}
	if parsed.Scheme != "https" {
		return "", fmt.Errorf("only HTTPS is allowed: %s", raw)
	}
	return parsed.String(), nil
}

// hostFromBaseURL returns the host of the API base URL.
func (c *Client) hostFromBaseURL() string {
	parsed, err := url.Parse(c.baseURL())
	if err != nil {
		return ""
	}
	return parsed.Host
}

// sortEpisodes sorts the episodes by their number, oldest first.
func sortEpisodes(episodes []Episode) {
	for i := 1; i < len(episodes); i++ {
		for j := i; j > 0 && episodes[j].Number < episodes[j-1].Number; j-- {
			episodes[j], episodes[j-1] = episodes[j-1], episodes[j]
		}
	}
}
