package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChatAvatarLoadsThePicture(t *testing.T) {
	var asked string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.EscapedPath()
		switch {
		case strings.HasSuffix(r.URL.Path, "/Text"):
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html>"))
		case strings.HasSuffix(r.URL.Path, "/Groß"):
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(make([]byte, chatAvatarLimit+10))
		default:
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("png"))
		}
	}))
	t.Cleanup(server.Close)
	previous := ChatAvatarURL
	ChatAvatarURL = server.URL + "/api/live/chat-avatar/"
	t.Cleanup(func() { ChatAvatarURL = previous })

	application := &App{}
	data, err := application.ChatAvatar(context.Background(), "Jürgen B")
	if err != nil || string(data) != "png" {
		t.Fatalf("data %q, err %v", data, err)
	}
	if asked != "/api/live/chat-avatar/J%C3%BCrgen%20B" {
		t.Errorf("path = %q", asked)
	}
	if _, err := application.ChatAvatar(context.Background(), "Text"); err == nil {
		t.Error("a page was taken for a picture")
	}
	if _, err := application.ChatAvatar(context.Background(), "Groß"); err == nil {
		t.Error("a huge answer was read")
	}
	if _, err := application.ChatAvatar(context.Background(), " "); err == nil {
		t.Error("an empty name was asked for")
	}
}
