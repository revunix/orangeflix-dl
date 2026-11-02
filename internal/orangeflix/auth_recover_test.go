package orangeflix

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestARejectedTokenIsRefreshed checks that a rejected access token is
// replaced with the refresh token and does not open a new session.
func TestARejectedTokenIsRefreshed(t *testing.T) {
	var logins, refreshes atomic.Int32
	client := testV3Client(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v3/auth/refresh":
			refreshes.Add(1)
			_, _ = w.Write([]byte(`{"ok":true,"auth":{"accessToken":"fresh","refreshToken":"next","expiresIn":3600}}`))
		case "/api/v3/auth/login":
			logins.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
		default:
			if r.Header.Get("Authorization") != "Bearer fresh" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	})
	client.SetToken("old", "refresh", time.Now().Add(time.Hour))

	if _, err := client.Profile(context.Background()); err != nil {
		t.Fatalf("Profile: %v", err)
	}
	if refreshes.Load() != 1 {
		t.Fatalf("refreshes = %d, want 1", refreshes.Load())
	}
	if logins.Load() != 0 {
		t.Fatalf("logins = %d, want 0", logins.Load())
	}
	if client.AccessToken != "fresh" {
		t.Fatalf("token = %q", client.AccessToken)
	}
}

// TestParallelLoginsShareOneSession checks that several callers that find
// no token open a single session.
func TestParallelLoginsShareOneSession(t *testing.T) {
	var logins atomic.Int32
	var mu sync.Mutex
	started := make(chan struct{})
	client := testV3Client(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/auth/login" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		mu.Lock()
		if logins.Add(1) == 1 {
			close(started)
		}
		mu.Unlock()
		<-started
		_, _ = w.Write([]byte(`{"ok":true,"auth":{"accessToken":"shared","expiresIn":3600}}`))
	})

	var group sync.WaitGroup
	for range 4 {
		group.Add(1)
		go func() {
			defer group.Done()
			if err := client.EnsureToken(context.Background()); err != nil {
				t.Errorf("EnsureToken: %v", err)
			}
		}()
	}
	group.Wait()
	if logins.Load() != 1 {
		t.Fatalf("logins = %d, want 1", logins.Load())
	}
}
