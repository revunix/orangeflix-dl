// Package download implements the HTTP download of the Orangeflix files and
// the bookkeeping around it: skipping existing files, resuming interrupted
// downloads and the final rename.
package download

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Defaults of the downloader.
const (
	// MaxRedirects is the number of redirects that are followed.
	MaxRedirects = 5
	// StallTimeout is the longest time without data before a download is
	// treated as broken. A total limit would abort every large file on a
	// slow line, so only a stalled transfer counts.
	StallTimeout = 2 * time.Minute
	// Attempts is the number of tries for a download. Every try continues
	// the partial file of the one before.
	Attempts = 3
	// PartSuffix is the ending of the file of an unfinished download.
	PartSuffix = ".part"
	// chunkSize is the size of the copy buffer.
	chunkSize = 256 << 10
	// progressInterval is the shortest time between two progress reports.
	progressInterval = 500 * time.Millisecond
	// maxRetryDelay caps the pause between two tries.
	maxRetryDelay = 30 * time.Second
)

// errStalled marks a transfer that delivered no data for StallTimeout.
var errStalled = errors.New("the server sends no data any more")

// Credentials carry the data for the Authorization header.
type Credentials struct {
	// Username and Password are used for the basic authentication of the
	// legacy API.
	Username string
	Password string
	// Bearer is the access token of the version 3 API.
	Bearer string
}

// Downloader downloads a file over HTTP.
type Downloader struct {
	HTTP *http.Client
	// UA is the browser identification of the requests.
	UA string
	// StallTimeout overrides the default StallTimeout.
	StallTimeout time.Duration
	// Attempts overrides the default number of tries.
	Attempts int
	// Sleep is injectable for the tests.
	Sleep func(context.Context, time.Duration)

	Logf func(format string, args ...any)
}

func (d *Downloader) logf(format string, args ...any) {
	if d.Logf != nil {
		d.Logf(format, args...)
	}
}

// httpClient returns a copy of the client, so the redirect policy does not
// change the client of the caller.
func (d *Downloader) httpClient() *http.Client {
	client := &http.Client{}
	if d.HTTP != nil {
		clone := *d.HTTP
		client = &clone
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= MaxRedirects {
			return fmt.Errorf("too many redirects")
		}
		return nil
	}
	return client
}

func (d *Downloader) stallTimeout() time.Duration {
	if d.StallTimeout > 0 {
		return d.StallTimeout
	}
	return StallTimeout
}

func (d *Downloader) attempts() int {
	if d.Attempts > 0 {
		return d.Attempts
	}
	return Attempts
}

func (d *Downloader) sleep(ctx context.Context, delay time.Duration) {
	if d.Sleep != nil {
		d.Sleep(ctx, delay)
		return
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// retryableError marks a failure that a new try can overcome: a broken
// connection, a stalled transfer or an error of the server.
type retryableError struct{ err error }

func (e *retryableError) Error() string { return e.err.Error() }
func (e *retryableError) Unwrap() error { return e.err }

func retryable(err error) error { return &retryableError{err: err} }

// Retryable reports whether a new try could finish the download.
func Retryable(err error) bool {
	var target *retryableError
	return errors.As(err, &target)
}

// Result describes a finished download.
type Result struct {
	// Path is the file that was written.
	Path string
	// Size is the number of bytes that were transferred.
	Size int64
	// MD5 is the checksum of the file.
	MD5 string
}

// Download loads the URL into the target file. An interrupted download is
// continued: the data is written to "<target>.part" and the file is renamed
// when it is complete. A broken connection is retried a few times; the
// partial file stays when all tries fail, so the next run continues it.
func (d *Downloader) Download(ctx context.Context, rawURL, target string, creds *Credentials, progress func(done, total int64)) (Result, error) {
	if target == "" {
		return Result{}, errors.New("no target file")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return Result{}, err
	}
	attempts := d.attempts()
	for attempt := 1; ; attempt++ {
		result, err := d.attempt(ctx, rawURL, target, creds, progress)
		if err == nil || attempt >= attempts || ctx.Err() != nil || !Retryable(err) {
			return result, err
		}
		delay := min(time.Duration(1<<uint(attempt-1))*time.Second, maxRetryDelay)
		d.logf("Download unterbrochen (%s), neuer Versuch %d/%d in %s", err.Error(), attempt+1, attempts, delay)
		d.sleep(ctx, delay)
	}
}

// attempt performs a single try of the download.
func (d *Downloader) attempt(parent context.Context, rawURL, target string, creds *Credentials, progress func(done, total int64)) (Result, error) {
	part := target + PartSuffix

	resumeFrom := int64(0)
	if info, err := os.Stat(part); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
		resumeFrom = info.Size()
		d.logf("Fortsetzung ab %d Bytes", resumeFrom)
	}

	// The watchdog cancels the request when no data arrives for the stall
	// timeout, while the server answers and while the body is read.
	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)
	stall := d.stallTimeout()
	watchdog := time.AfterFunc(stall, func() { cancel(errStalled) })
	defer watchdog.Stop()
	// failure turns an error of the transfer into the stall error when the
	// watchdog fired.
	failure := func(err error) error {
		if errors.Is(context.Cause(ctx), errStalled) && parent.Err() == nil {
			return retryable(fmt.Errorf("download failed: %w", errStalled))
		}
		if parent.Err() != nil {
			return parent.Err()
		}
		return retryable(fmt.Errorf("download failed: %w", err))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Result{}, err
	}
	if d.UA != "" {
		req.Header.Set("User-Agent", d.UA)
	}
	// The files of the delivery network work without credentials, so the
	// access data only travels to the API.
	if creds != nil {
		if creds.Bearer != "" {
			req.Header.Set("Authorization", "Bearer "+creds.Bearer)
		} else if creds.Username != "" || creds.Password != "" {
			req.SetBasicAuth(creds.Username, creds.Password)
		}
	}
	if resumeFrom > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(resumeFrom, 10)+"-")
	}

	resp, err := d.httpClient().Do(req)
	if err != nil {
		return Result{}, failure(err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return Result{}, errors.New("authentication failed (HTTP 401): invalid credentials")
	case resp.StatusCode == http.StatusOK:
		// The server ignored the range and starts over.
		if resumeFrom > 0 {
			d.logf("Server unterstützt keine Fortsetzung, Download beginnt von vorn")
			resumeFrom = 0
		}
	case resp.StatusCode == http.StatusPartialContent:
		if resumeFrom > 0 {
			if start, ok := rangeStart(resp.Header.Get("Content-Range")); !ok || start != resumeFrom {
				// The partial file does not fit to the answer, so it is
				// useless and would only produce a broken file. The next
				// try starts from the beginning.
				_ = os.Remove(part)
				return Result{}, retryable(errors.New("the server rejected the resume of the download"))
			}
		}
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && resumeFrom > 0:
		// The partial file is longer than the file on the server.
		_ = os.Remove(part)
		return Result{}, retryable(errors.New("the partial file does not fit to the file on the server"))
	case resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests:
		return Result{}, retryable(fmt.Errorf("HTTP %d", resp.StatusCode))
	default:
		return Result{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// The checksum covers the whole file, so the data of the partial file is
	// hashed before the new data is appended. rangeHash covers the new data
	// only: for a partial answer the server may announce the checksum of the
	// transferred range.
	fullHash := md5.New()
	rangeHash := md5.New()
	if resumeFrom > 0 {
		if err := hashFile(part, fullHash); err != nil {
			return Result{}, err
		}
	}

	flags := os.O_CREATE | os.O_WRONLY
	if resumeFrom > 0 {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	file, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return Result{}, err
	}

	writer := io.MultiWriter(file, fullHash, rangeHash)
	written := resumeFrom
	total := contentLength(resp)

	buffer := make([]byte, chunkSize)
	// The progress is reported at most every progressInterval, so a fast
	// download does not flood the interface.
	lastReport := time.Now()
	for {
		read, readErr := resp.Body.Read(buffer)
		if read > 0 {
			watchdog.Reset(stall)
			if _, err := writer.Write(buffer[:read]); err != nil {
				file.Close()
				return Result{}, err
			}
			written += int64(read)
			if progress != nil && time.Since(lastReport) >= progressInterval {
				lastReport = time.Now()
				progress(written, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			file.Close()
			// The partial file stays: the next try continues it.
			return Result{}, failure(readErr)
		}
	}
	if progress != nil {
		progress(written, total)
	}
	if err := file.Close(); err != nil {
		return Result{}, err
	}

	if total > 0 && written != total {
		// The partial file stays: the next try continues it.
		return Result{}, retryable(fmt.Errorf("the download is incomplete: %d of %d bytes", written, total))
	}

	checksum := hex.EncodeToString(fullHash.Sum(nil))
	if expected := expectedChecksum(resp.Header); expected != "" && expected != checksum {
		if resumeFrom == 0 || expected != hex.EncodeToString(rangeHash.Sum(nil)) {
			// The partial file is broken, so it is removed: the next run
			// starts from the beginning.
			_ = os.Remove(part)
			return Result{}, fmt.Errorf("the checksum does not match the one of the server")
		}
	}

	if err := os.Rename(part, target); err != nil {
		return Result{}, err
	}
	return Result{Path: target, Size: written, MD5: checksum}, nil
}

// hashFile feeds the content of the file into the hash.
func hashFile(path string, hash io.Writer) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(hash, file)
	return err
}

// contentLength returns the size of the whole file, 0 when the server does
// not announce it. A partial answer only carries the length of the rest, so
// the total is taken from the content range.
func contentLength(resp *http.Response) int64 {
	if resp.StatusCode == http.StatusPartialContent {
		if total, ok := rangeTotal(resp.Header.Get("Content-Range")); ok {
			return total
		}
	}
	if resp.ContentLength > 0 {
		return resp.ContentLength
	}
	return 0
}

// rangeStart returns the first byte of a content range header.
func rangeStart(header string) (int64, bool) {
	_, rest, found := strings.Cut(header, "bytes ")
	if !found {
		return 0, false
	}
	start, _, found := strings.Cut(rest, "-")
	if !found {
		return 0, false
	}
	value, err := strconv.ParseInt(strings.TrimSpace(start), 10, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// rangeTotal returns the total size of a content range header.
func rangeTotal(header string) (int64, bool) {
	_, rest, found := strings.Cut(header, "/")
	if !found {
		return 0, false
	}
	total, _, found := strings.Cut(strings.TrimSpace(rest), " ")
	if !found {
		total = strings.TrimSpace(rest)
	}
	value, err := strconv.ParseInt(strings.TrimSpace(total), 10, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// expectedChecksum returns the checksum the server announces, in hex.
func expectedChecksum(header http.Header) string {
	raw := header.Get("Content-MD5")
	if raw == "" {
		raw = header.Get("X-Md5")
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if len(raw) == 32 {
		if _, err := hex.DecodeString(raw); err == nil {
			return strings.ToLower(raw)
		}
	}
	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil && len(decoded) == md5.Size {
		return hex.EncodeToString(decoded)
	}
	return ""
}
