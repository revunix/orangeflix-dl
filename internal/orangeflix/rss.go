package orangeflix

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// rssFeed is the document of an RSS feed.
type rssFeed struct {
	Channel rssChannel `xml:"channel"`
}

// rssChannel holds the items of a feed.
type rssChannel struct {
	Title string    `xml:"title"`
	Items []rssItem `xml:"item"`
}

// rssItem is one episode of a feed.
type rssItem struct {
	Title       string      `xml:"title"`
	Link        string      `xml:"link"`
	PubDate     string      `xml:"pubDate"`
	GUID        string      `xml:"guid"`
	Description string      `xml:"description"`
	Duration    string      `xml:"duration"`
	Enclosure   rssEncloser `xml:"enclosure"`
}

// rssEncloser is the media file of an item.
type rssEncloser struct {
	URL    string `xml:"url,attr"`
	Length string `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}

// rssPageURL builds the URL of one feed page.
func (c *Client) rssPageURL(pid, submodule, page int, quality string) (string, error) {
	if quality == "" {
		quality = "hd"
	}
	raw := fmt.Sprintf("%s%d-%d/%s.xml?page=%d", c.feedURL(), pid, submodule, quality, page)
	endpoint, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	return endpoint.String(), nil
}

// RSSPage returns one page of an RSS feed.
func (c *Client) RSSPage(ctx context.Context, pid, submodule, page int, quality string) ([]Episode, error) {
	rawURL, err := c.rssPageURL(pid, submodule, page, quality)
	if err != nil {
		return nil, err
	}
	body, err := c.get(ctx, rawURL)
	if err != nil {
		return nil, err
	}

	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("could not parse the RSS feed: %w", err)
	}

	episodes := make([]Episode, 0, len(feed.Channel.Items))
	for _, item := range feed.Channel.Items {
		identifier := strings.TrimSpace(item.GUID)
		if identifier == "" {
			continue
		}
		episode := Episode{
			Identifier:  identifier,
			PID:         pid,
			FormatName:  strings.TrimSpace(feed.Channel.Title),
			Title:       item.Title,
			Description: StripHTML(item.Description),
			Duration:    strings.TrimSpace(item.Duration),
			Number:      numberFromIdentifier(identifier),
		}
		if published, err := time.Parse(time.RFC1123Z, strings.TrimSpace(item.PubDate)); err == nil {
			episode.Date = published.UTC()
		}
		if episode.Number == 0 {
			episode.Number = numberFromTitle(item.Title)
		}
		episodes = append(episodes, episode)
	}
	return episodes, nil
}

// EpisodesViaRSS returns every episode of a format over the RSS feed. One
// request covers ten episodes, so this is much cheaper than the JSON feed.
func (c *Client) EpisodesViaRSS(ctx context.Context, pid, submodule int, quality string) ([]Episode, error) {
	seen := map[string]bool{}
	var all []Episode
	const maxPages = 200
	for page := 1; page <= maxPages; page++ {
		episodes, err := c.RSSPage(ctx, pid, submodule, page, quality)
		if err != nil {
			return nil, err
		}
		if len(episodes) == 0 {
			break
		}
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
			break
		}
	}
	sortEpisodes(all)
	return all, nil
}

// numberFromTitle extracts the episode number from a title like
// "Folge 420 - Die Mediatheke".
func numberFromTitle(title string) int {
	fields := strings.Fields(title)
	for i, field := range fields {
		lowered := strings.ToLower(field)
		if lowered == "folge" || lowered == "folge:" {
			if i+1 < len(fields) {
				if value, err := strconv.Atoi(strings.Trim(fields[i+1], ".,:")); err == nil {
					return value
				}
			}
		}
	}
	return 0
}

var tagPattern = regexp.MustCompile(`<[^>]*>`)

// StripHTML removes the tags of a description and decodes the HTML entities.
func StripHTML(text string) string {
	cleaned := tagPattern.ReplaceAllString(text, " ")
	cleaned = html.UnescapeString(cleaned)
	return strings.Join(strings.Fields(cleaned), " ")
}
