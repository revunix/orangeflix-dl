// Package config parses the command line, resolves the credentials and
// computes the paths of the application.
package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// Env looks up an environment variable; injectable for the tests.
type Env func(string) string

// OSEnv is the process environment.
func OSEnv(key string) string { return os.Getenv(key) }

// Errors the caller handles itself.
var (
	// ErrHelp signals that the usage should be printed.
	ErrHelp = errors.New("help requested")
	// ErrVersion signals that the version should be printed.
	ErrVersion = errors.New("version requested")
	// ErrNoTarget signals that neither a format nor a list was requested.
	ErrNoTarget = errors.New("URL or --format required")
)

// Options is the parsed command line plus the resolved paths.
type Options struct {
	// Format is the selected format: a slug, a title or a numeric id.
	Format string
	// Episode is a single episode number.
	Episode string
	// EpisodeRange is a range of episode numbers ("10-20").
	EpisodeRange string
	// Year selects all episodes of a year.
	Year string
	// All selects every episode of the format.
	All bool
	// Quality is the requested download quality.
	Quality string
	// Layout is the directory layout of the downloads.
	Layout string
	// Submodule is the RSS feed variant.
	Submodule int

	// List prints the catalog.
	List bool
	// Search is the text of the full text search.
	Search string
	// Profile prints the state of the subscription.
	Profile bool
	// LibraryMode is one of "", "list", "check", "download-new", "add".
	LibraryMode string
	// AddFormat is the format of --add.
	AddFormat string
	// JSON prints the result in the machine readable form.
	JSON bool

	// Automatic runs without interactive prompts.
	Automatic bool
	// LinksFile is the file with the formats of the batch mode.
	LinksFile string
	// ForceRedownload downloads the episodes again.
	ForceRedownload bool
	// Watch plays the episodes with mpv instead of downloading them.
	Watch bool
	// Totp is the current code of the second factor.
	Totp string
	// Parallel is the number of downloads that run at the same time.
	Parallel int
	// Delay is the pause between two downloads in seconds.
	Delay float64
	// Thumb stores the thumbnail next to the video.
	Thumb bool
	// Meta stores the metadata as JSON next to the video.
	Meta bool
	// Since selects the episodes released from that date ("YYYY-MM-DD").
	Since string
	// Title selects the episodes whose title contains the text.
	Title string
	// Completions names the shell of the completion script.
	Completions string
	// Man prints the man page.
	Man bool

	Username string
	Password string

	DownloadsDir    string
	LibraryPath     string
	CredentialsFile string
	WorkDir         string
	// FileDownloadsDir is the downloads_dir key of the settings file.
	// It is the weakest source of the downloads folder.
	FileDownloadsDir string
}

// Batch reports whether the options select the links-file mode.
func (o *Options) Batch() bool { return o.LinksFile != "" }

var digitsOnly = regexp.MustCompile(`^[0-9]+$`)

// setLibraryMode stores the library mode and rejects a second one.
func (o *Options) setLibraryMode(mode string) error {
	if o.LibraryMode != "" {
		return errors.New("only one of --library, --check, --download-new, --add may be used")
	}
	o.LibraryMode = mode
	return nil
}

// ParseArgs parses the argument vector. The options have to come before the
// first positional argument.
func ParseArgs(args []string) (*Options, error) {
	o := &Options{}
	var positional []string
	var err error

	for i := 0; i < len(args); {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			return nil, ErrHelp
		case arg == "-v" || arg == "--version":
			return nil, ErrVersion
		case arg == "--list":
			o.List = true
			i++
		case arg == "--profile":
			o.Profile = true
			i++
		case arg == "--search":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return nil, errors.New("error: --search requires a term")
			}
			o.Search = args[i+1]
			i += 2
		case arg == "--library":
			if err := o.setLibraryMode("list"); err != nil {
				return nil, err
			}
			i++
		case arg == "--json":
			o.JSON = true
			i++
		case arg == "--check":
			if err := o.setLibraryMode("check"); err != nil {
				return nil, err
			}
			i++
		case arg == "--download-new":
			if err := o.setLibraryMode("download-new"); err != nil {
				return nil, err
			}
			i++
		case arg == "--add":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return nil, errors.New("error: --add requires a format")
			}
			if err := o.setLibraryMode("add"); err != nil {
				return nil, err
			}
			o.AddFormat = args[i+1]
			i += 2
		case arg == "--automatic":
			o.Automatic = true
			i++
		case arg == "--force-redownload":
			o.ForceRedownload = true
			i++
		case arg == "--watch":
			o.Watch = true
			i++
		case arg == "--all":
			o.All = true
			i++
		case arg == "--format":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return nil, errors.New("error: --format requires a value")
			}
			o.Format = args[i+1]
			i += 2
		case arg == "--episode":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return nil, errors.New("error: --episode requires a number")
			}
			o.Episode = args[i+1]
			i += 2
		case arg == "--range":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return nil, errors.New("error: --range requires a range")
			}
			o.EpisodeRange = args[i+1]
			i += 2
		case arg == "--year":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return nil, errors.New("error: --year requires a year")
			}
			o.Year = args[i+1]
			i += 2
		case arg == "--quality":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return nil, errors.New("error: --quality requires a value")
			}
			o.Quality = args[i+1]
			i += 2
		case arg == "--layout":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return nil, errors.New("error: --layout requires a value")
			}
			o.Layout = args[i+1]
			i += 2
		case arg == "--links-file":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return nil, errors.New("error: --links-file requires a file")
			}
			o.LinksFile = args[i+1]
			i += 2
		case arg == "--totp":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return nil, errors.New("error: --totp requires a code")
			}
			o.Totp = args[i+1]
			i += 2
		case arg == "--parallel":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return nil, errors.New("error: --parallel requires a number")
			}
			o.Parallel, err = strconv.Atoi(args[i+1])
			if err != nil || o.Parallel < 1 || o.Parallel > 8 {
				o.Parallel = 0
				return nil, errors.New("error: --parallel expects a number from 1 to 8")
			}
			i += 2
		case arg == "--delay":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return nil, errors.New("error: --delay requires seconds")
			}
			o.Delay, err = strconv.ParseFloat(strings.ReplaceAll(args[i+1], ",", "."), 64)
			if err != nil || o.Delay < 0 || o.Delay > 3600 {
				o.Delay = 0
				return nil, errors.New("error: --delay expects a number of seconds from 0 to 3600")
			}
			i += 2
		case arg == "--thumb":
			o.Thumb = true
			i++
		case arg == "--meta":
			o.Meta = true
			i++
		case arg == "--since":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return nil, errors.New("error: --since requires a date")
			}
			if _, err := time.Parse("2006-01-02", args[i+1]); err != nil {
				return nil, fmt.Errorf("error: --since expects a date in the format YYYY-MM-DD: %s", args[i+1])
			}
			o.Since = args[i+1]
			i += 2
		case arg == "--title":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return nil, errors.New("error: --title requires a text")
			}
			o.Title = args[i+1]
			i += 2
		case arg == "--completions":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return nil, errors.New("error: --completions requires a shell")
			}
			o.Completions = strings.ToLower(args[i+1])
			i += 2
		case arg == "--man":
			o.Man = true
			i++
		case arg == "--":
			positional = append(positional, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(arg, "-"):
			return nil, fmt.Errorf("unknown option: %s", arg)
		default:
			positional = append(positional, args[i:]...)
			i = len(args)
		}
	}

	if o.LibraryMode != "" {
		if len(positional) > 0 {
			return nil, fmt.Errorf("unexpected argument: %s", positional[0])
		}
		if o.JSON && o.LibraryMode != "list" {
			return nil, errors.New("--json requires --library")
		}
		return o, nil
	}
	if o.JSON && !o.List {
		return nil, errors.New("--json requires --library or --list")
	}

	if o.Batch() {
		if len(positional) == 1 {
			o.Format = positional[0]
		} else if len(positional) > 1 {
			return nil, fmt.Errorf("unexpected argument: %s", positional[1])
		}
		return o, nil
	}

	if len(positional) == 1 && o.Format == "" {
		o.Format = positional[0]
	} else if len(positional) > 1 {
		return nil, fmt.Errorf("unexpected argument: %s", positional[1])
	}

	if o.Format == "" && !o.List && o.Search == "" && !o.Profile && !o.Man && o.Completions == "" {
		return nil, ErrNoTarget
	}
	return o, nil
}

// Resolve fills the values that were not given on the command line from the
// environment and the settings file. The order of the sources is: command
// line, environment, settings file, default.
func (o *Options) Resolve(env Env, stderr io.Writer) error {
	file := LoadConfigFile(ConfigFilePath(env), stderr)

	if o.Totp == "" {
		o.Totp = env("ORANGEFLIX_TOTP")
	}
	if o.Parallel == 0 {
		if value := env("ORANGEFLIX_PARALLEL"); value != "" {
			if parsed, err := strconv.Atoi(value); err == nil && parsed >= 1 && parsed <= 8 {
				o.Parallel = parsed
			}
		}
	}
	if o.Parallel == 0 {
		if file.Parallel >= 1 && file.Parallel <= 8 {
			o.Parallel = file.Parallel
		}
	}
	if o.Quality == "" {
		o.Quality = env("ORANGEFLIX_QUALITY")
	}

	if o.Quality == "" {
		o.Quality = file.Quality
	}
	if o.Quality == "" {
		o.Quality = "best"
	}
	if o.Layout == "" {
		o.Layout = env("ORANGEFLIX_LAYOUT")
	}
	if o.Layout == "" {
		o.Layout = file.Layout
	}
	if o.Layout == "" {
		o.Layout = "compat"
	}
	if value := env("ORANGEFLIX_SUBMODULE"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			o.Submodule = parsed
		}
	}
	if o.Submodule == 0 && file.Submodule > 0 {
		o.Submodule = file.Submodule
	}
	if o.Submodule == 0 {
		o.Submodule = 1
	}
	o.Thumb = o.Thumb || isTruthy(env("ORANGEFLIX_THUMB")) || (file.Thumb && env("ORANGEFLIX_THUMB") == "")
	o.Meta = o.Meta || isTruthy(env("ORANGEFLIX_META")) || (file.Meta && env("ORANGEFLIX_META") == "")
	if o.Username == "" {
		o.Username = env("ORANGEFLIX_USER")
	}
	if o.Password == "" {
		o.Password = env("ORANGEFLIX_PASSWORD")
	}
	o.FileDownloadsDir = file.DownloadsDir

	o.CredentialsFile = CredentialsPath(env)
	if o.Username == "" || o.Password == "" {
		username, password := LoadCredentialsFile(o.CredentialsFile, stderr)
		if o.Username == "" {
			o.Username = username
		}
		if o.Password == "" {
			o.Password = password
		}
	}
	return nil
}

// Validate checks the values that the API and the file system rely on.
func (o *Options) Validate() error {
	if !digitsOnly.MatchString(o.Episode) && o.Episode != "" {
		return fmt.Errorf("invalid episode number: %s (expected a number)", o.Episode)
	}
	if o.EpisodeRange != "" {
		from, to, found := strings.Cut(o.EpisodeRange, "-")
		if !found || !digitsOnly.MatchString(from) || !digitsOnly.MatchString(to) {
			return fmt.Errorf("invalid range: %s (expected for example 10-20)", o.EpisodeRange)
		}
	}
	if o.Year != "" && !digitsOnly.MatchString(o.Year) {
		return fmt.Errorf("invalid year: %s (expected a year)", o.Year)
	}
	if o.Since != "" {
		if _, err := time.Parse("2006-01-02", o.Since); err != nil {
			return fmt.Errorf("invalid date: %s (expected the format YYYY-MM-DD)", o.Since)
		}
	}
	return nil
}

// ValidateQuality checks the name of a download quality.
func ValidateQuality(name string) error {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "best", "hd", "sd", "mobile", "audio":
		return nil
	}
	return fmt.Errorf("invalid quality: %s (allowed: best, hd, sd, mobile, audio)", name)
}

// ValidateFormat checks the name of a format. The name may be a slug, a
// title or the numeric id of the format.
func ValidateFormat(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return errors.New("no format given")
	}
	if _, err := strconv.Atoi(trimmed); err == nil {
		return nil
	}
	for _, format := range orangeflix.Catalog {
		if strings.EqualFold(format.Slug, trimmed) || strings.EqualFold(format.Title, trimmed) {
			return nil
		}
	}
	return fmt.Errorf("unknown format: %s", value)
}

// CredentialsPath returns the location of the credentials file.
func CredentialsPath(env Env) string {
	if value := env("ORANGEFLIX_CREDENTIALS_FILE"); value != "" {
		return value
	}
	if runtime.GOOS == "windows" {
		if dir, err := os.UserConfigDir(); err == nil && dir != "" {
			return filepath.Join(dir, "orangeflix-dl", "credentials")
		}
	}
	base := env("XDG_CONFIG_HOME")
	if base == "" {
		base = env("HOME") + "/.config"
	}
	return base + "/orangeflix-dl/credentials"
}

// mkdirPrivate creates a directory that only the owner may read.
func mkdirPrivate(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.Chmod(dir, 0o700)
}

// StateDir returns the private state directory, falling back to
// "<workDir>/.orangeflix-dl" when the home based location cannot be created.
func StateDir(env Env, workDir string) string {
	if runtime.GOOS == "windows" {
		if dir, err := os.UserCacheDir(); err == nil && dir != "" {
			stateDir := filepath.Join(dir, "orangeflix-dl")
			if mkdirPrivate(stateDir) == nil {
				return stateDir
			}
		}
	}
	if env("HOME") != "" {
		base := env("XDG_STATE_HOME")
		if base == "" {
			base = env("HOME") + "/.local/state"
		}
		stateDir := base + "/orangeflix-dl"
		if mkdirPrivate(stateDir) == nil {
			return stateDir
		}
	}
	fallback := filepath.Join(workDir, ".orangeflix-dl")
	_ = mkdirPrivate(fallback)
	return fallback
}

// LibraryPath returns the location of the SQLite database.
func LibraryPath(env Env, workDir string) string {
	if value := env("ORANGEFLIX_LIBRARY"); value != "" {
		return value
	}
	return filepath.Join(StateDir(env, workDir), "library.db")
}

// DownloadsDir returns the root of the downloads.
func DownloadsDir(env Env, workDir string) string {
	if value := env("ORANGEFLIX_DOWNLOADS_DIR"); value != "" {
		return ExpandPath(value)
	}
	return filepath.Join(workDir, "downloads")
}

// ExpandPath resolves a leading "~" to the home directory, so a path from
// the interface or a settings file works without the shell.
func ExpandPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "~" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return home
		}
		return value
	}
	if strings.HasPrefix(value, "~/") || strings.HasPrefix(value, `~\`) {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, value[2:])
		}
	}
	return value
}

// EnsureWritableDir creates the directory and checks that a file can be
// written into it.
func EnsureWritableDir(dir string) error {
	if dir == "" {
		return errors.New("no directory")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	probe := filepath.Join(dir, ".orangeflix-dl-write-test")
	if err := os.WriteFile(probe, []byte("x"), 0o600); err != nil {
		return err
	}
	_ = os.Remove(probe)
	return nil
}

// LoadCredentialsFile reads the credentials from the credentials file. The
// first value of every key wins.
func LoadCredentialsFile(path string, stderr io.Writer) (username, password string) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", ""
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		fmt.Fprintf(stderr, "Warnung: Die Zugangsdaten-Datei %s ist für andere lesbar (chmod 600 empfohlen).\n", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		switch key {
		case "username", "ORANGEFLIX_USER":
			if username == "" {
				username = value
			}
		case "password", "ORANGEFLIX_PASSWORD":
			if password == "" {
				password = value
			}
		}
	}
	return username, password
}
