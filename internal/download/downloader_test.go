package download

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDownloadWritesTheFile(t *testing.T) {
	content := []byte("video data")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, password, ok := r.BasicAuth(); !ok || user != "user" || password != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(content)))
		_, _ = w.Write(content)
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "file.mp4")
	downloader := &Downloader{HTTP: server.Client()}
	creds := &Credentials{Username: "user", Password: "secret"}

	result, err := downloader.Download(context.Background(), server.URL+"/file.mp4", target, creds, nil)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("content = %q", string(data))
	}
	if result.Size != int64(len(content)) {
		t.Errorf("size = %d", result.Size)
	}
	if _, err := os.Stat(target + ".part"); err == nil {
		t.Error("the partial file was not removed")
	}
}

func TestDownloadContinuesThePartialFile(t *testing.T) {
	content := []byte("0123456789")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") == "bytes=4-" {
			w.Header().Set("Content-Range", "bytes 4-9/10")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(content[4:])
			return
		}
		_, _ = w.Write(content)
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "file.mp4")
	if err := os.WriteFile(target+".part", content[:4], 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	downloader := &Downloader{HTTP: server.Client()}
	if _, err := downloader.Download(context.Background(), server.URL+"/file.mp4", target, nil, nil); err != nil {
		t.Fatalf("Download: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("content = %q, want %q", string(data), string(content))
	}
}

func TestDownloadStartsOverWhenTheServerIgnoresTheRange(t *testing.T) {
	content := []byte("0123456789")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "file.mp4")
	if err := os.WriteFile(target+".part", content[:4], 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	downloader := &Downloader{HTTP: server.Client()}
	if _, err := downloader.Download(context.Background(), server.URL+"/file.mp4", target, nil, nil); err != nil {
		t.Fatalf("Download: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("content = %q, want %q", string(data), string(content))
	}
}

func TestDownloadChecksTheChecksum(t *testing.T) {
	content := []byte("video data")
	sum := md5.Sum(content)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-MD5", base64.StdEncoding.EncodeToString(sum[:]))
		_, _ = w.Write(content)
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "file.mp4")
	downloader := &Downloader{HTTP: server.Client()}
	if _, err := downloader.Download(context.Background(), server.URL+"/file.mp4", target, nil, nil); err != nil {
		t.Fatalf("Download: %v", err)
	}
}

func TestDownloadReportsAWrongChecksum(t *testing.T) {
	content := []byte("video data")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-MD5", "00000000000000000000000000000000")
		_, _ = w.Write(content)
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "file.mp4")
	downloader := &Downloader{HTTP: server.Client()}
	_, err := downloader.Download(context.Background(), server.URL+"/file.mp4", target, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("err = %v, want the checksum error", err)
	}
}

func TestDownloadReportsWrongCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "file.mp4")
	downloader := &Downloader{HTTP: server.Client()}
	wrong := &Credentials{Username: "user", Password: "wrong"}
	if _, err := downloader.Download(context.Background(), server.URL+"/file.mp4", target, wrong, nil); err == nil {
		t.Fatal("want an error for the wrong credentials")
	}
}

func TestDownloadFollowsARedirect(t *testing.T) {
	content := []byte("video data")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/file.mp4", http.StatusFound)
			return
		}
		_, _ = w.Write(content)
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "file.mp4")
	downloader := &Downloader{HTTP: server.Client()}
	result, err := downloader.Download(context.Background(), server.URL+"/redirect", target, nil, nil)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if result.Size != int64(len(content)) {
		t.Errorf("size = %d", result.Size)
	}
}

func TestDownloadReportsTheProgress(t *testing.T) {
	content := []byte("0123456789")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "file.mp4")
	downloader := &Downloader{HTTP: server.Client()}
	var last int64
	if _, err := downloader.Download(context.Background(), server.URL+"/file.mp4", target, nil,
		func(done, total int64) { last = done }); err != nil {
		t.Fatalf("Download: %v", err)
	}
	if last != int64(len(content)) {
		t.Errorf("progress = %d", last)
	}
}

func TestExpectedChecksum(t *testing.T) {
	header := http.Header{}
	header.Set("Content-MD5", "d41d8cd98f00b204e9800998ecf8427e")
	if got := expectedChecksum(header); got != "d41d8cd98f00b204e9800998ecf8427e" {
		t.Errorf("checksum = %q", got)
	}

	header = http.Header{}
	header.Set("Content-MD5", "1B2M2Y8AsgTpgAmY7PhCfg==")
	if got := expectedChecksum(header); got != "d41d8cd98f00b204e9800998ecf8427e" {
		t.Errorf("checksum = %q", got)
	}

	header = http.Header{}
	if got := expectedChecksum(header); got != "" {
		t.Errorf("checksum = %q, want the empty string", got)
	}
}

func TestRangeStart(t *testing.T) {
	value, ok := rangeStart("bytes 100-199/200")
	if !ok || value != 100 {
		t.Errorf("rangeStart = %d, %v", value, ok)
	}
	if _, ok := rangeStart("bytes */200"); ok {
		t.Error("want no value for the unknown range")
	}
}

func TestManagerSkipsTheExistingFile(t *testing.T) {
	dir := t.TempDir()
	manager := &Manager{DownloadsDir: dir}

	naming := testNaming()
	path := filepath.Join(dir, filepath.FromSlash(naming.RelativeFile(".mp4")))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	existing, err := manager.Prepare(naming)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if existing != path {
		t.Errorf("existing = %q, want %q", existing, path)
	}
}

func TestManagerRemovesTheFileWithForce(t *testing.T) {
	dir := t.TempDir()
	manager := &Manager{DownloadsDir: dir, Force: true}

	naming := testNaming()
	path := filepath.Join(dir, filepath.FromSlash(naming.RelativeFile(".mp4")))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	existing, err := manager.Prepare(naming)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if existing != "" {
		t.Error("the file was skipped although the force flag is set")
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("the file was not removed")
	}
}

func TestDownloadTimesOut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "file.mp4")
	downloader := &Downloader{HTTP: server.Client(), StallTimeout: 20 * time.Millisecond, Attempts: 1}
	_, err := downloader.Download(context.Background(), server.URL+"/file.mp4", target, nil, nil)
	if !errors.Is(err, errStalled) || !Retryable(err) {
		t.Fatalf("err = %v, want the retryable stall error", err)
	}
}

func TestDownloadKeepsThePartialFileWhenTheTransferStalls(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "10")
		_, _ = w.Write([]byte("0123"))
		w.(http.Flusher).Flush()
		<-release
	}))
	defer server.Close()
	defer close(release)

	target := filepath.Join(t.TempDir(), "file.mp4")
	downloader := &Downloader{HTTP: server.Client(), StallTimeout: 50 * time.Millisecond, Attempts: 1}
	if _, err := downloader.Download(context.Background(), server.URL+"/file.mp4", target, nil, nil); !errors.Is(err, errStalled) {
		t.Fatalf("err = %v, want the stall error", err)
	}
	data, err := os.ReadFile(target + PartSuffix)
	if err != nil || string(data) != "0123" {
		t.Fatalf("partial file = %q, %v; want the received data", data, err)
	}
}

func TestDownloadRetriesAndContinuesAfterABrokenConnection(t *testing.T) {
	content := []byte("0123456789")
	sum := md5.Sum(content)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			// The connection breaks after four bytes.
			w.Header().Set("Content-Length", "10")
			_, _ = w.Write(content[:4])
			w.(http.Flusher).Flush()
			conn, _, _ := w.(http.Hijacker).Hijack()
			conn.Close()
			return
		}
		if r.Header.Get("Range") != "bytes=4-" {
			t.Errorf("range = %q, want the rest", r.Header.Get("Range"))
		}
		w.Header().Set("Content-Range", "bytes 4-9/10")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(content[4:])
	}))
	defer server.Close()

	var pauses []time.Duration
	target := filepath.Join(t.TempDir(), "file.mp4")
	downloader := &Downloader{
		HTTP:  server.Client(),
		Sleep: func(_ context.Context, d time.Duration) { pauses = append(pauses, d) },
	}
	result, err := downloader.Download(context.Background(), server.URL+"/file.mp4", target, nil, nil)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if calls != 2 || len(pauses) != 1 {
		t.Errorf("calls = %d, pauses = %v; want one retry", calls, pauses)
	}
	if result.MD5 != hex.EncodeToString(sum[:]) {
		t.Errorf("md5 = %s, want the checksum of the whole file", result.MD5)
	}
}

func TestDownloadChecksTheWholeFileAfterAResume(t *testing.T) {
	content := []byte("0123456789")
	sum := md5.Sum(content)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-MD5", base64.StdEncoding.EncodeToString(sum[:]))
		w.Header().Set("Content-Range", "bytes 4-9/10")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(content[4:])
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "file.mp4")
	if err := os.WriteFile(target+PartSuffix, content[:4], 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	downloader := &Downloader{HTTP: server.Client()}
	result, err := downloader.Download(context.Background(), server.URL+"/file.mp4", target, nil, nil)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if result.MD5 != hex.EncodeToString(sum[:]) {
		t.Errorf("md5 = %s, want the checksum of the whole file", result.MD5)
	}
}

func TestDownloadDoesNotRetryAClientError(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "file.mp4")
	downloader := &Downloader{HTTP: server.Client(), Sleep: func(context.Context, time.Duration) {}}
	if _, err := downloader.Download(context.Background(), server.URL+"/file.mp4", target, nil, nil); err == nil {
		t.Fatal("want an error for HTTP 404")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}
