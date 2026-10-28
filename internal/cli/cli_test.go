package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/revunix/orangeflix-dl/internal/config"
	"github.com/revunix/orangeflix-dl/internal/secret"
	"github.com/revunix/orangeflix-dl/internal/store"
)

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"--help"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Errorf("code = %d", code)
	}
	if !strings.Contains(stdout.String(), "Usage: orangeflix-dl") {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"--version"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Errorf("code = %d", code)
	}
	if !strings.Contains(stdout.String(), "orangeflix-dl") {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestRunRejectsAnUnknownOption(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"--nonsense"}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "unknown option") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestRunRejectsAWrongQuality(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"--format", "mediatheke", "--quality", "fhd"},
		strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "Qualität") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestRunListsTheFormats(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"--list"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Errorf("code = %d", code)
	}
	if !strings.Contains(stdout.String(), "mediatheke") {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestRunListsTheFormatsAsJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"--list", "--json"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Errorf("code = %d", code)
	}
	var entries []struct {
		PID   int    `json:"pid"`
		Slug  string `json:"slug"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &entries); err != nil {
		t.Fatalf("the output is no JSON: %v (%q)", err, stdout.String())
	}
	if len(entries) == 0 {
		t.Error("the list is empty")
	}
}

func TestRunNeedsAFormatWithoutATerminal(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), nil, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Errorf("code = %d, want 1", code)
	}
}

func TestRunNeedsTheCredentials(t *testing.T) {
	t.Setenv("ORANGEFLIX_USER", "")
	t.Setenv("ORANGEFLIX_PASSWORD", "")

	// Ohne Zugangsdaten in der Umgebung, im Schlüsselbund und in der
	// Bibliothek muss das Programm die Eingabe verlangen.
	opts := &config.Options{}
	hasToken, err := resolveCredentials(opts, config.OSEnv, emptyStore{}, nil)
	if err != nil {
		t.Fatalf("resolveCredentials: %v", err)
	}
	if hasToken {
		t.Error("there is a token although none is stored")
	}
	if opts.Username != "" || opts.Password != "" {
		t.Errorf("credentials = %q/%q", opts.Username, opts.Password)
	}
}

func TestRunUsesTheStoredCredentials(t *testing.T) {
	t.Setenv("ORANGEFLIX_USER", "")
	t.Setenv("ORANGEFLIX_PASSWORD", "")

	// Das gespeicherte Passwort wird genommen, der Benutzername aus der
	// Bibliothek.
	opts := &config.Options{}
	hasToken, err := resolveCredentials(opts, config.OSEnv, storedStore{}, nil)
	if err != nil {
		t.Fatalf("resolveCredentials: %v", err)
	}
	if hasToken {
		t.Error("there is no token in the store")
	}
	if opts.Password != "secret" {
		t.Errorf("password = %q", opts.Password)
	}
}

// emptyStore is a store without any data.
type emptyStore struct{}

func (emptyStore) Get() (string, error)      { return "", secret.ErrNotFound }
func (emptyStore) Set(string) error          { return nil }
func (emptyStore) Clear() error              { return nil }
func (emptyStore) Backend() string           { return "leer" }
func (emptyStore) GetToken() (string, error) { return "", secret.ErrNotFound }
func (emptyStore) SetToken(string) error     { return nil }
func (emptyStore) ClearToken() error         { return nil }

// storedStore holds a password.
type storedStore struct{}

func (storedStore) Get() (string, error)      { return "secret", nil }
func (storedStore) Set(string) error          { return nil }
func (storedStore) Clear() error              { return nil }
func (storedStore) Backend() string           { return "leer" }
func (storedStore) GetToken() (string, error) { return "", secret.ErrNotFound }
func (storedStore) SetToken(string) error     { return nil }
func (storedStore) ClearToken() error         { return nil }

func TestStoredDownloadsDirIsUsed(t *testing.T) {
	dir := t.TempDir()
	state, err := store.Open(filepath.Join(dir, "library.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer state.Close()
	if err := state.SetSetting(store.SettingDownloadsDir, filepath.Join(dir, "archiv")); err != nil {
		t.Fatalf("setting: %v", err)
	}

	opts := &config.Options{DownloadsDir: filepath.Join(dir, "downloads")}
	applyStoredDownloadsDir(opts, state, func(string) string { return "" })
	if opts.DownloadsDir != filepath.Join(dir, "archiv") {
		t.Errorf("DownloadsDir = %q", opts.DownloadsDir)
	}

	// Die Umgebung hat Vorrang vor dem gespeicherten Ordner.
	opts.DownloadsDir = filepath.Join(dir, "downloads")
	env := func(key string) string {
		if key == "ORANGEFLIX_DOWNLOADS_DIR" {
			return filepath.Join(dir, "umgebung")
		}
		return ""
	}
	applyStoredDownloadsDir(opts, state, env)
	if opts.DownloadsDir != filepath.Join(dir, "downloads") {
		t.Errorf("DownloadsDir = %q", opts.DownloadsDir)
	}
}
