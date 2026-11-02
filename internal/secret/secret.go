// Package secret stores the Orangeflix password. It prefers the operating
// system's keychain and falls back to the 0600 credentials file when no
// keychain is reachable (for example in a container without a session bus).
package secret

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zalando/go-keyring"
)

// ErrNotFound is returned when no password is stored.
var ErrNotFound = errors.New("no password stored")

// Store keeps the password and the access token.
type Store interface {
	Get() (string, error)
	Set(password string) error
	Clear() error
	// Backend names the storage in user-facing messages.
	Backend() string
	// GetToken returns the stored access token.
	GetToken() (string, error)
	// SetToken stores the access token.
	SetToken(token string) error
	// ClearToken removes the stored access token.
	ClearToken() error
	// GetRefresh returns the stored refresh token.
	GetRefresh() (string, error)
	// SetRefresh stores the refresh token. An empty token removes it.
	SetRefresh(token string) error
	// GetExpiry returns the time the access token runs out.
	GetExpiry() (time.Time, error)
	// SetExpiry stores the time the access token runs out. The zero time
	// removes it.
	SetExpiry(expiry time.Time) error
}

const (
	keyringService = "orangeflix-dl"
	keyringUser    = "password"
	// keyringToken holds the access token of the API.
	keyringToken = "access-token"
	// keyringRefresh holds the refresh token of the API.
	keyringRefresh = "refresh-token"
	// keyringExpiry holds the unix time the access token runs out.
	keyringExpiry = "access-expires"
)

// Open returns a keychain-backed store when the platform keychain answers, and
// otherwise a store backed by the credentials file.
func Open(credentialsFile string) Store {
	keychain := &Keychain{}
	if err := keychain.available(); err != nil {
		return &File{Path: credentialsFile}
	}
	return keychain
}

// keychainWrite serialises the writes to the keychain.
var keychainWrite sync.Mutex

// Keychain uses the operating system keychain (macOS Keychain, Windows
// Credential Manager, Linux Secret Service).
type Keychain struct{}

func (k *Keychain) available() error {
	_, err := keyring.Get(keyringService, keyringUser)
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

// Get returns the stored password.
func (k *Keychain) Get() (string, error) {
	value, err := keyring.Get(keyringService, keyringUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	return value, err
}

// Set stores the password.
func (k *Keychain) Set(password string) error {
	return setKeychain(keyringUser, password)
}

// Clear removes the password.
func (k *Keychain) Clear() error {
	err := keyring.Delete(keyringService, keyringUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

// Backend names the storage.
func (k *Keychain) Backend() string { return "Schlüsselbund" }

// GetToken returns the stored access token.
func (k *Keychain) GetToken() (string, error) {
	value, err := keyring.Get(keyringService, keyringToken)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}
	return value, nil
}

// SetToken stores the access token.
func (k *Keychain) SetToken(token string) error {
	return setKeychain(keyringToken, token)
}

// GetRefresh returns the stored refresh token.
func (k *Keychain) GetRefresh() (string, error) {
	return getKeychain(keyringRefresh)
}

// SetRefresh stores the refresh token. An empty token removes it.
func (k *Keychain) SetRefresh(token string) error {
	return setKeychain(keyringRefresh, token)
}

// GetExpiry returns the time the access token runs out.
func (k *Keychain) GetExpiry() (time.Time, error) {
	value, err := getKeychain(keyringExpiry)
	if err != nil {
		return time.Time{}, err
	}
	return parseExpiry(value)
}

// SetExpiry stores the time the access token runs out.
func (k *Keychain) SetExpiry(expiry time.Time) error {
	if expiry.IsZero() {
		return setKeychain(keyringExpiry, "")
	}
	return setKeychain(keyringExpiry, strconv.FormatInt(expiry.Unix(), 10))
}

// getKeychain reads one item.
func getKeychain(account string) (string, error) {
	value, err := keyring.Get(keyringService, account)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", explainKeychain(err)
	}
	return value, nil
}

// setKeychain writes one item. The writes are serialised, an unchanged value
// is skipped, and a stale item that cannot be updated is replaced. An empty
// value removes the item. Two concurrent updates of the same item make the
// macOS keychain answer "item already exists" (exit 45).
func setKeychain(account, value string) error {
	keychainWrite.Lock()
	defer keychainWrite.Unlock()
	if value == "" {
		err := keyring.Delete(keyringService, account)
		if errors.Is(err, keyring.ErrNotFound) {
			return nil
		}
		return explainKeychain(err)
	}
	if current, err := keyring.Get(keyringService, account); err == nil && current == value {
		return nil
	}
	err := keyring.Set(keyringService, account, value)
	if err == nil {
		return nil
	}
	if deleteErr := keyring.Delete(keyringService, account); deleteErr != nil && !errors.Is(deleteErr, keyring.ErrNotFound) {
		return explainKeychain(err)
	}
	return explainKeychain(keyring.Set(keyringService, account, value))
}

// explainKeychain names the keychain error "item already exists".
func explainKeychain(err error) error {
	if err == nil {
		return nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 45 {
		return fmt.Errorf("der Eintrag im Schlüsselbund existiert schon (exit 45): %w", err)
	}
	return err
}

// ClearToken removes the stored access token.
func (k *Keychain) ClearToken() error {
	err := keyring.Delete(keyringService, keyringToken)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

// File stores the password in the credentials file, keeping the other keys.
type File struct {
	Path string
}

// passwordKeys name the password line. The file accepts the name of the
// environment variable as well.
var passwordKeys = []string{"password", "ORANGEFLIX_PASSWORD"}

// tokenKeys name the line of the access token.
var tokenKeys = []string{"access-token"}

// refreshKeys name the line of the refresh token.
var refreshKeys = []string{"refresh-token"}

// expiryKeys name the line with the unix time the access token runs out.
var expiryKeys = []string{"access-expires"}

// Get reads the password from the credentials file.
func (f *File) Get() (string, error) { return f.read(passwordKeys) }

// Set writes the password, replacing an existing password line.
func (f *File) Set(password string) error {
	return f.write(passwordKeys, passwordKeys[0]+"="+password)
}

// Clear removes the password from the credentials file.
func (f *File) Clear() error { return f.write(passwordKeys, "") }

// Backend names the storage.
func (f *File) Backend() string { return fmt.Sprintf("Datei %s", f.Path) }

// GetToken returns the stored access token.
func (f *File) GetToken() (string, error) { return f.read(tokenKeys) }

// SetToken stores the access token, keeping the other keys.
func (f *File) SetToken(token string) error {
	return f.write(tokenKeys, tokenKeys[0]+"="+token)
}

// ClearToken removes the stored access token.
func (f *File) ClearToken() error { return f.write(tokenKeys, "") }

// GetRefresh returns the stored refresh token.
func (f *File) GetRefresh() (string, error) { return f.read(refreshKeys) }

// SetRefresh stores the refresh token. An empty token removes it.
func (f *File) SetRefresh(token string) error {
	if token == "" {
		return f.write(refreshKeys, "")
	}
	return f.write(refreshKeys, refreshKeys[0]+"="+token)
}

// GetExpiry returns the time the access token runs out.
func (f *File) GetExpiry() (time.Time, error) {
	value, err := f.read(expiryKeys)
	if err != nil {
		return time.Time{}, err
	}
	return parseExpiry(value)
}

// SetExpiry stores the time the access token runs out.
func (f *File) SetExpiry(expiry time.Time) error {
	if expiry.IsZero() {
		return f.write(expiryKeys, "")
	}
	return f.write(expiryKeys, expiryKeys[0]+"="+strconv.FormatInt(expiry.Unix(), 10))
}

// parseExpiry reads a unix time.
func parseExpiry(value string) (time.Time, error) {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds <= 0 {
		return time.Time{}, ErrNotFound
	}
	return time.Unix(seconds, 0), nil
}

// lines returns the lines of the credentials file without line endings, nil
// when the file does not exist.
func (f *File) lines() ([]string, error) {
	data, err := os.ReadFile(f.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	for index, line := range lines {
		lines[index] = strings.TrimSuffix(line, "\r")
	}
	return lines, nil
}

// read returns the first non-empty value of one of the keys.
func (f *File) read(keys []string) (string, error) {
	lines, err := f.lines()
	if err != nil {
		return "", err
	}
	for _, line := range lines {
		key, value, found := strings.Cut(line, "=")
		if found && slices.Contains(keys, key) && value != "" {
			return value, nil
		}
	}
	return "", ErrNotFound
}

// write removes the lines of the keys and appends the replacement, unless it
// is empty. The other lines stay. A missing file is only created for a
// replacement.
func (f *File) write(keys []string, replacement string) error {
	lines, err := f.lines()
	if err != nil {
		return err
	}
	if lines == nil && replacement == "" {
		return nil
	}
	kept := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		key, _, found := strings.Cut(line, "=")
		if found && slices.Contains(keys, key) {
			continue
		}
		kept = append(kept, line)
	}
	// Drop the empty element the trailing newline produces, so that repeated
	// writes do not pile up blank lines above the new line.
	for len(kept) > 0 && kept[len(kept)-1] == "" {
		kept = kept[:len(kept)-1]
	}
	if replacement != "" {
		kept = append(kept, replacement)
	}
	content := strings.TrimLeft(strings.Join(kept, "\n"), "\n")
	if content != "" {
		content += "\n"
	}
	return writeFileAtomic(f.Path, []byte(content))
}

// writeFileAtomic replaces the file with a new 0600 file. The data goes to a
// temporary file first, so a crash never leaves a half written file and the
// secret is never readable for others, not even for a moment.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	// After the rename the temporary file is gone and the removal fails.
	defer func() { _ = os.Remove(tempPath) }()
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}
