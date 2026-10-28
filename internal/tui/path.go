package tui

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/revunix/orangeflix-dl/internal/config"
)

// pathListRows is the number of folders that the window shows at once.
const pathListRows = 8

// pathPopup builds the window that changes the storage location of the
// downloads. The window floats over the page and is centered in both
// directions.
func (m *Model) pathPopup(width int) []string {
	inner := pathInnerWidth(width)
	// Der Prompt "Ordner " nimmt sieben Zeichen der Zeile ein.
	m.pathInput.Width = max(10, inner-8)
	m.pathInput.PromptStyle = theme.AccentBold
	m.pathInput.TextStyle = theme.Text

	content := []string{
		theme.Muted.Render("Zielordner der Downloads"),
		"",
		m.pathInput.View(),
		"",
	}
	content = append(content, m.pathListLines(inner)...)
	if m.pathError != "" {
		content = append(content, "")
		for _, line := range wrapText(m.pathError, inner) {
			content = append(content, theme.Danger.Render(line))
		}
	}

	return windowBoxFootColor(inner+4, "Speicherort", "", theme.BoxList, content)
}

// pathListLines draws the folders of the folder that stands in the field.
func (m *Model) pathListLines(inner int) []string {
	rows := m.pathVisibleRows()
	title := strings.Repeat(" ", rowIndent) + theme.Subtle.Render("ORDNER")
	if len(m.pathDirs) > rows {
		end := min(m.pathOffset+rows, len(m.pathDirs))
		title += " " + theme.Subtle.Render(fmt.Sprintf("%d–%d von %d", m.pathOffset+1, end, len(m.pathDirs)))
	}
	lines := []string{title}
	if len(m.pathDirs) == 0 {
		note := m.pathListNote
		if note == "" {
			note = "Keine Unterordner."
		}
		return append(lines, strings.Repeat(" ", rowIndent)+theme.Muted.Render(fitToWidth(note, inner-rowIndent)))
	}

	end := min(m.pathOffset+rows, len(m.pathDirs))
	for index := m.pathOffset; index < end; index++ {
		name := m.pathDirs[index]
		label := name + "/"
		if name == ".." {
			label = name
		}
		label = fitToWidth(label, max(10, inner-3))
		lines = append(lines, renderWindowRow(theme.Row.Render(label), index == m.pathCursor, inner))
	}
	return lines
}

// pathInnerWidth returns the width of the text inside the storage window.
func pathInnerWidth(width int) int {
	inner := width - 16
	if inner > 58 {
		inner = 58
	}
	if inner < 30 {
		inner = 30
	}
	return inner
}

// pathVisibleRows returns how many folders fit into the window. The rows are
// trimmed when the terminal is short, so the hints stay visible.
func (m *Model) pathVisibleRows() int {
	rows := pathListRows
	if m.height > 0 {
		rows = min(rows, m.height-16)
	}
	return max(rows, 3)
}

// loadPathDirs fills the folder list from the field. A folder that does not
// exist yet keeps the list empty and explains why.
func (m *Model) loadPathDirs() {
	m.pathDirs = nil
	m.pathCursor = 0
	m.pathOffset = 0
	m.pathListNote = ""

	dir := m.pathBase()
	if dir == "" {
		m.pathListNote = "Noch kein Ordner angegeben."
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			m.pathListNote = "Ordner existiert noch nicht."
		} else {
			m.pathListNote = "Ordner nicht lesbar: " + err.Error()
		}
		return
	}

	// Die ".."-Zeile führt eine Ebene hoch, wie im Explorer.
	if filepath.Dir(dir) != dir {
		m.pathDirs = append(m.pathDirs, "..")
	}
	for _, entry := range entries {
		if entry.IsDir() || isDirLink(dir, entry) {
			m.pathDirs = append(m.pathDirs, entry.Name())
		}
	}
	if len(m.pathDirs) == 0 {
		m.pathListNote = "Keine Unterordner."
	}
}

// isDirLink reports whether the entry is a symbolic link to a folder.
// os.ReadDir does not follow the link.
func isDirLink(dir string, entry fs.DirEntry) bool {
	if entry.Type()&fs.ModeSymlink == 0 {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, entry.Name()))
	return err == nil && info.IsDir()
}

// pathBase returns the folder that stands in the field, expanded and
// absolute. An empty field returns "".
func (m *Model) pathBase() string {
	value := strings.TrimSpace(m.pathInput.Value())
	if value == "" {
		return ""
	}
	dir := config.ExpandPath(value)
	if absolute, err := filepath.Abs(dir); err == nil {
		dir = absolute
	}
	return dir
}

// setPathDir writes a folder into the field and shows its subfolders. The
// name in highlight is marked afterwards, so the folder that was left stays
// in view.
func (m *Model) setPathDir(dir, highlight string) {
	m.pathInput.SetValue(displayPath(dir))
	m.pathInput.CursorEnd()
	m.loadPathDirs()
	for index, name := range m.pathDirs {
		if name == highlight {
			m.pathCursor = index
			m.scrollPathList()
			return
		}
	}
}

// enterPathDir opens the marked folder. The ".." line goes one level up.
func (m *Model) enterPathDir() {
	if m.pathCursor < 0 || m.pathCursor >= len(m.pathDirs) {
		return
	}
	name := m.pathDirs[m.pathCursor]
	if name == ".." {
		m.leavePathDir()
		return
	}
	m.setPathDir(filepath.Join(m.pathBase(), name), "")
}

// leavePathDir goes one level up and marks the folder that was left.
func (m *Model) leavePathDir() {
	dir := m.pathBase()
	if dir == "" {
		return
	}
	parent := filepath.Dir(dir)
	if parent == dir {
		return
	}
	m.setPathDir(parent, filepath.Base(dir))
}

// movePathCursor moves the mark in the folder list.
func (m *Model) movePathCursor(delta int) {
	if len(m.pathDirs) == 0 {
		return
	}
	m.pathCursor = min(max(m.pathCursor+delta, 0), len(m.pathDirs)-1)
	m.scrollPathList()
}

// scrollPathList keeps the mark inside the visible rows.
func (m *Model) scrollPathList() {
	rows := m.pathVisibleRows()
	if m.pathCursor < m.pathOffset {
		m.pathOffset = m.pathCursor
	}
	if m.pathCursor >= m.pathOffset+rows {
		m.pathOffset = m.pathCursor - rows + 1
	}
	m.pathOffset = min(m.pathOffset, max(len(m.pathDirs)-rows, 0))
	m.pathOffset = max(m.pathOffset, 0)
}

// displayPath shortens a folder below the home directory to "~", so the
// field keeps the short form that was typed.
func displayPath(dir string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return dir
	}
	if dir == home {
		return "~"
	}
	if strings.HasPrefix(dir, home+string(filepath.Separator)) {
		return "~" + dir[len(home):]
	}
	return dir
}
