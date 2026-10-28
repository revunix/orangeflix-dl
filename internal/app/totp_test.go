package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// totpApp builds an application whose account asks for the second factor.
// The API takes the code "123456" only.
func totpApp(t *testing.T) *App {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "auth/login"):
			_, _ = w.Write([]byte(`{"ok":false,"requiresTotp":true,"totpToken":"totp-1"}`))
		case strings.HasSuffix(r.URL.Path, "auth/login/totp"):
			var body struct {
				Code string `json:"code"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Code != "123456" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"ok":false,"error":"invalid code"}`))
				return
			}
			_, _ = w.Write([]byte(`{"ok":true,"auth":{"accessToken":"token-1","expiresIn":3600}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return &App{V3: &V3{Client: &orangeflix.V3Client{
		HTTP:     server.Client(),
		BaseURL:  server.URL + "/api/v3/",
		Username: "user@example.com",
		Password: "secret",
		Attempts: 1,
		Sleep:    func(context.Context, time.Duration) {},
	}}}
}

// TestTheLoginAsksForTheSecondFactor checks that a missing code is named as
// such, so the interface can ask for it.
func TestTheLoginAsksForTheSecondFactor(t *testing.T) {
	application := totpApp(t)
	err := application.EnsureV3(context.Background())
	if !errors.Is(err, ErrTotpRequired) {
		t.Fatalf("err = %v, want ErrTotpRequired", err)
	}
}

// TestTheCodeOfTheInterfaceSignsIn checks that the code of the login form
// finishes the login and is used once only.
func TestTheCodeOfTheInterfaceSignsIn(t *testing.T) {
	application := totpApp(t)
	application.SetTotp("123456")
	if err := application.EnsureV3(context.Background()); err != nil {
		t.Fatalf("EnsureV3: %v", err)
	}
	if application.V3.Client.AccessToken != "token-1" {
		t.Fatalf("token = %q", application.V3.Client.AccessToken)
	}
	if code := application.V3.takeTotp(); code != "" {
		t.Fatalf("the code was kept: %q", code)
	}
}

// TestAWrongCodeIsRejected checks that a wrong code is told apart from a
// missing one.
func TestAWrongCodeIsRejected(t *testing.T) {
	application := totpApp(t)
	application.SetTotp("000000")
	err := application.EnsureV3(context.Background())
	if !errors.Is(err, ErrTotpRejected) {
		t.Fatalf("err = %v, want ErrTotpRejected", err)
	}
}
