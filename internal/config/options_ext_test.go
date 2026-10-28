package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseArgsOfTheNewOptions checks the flags of the extensions.
func TestParseArgsOfTheNewOptions(t *testing.T) {
	opts, err := ParseArgs([]string{"--format", "t", "--totp", "123456", "--parallel", "3", "--delay", "2.5", "--thumb", "--meta", "--since", "2026-01-02", "--title", "Tech"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if opts.Totp != "123456" {
		t.Errorf("totp = %q", opts.Totp)
	}
	if opts.Parallel != 3 {
		t.Errorf("parallel = %d", opts.Parallel)
	}
	if opts.Delay != 2.5 {
		t.Errorf("delay = %f", opts.Delay)
	}
	if !opts.Thumb || !opts.Meta {
		t.Errorf("thumb = %v, meta = %v", opts.Thumb, opts.Meta)
	}
	if opts.Since != "2026-01-02" {
		t.Errorf("since = %q", opts.Since)
	}
	if opts.Title != "Tech" {
		t.Errorf("title = %q", opts.Title)
	}
}

func TestParseArgsRejectsBadExtensions(t *testing.T) {
	for _, args := range [][]string{{"--parallel", "9"}, {"--parallel", "0"}, {"--delay", "-2"}, {"--delay", "abc"}, {"--since", "02-03-2026"}} {
		if _, err := ParseArgs(append([]string{"--format", "t"}, args...)); err == nil {
			t.Errorf("ParseArgs %v: want an error", args)
		}
	}
}

// TestManAndCompletionsNeedNoTarget checks that they pass the target check.
func TestManAndCompletionsNeedNoTarget(t *testing.T) {
	if opts, err := ParseArgs([]string{"--man"}); err != nil || !opts.Man {
		t.Fatalf("man: opts = %+v, err = %v", opts, err)
	}
	if _, err := ParseArgs([]string{"--completions", "zsh"}); err != nil {
		t.Fatalf("completions: err = %v", err)
	}
	if _, err := ParseArgs(nil); err != ErrNoTarget {
		t.Fatalf("err = %v, want ErrNoTarget", err)
	}
}

func TestLoadConfigFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	data := "quality=hd\nlayout=flat\nparallel=2\ndelay=1,5\nthumb=1\ndownloads_dir=/data/eigens\nsubmodule=2\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	file := LoadConfigFile(path, os.Stderr)
	switch {
	case file.Quality != "hd":
		t.Errorf("quality = %q", file.Quality)
	case file.Layout != "flat":
		t.Errorf("layout = %q", file.Layout)
	case file.Parallel != 2:
		t.Errorf("parallel = %d", file.Parallel)
	case file.Delay != 1.5:
		t.Errorf("delay = %f", file.Delay)
	case !file.Thumb:
		t.Errorf("thumb = %v", file.Thumb)
	case file.DownloadsDir != "/data/eigens":
		t.Errorf("downloadsDir = %q", file.DownloadsDir)
	case file.Submodule != 2:
		t.Errorf("submodule = %d", file.Submodule)
	}
	// Eine fehlende Datei liefert ein leeres Ergebnis.
	if file := LoadConfigFile(filepath.Join(dir, "fehlt"), os.Stderr); file != (FileConfig{}) {
		t.Errorf("missing file: %+v", file)
	}
}

func TestResolveReadsTheConfigFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte("quality=hd\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := func(key string) string {
		switch key {
		case "ORANGEFLIX_CONFIG_FILE":
			return path
		}
		return ""
	}
	opts := &Options{}
	if err := opts.Resolve(env, os.Stderr); err != nil {
		t.Fatal(err)
	}
	if opts.Quality != "hd" {
		t.Errorf("quality = %q", opts.Quality)
	}
}
