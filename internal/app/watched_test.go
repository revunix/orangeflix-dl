package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// progressServer records the positions the client sends.
func progressServer(t *testing.T, duration int) (*App, *[]int) {
	t.Helper()
	sent := []int{}
	application, _ := moreApp(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v3/playback/mt420/progress":
			body, _ := io.ReadAll(r.Body)
			var payload struct {
				Time int `json:"time"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Errorf("body %q: %v", body, err)
			}
			sent = append(sent, payload.Time)
			_, _ = w.Write([]byte(`{"ok":true}`))
		case r.URL.Path == "/api/v3/content/clips/mt420":
			_, _ = w.Write([]byte(`{"ok":true}`))
		case r.URL.Path == "/api/v3/playback/mt420":
			_, _ = w.Write([]byte(`{"ok":true,"clip":{"id":"mt420","durationSeconds":` + itoa(duration) + `}}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	return application, &sent
}

func itoa(value int) string {
	b, _ := json.Marshal(value)
	return string(b)
}

func TestSetWatchedSendsTheEnd(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name    string
		episode orangeflix.Episode
		want    int
	}{
		{"seconds of the list", orangeflix.Episode{Identifier: "mt420", DurationSeconds: 1200}, 1200},
		{"running time", orangeflix.Episode{Identifier: "mt420", Duration: "00:31:52"}, 1912},
		{"asks the playback", orangeflix.Episode{Identifier: "mt420"}, 777},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			application, sent := progressServer(t, 777)
			if err := application.SetWatched(ctx, test.episode, true); err != nil {
				t.Fatal(err)
			}
			if len(*sent) != 1 || (*sent)[0] != test.want {
				t.Errorf("sent = %v, want [%d]", *sent, test.want)
			}
		})
	}
}

func TestSetWatchedOffResetsThePosition(t *testing.T) {
	application, sent := progressServer(t, 777)
	if err := application.SetWatched(context.Background(), orangeflix.Episode{Identifier: "mt420", DurationSeconds: 1200}, false); err != nil {
		t.Fatal(err)
	}
	if len(*sent) != 1 || (*sent)[0] != 0 {
		t.Errorf("sent = %v, want [0]", *sent)
	}
}

func TestClockToSeconds(t *testing.T) {
	for value, want := range map[string]int{"00:31:52": 1912, "31:52": 1912, "1:00:00": 3600, "": 0, "abc": 0, "1:x": 0} {
		if got := clockToSeconds(value); got != want {
			t.Errorf("clockToSeconds(%q) = %d, want %d", value, got, want)
		}
	}
}
