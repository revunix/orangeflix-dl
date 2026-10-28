package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseArgsReadsTheOptions(t *testing.T) {
	opts, err := ParseArgs([]string{"--format", "mediatheke", "--episode", "420", "--quality", "hd"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if opts.Format != "mediatheke" {
		t.Errorf("format = %q", opts.Format)
	}
	if opts.Episode != "420" {
		t.Errorf("episode = %q", opts.Episode)
	}
	if opts.Quality != "hd" {
		t.Errorf("quality = %q", opts.Quality)
	}
}

func TestParseArgsAcceptsTheFormatAsArgument(t *testing.T) {
	opts, err := ParseArgs([]string{"mediatheke"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if opts.Format != "mediatheke" {
		t.Errorf("format = %q", opts.Format)
	}
}

func TestParseArgsRejectsAnUnknownOption(t *testing.T) {
	if _, err := ParseArgs([]string{"--nonsense"}); err == nil {
		t.Fatal("want an error for the unknown option")
	}
}

func TestParseArgsNeedsAFormat(t *testing.T) {
	if _, err := ParseArgs(nil); err != ErrNoTarget {
		t.Fatalf("err = %v, want ErrNoTarget", err)
	}
}

func TestParseArgsAllowsOneLibraryMode(t *testing.T) {
	if _, err := ParseArgs([]string{"--library", "--check"}); err == nil {
		t.Fatal("want an error for the two library modes")
	}
}

func TestParseArgsNeedsAValue(t *testing.T) {
	if _, err := ParseArgs([]string{"--format"}); err == nil {
		t.Fatal("want an error for the missing value")
	}
}

func TestParseArgsHelpAndVersion(t *testing.T) {
	if _, err := ParseArgs([]string{"--help"}); err != ErrHelp {
		t.Errorf("err = %v, want ErrHelp", err)
	}
	if _, err := ParseArgs([]string{"--version"}); err != ErrVersion {
		t.Errorf("err = %v, want ErrVersion", err)
	}
}

func TestValidateChecksTheSelection(t *testing.T) {
	opts := &Options{Episode: "420"}
	if err := opts.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}

	opts = &Options{Episode: "abc"}
	if err := opts.Validate(); err == nil {
		t.Error("want an error for the episode number")
	}

	opts = &Options{EpisodeRange: "10-20"}
	if err := opts.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}

	opts = &Options{EpisodeRange: "10"}
	if err := opts.Validate(); err == nil {
		t.Error("want an error for the range")
	}

	opts = &Options{Year: "2026"}
	if err := opts.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestResolveReadsTheEnvironment(t *testing.T) {
	opts := &Options{}
	env := func(key string) string {
		switch key {
		case "ORANGEFLIX_USER":
			return "user@example.com"
		case "ORANGEFLIX_PASSWORD":
			return "secret"
		case "ORANGEFLIX_QUALITY":
			return "hd"
		}
		return ""
	}
	var stderr bytes.Buffer
	if err := opts.Resolve(env, &stderr); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if opts.Username != "user@example.com" || opts.Password != "secret" {
		t.Errorf("credentials = %q/%q", opts.Username, opts.Password)
	}
	if opts.Quality != "hd" {
		t.Errorf("quality = %q", opts.Quality)
	}
}

func TestResolveReadsTheCredentialsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials")
	content := "username=file@example.com\npassword=file-secret\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	opts := &Options{}
	env := func(key string) string {
		if key == "ORANGEFLIX_CREDENTIALS_FILE" {
			return path
		}
		return ""
	}
	var stderr bytes.Buffer
	if err := opts.Resolve(env, &stderr); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if opts.Username != "file@example.com" || opts.Password != "file-secret" {
		t.Errorf("credentials = %q/%q", opts.Username, opts.Password)
	}
}

func TestLoadCredentialsFileIgnoresTheComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials")
	content := "# ein Kommentar\nusername=alice\nORANGEFLIX_USER=bob\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	var stderr bytes.Buffer
	username, password := LoadCredentialsFile(path, &stderr)
	if username != "alice" {
		t.Errorf("username = %q", username)
	}
	if password != "" {
		t.Errorf("password = %q", password)
	}
}

func TestCredentialsPathUsesTheEnvironment(t *testing.T) {
	env := func(key string) string {
		if key == "ORANGEFLIX_CREDENTIALS_FILE" {
			return "/tmp/credentials"
		}
		return ""
	}
	if got := CredentialsPath(env); got != "/tmp/credentials" {
		t.Errorf("CredentialsPath = %q", got)
	}
}

func TestLibraryPathUsesTheEnvironment(t *testing.T) {
	env := func(key string) string {
		if key == "ORANGEFLIX_LIBRARY" {
			return "/tmp/library.db"
		}
		return ""
	}
	if got := LibraryPath(env, "/work"); got != "/tmp/library.db" {
		t.Errorf("LibraryPath = %q", got)
	}
}

func TestDownloadsDirUsesTheEnvironment(t *testing.T) {
	env := func(key string) string {
		if key == "ORANGEFLIX_DOWNLOADS_DIR" {
			return "/tmp/downloads"
		}
		return ""
	}
	if got := DownloadsDir(env, "/work"); got != "/tmp/downloads" {
		t.Errorf("DownloadsDir = %q", got)
	}
}

func TestExpandPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	if got := ExpandPath("~/Videos"); got != filepath.Join(home, "Videos") {
		t.Errorf("ExpandPath = %q", got)
	}
	if got := ExpandPath("~"); got != home {
		t.Errorf("ExpandPath = %q", got)
	}
	if got := ExpandPath("  /tmp/videos  "); got != "/tmp/videos" {
		t.Errorf("ExpandPath = %q", got)
	}
}

func TestEnsureWritableDirCreatesTheDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b")
	if err := EnsureWritableDir(dir); err != nil {
		t.Fatalf("EnsureWritableDir: %v", err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Errorf("the directory is missing: %v", err)
	}

	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := EnsureWritableDir(file); err == nil {
		t.Error("want an error for a file")
	}
}

func TestCredentialsFileWarnsAboutThePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials")
	if err := os.WriteFile(path, []byte("username=alice\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var stderr bytes.Buffer
	LoadCredentialsFile(path, &stderr)
	if !strings.Contains(stderr.String(), "chmod 600") {
		t.Errorf("the warning is missing: %q", stderr.String())
	}
}

func TestValidateQuality(t *testing.T) {
	for _, quality := range []string{"", "best", "hd", "sd", "mobile", "audio", "HD"} {
		if err := ValidateQuality(quality); err != nil {
			t.Errorf("ValidateQuality(%q): %v", quality, err)
		}
	}
	if err := ValidateQuality("fhd"); err == nil {
		t.Error("want an error for the unknown quality")
	}
}

func TestValidateFormat(t *testing.T) {
	for _, format := range []string{"mediatheke", "Die Mediatheke", "23", "999"} {
		if err := ValidateFormat(format); err != nil {
			t.Errorf("ValidateFormat(%q): %v", format, err)
		}
	}
	if err := ValidateFormat("gibtsnicht"); err == nil {
		t.Error("want an error for the unknown format")
	}
	if err := ValidateFormat(""); err == nil {
		t.Error("want an error for the empty format")
	}
}
