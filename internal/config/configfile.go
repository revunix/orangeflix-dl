// The settings file holds the values that repeat on every run: quality,
// layout, tag and the options of the downloads. The file follows the same
// "key=value" format as the credentials file.
package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// FileConfig are the values of the settings file.
type FileConfig struct {
	Quality      string
	Layout       string
	Submodule    int
	Parallel     int
	Delay        float64
	Thumb, Meta  bool
	DownloadsDir string
}

// ConfigFilePath returns the location of the settings file.
func ConfigFilePath(env Env) string {
	if value := env("ORANGEFLIX_CONFIG_FILE"); value != "" {
		return value
	}
	base := env("XDG_CONFIG_HOME")
	if base == "" {
		base = env("HOME") + "/.config"
	}
	return filepath.Join(base, "orangeflix-dl", "config")
}

// LoadConfigFile reads the keys of the settings file. The first value of
// every key wins, unknown keys are reported on the error stream as a hint.
func LoadConfigFile(path string, stderr io.Writer) FileConfig {
	var result FileConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return result
	}
	if runtime.GOOS != "windows" {
		if info, statErr := os.Stat(path); statErr == nil && info.Mode().Perm()&0o077 != 0 {
			fmt.Fprintf(stderr, "Warnung: Die Konfigurationsdatei %s ist für andere lesbar (chmod 600 empfohlen).\n", path)
		}
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		switch key {
		case "quality", "ORANGEFLIX_QUALITY":
			if result.Quality == "" {
				result.Quality = value
			}
		case "layout", "ORANGEFLIX_LAYOUT":
			if result.Layout == "" {
				result.Layout = value
			}
		case "submodule", "ORANGEFLIX_SUBMODULE":
			if result.Submodule == 0 {
				if parsed, err := strconv.Atoi(value); err == nil {
					result.Submodule = parsed
				}
			}
		case "parallel", "ORANGEFLIX_PARALLEL":
			if result.Parallel == 0 {
				if parsed, err := strconv.Atoi(value); err == nil {
					result.Parallel = parsed
				}
			}
		case "delay", "ORANGEFLIX_DELAY":
			if result.Delay == 0 {
				if parsed, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64); err == nil {
					result.Delay = parsed
				}
			}
		case "downloads_dir", "ORANGEFLIX_DOWNLOADS_DIR":
			if result.DownloadsDir == "" {
				result.DownloadsDir = value
			}
		case "thumb", "ORANGEFLIX_THUMB":
			result.Thumb = result.Thumb || isTruthy(value)
		case "meta", "ORANGEFLIX_META":
			result.Meta = result.Meta || isTruthy(value)
		}
	}
	return result
}

// isTruthy reads the values "1", "true", "yes", "ja" and "an" as on.
func isTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "ja", "an":
		return true
	}
	return false
}
