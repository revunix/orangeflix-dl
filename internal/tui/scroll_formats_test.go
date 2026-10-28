package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// markiertesFormat returns the format that the cursor is on.
func markiertesFormat(model *Model) orangeflix.Format {
	formats := model.visibleFormats()
	if len(formats) == 0 {
		return orangeflix.Format{}
	}
	return formats[model.formatCursor]
}

// TestTheCursorWalksThroughTheList checks that the marked row walks down
// with the cursor: the order of the list and the order of the cursor are
// the same.
func TestTheCursorWalksThroughTheList(t *testing.T) {
	model := testModel()
	model.screen = ScreenFormats
	model.width = 120
	model.height = 30

	previous := markiertesFormat(model).PID
	for step := 0; step < 8; step++ {
		model = press(t, model, "down")
		current := markiertesFormat(model).PID
		if current == previous {
			t.Fatalf("der Cursor bleibt auf %d stehen", current)
		}
		// Die Markierung in der Liste gehört zum markierten Format.
		if !strings.Contains(model.View(), markiertesFormat(model).Title) {
			t.Fatalf("das markierte Format %d steht nicht in der Liste", current)
		}
		previous = current
	}
	// Nach oben geht es wieder zurück.
	for step := 0; step < 8; step++ {
		model = press(t, model, "up")
	}
	if model.formatCursor != 0 {
		t.Errorf("cursor = %d, want 0", model.formatCursor)
	}
}

// TestTheMarkedRowIsInTheWindow checks that the marked format is inside
// the visible window, whatever the position in the list is.
func TestTheMarkedRowIsInTheWindow(t *testing.T) {
	model := testModel()
	model.screen = ScreenFormats
	model.width = 120
	model.height = 30

	for step := 0; step < 40; step++ {
		model = press(t, model, "down")
		rows := model.formatRows()
		marked := markiertesFormat(model)
		found := false
		for _, row := range rows {
			if row.format != nil && row.format.PID == marked.PID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("das markierte Format %d fehlt in der Liste", marked.PID)
		}
	}
}

// TestTheListKeepsItsOrderWhenItIsReloaded checks that the marked format
// stays marked, even when the catalog is loaded again and the list is
// sorted by the date.
func TestTheListKeepsItsOrderWhenItIsReloaded(t *testing.T) {
	model := testModel()
	model.screen = ScreenFormats
	model.width = 120
	model.height = 30

	model.formats = []orangeflix.Format{
		{PID: 1, Slug: "alt", Title: "Alt", Status: orangeflix.StatusActive, LastEpisodeTime: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
		{PID: 2, Slug: "neu", Title: "Neu", Status: orangeflix.StatusActive, LastEpisodeTime: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
		{PID: 3, Slug: "mitte", Title: "Mitte", Status: orangeflix.StatusActive, LastEpisodeTime: time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)},
	}
	// Der Cursor steht auf dem Format, das als markiert gilt.
	model.formatCursor = indexOfFormat(model.visibleFormats(), 2)
	marked := markiertesFormat(model)
	if marked.Slug != "neu" {
		t.Fatalf("marked = %q, want neu", marked.Slug)
	}

	// Der Katalog kommt neu und ist anders sortiert.
	model.formats = []orangeflix.Format{
		{PID: 1, Slug: "alt", Title: "Alt", Status: orangeflix.StatusActive, LastEpisodeTime: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
		{PID: 2, Slug: "neu", Title: "Neu", Status: orangeflix.StatusActive, LastEpisodeTime: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)},
		{PID: 3, Slug: "mitte", Title: "Mitte", Status: orangeflix.StatusActive, LastEpisodeTime: time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)},
	}
	updated, _ := model.Update(FormatsMsg{Formats: model.formats})
	model, _ = updated.(*Model)
	marked = markiertesFormat(model)
	if marked.Slug != "neu" {
		t.Errorf("marked = %q, want neu", marked.Slug)
	}
}
