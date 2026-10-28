package orangeflix

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestRetryAfterHeaderReadsSecondsAndDates(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		value string
		want  time.Duration
		ok    bool
	}{
		{"", 0, false},
		{"abc", 0, false},
		{"120", 2 * time.Minute, true},
		{now.Add(90 * time.Second).Format(http.TimeFormat), 90 * time.Second, true},
	}
	for _, tc := range cases {
		got, ok := retryAfterHeader(tc.value, now)
		if ok != tc.ok || got != tc.want {
			t.Errorf("retryAfterHeader(%q) = %v, %v; want %v, %v", tc.value, got, ok, tc.want, tc.ok)
		}
	}
}

func TestRateLimitPrefersTheRetryAfterHeader(t *testing.T) {
	var pauses []time.Duration
	calls := 0
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"retryAfter":60}`))
			return
		}
		_, _ = w.Write([]byte(`{"eps":[]}`))
	})
	client.Attempts = 2
	client.Sleep = func(_ context.Context, d time.Duration) { pauses = append(pauses, d) }

	if _, err := client.Feed(context.Background(), 23, 1); err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if len(pauses) != 1 || pauses[0] != 7*time.Second {
		t.Errorf("pauses = %v, want [7s]", pauses)
	}
}

func TestBackoffStaysWithinTheJitter(t *testing.T) {
	for attempt := 1; attempt <= 10; attempt++ {
		base := min(time.Duration(1<<uint(attempt-1))*time.Second, maxRateLimitDelay)
		for range 20 {
			got := backoff(attempt)
			if got < base || got > base+base/4 {
				t.Fatalf("backoff(%d) = %v, want between %v and %v", attempt, got, base, base+base/4)
			}
		}
	}
}
