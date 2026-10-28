package store

import (
	"path/filepath"
	"testing"
	"time"
)

// testStore opens a store in a temporary directory.
func testStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "library.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestSettings(t *testing.T) {
	store := testStore(t)

	if err := store.SetSetting(SettingUsername, "user@example.com"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	value, err := store.Setting(SettingUsername)
	if err != nil {
		t.Fatalf("Setting: %v", err)
	}
	if value != "user@example.com" {
		t.Errorf("value = %q", value)
	}

	// An empty value removes the setting.
	if err := store.SetSetting(SettingUsername, ""); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	value, err = store.Setting(SettingUsername)
	if err != nil {
		t.Fatalf("Setting: %v", err)
	}
	if value != "" {
		t.Errorf("value = %q, want the empty string", value)
	}
}

func TestUpsertFormat(t *testing.T) {
	store := testStore(t)

	if err := store.UpsertFormat(23, "mediatheke", "Die Mediatheke", "archived"); err != nil {
		t.Fatalf("UpsertFormat: %v", err)
	}
	// The second call updates the title and keeps the entry.
	if err := store.UpsertFormat(23, "mediatheke", "Die Mediatheke", "archived"); err != nil {
		t.Fatalf("UpsertFormat: %v", err)
	}

	formats, err := store.Formats()
	if err != nil {
		t.Fatalf("Formats: %v", err)
	}
	if len(formats) != 1 {
		t.Fatalf("got %d formats, want 1", len(formats))
	}
	if formats[0].PID != 23 || formats[0].Title != "Die Mediatheke" {
		t.Errorf("format = %+v", formats[0])
	}
	if !formats[0].Enabled {
		t.Error("the format should be enabled")
	}
}

func TestSyncEpisodesReportsTheNewOnes(t *testing.T) {
	store := testStore(t)
	now := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)

	fresh, err := store.SyncEpisodes(23, []EpisodeRef{
		{Identifier: "mt420", Number: 420, Title: "Folge 420", Date: now},
		{Identifier: "mt419", Number: 419, Title: "Folge 419", Date: now},
	})
	if err != nil {
		t.Fatalf("SyncEpisodes: %v", err)
	}
	if len(fresh) != 2 {
		t.Fatalf("got %d new episodes, want 2", len(fresh))
	}

	// The second run knows the episodes.
	fresh, err = store.SyncEpisodes(23, []EpisodeRef{
		{Identifier: "mt420", Number: 420, Title: "Folge 420", Date: now},
	})
	if err != nil {
		t.Fatalf("SyncEpisodes: %v", err)
	}
	if len(fresh) != 0 {
		t.Fatalf("got %d new episodes, want 0", len(fresh))
	}

	known, err := store.KnownIdentifiers(23)
	if err != nil {
		t.Fatalf("KnownIdentifiers: %v", err)
	}
	if !known["mt420"] || !known["mt419"] {
		t.Errorf("known = %v", known)
	}
}

func TestPendingAndMarkDownloaded(t *testing.T) {
	store := testStore(t)
	if err := store.UpsertFormat(23, "mediatheke", "Die Mediatheke", "archived"); err != nil {
		t.Fatalf("UpsertFormat: %v", err)
	}
	if _, err := store.SyncEpisodes(23, []EpisodeRef{
		{Identifier: "mt420", Number: 420},
		{Identifier: "mt419", Number: 419},
	}); err != nil {
		t.Fatalf("SyncEpisodes: %v", err)
	}

	count, err := store.CountPending(23)
	if err != nil {
		t.Fatalf("CountPending: %v", err)
	}
	if count != 2 {
		t.Fatalf("pending = %d, want 2", count)
	}

	if err := store.MarkDownloaded(23, "mt420", "/tmp/x.mp4"); err != nil {
		t.Fatalf("MarkDownloaded: %v", err)
	}
	pending, err := store.Pending(23)
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if len(pending) != 1 || pending[0].Identifier != "mt419" {
		t.Fatalf("pending = %+v", pending)
	}

	episodes, err := store.Episodes(23)
	if err != nil {
		t.Fatalf("Episodes: %v", err)
	}
	var found bool
	for _, episode := range episodes {
		if episode.Identifier == "mt420" {
			found = true
			if episode.Path != "/tmp/x.mp4" {
				t.Errorf("path = %q", episode.Path)
			}
			if episode.DownloadedAt == nil {
				t.Error("the episode has no download time")
			}
		}
	}
	if !found {
		t.Error("the downloaded episode is missing")
	}

	// A cleared episode is offered again.
	if err := store.ClearDownloaded(23, "mt420"); err != nil {
		t.Fatalf("ClearDownloaded: %v", err)
	}
	count, err = store.CountPending(23)
	if err != nil {
		t.Fatalf("CountPending: %v", err)
	}
	if count != 2 {
		t.Errorf("pending = %d, want 2", count)
	}
}

func TestRecordCheck(t *testing.T) {
	store := testStore(t)
	if err := store.UpsertFormat(23, "mediatheke", "Die Mediatheke", "archived"); err != nil {
		t.Fatalf("UpsertFormat: %v", err)
	}
	if err := store.RecordCheck(23, 3, ""); err != nil {
		t.Fatalf("RecordCheck: %v", err)
	}
	entry, err := store.FormatByPID(23)
	if err != nil {
		t.Fatalf("FormatByPID: %v", err)
	}
	if entry == nil {
		t.Fatal("the format is missing")
	}
	if entry.NewCount != 3 {
		t.Errorf("new count = %d", entry.NewCount)
	}
	if entry.CheckedAt == nil {
		t.Error("the format has no check time")
	}
}

func TestRemoveFormat(t *testing.T) {
	store := testStore(t)
	if err := store.UpsertFormat(23, "mediatheke", "Die Mediatheke", "archived"); err != nil {
		t.Fatalf("UpsertFormat: %v", err)
	}
	if err := store.RemoveFormat(23); err != nil {
		t.Fatalf("RemoveFormat: %v", err)
	}
	formats, err := store.Formats()
	if err != nil {
		t.Fatalf("Formats: %v", err)
	}
	if len(formats) != 0 {
		t.Errorf("got %d formats, want 0", len(formats))
	}
}
