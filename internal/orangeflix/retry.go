package orangeflix

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// retryPolicy describes how a client repeats a request. Transport errors,
// unreadable answers, server errors and rate limits are retried; every other
// answer goes back to the caller.
type retryPolicy struct {
	attempts int
	sleep    func(context.Context, time.Duration)
	// retryRateLimit repeats a request after HTTP 429. A login must not be
	// repeated, it would only extend the lock of the account.
	retryRateLimit bool
	// describe turns the body of a rate limit into a message.
	describe func([]byte) string
	// rateLimited reports a rate limit that hides in a successful answer.
	rateLimited func([]byte) bool
}

// apiResponse is an answer that was read completely.
type apiResponse struct {
	StatusCode int
	Body       []byte
}

// send performs the request that newRequest builds, with a fresh request for
// every attempt.
func (p retryPolicy) send(ctx context.Context, client *http.Client, newRequest func() (*http.Request, error)) (apiResponse, error) {
	attempts := max(p.attempts, 1)
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return apiResponse{}, err
		}
		req, err := newRequest()
		if err != nil {
			return apiResponse{}, err
		}

		var delay time.Duration
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			delay = backoff(attempt)
		} else {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
			resp.Body.Close()
			switch {
			case readErr != nil:
				lastErr = fmt.Errorf("could not read the response: %w", readErr)
				delay = backoff(attempt)
			case resp.StatusCode == http.StatusTooManyRequests:
				lastErr = fmt.Errorf("the API is rate limited: %s", p.describeBody(body))
				if !p.retryRateLimit {
					return apiResponse{}, lastErr
				}
				delay = rateLimitDelay(resp.Header, body, attempt)
			case resp.StatusCode >= 500:
				lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateBody(body))
				delay = backoff(attempt)
			case resp.StatusCode < 400 && p.rateLimited != nil && p.rateLimited(body):
				lastErr = errors.New("the API is rate limited")
				delay = rateLimitDelay(resp.Header, body, attempt)
			default:
				return apiResponse{StatusCode: resp.StatusCode, Body: body}, nil
			}
		}
		if attempt < attempts {
			p.wait(ctx, delay)
		}
	}
	return apiResponse{}, lastErr
}

func (p retryPolicy) describeBody(body []byte) string {
	if p.describe != nil {
		return p.describe(body)
	}
	return truncateBody(body)
}

func (p retryPolicy) wait(ctx context.Context, d time.Duration) {
	if p.sleep != nil {
		p.sleep(ctx, d)
		return
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// rateLimitDelay returns the pause after a rate limit. The pause that the
// API announces is respected, in the Retry-After header or in the body,
// otherwise the backoff is used. Cutting the announced pause short would
// only provoke the next rate limit.
func rateLimitDelay(header http.Header, body []byte, attempt int) time.Duration {
	if delay, ok := retryAfterHeader(header.Get("Retry-After"), time.Now()); ok {
		return clampServerDelay(delay)
	}
	if seconds, ok := retryAfterSeconds(body); ok {
		return clampServerDelay(time.Duration(seconds) * time.Second)
	}
	return backoff(attempt)
}

func clampServerDelay(delay time.Duration) time.Duration {
	return min(max(delay, time.Second), maxServerDelay)
}

// retryAfterHeader reads the Retry-After header, which holds either a number
// of seconds or an HTTP date.
func retryAfterHeader(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second, true
	}
	if when, err := http.ParseTime(value); err == nil {
		return when.Sub(now), true
	}
	return 0, false
}

// retryAfterSeconds reads the announced pause of the API. The announcement
// sits sometimes in the error object.
func retryAfterSeconds(body []byte) (int, bool) {
	var answer struct {
		RetryAfter int `json:"retryAfter"`
		Error      struct {
			RetryAfter int `json:"retryAfter"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &answer); err == nil {
		if answer.RetryAfter > 0 {
			return answer.RetryAfter, true
		}
		if answer.Error.RetryAfter > 0 {
			return answer.Error.RetryAfter, true
		}
	}
	return 0, false
}

// isRateLimited reports whether the body carries the rate limit marker.
func isRateLimited(body []byte) bool {
	return strings.Contains(strings.ToUpper(truncateBody(body)), "RATE LIMITED")
}

// backoff returns the delay before the next attempt. The delay grows with
// every attempt, is capped at maxRateLimitDelay and carries up to 25 percent
// jitter, so parallel clients do not retry in lockstep.
func backoff(attempt int) time.Duration {
	delay := min(time.Duration(1<<uint(min(attempt-1, 30)))*time.Second, maxRateLimitDelay)
	return delay + rand.N(delay/4+1)
}
