package download

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/revunix/orangeflix-dl/internal/media"
)

// testNaming returns the naming of one episode.
func testNaming() media.Naming {
	return media.Naming{
		Title:      "Die Mediatheke",
		Year:       2026,
		Number:     420,
		Resolution: "1080p",
		Layout:     media.LayoutCompat,
	}
}

func TestManagerIgnoresThePartialFile(t *testing.T) {
	dir := t.TempDir()
	manager := &Manager{DownloadsDir: dir}

	naming := testNaming()
	path := filepath.Join(dir, filepath.FromSlash(naming.RelativeFile(".mp4")))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// The partial file of an interrupted download must not count as a
	// finished episode.
	if err := os.WriteFile(path+".part", []byte("half"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	existing, err := manager.Prepare(naming)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if existing != "" {
		t.Error("the partial file was treated as a finished episode")
	}
}

func TestManagerRemovesThePartialFileWithForce(t *testing.T) {
	dir := t.TempDir()
	manager := &Manager{DownloadsDir: dir, Force: true}

	naming := testNaming()
	path := filepath.Join(dir, filepath.FromSlash(naming.RelativeFile(".mp4")))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path+".part", []byte("half"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := manager.Prepare(naming); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if _, err := os.Stat(path + ".part"); err == nil {
		t.Error("the partial file was not removed")
	}
}
