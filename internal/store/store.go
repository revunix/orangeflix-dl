// Package store keeps the application state in a local SQLite database: the
// settings, the formats of the library and the episode history that is used
// to detect new episodes.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Setting keys.
const (
	SettingUsername = "username"
	SettingQuality  = "quality"
	SettingLayout   = "layout"
	SettingTheme    = "theme"
	// SettingDownloadsDir stores the folder chosen in the interface.
	SettingDownloadsDir = "downloads_dir"
)

// FormatEntry is one format of the library.
type FormatEntry struct {
	PID       int
	Slug      string
	Title     string
	Status    string
	Enabled   bool
	AddedAt   time.Time
	CheckedAt *time.Time
	NewCount  int
	LastError string
}

// EpisodeRef identifies one episode as the API reports it.
type EpisodeRef struct {
	Identifier string
	Number     int
	Title      string
	Date       time.Time
}

// KnownEpisode is a stored episode reference with its history.
type KnownEpisode struct {
	EpisodeRef
	FirstSeen    time.Time
	DownloadedAt *time.Time
	// Path is the file the episode was downloaded to.
	Path string
}

// Store is the SQLite backed application state.
type Store struct {
	db   *sql.DB
	path string
}

const schema = `
CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS formats (
    pid        INTEGER PRIMARY KEY,
    slug       TEXT NOT NULL DEFAULT '' UNIQUE,
    title      TEXT NOT NULL DEFAULT '',
    status     TEXT NOT NULL DEFAULT '',
    enabled    INTEGER NOT NULL DEFAULT 1,
    added_at   TEXT NOT NULL,
    checked_at TEXT,
    new_count  INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS episodes (
    pid           INTEGER NOT NULL,
    identifier    TEXT NOT NULL,
    number        INTEGER NOT NULL DEFAULT 0,
    title         TEXT NOT NULL DEFAULT '',
    date          TEXT NOT NULL DEFAULT '',
    first_seen    TEXT NOT NULL,
    downloaded_at TEXT,
    path          TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (pid, identifier)
);
CREATE INDEX IF NOT EXISTS episodes_open ON episodes(pid, downloaded_at);
CREATE TABLE IF NOT EXISTS opened (
    pid       INTEGER PRIMARY KEY,
    opened_at TEXT NOT NULL
);
`

// Open opens (and creates) the database at path.
func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("no database path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	// The path is escaped: a "?" or "#" in it would cut the DSN short.
	dsn := (&url.URL{
		Scheme:   "file",
		Opaque:   path,
		RawQuery: "_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)",
	}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}
	// The database holds the library and the username, so keep it private.
	_ = os.Chmod(path, 0o600)
	return &Store{db: db, path: path}, nil
}

// Path returns the database file path.
func (s *Store) Path() string { return s.path }

// Close closes the database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func nowStamp() string { return time.Now().UTC().Format(time.RFC3339) }

func parseStamp(value string) *time.Time {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	return &parsed
}

// Setting returns a stored setting, or "" when it is not set.
func (s *Store) Setting(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

// SetSetting stores a setting; an empty value removes it.
func (s *Store) SetSetting(key, value string) error {
	if value == "" {
		_, err := s.db.Exec(`DELETE FROM settings WHERE key = ?`, key)
		return err
	}
	_, err := s.db.Exec(`INSERT INTO settings(key, value) VALUES(?, ?)
        ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// UpsertFormat adds a format to the library or updates its title and status.
// The enabled flag and the check history are kept.
func (s *Store) UpsertFormat(pid int, slug, title, status string) error {
	if pid <= 0 {
		return errors.New("no format id")
	}
	_, err := s.db.Exec(`INSERT INTO formats(pid, slug, title, status, added_at) VALUES(?, ?, ?, ?, ?)
        ON CONFLICT(pid) DO UPDATE SET
            slug  = CASE WHEN excluded.slug  != '' THEN excluded.slug  ELSE formats.slug  END,
            title = CASE WHEN excluded.title != '' THEN excluded.title ELSE formats.title END,
            status = CASE WHEN excluded.status != '' THEN excluded.status ELSE formats.status END`,
		pid, slug, title, status, nowStamp())
	return err
}

// Formats returns the library, newest entry first.
func (s *Store) Formats() ([]FormatEntry, error) {
	rows, err := s.db.Query(`SELECT pid, slug, title, status, enabled, added_at, checked_at,
        new_count, last_error FROM formats ORDER BY title COLLATE NOCASE, pid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []FormatEntry
	for rows.Next() {
		var (
			entry     FormatEntry
			enabled   int
			added     string
			checkedAt sql.NullString
		)
		if err := rows.Scan(&entry.PID, &entry.Slug, &entry.Title, &entry.Status, &enabled,
			&added, &checkedAt, &entry.NewCount, &entry.LastError); err != nil {
			return nil, err
		}
		entry.Enabled = enabled != 0
		if parsed := parseStamp(added); parsed != nil {
			entry.AddedAt = *parsed
		}
		entry.CheckedAt = parseStamp(checkedAt.String)
		result = append(result, entry)
	}
	return result, rows.Err()
}

// MarkOpened remembers that the user has been in a format.
func (s *Store) MarkOpened(pid int) error {
	_, err := s.db.Exec(`INSERT INTO opened (pid, opened_at) VALUES (?, ?)
        ON CONFLICT(pid) DO UPDATE SET opened_at = excluded.opened_at`, pid, nowStamp())
	return err
}

// OpenedFormats returns the formats the user has already been in.
func (s *Store) OpenedFormats() (map[int]bool, error) {
	rows, err := s.db.Query(`SELECT pid FROM opened`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[int]bool{}
	for rows.Next() {
		var pid int
		if err := rows.Scan(&pid); err != nil {
			return nil, err
		}
		result[pid] = true
	}
	return result, rows.Err()
}

// FormatByPID returns one format of the library.
func (s *Store) FormatByPID(pid int) (*FormatEntry, error) {
	formats, err := s.Formats()
	if err != nil {
		return nil, err
	}
	for i := range formats {
		if formats[i].PID == pid {
			return &formats[i], nil
		}
	}
	return nil, nil
}

// RemoveFormat deletes a format of the library together with its history.
func (s *Store) RemoveFormat(pid int) error {
	_, err := s.db.Exec(`DELETE FROM formats WHERE pid = ?`, pid)
	return err
}

// SetFormatEnabled toggles whether a format is checked and downloaded.
func (s *Store) SetFormatEnabled(pid int, enabled bool) error {
	value := 0
	if enabled {
		value = 1
	}
	_, err := s.db.Exec(`UPDATE formats SET enabled = ? WHERE pid = ?`, value, pid)
	return err
}

// SyncEpisodes stores the episodes the API currently reports and returns the
// ones that were not known before.
func (s *Store) SyncEpisodes(pid int, episodes []EpisodeRef) ([]EpisodeRef, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	known := map[string]bool{}
	rows, err := tx.Query(`SELECT identifier FROM episodes WHERE pid = ?`, pid)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var identifier string
		if err := rows.Scan(&identifier); err != nil {
			rows.Close()
			return nil, err
		}
		known[identifier] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var fresh []EpisodeRef
	stamp := nowStamp()
	for _, episode := range episodes {
		if episode.Identifier == "" {
			continue
		}
		date := ""
		if !episode.Date.IsZero() {
			date = episode.Date.UTC().Format(time.RFC3339)
		}
		if _, err := tx.Exec(`INSERT INTO episodes(pid, identifier, number, title, date, first_seen)
            VALUES(?, ?, ?, ?, ?, ?)
            ON CONFLICT(pid, identifier) DO UPDATE SET
                number = excluded.number, title = excluded.title, date = excluded.date`,
			pid, episode.Identifier, episode.Number, episode.Title, date, stamp); err != nil {
			return nil, err
		}
		if !known[episode.Identifier] {
			fresh = append(fresh, episode)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return fresh, nil
}

// Pending returns the stored episodes that were not downloaded yet.
func (s *Store) Pending(pid int) ([]KnownEpisode, error) {
	rows, err := s.db.Query(`SELECT identifier, number, title, date, first_seen, downloaded_at, path
        FROM episodes WHERE pid = ? AND downloaded_at IS NULL
        ORDER BY number DESC`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEpisodes(rows)
}

func scanEpisodes(rows *sql.Rows) ([]KnownEpisode, error) {
	var result []KnownEpisode
	for rows.Next() {
		var (
			episode    KnownEpisode
			date       string
			firstSeen  string
			downloaded sql.NullString
		)
		if err := rows.Scan(&episode.Identifier, &episode.Number, &episode.Title, &date, &firstSeen, &downloaded, &episode.Path); err != nil {
			return nil, err
		}
		if parsed := parseStamp(date); parsed != nil {
			episode.Date = *parsed
		}
		if parsed := parseStamp(firstSeen); parsed != nil {
			episode.FirstSeen = *parsed
		}
		episode.DownloadedAt = parseStamp(downloaded.String)
		result = append(result, episode)
	}
	return result, rows.Err()
}

// Episodes returns every stored episode of one format.
func (s *Store) Episodes(pid int) ([]KnownEpisode, error) {
	rows, err := s.db.Query(`SELECT identifier, number, title, date, first_seen, downloaded_at, path
        FROM episodes WHERE pid = ? ORDER BY number DESC`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEpisodes(rows)
}

// KnownIdentifiers returns the identifiers of the episodes that are already
// known of one format.
func (s *Store) KnownIdentifiers(pid int) (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT identifier FROM episodes WHERE pid = ?`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	known := map[string]bool{}
	for rows.Next() {
		var identifier string
		if err := rows.Scan(&identifier); err != nil {
			return nil, err
		}
		known[identifier] = true
	}
	return known, rows.Err()
}

// CountPending counts the episodes that are known but not downloaded.
func (s *Store) CountPending(pid int) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM episodes WHERE pid = ? AND downloaded_at IS NULL`, pid).Scan(&count)
	return count, err
}

// MarkDownloaded records that one episode was downloaded and where the file
// ended up.
func (s *Store) MarkDownloaded(pid int, identifier, path string) error {
	_, err := s.db.Exec(`UPDATE episodes SET downloaded_at = ?, path = ? WHERE pid = ? AND identifier = ?`,
		nowStamp(), path, pid, identifier)
	return err
}

// ClearDownloaded forgets that one episode was downloaded, so it is offered
// again.
func (s *Store) ClearDownloaded(pid int, identifier string) error {
	_, err := s.db.Exec(`UPDATE episodes SET downloaded_at = NULL WHERE pid = ? AND identifier = ?`, pid, identifier)
	return err
}

// ClearEpisodes forgets the history of one format, so every episode is
// offered again.
func (s *Store) ClearEpisodes(pid int) error {
	_, err := s.db.Exec(`DELETE FROM episodes WHERE pid = ?`, pid)
	return err
}

// RecordCheck stores the result of the last new episode check.
func (s *Store) RecordCheck(pid, newCount int, lastError string) error {
	_, err := s.db.Exec(`UPDATE formats SET checked_at = ?, new_count = ?, last_error = ? WHERE pid = ?`,
		nowStamp(), newCount, lastError, pid)
	return err
}

// String renders the format for the log output.
func (f FormatEntry) String() string {
	name := f.Title
	if name == "" {
		name = fmt.Sprintf("Format %d", f.PID)
	}
	return fmt.Sprintf("%s (#%d)", name, f.PID)
}

// TrimError shortens an error for the database column.
func TrimError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if len(message) > 500 {
		message = message[:500]
	}
	return strings.TrimSpace(message)
}
