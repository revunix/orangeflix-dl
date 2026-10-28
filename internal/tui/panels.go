package tui

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
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
	for index := start; index < end; index++ {
		row := rows[index]
		if row.format == nil {
			content = append(content, groupHeader(row.header, fmt.Sprintf("%d", row.count)))
			continue
		}
		content = append(content, renderRowWidth(m.formatLine(*row.format),
			row.format.PID == formats[m.formatCursor].PID, box.innerWidth()))
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
	for index := start; index < end; index++ {
		row := rows[index]
		if row.episode == nil {
			content = append(content, groupHeader(row.header,
				fmt.Sprintf("%s · %d geladen", plural(int64(row.count), "Folge", "Folgen"), row.loaded)))
			continue
		}
		content = append(content, renderRowWidth(m.episodeLine(*row.episode),
			row.episode.Identifier == episodes[m.episodeCursor].Identifier, box.innerWidth()))
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
	cols := episodeColumns{num: 5, date: 10, length: 8, state: 10}
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
	episodes := m.filteredEpisodes()
	if len(episodes) == 0 {
		return renderBoxColor(box.width, "Angaben", theme.BoxDetail, nil)
	}
	episode := episodes[m.episodeCursor]

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
		content = append(content, renderRowWidth(m.searchLine(m.searchResults[index]), index == m.searchCursor, box.innerWidth()))
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

// truncateColumn shortens a text to the width of its column.
func truncateColumn(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	return string(runes[:max(1, width-1)]) + "…"
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

// episodeState returns the state of an episode: geladen, neu or offen.
func (m *Model) episodeState(identifier string) string {
	switch {
	case m.episodeLoaded(identifier):
		return statusBadge("geladen")
	case m.episodeNew(identifier):
		return statusBadge("neu")
	default:
		return statusBadge("offen")
	}
}

// episodeIdentifier returns the identifier of the marked episode.
func episodeIdentifier(m *Model) string {
	episodes := m.filteredEpisodes()
	if len(episodes) == 0 {
		return ""
	}
	return episodes[m.episodeCursor].Identifier
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
