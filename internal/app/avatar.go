package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ChatAvatarURL is the address of the pictures of the chat users; the name
// follows it. The web chat uses the same address.
var ChatAvatarURL = "https://orangeflix.de/api/live/chat-avatar/"

// chatAvatarLimit is the largest picture that is read. The server sends
// 50 by 50 pixels.
const chatAvatarLimit = 256 << 10

// avatarClient loads the pictures; a slow server must not hold the chat.
var avatarClient = &http.Client{Timeout: 10 * time.Second}

// ChatAvatar loads the picture of a chat user.
func (a *App) ChatAvatar(ctx context.Context, name string) ([]byte, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("no name")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ChatAvatarURL+url.PathEscape(name), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "orangeflix-dl")
	req.Header.Set("Accept", "image/*")
	resp, err := avatarClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("avatar of %s: %s", name, resp.Status)
	}
	if kind := resp.Header.Get("Content-Type"); kind != "" && !strings.HasPrefix(kind, "image/") {
		return nil, fmt.Errorf("avatar of %s: %s", name, kind)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, chatAvatarLimit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > chatAvatarLimit {
		return nil, fmt.Errorf("avatar of %s is too large", name)
	}
	return data, nil
}
