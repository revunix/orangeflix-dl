package orangeflix

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

const sampleFeed = `<?xml version="1.0" encoding="utf-8" ?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
	<channel>
		<title>Die Mediatheke</title>
		<item>
			<title>Folge 420 - Die Mediatheke</title>
			<link>https://massengeschmack.tv/clip/mt420</link>
			<pubDate>Sat, 10 Oct 2026 05:25:10 +0200</pubDate>
			<guid isPermaLink="false">mt420</guid>
			<description><![CDATA[<img src="x.jpg" />Hier ist die <b>Mediatheke</b>]]></description>
			<enclosure url="https://massengeschmack.tv/dl/mt420/token-mt/mt420HD.mp4" length="319830856" type="video/mp4" />
			<itunes:duration>00:31:52</itunes:duration>
		</item>
		<item>
			<title>Folge 419 - Die Mediatheke</title>
			<guid isPermaLink="false">mt419</guid>
			<pubDate>Sat, 03 Oct 2026 05:25:10 +0200</pubDate>
			<enclosure url="https://massengeschmack.tv/dl/mt419/token-mt/mt419HD.mp4" length="1" type="video/mp4" />
			<itunes:duration>00:30:00</itunes:duration>
		</item>
	</channel>
</rss>`

func TestRSSPageReadsTheItems(t *testing.T) {
	var path string
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(sampleFeed))
	})

	episodes, err := client.RSSPage(context.Background(), 23, 1, 1, "hd")
	if err != nil {
		t.Fatalf("RSSPage: %v", err)
	}
	if path != "/feed/23-1/hd.xml" {
		t.Errorf("path = %q", path)
	}
	if len(episodes) != 2 {
		t.Fatalf("got %d episodes, want 2", len(episodes))
	}
	first := episodes[0]
	if first.Identifier != "mt420" {
		t.Errorf("identifier = %q", first.Identifier)
	}
	if first.Number != 420 {
		t.Errorf("number = %d", first.Number)
	}
	if first.FormatName != "Die Mediatheke" {
		t.Errorf("format name = %q", first.FormatName)
	}
	if first.Duration != "00:31:52" {
		t.Errorf("duration = %q", first.Duration)
	}
	if first.Date.Year() != 2026 || first.Date.Month() != 10 {
		t.Errorf("date = %v", first.Date)
	}
	if strings.Contains(first.Description, "<") {
		t.Errorf("the description still holds the tags: %q", first.Description)
	}
	if !strings.Contains(first.Description, "Mediatheke") {
		t.Errorf("description = %q", first.Description)
	}
}

func TestRSSPageReportsTheRateLimit(t *testing.T) {
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("RATE LIMITED"))
	})

	if _, err := client.RSSPage(context.Background(), 23, 1, 1, "hd"); err == nil {
		t.Fatal("want an error for the rate limit")
	}
}

func TestStripHTML(t *testing.T) {
	got := StripHTML(`<p>Hallo &amp; <b>Welt</b></p>`)
	if got != "Hallo & Welt" {
		t.Errorf("StripHTML = %q", got)
	}
}

func TestNumberFromTitle(t *testing.T) {
	if got := numberFromTitle("Folge 420 - Die Mediatheke"); got != 420 {
		t.Errorf("numberFromTitle = %d", got)
	}
	if got := numberFromTitle("Folge: 7"); got != 7 {
		t.Errorf("numberFromTitle = %d", got)
	}
	if got := numberFromTitle("Ohne Nummer"); got != 0 {
		t.Errorf("numberFromTitle = %d", got)
	}
}
