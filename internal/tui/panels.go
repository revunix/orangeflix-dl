package tui

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/revunix/orangeflix-dl/internal/app"
	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// formatRow is one line of the format list. A row without a format is the
// header of a group.
type formatRow struct {
	format *orangeflix.Format
	header string
	count  int
}

// loginPrompt is the label of a field of the login form; the labels share
// one width, so the fields stand in one column.
func loginPrompt(label string) string {
	return padRight(label, 10)
}

// startPanel renders the form for the credentials.
func (m *Model) startPanel(width int) []string {
	// Die Bezeichner stehen gedämpft vor den Feldern, das aktive Feld trägt
	// den Akzent; die Farben folgen dem gewählten Erscheinungsbild.
	for _, input := range []*textinput.Model{&m.userInput, &m.passInput, &m.totpInput} {
		input.PromptStyle = theme.Muted
		input.PlaceholderStyle = theme.Subtle
		if input.Focused() {
			input.PromptStyle = theme.AccentBold
		}
	}
	content := []string{
		theme.Muted.Render("Die Zugangsdaten werden für die API und die Downloads gebraucht."),
		"",
		m.userInput.View(),
		m.passInput.View(),
		m.totpInput.View(),
		"",
		theme.Muted.Render("Den 2FA-Code nur eintragen, wenn das Konto ihn verlangt; er gilt einmal."),
		theme.Muted.Render("Erscheinungsbild: " + string(m.theme) + "   (ctrl+t wechselt: hell, dunkel, automatisch)"),
	}
	if m.profile != nil {
		content = append(content, "", m.profileLine())
	}
	return renderBox(width, "Zugangsdaten", content)
}

// formatsPanel renders the box with the list of the formats.
func (m *Model) formatsPanel(box rect) []string {
	formats := m.visibleFormats()
	if len(formats) == 0 {
		return renderBoxColor(box.width, "Formate", theme.BoxList,
			[]string{theme.Muted.Render("Kein Format gefunden.")})
	}

	rows := m.formatRows()
	cursorRow := 0
	for index, row := range rows {
		if row.format != nil && row.format.PID == formats[m.formatCursor].PID {
			cursorRow = index
			break
		}
	}
	budget := box.innerHeight()
	// Die Kopfzeile und der Filter<\/? nehmen Zeilen weg, so der Vorrat
	// der Liste kleiner ist als der Kasten.
	budget -= 1 // Spaltenüberschrift
	if m.filtering || m.filter != "" {
		budget -= 1
	}
	if budget < 1 {
		budget = 1
	}
	start, end := fitListWindow(&m.formatScroll, cursorRow, budget, len(rows))
	top, bottom := start > 0, end < len(rows)

	content := []string{}
	if m.filtering || m.filter != "" {
		content = append(content, m.filterInput.View())
	}
	content = append(content, m.formatHeaderLine(m.formatWidth))
	if top {
		content = append(content, theme.Muted.Render(fmt.Sprintf("… %d weitere", start)))
	}
	positions := map[int]int{}
	for index, format := range formats {
		positions[format.PID] = index
	}
	for index := start; index < end; index++ {
		row := rows[index]
		if row.format == nil {
			content = append(content, groupHeader(row.header, fmt.Sprintf("%d", row.count)))
			continue
		}
		if m.screen == ScreenFormats {
			m.addRowZone(box, len(content), m.pick(&m.formatCursor, positions[row.format.PID], "enter"), m.menuAt(&m.formatCursor, positions[row.format.PID]))
		}
		selected := row.format.PID == formats[m.formatCursor].PID
		content = append(content, m.hoverRow(box, len(content),
			renderRowWidth(m.formatLine(*row.format), selected, box.innerWidth()), selected))
	}
	if bottom {
		content = append(content, theme.Muted.Render(fmt.Sprintf("… %d weitere", len(rows)-end)))
	}
	return renderBoxAt(box.width, box.height, m.formatCardTitle(len(formats)), theme.BoxList, trimContent(content, box.innerHeight()))
}

// formatCardTitle is the title of the format box.
func (m *Model) formatCardTitle(count int) string {
	if m.libraryOnly {
		return fmt.Sprintf("Bibliothek (%d)", count)
	}
	return fmt.Sprintf("Formate (%d)", count)
}

// formatGroup is a group of formats of one status.
type formatGroup struct {
	status  string
	formats []orangeflix.Format
}

// formatGroups returns the visible formats in groups by their status. Inside
// a group the newest format comes first: the one with the most recent
// episode leads. Cursor und Liste lesen aus derselben Gruppe, darum
// stimmt die Markierung mit der Anzeige überein.
func (m *Model) formatGroups() []formatGroup {
	// Die Liste folgt der Quelle, aus der sie gefiltert wurde: dem Katalog
	// der API, sonst dem Katalog im Programm.
	source := m.formats
	if len(source) == 0 {
		source = orangeflix.Catalog
	}
	needle := strings.ToLower(m.filter)
	order, grouped := orangeflix.GroupFormatsByStatus(source)

	groups := []formatGroup{}
	for _, status := range order {
		group := formatGroup{status: status}
		for _, format := range grouped[status] {
			if needle != "" &&
				!strings.Contains(strings.ToLower(format.Title), needle) &&
				!strings.Contains(strings.ToLower(format.Slug), needle) {
				continue
			}
			if m.libraryOnly {
				if _, ok := m.libraryEntry(format.PID); !ok {
					continue
				}
			}
			group.formats = append(group.formats, format)
		}
		if len(group.formats) == 0 {
			continue
		}
		sortFormatsByLastEpisode(group.formats)
		groups = append(groups, group)
	}
	return groups
}

// formatRows builds the rows of the format list. The formats are grouped by
// their status.
func (m *Model) formatRows() []formatRow {
	rows := []formatRow{}
	for _, group := range m.formatGroups() {
		rows = append(rows, formatRow{header: statusLabel(group.status), count: len(group.formats)})
		for _, format := range group.formats {
			entry := format
			rows = append(rows, formatRow{format: &entry})
		}
	}
	return rows
}

// sortFormatsByLastEpisode sorts the formats by the date of their most
// recent episode, the newest first. Formats without a date come last.
func sortFormatsByLastEpisode(formats []orangeflix.Format) {
	sort.SliceStable(formats, func(i, j int) bool {
		a, b := formats[i], formats[j]
		switch {
		case a.LastEpisodeTime.IsZero() && b.LastEpisodeTime.IsZero():
			return a.Title < b.Title
		case a.LastEpisodeTime.IsZero():
			return false
		case b.LastEpisodeTime.IsZero():
			return true
		}
		if !a.LastEpisodeTime.Equal(b.LastEpisodeTime) {
			return a.LastEpisodeTime.After(b.LastEpisodeTime)
		}
		return a.Title < b.Title
	})
}

// formatColumns measures the columns of the format list. The columns that
// are less important are left out when the width does not carry them, the
// title takes the space that is left over.
type formatColumns struct {
	num   int
	new   int
	seen  int
	title int
	state int
	last  int
	count int
}

// measureFormatColumns measures the table of the formats. The markers and
// the numbers are narrow, the title takes the space that is left over; the
// less important columns are left out when the width does not carry them.
func measureFormatColumns(width, indent int) formatColumns {
	cols := formatColumns{num: 4, new: 1, seen: 1, state: 10, last: 10, count: 6}
	// Der Titel braucht Platz, sonst wird die Liste unlesbar.
	const minTitle = 14
	// Die Spalten fallen in dieser Reihenfolge weg: erst die Zahl der
	// Folgen, dann das Datum, dann die Marker.
	drop := []func(*formatColumns){
		func(c *formatColumns) { c.count = 0 },
		func(c *formatColumns) { c.last = 0 },
		func(c *formatColumns) { c.seen = 0 },
		func(c *formatColumns) { c.new = 0 },
	}
	for _, dropColumn := range drop {
		if cols.fixed(indent)+cols.gaps() <= width-minTitle {
			break
		}
		dropColumn(&cols)
	}
	cols.title = max(0, width-cols.fixed(indent)-cols.gaps())
	return cols
}

// fixed returns the width of the columns that are not the title.
func (c formatColumns) fixed(indent int) int {
	return indent + c.num + c.new + c.seen + c.state + c.last + c.count
}

// gaps returns the number of the spaces between the columns.
func (c formatColumns) gaps() int {
	gaps := 2
	for _, column := range []int{c.new, c.seen, c.last, c.count} {
		if column > 0 {
			gaps++
		}
	}
	return gaps
}

// formatHeaderLine names the columns of the format list, in the muted color
// so the rows come first. On a narrow terminal the less important columns
// are left out.
func (m *Model) formatHeaderLine(width int) string {
	cols := measureFormatColumns(width, 0)
	header := fmt.Sprintf("%-*s", cols.num, "#")
	if cols.new > 0 {
		header += " " + strings.Repeat(" ", cols.new)
	}
	if cols.seen > 0 {
		header += " " + strings.Repeat(" ", cols.seen)
	}
	header += fmt.Sprintf(" %-*s %-*s", cols.title, "TITEL", cols.state, "STATUS")
	if cols.last > 0 {
		header += fmt.Sprintf(" %-*s", cols.last, "LETZTE")
	}
	if cols.count > 0 {
		header += fmt.Sprintf(" %*s", cols.count, "FOLGEN")
	}
	return strings.Repeat(" ", rowIndent) + theme.Subtle.Render(header)
}

// formatLine renders one row of the format list: the number, the markers
// for a new episode and an opened format, the title and the dates.
func (m *Model) formatLine(format orangeflix.Format) string {
	cols := measureFormatColumns(m.formatWidth, 0)
	columns := theme.Muted.Render(fmt.Sprintf("%-*d", cols.num, format.PID))
	if cols.new > 0 {
		columns += " " + padStyled(theme.Warning, m.newMarker(format), cols.new)
	}
	if cols.seen > 0 {
		columns += " " + padStyled(theme.Success, m.seenMarker(format), cols.seen)
	}
	columns += " " + theme.Text.Render(fmt.Sprintf("%-*s", cols.title, truncateColumn(format.Title, cols.title)))
	columns += " " + theme.Muted.Render(fmt.Sprintf("%-*s", cols.state, format.StatusLabel()))
	if cols.last > 0 {
		columns += " " + theme.Muted.Render(fmt.Sprintf("%-*s", cols.last, format.LastEpisode))
	}
	if cols.count > 0 {
		columns += " " + theme.Muted.Render(fmt.Sprintf("%*s", cols.count, m.formatEpisodeCount(format.PID)))
	}
	return columns
}

// newMarker returns the mark for a format with a new episode. The age of
// the episode is measured against the current date.
func (m *Model) newMarker(format orangeflix.Format) string {
	if !format.NewEpisode {
		return ""
	}
	return "●"
}

// seenMarker returns the mark for a format the user has already been in.
func (m *Model) seenMarker(format orangeflix.Format) string {
	if !format.Opened {
		return ""
	}
	return "✓"
}

// formatEpisodeCount returns the number of the known episodes of a format.
func (m *Model) formatEpisodeCount(pid int) string {
	counts := m.knownCounts[pid]
	if counts.total == 0 {
		// Der Live-Katalog kennt die Zahl der Folgen.
		for _, format := range m.visibleFormats() {
			if format.PID == pid && format.ClipCount > 0 {
				return fmt.Sprintf("%d", format.ClipCount)
			}
		}
		return "-"
	}
	return fmt.Sprintf("%d", counts.total)
}

// formatDetail renders the box with the details of the marked format.
func (m *Model) formatDetail(box rect) []string {
	formats := m.visibleFormats()
	if len(formats) == 0 {
		return renderBoxColor(box.width, "Angaben", theme.BoxDetail, nil)
	}
	format := formats[m.formatCursor]

	lines := []string{}
	lines = append(lines, detailRow("Nummer", itoa(int64(format.PID))))
	lines = append(lines, detailRow("Slug", format.Slug))
	lines = append(lines, detailRow("Status", format.StatusLabel()))
	lines = append(lines, detailRow("Letzte Folge", format.LastEpisode))
	if format.NewEpisode {
		lines = append(lines, detailRow("Neue Folge", "ja"))
	}
	if format.Opened {
		lines = append(lines, detailRow("Besucht", "ja"))
	}

	counts := m.knownCounts[format.PID]
	if counts.total > 0 {
		lines = append(lines, detailRow("Folgen", fmt.Sprintf("%d (%d geladen)", counts.total, counts.loaded)))
	}
	if entry, ok := m.libraryEntry(format.PID); ok {
		lines = append(lines, detailRow("Bibliothek", "ja"))
		if entry.CheckedAt != nil {
			lines = append(lines, detailRow("Geprüft", entry.CheckedAt.Format("02.01.2006 15:04")))
		}
		lines = append(lines, detailRow("Neue Folgen", itoa(int64(entry.NewCount))))
		if entry.LastError != "" {
			lines = append(lines, detailRow("Letzter Fehler", entry.LastError))
		}
	} else {
		lines = append(lines, detailRow("Bibliothek", "nein"))
	}
	if format.Description != "" {
		lines = append(lines, "", theme.Muted.Render("Beschreibung"))
		lines = append(lines, wrapText(format.Description, box.innerWidth())...)
	}
	return renderBoxAt(box.width, box.height, format.Title, theme.BoxDetail, trimContent(lines, box.innerHeight()))
}

// episodeGroup is a group of episodes of one year.
type episodeGroup struct {
	year   string
	items  []orangeflix.Episode
	loaded int
}

// episodeGroups groups the episodes by their year, the newest year first.
func (m *Model) episodeGroups() []episodeGroup {
	groups := []episodeGroup{}
	index := map[string]int{}
	for _, episode := range m.filteredEpisodes() {
		year := "unbekannt"
		if !episode.Date.IsZero() {
			year = episode.Date.Local().Format("2006")
		}
		position, ok := index[year]
		if !ok {
			position = len(groups)
			index[year] = position
			groups = append(groups, episodeGroup{year: year})
		}
		groups[position].items = append(groups[position].items, episode)
		if m.episodeLoaded(episode.Identifier) {
			groups[position].loaded++
		}
	}
	return groups
}

// episodesPanel renders the box with the list of the episodes.
func (m *Model) episodesPanel(box rect) []string {
	if m.loading {
		return renderBoxColor(box.width, "Folgen", theme.BoxList,
			[]string{theme.Muted.Render(m.spinner.View() + " " + m.loadingLabel)})
	}
	episodes := m.filteredEpisodes()
	if len(episodes) == 0 {
		return renderBoxColor(box.width, "Folgen", theme.BoxList,
			[]string{theme.Muted.Render("Keine Folge gefunden.")})
	}

	groups := m.episodeGroups()
	rows := m.episodeRows(groups)
	cursorRow := 0
	for index, row := range rows {
		if row.episode != nil && row.episode.Identifier == episodes[m.episodeCursor].Identifier {
			cursorRow = index
			break
		}
	}
	budget := box.innerHeight()
	// Die Kopfzeile und der Jahreseingang nehmen Zeilen weg.
	budget -= 1
	if m.yearFiltering || m.episodeYear != "" {
		budget -= 1
	}
	if budget < 1 {
		budget = 1
	}
	start, end := fitListWindow(&m.episodeScroll, cursorRow, budget, len(rows))
	top, bottom := start > 0, end < len(rows)

	content := []string{}
	if m.yearFiltering || m.episodeYear != "" {
		content = append(content, m.yearInput.View())
	}
	content = append(content, m.episodeHeaderLine(m.episodeWidth))
	if top {
		content = append(content, theme.Muted.Render(fmt.Sprintf("… %d weitere", start)))
	}
	positions := map[string]int{}
	for index, episode := range episodes {
		positions[episode.Identifier] = index
	}
	for index := start; index < end; index++ {
		row := rows[index]
		if row.episode == nil {
			content = append(content, groupHeader(row.header,
				fmt.Sprintf("%s · %d geladen", plural(int64(row.count), "Folge", "Folgen"), row.loaded)))
			continue
		}
		// Ein Klick wählt die Folge, ein zweiter öffnet das Menü; die
		// rechte Taste öffnet das Kontextmenü.
		position := positions[row.episode.Identifier]
		m.addRowZone(box, len(content), m.pick(&m.episodeCursor, position, "enter"), m.menuAt(&m.episodeCursor, position))
		selected := row.episode.Identifier == episodes[m.episodeCursor].Identifier
		content = append(content, m.hoverRow(box, len(content),
			renderRowWidth(m.episodeLine(*row.episode), selected, box.innerWidth()), selected))
	}
	if bottom {
		content = append(content, theme.Muted.Render(fmt.Sprintf("… %d weitere", len(episodes)-end)))
	}
	return renderBoxAt(box.width, box.height, m.episodeCardTitle(len(episodes)), theme.BoxList, trimContent(content, box.innerHeight()))
}

// episodeCardTitle is the title of the episode box.
func (m *Model) episodeCardTitle(count int) string {
	title := fmt.Sprintf("Folgen (%d)", count)
	if m.episodeYear != "" {
		title = fmt.Sprintf("Folgen %s (%d)", m.episodeYear, count)
	}
	return title
}

// episodeRow is one line of the episode list.
type episodeRow struct {
	episode *orangeflix.Episode
	header  string
	count   int
	loaded  int
}

// episodeRows builds the rows of the episode list.
func (m *Model) episodeRows(groups []episodeGroup) []episodeRow {
	rows := []episodeRow{}
	for _, group := range groups {
		rows = append(rows, episodeRow{header: group.year, count: len(group.items), loaded: group.loaded})
		for index := range group.items {
			episode := group.items[index]
			rows = append(rows, episodeRow{episode: &episode})
		}
	}
	return rows
}

// episodeColumns measures the columns of the episode list.
type episodeColumns struct {
	num    int
	date   int
	length int
	title  int
	state  int
}

// measureEpisodeColumns measures the table of the episodes. The length is
// left out when the width does not carry it.
func measureEpisodeColumns(width, indent int) episodeColumns {
	cols := episodeColumns{num: 5, date: 10, length: 8, state: episodeStateWidth}
	if width < indent+cols.num+cols.date+cols.length+cols.state+4+20 {
		cols.length = 0
	}
	gaps := 3
	if cols.length > 0 {
		gaps++
	}
	fixed := indent + cols.num + cols.date + cols.length + cols.state + gaps
	cols.title = max(0, width-fixed)
	return cols
}

// episodeHeaderLine names the columns of the episode list.
func (m *Model) episodeHeaderLine(width int) string {
	cols := measureEpisodeColumns(width, 0)
	header := fmt.Sprintf("%-*s %-*s", cols.num, "#", cols.date, "DATUM")
	if cols.length > 0 {
		header += fmt.Sprintf(" %-*s", cols.length, "LÄNGE")
	}
	header += fmt.Sprintf(" %-*s %s", cols.title, "TITEL", "STATUS")
	return strings.Repeat(" ", rowIndent) + theme.Subtle.Render(header)
}

// episodeLine renders one row of the episode list.
func (m *Model) episodeLine(episode orangeflix.Episode) string {
	cols := measureEpisodeColumns(m.episodeWidth, 0)
	columns := theme.Muted.Render(fmt.Sprintf("%-*s", cols.num, padNumber(episode.Number, 3)))
	columns += " " + theme.Muted.Render(fmt.Sprintf("%-*s", cols.date, formatDate(episode)))
	if cols.length > 0 {
		columns += " " + theme.Muted.Render(fmt.Sprintf("%-*s", cols.length, episode.Duration))
	}
	columns += " " + theme.Text.Render(fmt.Sprintf("%-*s", cols.title, truncateColumn(episode.Title, cols.title)))
	columns += " " + m.episodeState(episode.Identifier)
	return columns
}

// episodeDetail renders the box with the details of the marked episode.
func (m *Model) episodeDetail(box rect) []string {
	episode, ok := m.detailEpisode()
	if !ok {
		return renderBoxColor(box.width, "Angaben", theme.BoxDetail, nil)
	}

	lines := []string{}
	lines = append(lines, detailRow("Nummer", itoa(int64(episode.Number))))
	lines = append(lines, detailRow("Kennung", episode.Identifier))
	lines = append(lines, detailRow("Datum", formatDate(episode)))
	if episode.Duration != "" {
		lines = append(lines, detailRow("Länge", episode.Duration))
	}
	if episode.FormatName != "" {
		lines = append(lines, detailRow("Format", episode.FormatName))
	}
	lines = append(lines, detailRow("Status", m.episodeState(episode.Identifier)))
	bookmark := "nein"
	if episode.Bookmarked {
		bookmark = "ja"
	}
	lines = append(lines, detailRow("Merkliste", bookmark))
	if episode.ProgressPercent > 0 {
		lines = append(lines, detailRow("Gesehen", fmt.Sprintf("%d %%", episode.ProgressPercent)))
	}
	if len(episode.Participants) > 0 {
		names := make([]string, 0, len(episode.Participants))
		for _, participant := range episode.Participants {
			names = append(names, participant.Name)
		}
		lines = append(lines, detailRow("Gäste", strings.Join(names, ", ")))
	}
	if known, ok := m.known[episode.Identifier]; ok && known.Path != "" {
		lines = append(lines, detailRow("Datei", known.Path))
	}
	if episode.Description != "" {
		lines = append(lines, "", theme.Muted.Render("Beschreibung"))
		lines = append(lines, wrapText(episode.Description, box.innerWidth())...)
	}
	if episode.Thumb != "" {
		lines = append(lines, "", theme.Muted.Render("Vorschaubild"))
		lines = append(lines, theme.Muted.Render(fitToWidth(episode.Thumb, box.innerWidth())))
	}

	lines = append(lines, m.clipDetail(box.innerWidth())...)
	return renderBoxAt(box.width, box.height, episode.Title, theme.BoxDetail, trimContent(lines, box.innerHeight()))
}

// clipDetail renders the available files of the marked episode.
func (m *Model) clipDetail(width int) []string {
	if m.clipLoading {
		return []string{"", theme.Muted.Render(m.spinner.View() + " lade die Qualitäten …")}
	}
	files := m.clips[episodeIdentifier(m)]
	if len(files) == 0 {
		return []string{"", theme.Muted.Render("Mit 'i' die Qualitäten laden.")}
	}
	lines := []string{"", theme.Muted.Render("Qualitäten")}
	withDimensions := width >= 34
	for _, file := range files {
		name := fmt.Sprintf("  %-11s", truncateColumn(file.Desc, 11))
		if withDimensions {
			dimensions := file.Dimensions
			if dimensions == "" {
				dimensions = "-"
			}
			name += fmt.Sprintf("%-11s", truncateColumn(dimensions, 11))
		}
		lines = append(lines, fitToWidth(fmt.Sprintf("%s %s", name, humanBytes(file.Size)), width))
	}
	return lines
}

// logPanel renders the box with the protocol.
func (m *Model) logPanel(box rect) []string {
	content := []string{}
	lines := m.logWindow(box.innerHeight())
	if len(lines) == 0 {
		content = append(content, theme.Muted.Render("Noch nichts passiert."))
	}
	for _, line := range lines {
		content = append(content, theme.Muted.Render(fitToWidth(line, box.innerWidth())))
	}
	return renderBoxAt(box.width, box.height, "Protokoll", theme.BoxLog, trimContent(content, box.innerHeight()))
}

// logWindow returns the lines of the protocol that are shown. The offset
// scrolls back into the history.
func (m *Model) logWindow(height int) []string {
	lines := m.log
	if m.logOffset > 0 {
		end := len(lines) - m.logOffset
		if end < 0 {
			end = 0
		}
		start := end - height
		if start < 0 {
			start = 0
		}
		return lines[start:end]
	}
	return m.tailLog(height)
}

// infoPanel renders the box with the account and the settings.
func (m *Model) infoPanel(box rect) []string {
	rows := []string{
		detailRow("Neue Folgen", itoa(int64(m.newCount()))),
		detailRow("Bibliothek", plural(int64(len(m.library)), "Format", "Formate")),
	}
	if m.config != nil {
		rows = append(rows,
			detailRow("Qualität", m.config.Quality),
			detailRow("Layout", m.config.Layout),
			detailRow("Zielordner", m.config.DownloadsDir),
		)
	}
	rows = append(rows, detailRow("Version", "v"+m.version))
	// Der Zugang steht oben, abgesetzt; leere Angaben hinterlassen keine
	// Lücke.
	content := []string{m.profileLine(), ""}
	for _, row := range rows {
		if row != "" {
			content = append(content, row)
		}
	}
	return renderBoxAt(box.width, box.height, "Konto", theme.BoxInfo, trimContent(content, box.innerHeight()))
}

// profileLine describes the state of the subscription.
func (m *Model) profileLine() string {
	if m.profile == nil {
		switch {
		case m.profileLoading:
			return detailRow("Zugang", theme.Muted.Render("wird geprüft …"))
		case errors.Is(m.profileErr, app.ErrTotpRequired):
			return detailRow("Zugang", theme.Warning.Render("2FA-Code nötig"))
		case m.profileErr != nil:
			return detailRow("Zugang", theme.Danger.Render("Anmeldung fehlgeschlagen"))
		}
		return detailRow("Zugang", theme.Muted.Render("unbekannt"))
	}
	if !m.profile.HasAccess {
		return detailRow("Zugang", theme.Warning.Render("● abgelaufen"))
	}
	return detailRow("Zugang", theme.Success.Render("● "+m.profile.Summarize()))
}

// newCount returns the number of the new episodes of the library.
func (m *Model) newCount() int {
	total := 0
	for _, entry := range m.library {
		total += entry.NewCount
	}
	return total
}

// queuePanel renders the box of the running downloads.
func (m *Model) queuePanel(box rect) []string {
	content := []string{}
	if len(m.jobs) == 0 {
		content = append(content, theme.Muted.Render(m.spinner.View()+" Der Download läuft …"))
	}
	width := box.innerWidth()
	for index, job := range m.jobs {
		if index > 0 {
			content = append(content, "")
		}
		percent := 0
		if job.Total > 0 {
			percent = int(float64(job.Done) / float64(job.Total) * 100)
		}
		// Oben das Zeichen des Zustands und der Titel, rechts die Zahlen.
		facts := []string{job.State, fmt.Sprintf("%d%%", percent)}
		if job.Total > 0 {
			facts = append(facts, humanBytes(job.Done)+" / "+humanBytes(job.Total))
		}
		if remaining := job.remaining(); remaining != "" {
			facts = append(facts, remaining)
		}
		right := theme.Muted.Render(strings.Join(facts, " · "))
		left := jobIcon(job.State) + " " + theme.Title.Render(fitToWidth(job.Label, max(1, width-ansi.StringWidth(right)-4)))
		gap := max(1, width-ansi.StringWidth(left)-ansi.StringWidth(right))
		content = append(content, left+strings.Repeat(" ", gap)+right)
		// Darunter der Balken über die ganze Breite und der Pfad.
		content = append(content, "  "+renderMeter(max(10, width-2), percent, ""))
		if job.Path != "" {
			content = append(content, "  "+theme.Subtle.Render(fitToWidth(job.Path, max(1, width-2))))
		}
	}
	return renderBoxAt(box.width, box.height, fmt.Sprintf("Downloads (%d)", len(m.jobs)), theme.BoxList, trimContent(content, box.innerHeight()))
}

// jobIcon returns the mark of the state of a download in its color.
func jobIcon(state string) string {
	switch state {
	case "fertig":
		return theme.Success.Render("✓")
	case "übersprungen":
		return theme.Muted.Render("–")
	case "fehlgeschlagen":
		return theme.Danger.Render("✗")
	case "wartet":
		return theme.Muted.Render("○")
	}
	return theme.Accent.Render("●")
}

// summaryPanel renders the result of a run.
func (m *Model) summaryPanel(box rect) []string {
	content := []string{
		fmt.Sprintf("  %-18s %d", theme.Success.Render("geladen"), m.summary.done),
		fmt.Sprintf("  %-18s %d", theme.Muted.Render("übersprungen"), m.summary.skipped),
		fmt.Sprintf("  %-18s %d", theme.Danger.Render("fehlgeschlagen"), m.summary.failed),
	}
	if len(m.jobs) > 0 {
		content = append(content, "", theme.Muted.Render("Dateien"))
		for _, job := range m.jobs {
			content = append(content, "  "+theme.Muted.Render(job.Label)+"  "+job.State)
		}
	}
	if m.message != "" {
		content = append(content, "")
		if m.isError {
			content = append(content, theme.Danger.Render(fitToWidth(m.message, box.innerWidth())))
		} else {
			content = append(content, theme.Muted.Render(fitToWidth(m.message, box.innerWidth())))
		}
	}
	return renderBoxAt(box.width, box.height, "Fertig", theme.BoxInfo, trimContent(content, box.innerHeight()))
}

// searchPanel renders the results of the full text search.
func (m *Model) searchPanel(box rect) []string {
	content := []string{m.searchInput.View(), ""}
	if m.loading {
		content = append(content, theme.Muted.Render(m.spinner.View()+" "+m.loadingLabel))
		return renderBoxColor(box.width, "Volltextsuche", theme.BoxList, content)
	}
	if len(m.searchResults) == 0 {
		if m.searchQuery == "" {
			content = append(content, theme.Muted.Render("Ein Suchbegriff sucht in allen Titeln, Beschreibungen und Transkripten."))
		} else {
			content = append(content, theme.Muted.Render("Keine Treffer für "+m.searchQuery))
		}
		return renderBoxColor(box.width, "Volltextsuche", theme.BoxList, content)
	}
	budget := box.innerHeight() - 3
	if budget < 1 {
		budget = 1
	}
	start, end := fitListWindow(&m.searchScroll, m.searchCursor, budget, len(m.searchResults))
	top, bottom := start > 0, end < len(m.searchResults)
	if top {
		content = append(content, theme.Muted.Render(fmt.Sprintf("… %d weitere", start)))
	}
	for index := start; index < end; index++ {
		hit := index
		m.addRowZone(box, len(content), func() (tea.Model, tea.Cmd) {
			if m.searchCursor == hit {
				m.openActionMenu()
				return m, nil
			}
			m.searchCursor = hit
			return m, nil
		}, m.menuAt(&m.searchCursor, hit))
		content = append(content, m.hoverRow(box, len(content),
			renderRowWidth(m.searchLine(m.searchResults[index]), index == m.searchCursor, box.innerWidth()), index == m.searchCursor))
	}
	if bottom {
		content = append(content, theme.Muted.Render(fmt.Sprintf("… %d weitere", len(m.searchResults)-end)))
	}
	return renderBoxAt(box.width, box.height, fmt.Sprintf("Volltextsuche (%d)", len(m.searchResults)), theme.BoxList, content)
}

// searchLine renders one row of the search results.
func (m *Model) searchLine(episode orangeflix.Episode) string {
	return theme.Muted.Render(fmt.Sprintf("%-5s", padNumber(episode.Number, 3))) + " " +
		theme.AccentSoft.Render(fmt.Sprintf("%-22s", truncateColumn(episode.FormatName, 22))) + " " +
		theme.Muted.Render(fmt.Sprintf("%-8s", episode.Duration)) + " " +
		theme.Text.Render(truncateColumn(episode.Title, 40))
}

// trimContent shortens the content of a box to the space it has.
func trimContent(content []string, height int) []string {
	if height > 0 && len(content) > height {
		return content[:height]
	}
	return content
}

// truncateColumn shortens a text to the width of its column, counted in
// terminal cells.
func truncateColumn(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(value) <= width {
		return value
	}
	return ansi.Truncate(value, width, "…")
}

// episodeLoaded reports whether the episode was downloaded.
func (m *Model) episodeLoaded(identifier string) bool {
	known, ok := m.known[identifier]
	if !ok {
		return false
	}
	return known.DownloadedAt != nil
}

// episodeNew reports whether the episode was seen for the first time in the
// last week.
func (m *Model) episodeNew(identifier string) bool {
	known, ok := m.known[identifier]
	if !ok || known.DownloadedAt != nil {
		return false
	}
	return time.Since(known.FirstSeen) < 7*24*time.Hour
}

// episodeStateWidth is the room of the state column. It fits
// "✓ heruntergeladen".
const episodeStateWidth = 18

// episodeState returns the state of an episode: heruntergeladen, gesehen,
// the watched share, neu or offen.
func (m *Model) episodeState(identifier string) string {
	if m.episodeLoaded(identifier) {
		return statusBadge("heruntergeladen")
	}
	if percent := m.episodeProgress(identifier); percent > 0 {
		if percent >= watchedPercent {
			return statusBadge("gesehen")
		}
		return statusBadge(fmt.Sprintf("▶ %d %%", percent))
	}
	if m.episodeNew(identifier) {
		return statusBadge("neu")
	}
	return statusBadge("offen")
}

// watchedPercent is the share from which a clip counts as watched. The
// player rarely stops on the very last second.
const watchedPercent = 90

// episodeProgress returns how much of an episode was watched, from the list
// or from the clip screen.
func (m *Model) episodeProgress(identifier string) int {
	if identifier == "" {
		return 0
	}
	for _, episode := range m.episodes {
		if episode.Identifier == identifier {
			return episode.ProgressPercent
		}
	}
	for _, episode := range m.clipList.episodes {
		if episode.Identifier == identifier {
			return episode.ProgressPercent
		}
	}
	for _, episode := range m.searchResults {
		if episode.Identifier == identifier {
			return episode.ProgressPercent
		}
	}
	return 0
}

// episodeIdentifier returns the identifier of the marked episode.
func episodeIdentifier(m *Model) string {
	episode, ok := m.detailEpisode()
	if !ok {
		return ""
	}
	return episode.Identifier
}

// logTime returns the time of a log line.
func logTime(value time.Time) string {
	return value.Local().Format("15:04:05")
}

// groupHeader renders the head of a group in a list: the name in the brand
// color and the counts behind it in the muted color.
func groupHeader(name, counts string) string {
	return theme.AccentBold.Render(name) + "  " + theme.Subtle.Render(counts)
}

// clipPanel renders the generic clip list.
func (m *Model) clipPanel(box rect) []string {
	empty := "Keine Folgen."
	switch m.clipList.kind {
	case clipListRecents:
		empty = "Noch nichts angefangen."
	case clipListBookmarks:
		empty = "Keine gemerkten Folgen."
	case clipListSimilar:
		empty = "Keine ähnlichen Folgen gefunden."
	case clipListHost:
		empty = "Keine Folgen dieses Gasts."
	}
	if m.loading && len(m.clipList.episodes) == 0 {
		return renderBoxColor(box.width, m.clipList.title, theme.BoxList,
			[]string{theme.Muted.Render(m.spinner.View() + " " + m.loadingLabel)})
	}
	if len(m.clipList.episodes) == 0 {
		return renderBoxColor(box.width, m.clipList.title, theme.BoxList,
			[]string{theme.Muted.Render(empty)})
	}
	budget := box.innerHeight() - 1
	if budget < 1 {
		budget = 1
	}
	start, end := fitListWindow(&m.clipList.scroll, m.clipList.cursor, budget, len(m.clipList.episodes))
	content := []string{m.clipHeaderLine(box.innerWidth())}
	if start > 0 {
		content = append(content, theme.Muted.Render(fmt.Sprintf("… %d weitere", start)))
	}
	for index := start; index < end; index++ {
		m.addRowZone(box, len(content), m.pick(&m.clipList.cursor, index, "enter"), m.menuAt(&m.clipList.cursor, index))
		content = append(content, m.hoverRow(box, len(content),
			renderRowWidth(m.clipLine(m.clipList.episodes[index]), index == m.clipList.cursor, box.innerWidth()), index == m.clipList.cursor))
	}
	if end < len(m.clipList.episodes) {
		content = append(content, theme.Muted.Render(fmt.Sprintf("… %d weitere", len(m.clipList.episodes)-end)))
	}
	return renderBoxAt(box.width, box.height, m.clipList.title, theme.BoxList, trimContent(content, box.innerHeight()))
}

// clipHeaderLine names the columns of the clip list.
func (m *Model) clipHeaderLine(width int) string {
	header := fmt.Sprintf("%-1s %-*s %-22s %-8s %s", "", episodeStateWidth, "STATUS", "FORMAT", "LÄNGE", "TITEL")
	return theme.Subtle.Render(fitToWidth(header, width))
}

// clipLine renders one row of the clip list.
func (m *Model) clipLine(episode orangeflix.Episode) string {
	star := " "
	if episode.Bookmarked {
		star = "★"
	}
	state := m.episodeState(episode.Identifier)
	if gap := episodeStateWidth - ansi.StringWidth(state); gap > 0 {
		state += strings.Repeat(" ", gap)
	}
	line := theme.Accent.Render(star) + " " +
		state + " " +
		padStyled(theme.AccentSoft, truncateColumn(episode.FormatName, 22), 22) + " " +
		padStyled(theme.Muted, episode.Duration, 8) + " " +
		theme.Text.Render(truncateColumn(episode.Title, 60))
	return line
}

// clipDetailPanel renders the detail pane of the clip screen.
func (m *Model) clipDetailPanel(box rect) []string {
	if m.clipList.kind == clipListHost {
		return m.hostDetail(box)
	}
	return m.episodeDetail(box)
}

// hostDetail renders the page of a guest.
func (m *Model) hostDetail(box rect) []string {
	if m.host == nil {
		return renderBoxColor(box.width, "Gast", theme.BoxDetail, nil)
	}
	host := m.host.Host
	lines := []string{detailRow("Name", host.Name)}
	if host.Description != "" {
		lines = append(lines, "", theme.Muted.Render("Beschreibung"))
		lines = append(lines, wrapText(host.Description, box.innerWidth())...)
	}
	if len(m.host.Magazines) > 0 {
		names := make([]string, 0, len(m.host.Magazines))
		for _, magazine := range m.host.Magazines {
			names = append(names, magazine.Title)
		}
		lines = append(lines, detailRow("Formate", strings.Join(names, ", ")))
	}
	if host.ClipCount > 0 {
		lines = append(lines, detailRow("Folgen", itoa(int64(host.ClipCount))))
	}
	return renderBoxAt(box.width, box.height, "Gast", theme.BoxDetail, trimContent(lines, box.innerHeight()))
}

// calendarRow is one line of the calendar.
type calendarRow struct {
	header string
	entry  *orangeflix.V3CalendarEntry
}

// calendarPanel renders the calendar grouped by day.
func (m *Model) calendarPanel(box rect) []string {
	if m.loading && len(m.calendar) == 0 {
		return renderBoxColor(box.width, "Kalender", theme.BoxList,
			[]string{theme.Muted.Render(m.spinner.View() + " " + m.loadingLabel)})
	}
	if len(m.calendar) == 0 {
		return renderBoxColor(box.width, "Kalender", theme.BoxList,
			[]string{theme.Muted.Render("Keine Termine in der nächsten Zeit.")})
	}
	rows := m.calendarRows()
	budget := box.innerHeight()
	if budget < 1 {
		budget = 1
	}
	cursorRow := 0
	for index, row := range rows {
		if row.entry != nil && m.calendarCursor < len(m.calendar) && row.entry.ID == m.calendar[m.calendarCursor].ID {
			cursorRow = index
			break
		}
	}
	start, end := fitListWindow(&m.calendarScroll, cursorRow, budget, len(rows))
	content := []string{}
	if start > 0 {
		content = append(content, theme.Muted.Render(fmt.Sprintf("… %d weitere", start)))
	}
	for index := start; index < end; index++ {
		row := rows[index]
		if row.entry == nil {
			content = append(content, groupHeader(row.header, ""))
			continue
		}
		for position, entry := range m.calendar {
			if entry.ID == row.entry.ID {
				m.addRowZone(box, len(content), m.pick(&m.calendarCursor, position, "enter"), m.menuAt(&m.calendarCursor, position))
				break
			}
		}
		selected := row.entry.ID == m.calendar[m.calendarCursor].ID
		content = append(content, m.hoverRow(box, len(content),
			renderRowWidth(m.calendarLine(*row.entry), selected, box.innerWidth()), selected))
	}
	if end < len(rows) {
		content = append(content, theme.Muted.Render(fmt.Sprintf("… %d weitere", len(rows)-end)))
	}
	return renderBoxAt(box.width, box.height, "Kalender", theme.BoxList, trimContent(content, box.innerHeight()))
}

// calendarRows groups the entries by their date and sorts each day by time.
func (m *Model) calendarRows() []calendarRow {
	type group struct {
		date  string
		items []orangeflix.V3CalendarEntry
	}
	var groups []group
	for _, entry := range m.calendar {
		if len(groups) == 0 || groups[len(groups)-1].date != entry.Date {
			groups = append(groups, group{date: entry.Date})
		}
		groups[len(groups)-1].items = append(groups[len(groups)-1].items, entry)
	}
	rows := []calendarRow{}
	for _, group := range groups {
		sort.SliceStable(group.items, func(i, j int) bool { return group.items[i].Time < group.items[j].Time })
		rows = append(rows, calendarRow{header: group.date})
		for index := range group.items {
			entry := group.items[index]
			rows = append(rows, calendarRow{entry: &entry})
		}
	}
	return rows
}

// calendarLine renders one calendar entry.
func (m *Model) calendarLine(entry orangeflix.V3CalendarEntry) string {
	badge := "TERMIN"
	switch entry.Type {
	case "live":
		badge = "LIVE"
	case "event":
		badge = "EVENT"
	}
	line := padStyled(theme.Muted, entry.Time, 6) + " " +
		padStyled(theme.Accent, badge, 7) + " " +
		theme.Text.Render(truncateColumn(entry.Title, 36))
	if entry.Description != "" {
		line += " " + theme.Muted.Render(truncateColumn(entry.Description, 30))
	}
	return line
}

// livePanel renders the live state. The box stands as a narrow column
// beside the chat, so every text wraps instead of being cut.
func (m *Model) livePanel(box rect) []string {
	width := box.innerWidth()
	lines := []string{}
	add := func(style lipgloss.Style, text string) {
		for _, part := range wrapText(text, width) {
			lines = append(lines, style.Render(part))
		}
	}
	gapLine := func() {
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
	}
	if m.liveLoading || (m.live == nil && m.loading) {
		lines = append(lines, theme.Muted.Render(m.spinner.View()+" Lade den Live-Stand …"))
		return renderBoxAt(box.width, box.height, "Live", theme.BoxList, trimContent(lines, box.innerHeight()))
	}
	if m.live == nil {
		lines = append(lines, theme.Muted.Render("Keine Live-Angaben."))
		return renderBoxAt(box.width, box.height, "Live", theme.BoxList, trimContent(lines, box.innerHeight()))
	}
	live := m.live
	if live.Message != "" {
		add(theme.Warning, live.Message)
		gapLine()
	}
	if live.IsLive {
		lines = append(lines, theme.Success.Bold(true).Render("● LIVE"))
		title := "Livesendung"
		if live.Show != nil && live.Show.Title != "" {
			title = live.Show.Title
		}
		add(theme.Title.Bold(true), title)
		if show := live.Show; show != nil {
			if show.Begin > 0 || show.End > 0 {
				when := formatClock(show.Begin) + " – " + formatClock(show.End)
				if now := time.Now(); show.End > now.Unix() {
					when += " · noch " + strings.TrimPrefix(liveCountdown(time.Unix(show.End, 0), now), "in ")
				}
				add(theme.Muted, when)
			}
			if show.Description != "" {
				gapLine()
				add(theme.Muted, show.Description)
			}
		}
		gapLine()
		facts := []string{}
		if live.Show != nil {
			facts = append(facts, "Chat "+onOff(live.Show.Chat))
		}
		facts = append(facts, "Zugang "+yesNo(live.HasAccess()))
		add(theme.Muted, strings.Join(facts, " · "))
		if m.chatPeople != "" {
			add(theme.Muted, m.chatPeople)
		}
		if live.HasAccess() {
			gapLine()
			lines = append(lines, theme.AccentBold.Render("▶ ")+theme.Title.Render("enter")+theme.Accent.Render(" ansehen"))
		}
	} else {
		lines = append(lines, theme.Muted.Render("○ Keine Livesendung"))
	}
	if replay := live.Replay; replay != nil && replay.Possible {
		gapLine()
		lines = append(lines, theme.AccentSoft.Bold(true).Render("↺ Replay"))
		title := replay.Title
		if title == "" {
			title = replay.ID
		}
		add(theme.Text, title)
		if replay.End > 0 {
			add(theme.Muted, "bis "+formatUnix(replay.End))
		}
		if replay.CanAccess {
			lines = append(lines, theme.AccentBold.Render("▶ ")+theme.Title.Render("r")+theme.Accent.Render(" Replay ansehen"))
		}
	}
	if next := live.Scheduled; next != nil {
		gapLine()
		lines = append(lines, theme.Muted.Bold(true).Render("Nächste Sendung"))
		add(theme.Text, next.Title)
		if next.StartsIn > 0 {
			now := time.Now()
			add(theme.Muted, liveCountdown(now.Add(time.Duration(next.StartsIn)*time.Second), now))
		}
	}
	gapLine()
	lines = append(lines, theme.Subtle.Render("R aktualisieren"))
	return renderBoxAt(box.width, box.height, "Live", theme.BoxList, trimContent(lines, box.innerHeight()))
}

// formatClock formats the hour of a Unix timestamp.
func formatClock(value int64) string {
	if value <= 0 {
		return "–"
	}
	return time.Unix(value, 0).Local().Format("15:04")
}

// onOff formats a switch for the interface.
func onOff(value bool) string {
	if value {
		return "an"
	}
	return "aus"
}

// announcementPanel renders the announcements.
func (m *Model) announcementPanel(box rect) []string {
	if m.loading && len(m.announcements) == 0 {
		return renderBoxColor(box.width, "Ankündigungen", theme.BoxList,
			[]string{theme.Muted.Render(m.spinner.View() + " " + m.loadingLabel)})
	}
	if len(m.announcements) == 0 {
		return renderBoxColor(box.width, "Ankündigungen", theme.BoxList,
			[]string{theme.Muted.Render("Keine Ankündigungen.")})
	}
	budget := box.innerHeight()
	if budget < 1 {
		budget = 1
	}
	start, end := fitListWindow(&m.announceScroll, m.announceCursor, budget, len(m.announcements))
	content := []string{}
	for index := start; index < end; index++ {
		item := m.announcements[index]
		line := theme.Muted.Render(fmt.Sprintf("%-7s", strings.ToUpper(item.Type))) + " " +
			theme.Text.Render(truncateColumn(item.Title, 50))
		m.addRowZone(box, len(content), m.pick(&m.announceCursor, index, "o"), m.menuAt(&m.announceCursor, index))
		content = append(content, m.hoverRow(box, len(content),
			renderRowWidth(line, index == m.announceCursor, box.innerWidth()), index == m.announceCursor))
	}
	return renderBoxAt(box.width, box.height, "Ankündigungen", theme.BoxList, trimContent(content, box.innerHeight()))
}

// announcementDetail renders the selected announcement.
func (m *Model) announcementDetail(box rect) []string {
	if m.announceCursor < 0 || m.announceCursor >= len(m.announcements) {
		return renderBoxColor(box.width, "Hinweis", theme.BoxDetail, nil)
	}
	item := m.announcements[m.announceCursor]
	lines := []string{detailRow("Typ", item.Type)}
	if item.TimeDescription != "" {
		lines = append(lines, detailRow("Zeit", item.TimeDescription))
	}
	text := item.Description
	if text == "" {
		text = item.ShortDescription
	}
	if text == "" {
		text = item.BodyMarkdown
	}
	if text != "" {
		lines = append(lines, "", theme.Muted.Render("Text"))
		lines = append(lines, wrapText(text, box.innerWidth())...)
	}
	if item.URL != "" {
		lines = append(lines, "", detailRow("Link", item.URL))
	}
	return renderBoxAt(box.width, box.height, item.Title, theme.BoxDetail, trimContent(lines, box.innerHeight()))
}

// yesNo formats a boolean for the interface.
func yesNo(value bool) string {
	if value {
		return "ja"
	}
	return "nein"
}

// formatUnix formats a Unix timestamp for the interface.
func formatUnix(value int64) string {
	if value <= 0 {
		return "-"
	}
	return time.Unix(value, 0).Local().Format("02.01. 15:04")
}

// participantPopup asks which guest should be opened.
func (m *Model) participantPopup(width int) []string {
	episode, ok := m.currentEpisode()
	if !ok || len(episode.Participants) == 0 {
		return windowBoxFootColor(36, "Gast", "", theme.BoxList, []string{theme.Muted.Render("Keine Gäste bekannt.")})
	}
	content := []string{theme.Muted.Render("Welcher Gast?"), ""}
	for index, participant := range episode.Participants {
		content = append(content, renderWindowRow(theme.Row.Render(participant.Name), index == m.participantCursor, 32))
	}
	return windowBoxFootColor(36, "Gast", "", theme.BoxList, content)
}

// chatPanel renders the live chat with its input line.
func (m *Model) chatPanel(box rect) []string {
	width := box.innerWidth()
	// Die erste Spalte trägt den Balken der gewählten Nachricht.
	textWidth := max(10, width-1)
	content := []string{}
	lines, owners := m.chatBody(textWidth)
	// The foot takes only the lines it really shows: the status when the
	// chat is not connected, the spinner while it connects, otherwise the
	// field to write with its rule and, when it answers, the quote.
	reserved := 0
	if !m.chatConnected && m.chatStatus != "" {
		reserved++
	}
	switch {
	case m.chatConnected:
		reserved += m.composerRows()
	case m.chatLoading:
		reserved++
	}
	height := box.innerHeight() - reserved
	if height < 1 {
		height = 1
	}
	// Der Versatz zählt Zeilen vom Ende. Wer zurückgeblättert hat, behält
	// seinen Ausschnitt: neue Nachrichten schieben ihn nicht weiter.
	if added := m.chatAppended - m.chatAppendedSeen; added > 0 && m.chatScroll > 0 {
		m.chatScroll += m.chatLinesOfLast(min(added, len(m.chatMessages)), textWidth)
	}
	m.chatAppendedSeen = m.chatAppended
	// Ganz oben bleibt das Fenster voll: der älteste Ausschnitt ist der,
	// der mit der ersten Zeile beginnt.
	m.chatMaxScroll = max(0, len(lines)-height)
	m.chatPage = height
	selected := m.chatIndex(m.chatSelectedID)
	if selected < 0 {
		m.chatSelectedID = ""
	}
	// Nach einem Schritt mit den Tasten bleibt die gewählte Nachricht im
	// Fenster.
	if m.chatFollow && selected >= 0 {
		first, last := -1, -1
		for index, owner := range owners {
			if owner == selected {
				if first < 0 {
					first = index
				}
				last = index
			}
		}
		if first >= 0 {
			m.chatScroll = min(m.chatScroll, len(lines)-1-last)
			m.chatScroll = max(m.chatScroll, len(lines)-first-height)
		}
	}
	m.chatFollow = false
	m.chatScroll = max(0, min(m.chatScroll, m.chatMaxScroll))
	end := len(lines) - m.chatScroll
	start := max(0, end-height)
	for index := start; index < end; index++ {
		owner := owners[index]
		line := lines[index]
		// Die gewählte Nachricht trägt den Balken am Rand; ihre Blase hat
		// die kräftigere Farbe.
		gutter := " "
		if owner == selected && selected >= 0 {
			gutter = theme.Accent.Render("▌")
		}
		if owner >= 0 && chatSelectable(m.chatMessages[owner]) {
			m.addRowZone(box, len(content), m.chatClick(owner), m.chatRightClick(owner))
		}
		content = append(content, gutter+line)
	}
	liveChat := m.live != nil && m.live.IsLive && (m.live.Show == nil || m.live.Show.Chat)
	replayChat := m.live != nil && m.live.Replay != nil && m.live.Replay.ClipID != ""
	if !liveChat && !replayChat {
		hint := "Chat gibt es nur bei Livesendungen."
		if m.live != nil && m.live.IsLive {
			hint = "Für diese Sendung ist kein Chat eingeschaltet."
		}
		content = append(content, theme.Muted.Render(hint))
		return renderBoxAt(box.width, box.height, "Chat", theme.BoxList, trimContent(content, box.innerHeight()))
	}
	if !liveChat {
		if m.chatStatus != "" {
			content = append(content, theme.Muted.Render(truncateColumn(m.chatStatus, width)))
		}
		return renderBoxAt(box.width, box.height, "Chat", theme.BoxList, trimContent(content, box.innerHeight()))
	}
	if !m.chatConnected && m.chatStatus != "" {
		content = append(content, theme.Warning.Render(truncateColumn(m.chatStatus, width)))
	}
	if m.chatConnected {
		// Das Feld steht unten im Kasten, auch wenn wenig geschrieben ist.
		for len(content) < height {
			content = append(content, "")
		}
		composer, emoji := m.chatComposer(width, selected >= 0)
		first := len(content)
		content = append(content, composer...)
		// Ein Klick ins Feld setzt den Cursor hinein, ein Klick auf ein
		// Emoji fügt es ein.
		for line := first + 1; line < len(content)-1; line++ {
			m.addRowZone(box, line, func() (tea.Model, tea.Cmd) {
				if m.chatFocused {
					return m, nil
				}
				m.chatSelectedID = ""
				return m.pressKey("c")
			}, nil)
		}
		if row := len(content) - 1; row < box.innerHeight() {
			y := m.bodyTop + box.y + 1 + row
			for _, spot := range emoji {
				pick := spot.run
				m.addButton(y, box.x+2+spot.x0, box.x+2+spot.x1, func() (tea.Model, tea.Cmd) {
					m.insertEmoji(pick)
					return m, nil
				})
			}
		}
	} else if m.chatLoading {
		content = append(content, theme.Muted.Render(m.spinner.View()+" verbinde …"))
	}
	return renderBoxAt(box.width, box.height, "Chat", theme.BoxList, trimContent(content, box.innerHeight()))
}

// chatClick chooses a message; a click on the chosen one opens its menu at
// the pointer.
func (m *Model) chatClick(index int) clickFunc {
	return func() (tea.Model, tea.Cmd) {
		if index >= len(m.chatMessages) {
			return m, nil
		}
		id := m.chatMessages[index].id
		if m.chatSelectedID == id {
			return m.openContextMenu(true)
		}
		m.chatSelectedID = id
		return m, nil
	}
}

// chatRightClick chooses a message and opens its menu at the pointer.
func (m *Model) chatRightClick(index int) clickFunc {
	return func() (tea.Model, tea.Cmd) {
		if index >= len(m.chatMessages) {
			return m, nil
		}
		m.chatSelectedID = m.chatMessages[index].id
		return m.openContextMenu(true)
	}
}

// chatBody draws every chat message as a bubble. The owners name the
// message of every line; the gaps between the bubbles belong to none.
func (m *Model) chatBody(width int) ([]string, []int) {
	lines := []string{}
	owners := []int{}
	for index := range m.chatMessages {
		for _, line := range m.chatMessageLines(index, width) {
			lines = append(lines, line)
			if line == "" {
				owners = append(owners, -1)
			} else {
				owners = append(owners, index)
			}
		}
	}
	return lines, owners
}

// chatLinesOfLast counts the lines of the newest messages.
func (m *Model) chatLinesOfLast(count, width int) int {
	total := 0
	for index := len(m.chatMessages) - count; index < len(m.chatMessages); index++ {
		total += len(m.chatMessageLines(index, width))
	}
	return total
}
