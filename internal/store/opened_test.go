package store

import (
	"path/filepath"
	"testing"
)

// TestOpenedFormatsRemembersTheVisit checks that the store remembers in
// which formats the user has been.
func TestOpenedFormatsRemembersTheVisit(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	opened, err := store.OpenedFormats()
	if err != nil {
		t.Fatal(err)
	}
	if len(opened) != 0 {
		t.Fatalf("opened = %v, want none", opened)
	}

	if err := store.MarkOpened(2); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkOpened(19); err != nil {
		t.Fatal(err)
	}
	// Ein zweiter Besuch ändert nichts.
	if err := store.MarkOpened(2); err != nil {
		t.Fatal(err)
	}

	opened, err = store.OpenedFormats()
	if err != nil {
		t.Fatal(err)
	}
	if !opened[2] || !opened[19] {
		t.Fatalf("opened = %v, want 2 and 19", opened)
	}
	if len(opened) != 2 {
		t.Fatalf("opened = %v, want two formats", opened)
	}
}
