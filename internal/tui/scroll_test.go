package tui

import "testing"

// TestAdjustScrollFollowsMinimally checks that the visible window only
// scrolls when the cursor reaches the edge instead of centering it again
// and again, so long lists move step by step.
func TestAdjustScrollFollowsMinimally(t *testing.T) {
	offset := 0
	total, budget := 50, 10

	// Von oben bis an die Grenze: kein Verschieben im Fenster.
	for cursor := 0; cursor < 9; cursor++ {
		start, _, _, _ := adjustScroll(&offset, cursor, budget, total)
		if offset != 0 {
			t.Fatalf("cursor %d: offset = %d, want 0", cursor, offset)
		}
		if start != 0 {
			t.Fatalf("cursor %d: start = %d, want 0", cursor, start)
		}
	}
	// Einer über den Rand: eine Zeile weiter.
	if start, _, _, _ := adjustScroll(&offset, 10, budget, total); start != 1 {
		t.Fatalf("start = %d, want 1", start)
	}
	if offset != 1 {
		t.Fatalf("offset = %d, want 1", offset)
	}

	// Schneller Sprung weit unten: das Fenster folgt bis zum Ende.
	if _, end, _, _ := adjustScroll(&offset, 49, budget, total); end != total {
		t.Fatalf("end = %d, want %d", end, total)
	}
	if offset != total-budget {
		t.Fatalf("offset = %d, want %d", offset, total-budget)
	}

	// Nach oben: das Fenster folgt wieder am oberen Rand mit.
	if start, _, _, _ := adjustScroll(&offset, 0, budget, total); start != 0 {
		t.Fatalf("start = %d, want 0", start)
	}
}

// TestAdjustScrollClampsOnShrinking keeps the offset inside the list when
// the list or the window shrinks.
func TestAdjustScrollClampsOnShrinking(t *testing.T) {
	offset := 30
	start, end, _, _ := adjustScroll(&offset, 0, 10, 15)
	if start != 0 || end != 10 || offset != 0 {
		t.Fatalf("start = %d, end = %d, offset = %d", start, end, offset)
	}
	offset = 5
	if start, end, _, _ = adjustScroll(&offset, 3, 0, 15); start != 0 || end != 0 {
		t.Fatalf("no budget: start = %d, end = %d", start, end)
	}
}
