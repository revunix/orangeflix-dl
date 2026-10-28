package cli

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/revunix/orangeflix-dl/internal/config"
	"github.com/revunix/orangeflix-dl/internal/secret"
	"github.com/revunix/orangeflix-dl/internal/store"
)

// fileExists reports whether the path exists.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// resolveWorkDir returns the directory of the application.
func resolveWorkDir(opts *config.Options) (string, error) {
	if opts.WorkDir != "" {
		return opts.WorkDir, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return dir, nil
}

// logRouter lets the components log before the interface exists and reroutes
// the output into the running interface afterwards.
type logRouter struct {
	mu   sync.Mutex
	sink func(line string)
	// file holds the diagnostic log, when ORANGEFLIX_DEBUG_DIR is set.
	file *os.File
}

// newLogRouter builds a router that writes to the output.
func newLogRouter(out io.Writer) *logRouter {
	return &logRouter{sink: func(line string) { fmt.Fprintln(out, line) }}
}

// SetFile writes every log line into the file as well.
func (r *logRouter) SetFile(file *os.File) {
	r.mu.Lock()
	r.file = file
	r.mu.Unlock()
}

// CloseFile closes the diagnostic log.
func (r *logRouter) CloseFile() {
	r.mu.Lock()
	file := r.file
	r.file = nil
	r.mu.Unlock()
	if file != nil {
		_ = file.Close()
	}
}

// SetSink sets the target of the log lines.
func (r *logRouter) SetSink(sink func(line string)) {
	r.mu.Lock()
	r.sink = sink
	r.mu.Unlock()
}

// Logf writes one log line.
func (r *logRouter) Logf(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	r.mu.Lock()
	sink := r.sink
	file := r.file
	r.mu.Unlock()
	if sink != nil {
		sink(line)
	}
	if file != nil {
		_, _ = file.WriteString(line + "\n")
	}
}

// resolveDebugDir returns the directory for the diagnostic files, or "" when
// the feature is off. The directory is created so the components can write
// into it right away.
func resolveDebugDir(env config.Env) string {
	dir := env("ORANGEFLIX_DEBUG_DIR")
	if dir == "" {
		return ""
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Warnung: Diagnose-Verzeichnis nicht anlegbar (%s): %v\n", dir, err)
		return ""
	}
	return dir
}

// resolveCredentials fills the missing credentials from the keychain and the
// library. It reports whether a usable access token of a former run is there.
func resolveCredentials(opts *config.Options, env config.Env, secrets secret.Store, state *store.Store) (bool, error) {
	if opts.Username == "" {
		opts.Username = env("ORANGEFLIX_USER")
	}
	if opts.Password == "" {
		opts.Password = env("ORANGEFLIX_PASSWORD")
	}
	if opts.Password == "" && secrets != nil {
		if password, err := secrets.Get(); err == nil {
			opts.Password = password
		}
	}
	if opts.Username == "" && state != nil {
		if username, err := state.Setting(store.SettingUsername); err == nil {
			opts.Username = username
		}
	}
	// Der Benutzername wird gemerkt, damit er nach dem ersten Mal nicht wieder
	// eingegeben werden muss.
	if state != nil && opts.Username != "" {
		_ = state.SetSetting(store.SettingUsername, opts.Username)
	}
	// Ein Token in der Anmeldung reicht für den Start: es wird sieben Tage
	// gültig und wird beim Start wiederverwendet.
	hasToken := false
	if secrets != nil {
		if _, err := secrets.GetToken(); err == nil {
			hasToken = true
		}
	}
	return hasToken, nil
}
