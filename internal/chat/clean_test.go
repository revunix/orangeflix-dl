package chat

import (
	"bytes"
	"encoding/json"
	"log"
	"testing"
)

func TestCleanTextTurnsMarkupIntoText(t *testing.T) {
	cases := map[string]string{
		"Kannst du mal schauen @Spymac <br/>Ist nicht das erste Mal": "Kannst du mal schauen @Spymac Ist nicht das erste Mal",
		"eins<br>zwei<BR />drei":                          "eins zwei drei",
		`<a href="https://x.de">Link</a> und <b>fett</b>`: "Link und fett",
		"ich &lt;3 Holger":                                "ich <3 Holger",
		"a &lt;b&gt; c":                                   "a <b> c",
		"doppelt &lt;br/&gt;escaped":                      "doppelt escaped",
		"Tom &amp; Jerry":                                 "Tom & Jerry",
	}
	for raw, want := range cases {
		if got := cleanText(raw); got != want {
			t.Errorf("cleanText(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestPinnedTextReadsTheShapes(t *testing.T) {
	cases := map[string]string{
		`"Heute mit Gast"`:                     "Heute mit Gast",
		`{"text":"Heute <br/>mit Gast"}`:       "Heute mit Gast",
		`{"txt":"Pause bis 20 Uhr"}`:           "Pause bis 20 Uhr",
		`[{"msg":"Erster"},{"msg":"Zweiter"}]`: "Erster",
		`null`:                                 "",
		`""`:                                   "",
		`{}`:                                   "",
	}
	for raw, want := range cases {
		if got := pinnedText(json.RawMessage(raw)); got != want {
			t.Errorf("pinnedText(%s) = %q, want %q", raw, got, want)
		}
	}
}

// TestTheLibraryStaysQuiet checks that the chat hands the library a logger
// that writes nothing, so its packet diagnostics never reach the terminal.
func TestTheLibraryStaysQuiet(t *testing.T) {
	var out bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&out)
	t.Cleanup(func() { log.SetOutput(previous) })

	logger := quietLogger{}
	logger.Debugf("receiveWs: %d bytes", 8045)
	logger.Infof("open ws")
	logger.Warnf("No handlers for event: %s", "pinnedmsg")
	logger.Errorf("wsReadLoop: context canceled")
	if out.Len() != 0 {
		t.Errorf("the logger wrote: %q", out.String())
	}
}
