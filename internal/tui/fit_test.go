package tui

import "testing"

// TestFitListWindowKeepsTheMarkedRowInside checks that the visible window
// leaves room for the head and the hints, so the marked row never falls out
// of the box when the list is scrolled to the end.
func TestFitListWindowKeepsTheMarkedRowInside(t *testing.T) {
	// The panel reserves two lines (Spaltenüberschrift und Filter) next to
	// the rows; the hints come on top of the rows.
	inner, reserve, total := 10, 2, 100
	budget := inner - reserve
	offset := 0

	for _, cursorRow := range []int{0, 1, 8, 9, 50, 98, 99} {
		offset = 0
		start, end := fitListWindow(&offset, cursorRow, budget, total)
		if cursorRow < start || cursorRow >= end {
			t.Fatalf("cursor %d: window [%d,%d) misses the cursor", cursorRow, start, end)
		}
		hints := 0
		if start > 0 {
			hints++
		}
		if end < total {
			hints++
		}
		if end-start+hints+reserve > inner {
			t.Fatalf("cursor %d: %d Zeilen + %d Hinweise + %d fest passen nicht in %d",
				cursorRow, end-start, hints, reserve, inner)
		}
	}

	// Am Ende der Liste taucht der untere Hinweis nicht auf: dann passt das
	// Fenster ohne ihn, die markierte Zeile bleibt sichtbar, und die Zeilen
	// mit dem oberen Hinweis passen zusammen in den Kasten.
	start, end := fitListWindow(&offset, total-1, budget, total)
	if end != total || total-1 < start || total-1 >= end {
		t.Fatalf("end: window [%d,%d)", start, end)
	}
	if end-start+1+reserve > inner {
		t.Fatalf("end: %d Zeilen + %d fest passen nicht in %d", end-start, reserve, inner)
	}
}
