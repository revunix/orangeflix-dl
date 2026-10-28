// Package tui implements the interactive Bubble Tea front end.
package tui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/revunix/orangeflix-dl/internal/app"
	"github.com/revunix/orangeflix-dl/internal/config"
	"github.com/revunix/orangeflix-dl/internal/orangeflix"
	"github.com/revunix/orangeflix-dl/internal/store"
)

// Messages of the program.
type (
	// EventMsg forwards one event of the application.
	EventMsg struct{ Event app.Event }
	// RunDoneMsg reports the end of a download run.
	RunDoneMsg struct{ Err error }
	// EpisodesMsg carries the episodes of a format.
	EpisodesMsg struct {
		Format   orangeflix.Format
		Episodes []orangeflix.Episode
		Err      error
	}
	// CheckDoneMsg carries the result of a library check.
	CheckDoneMsg struct {
		Results []app.CheckResult
		Err     error
	}
	// LibraryMsg carries the stored formats.
	LibraryMsg struct{ Entries []store.FormatEntry }
	// KnownMsg carries the episodes of the library of one format.
	KnownMsg struct {
		PID      int
		Episodes []store.KnownEpisode
	}
	// ClipMsg carries the files of one episode.
	ClipMsg struct {
		Identifier string
		Files      []orangeflix.File
		Err        error
	}
	// FormatsMsg carries the catalog of the formats.
	FormatsMsg struct {
		Formats []orangeflix.Format
		Err     error
	}
	// ProfileMsg carries the state of the subscription.
	ProfileMsg struct {
		Profile *app.ProfileSummary
		Err     error
	}
	// SearchMsg carries the hits of the search.
	SearchMsg struct {
		Query    string
		Episodes []orangeflix.Episode
		Err      error
	}
	// splashDoneMsg moves on from the start screen.
	splashDoneMsg struct{}
	// splashTickMsg is one frame of the intro animation.
	splashTickMsg struct{}
)

// job is one running or finished download.
type job struct {
	Label   string
	Path    string
	Done    int64
	Total   int64
	State   string
	Episode string
	PID     int
	// startedAt is the time the download began. It is used for the
	// estimation of the remaining time.
	startedAt time.Time
}

// remaining returns the estimated time until the download is done.
func (j job) remaining() string {
	if j.Total <= 0 || j.Done <= 0 || j.startedAt.IsZero() {
		return ""
	}
	elapsed := time.Since(j.startedAt).Seconds()
	if elapsed <= 0 {
		return ""
	}
	speed := float64(j.Done) / elapsed
	if speed <= 0 {
		return ""
	}
	rest := float64(j.Total-j.Done) / speed
	return "noch " + humanDuration(int64(rest))
}

// episodeCounts holds the number of the known and the loaded episodes of one
// format.
type episodeCounts struct {
	total  int
	loaded int
}

// summary counts the result of a run.
type summary struct {
	done    int
	skipped int
	failed  int
}

// Model is the state of the interface.
type Model struct {
	ctx     context.Context
	config  *config.Options
	store   *store.Store
	version string

	// screen is the visible view.
	screen Screen
	// theme is the color scheme.
	theme Theme
	// width and height are the size of the terminal.
	width  int
	height int

	// showHelp draws the help window over the page.
	showHelp bool
	// helpScroll is the first line of the help that is shown.
	helpScroll int
	// showQuality draws the quality window over the page.
	showQuality bool
	// showAction draws the menu over the page, das fragt, was mit der
	// Folge geschehen soll.
	showAction bool
	// actionCursor is the position in the menu.
	actionCursor int
	// actionEpisodeCursor is the episode the menu belongs to.
	actionEpisodeCursor int
	// showPath draws the storage location window over the page.
	showPath bool
	// pathError is the message of the storage location window.
	pathError string
	// showLogBox shows the box with the protocol and the account.
	showLogBox bool
	// formatWidth is the width of the columns of the format list.
	formatWidth int
	// episodeWidth is the width of the columns of the episode list.
	episodeWidth int
	// logOffset scrolls back into the protocol.
	logOffset int
	// formatScroll, episodeScroll, searchScroll carry the visible windows of
	// the list panels between the renders.
	formatScroll  int
	episodeScroll int
	searchScroll  int
	// qualityCursor is the position in the list of the qualities.
	qualityCursor int

	// program is the running tea program.
	program *tea.Program

	// start form.
	field       int
	userInput   textinput.Model
	passInput   textinput.Model
	totpInput   textinput.Model
	credentials bool

	// format list.
	formats      []orangeflix.Format
	formatCursor int
	filter       string
	filtering    bool
	filterInput  textinput.Model
	library      []store.FormatEntry

	// episode list.
	currentFormat *orangeflix.Format
	episodes      []orangeflix.Episode
	episodeCursor int
	episodeYear   string
	yearFiltering bool
	yearInput     textinput.Model
	pathInput     textinput.Model
	// pathDirs lists the folders of the folder in the field. The first
	// entry is ".." unless the folder is the root.
	pathDirs   []string
	pathCursor int
	// pathOffset scrolls the list of the folders.
	pathOffset int
	// pathListNote explains an empty list.
	pathListNote string
	selected     map[string]bool
	loading      bool
	loadingLabel string

	// known holds the episodes of the library of the opened format.
	known map[string]store.KnownEpisode
	// knownCounts holds the number of the known and the loaded episodes per
	// format.
	knownCounts map[int]episodeCounts
	// clips holds the available files of the loaded episodes.
	clips map[string][]orangeflix.File
	// clipLoading is true while the files of an episode are loaded.
	clipLoading bool
	// libraryOnly limits the format list to the formats of the library.
	libraryOnly bool
	// profile is the state of the subscription.
	profile *app.ProfileSummary
	// profileLoading tells that the state of the subscription is fetched.
	profileLoading bool
	// profileErr is the error of the last request of the subscription.
	profileErr error
	// searchQuery is the text of the search.
	searchQuery string
	// searchResults are the hits of the search.
	searchResults []orangeflix.Episode
	// searchCursor is the position in the search results.
	searchCursor int
	// searchInput is the field of the search.
	searchInput textinput.Model

	// splash carries the animation of the intro screen.
	splash *splashSim

	// queue.
	jobs    []job
	log     []string
	showLog bool
	spinner spinner.Model
	summary summary
	message string
	isError bool
	// quitting ends the program.
	quitting bool
	exitCode int

	// callbacks.
	run                func(context.Context) error
	check              func(context.Context) ([]app.CheckResult, error)
	downloadNew        func(context.Context, store.FormatEntry) error
	addFormat          func(context.Context, string) (store.FormatEntry, error)
	loadEpisodes       func(context.Context, int) ([]orangeflix.Episode, error)
	clipInfo           func(context.Context, string) ([]orangeflix.File, error)
	loadFormatsCatalog func(context.Context) ([]orangeflix.Format, error)
	loadProfileState   func(context.Context) (*app.ProfileSummary, error)
	runSearchCatalog   func(context.Context, string, int) ([]orangeflix.Episode, error)
	watchEpisodesFunc  func(context.Context, []orangeflix.Episode) (*app.Player, error)
	setSelection       func([]string)
	setCredentials     func(user, password string)
	setTotp            func(code string)
	setQuality         func(string)
	setDownloadsDir    func(string)
}

// NewModel builds the model of the interface.
func NewModel(ctx context.Context, opts Options) *Model {
	scheme := ThemeAuto
	applyTheme(scheme)
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = theme.Accent

	user := textinput.New()
	user.Placeholder = "name@example.com"
	user.CharLimit = 120
	user.Prompt = loginPrompt("E-Mail")
	user.Width = 40

	pass := textinput.New()
	pass.Placeholder = "Passwort"
	pass.EchoMode = textinput.EchoPassword
	pass.CharLimit = 120
	pass.Prompt = loginPrompt("Passwort")
	pass.Width = 40

	totp := textinput.New()
	totp.Placeholder = "nur mit 2FA"
	totp.CharLimit = 8
	totp.Prompt = loginPrompt("2FA-Code")
	totp.Width = 40

	filter := textinput.New()
	filter.Placeholder = "suchen"
	filter.CharLimit = 60
	filter.Width = 30

	search := textinput.New()
	search.Placeholder = "Suchbegriff"
	search.CharLimit = 80
	search.Prompt = "Suche "
	search.Width = 50

	year := textinput.New()
	year.Placeholder = "Jahr"
	year.CharLimit = 4
	year.Prompt = "Jahr "
	year.Width = 10

	path := textinput.New()
	path.Placeholder = "~/Videos/Orangeflix"
	path.CharLimit = 512
	path.Prompt = "Ordner "
	path.Width = 48

	model := &Model{
		ctx:                ctx,
		config:             opts.Config,
		store:              opts.Store,
		version:            opts.Version,
		theme:              scheme,
		selected:           map[string]bool{},
		showLogBox:         true,
		known:              map[string]store.KnownEpisode{},
		knownCounts:        map[int]episodeCounts{},
		clips:              map[string][]orangeflix.File{},
		spinner:            sp,
		userInput:          user,
		passInput:          pass,
		totpInput:          totp,
		filterInput:        filter,
		searchInput:        search,
		yearInput:          year,
		pathInput:          path,
		run:                opts.Run,
		check:              opts.Check,
		downloadNew:        opts.DownloadNew,
		addFormat:          opts.AddFormat,
		loadEpisodes:       opts.Episodes,
		clipInfo:           opts.ClipInfo,
		loadFormatsCatalog: opts.Formats,
		loadProfileState:   opts.Profile,
		watchEpisodesFunc:  opts.Watch,
		runSearchCatalog:   opts.Search,
		setSelection:       opts.SetSelection,
		setQuality:         opts.SetQuality,
		setDownloadsDir:    opts.SetDownloadsDir,
		setCredentials:     opts.SetCredentials,
		setTotp:            opts.SetTotp,
	}
	if opts.Config != nil {
		model.userInput.SetValue(opts.Config.Username)
		model.passInput.SetValue(opts.Config.Password)
		// Ein gespeichertes Token oder gespeicherte Zugangsdaten lassen den
		// Startbildschirm entfallen.
		model.credentials = opts.HasToken ||
			(opts.Config.Username != "" && opts.Config.Password != "")
	}
	if opts.Store != nil {
		if value, err := opts.Store.Setting(store.SettingTheme); err == nil {
			switch value {
			case string(ThemeLight):
				scheme = ThemeLight
			case string(ThemeAuto):
				scheme = ThemeAuto
			}
			applyTheme(scheme)
			sp.Style = theme.Accent
			model.theme = scheme
		}
	}
	model.formats = orangeflix.Catalog
	return model
}

// Init starts the program. Die Daten werden sofort geladen, damit sie
// fertig da sind, wenn der Startbildschirm vorbei ist.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.splashTimer(), m.loadLibrary(), m.loadFormats(), m.loadProfile())
}

// splashDuration is how long the intro screen runs before it moves on.
const splashDuration = 4200 * time.Millisecond

// splashTimer moves on from the start screen after a few seconds; the ticks
// of the animation run beside it until then.
func (m *Model) splashTimer() tea.Cmd {
	return tea.Batch(
		tea.Tick(splashDuration, func(time.Time) tea.Msg { return splashDoneMsg{} }),
		m.splashTick(),
	)
}

// splashTick schedules the next frame of the intro animation.
func (m *Model) splashTick() tea.Cmd {
	return tea.Tick(splashFrameTime, func(time.Time) tea.Msg { return splashTickMsg{} })
}

// Update handles the messages of the program.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch message := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = message.Width
		m.height = message.Height
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(message)
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case splashTickMsg:
		// Die Szene läuft nur, solange der Startbildschirm steht.
		if m.screen != ScreenSplash {
			return m, nil
		}
		return m, m.splashTick()
	case splashDoneMsg:
		if m.screen == ScreenSplash {
			if m.credentials {
				// Die Daten sind schon geladen, sie werden nur noch
				// einmal aufgefrischt, falls der Start länger gedauert hat.
				m.screen = ScreenFormats
				return m, tea.Batch(m.loadLibrary(), m.loadFormats(), m.loadProfile())
			}
			m.screen = ScreenStart
		}
		return m, nil
	case EventMsg:
		m.applyEvent(message.Event)
		return m, nil
	case RunDoneMsg:
		m.screen = ScreenSummary
		if message.Err != nil {
			m.message = message.Err.Error()
			m.isError = true
			m.exitCode = 1
		}
		return m, nil
	case EpisodesMsg:
		m.loading = false
		if message.Err != nil {
			m.message = message.Err.Error()
			m.isError = true
			return m, nil
		}
		m.currentFormat = &message.Format
		m.episodes = message.Episodes
		m.episodeCursor = 0
		m.selected = map[string]bool{}
		m.screen = ScreenEpisodes
		// Der Katalog wird neu geladen, damit die Liste den Besuch und
		// neue Folgen zeigt.
		return m, m.loadFormats()
	case CheckDoneMsg:
		if message.Err != nil {
			m.message = message.Err.Error()
			m.isError = true
			return m, nil
		}
		total := 0
		for _, result := range message.Results {
			total += len(result.New)
		}
		m.message = fmt.Sprintf("%d neue Folge(n) gefunden.", total)
		m.isError = false
		m.loadLibrary()
		return m, nil
	case LibraryMsg:
		m.library = message.Entries
		return m, nil
	case KnownMsg:
		m.setKnown(message.PID, message.Episodes)
		return m, nil
	case FormatsMsg:
		m.loading = false
		if message.Err != nil {
			m.message = message.Err.Error()
			m.isError = true
			return m, nil
		}
		if len(message.Formats) > 0 {
			// Der Cursor bleibt auf dem Format, das markiert war: die Liste
			// ist nach dem Datum sortiert und würde sonst verrutschen. Der
			// Platz wird in der sichtbaren Reihenfolge gesucht, nicht im
			// Katalog.
			current := 0
			if formats := m.visibleFormats(); m.formatCursor < len(formats) {
				current = formats[m.formatCursor].PID
			}
			m.formats = message.Formats
			m.formatCursor = indexOfFormat(m.visibleFormats(), current)
		}
		return m, nil
	case ProfileMsg:
		m.profileLoading = false
		m.profileErr = message.Err
		if message.Err == nil {
			m.profile = message.Profile
			return m, nil
		}
		// Das Konto verlangt den zweiten Faktor: das Anmeldefenster kommt
		// zurück, der Cursor steht im Feld für den Code.
		switch {
		case errors.Is(message.Err, app.ErrTotpRequired):
			m.message = "Das Konto verlangt einen 2FA-Code. Bitte den aktuellen Code eintragen."
		case errors.Is(message.Err, app.ErrTotpRejected):
			m.message = "Der 2FA-Code wurde nicht angenommen. Bitte den aktuellen Code eintragen."
		default:
			return m, nil
		}
		m.isError = true
		m.screen = ScreenStart
		m.field = 2
		return m, m.focusField()
	case SearchMsg:
		m.loading = false
		if message.Err != nil {
			m.message = message.Err.Error()
			m.isError = true
			return m, nil
		}
		m.searchResults = message.Episodes
		m.searchCursor = 0
		m.searchScroll = 0
		m.message = fmt.Sprintf("%d Treffer für %q", len(message.Episodes), message.Query)
		m.isError = false
		return m, nil
	case ClipMsg:
		m.clipLoading = false
		if message.Err != nil {
			m.message = message.Err.Error()
			m.isError = true
			return m, nil
		}
		m.clips[message.Identifier] = message.Files
		return m, nil
	case WatchMsg:
		m.loading = false
		if message.Err != nil {
			m.message = message.Err.Error()
			m.isError = true
			return m, nil
		}
		m.message = ""
		m.isError = false
		// Der Player übernimmt das Terminal, danach kommt die Oberfläche
		// zurück.
		return m, m.startPlayer(message.Player)
	case playerDoneMsg:
		if message.err != nil {
			m.message = "Der Player ist beendet: " + message.err.Error()
			m.isError = true
		}
		return m, nil
	}
	return m, nil
}

// applyEvent applies one event of the application.
func (m *Model) applyEvent(event app.Event) {
	switch event.Kind {
	case app.EventLog:
		m.log = append(m.log, logTime(time.Now())+" "+event.Message)
		if len(m.log) > 500 {
			m.log = m.log[len(m.log)-500:]
		}
	case app.EventQueued:
		m.jobs = append(m.jobs, job{
			Label:     event.Label,
			Path:      event.Path,
			State:     "wartet",
			Episode:   event.EpisodeID,
			PID:       event.PID,
			startedAt: time.Now(),
		})
	case app.EventProgress:
		for i := range m.jobs {
			if m.jobs[i].Episode == event.EpisodeID {
				m.jobs[i].Done = event.Done
				m.jobs[i].Total = event.Total
				m.jobs[i].State = "lädt"
			}
		}
	case app.EventSkipped:
		m.summary.skipped++
		for i := range m.jobs {
			if m.jobs[i].Episode == event.EpisodeID {
				m.jobs[i].State = "übersprungen"
			}
		}
	case app.EventDone:
		m.summary.done++
		for i := range m.jobs {
			if m.jobs[i].Episode == event.EpisodeID {
				m.jobs[i].State = "fertig"
				m.jobs[i].Done = m.jobs[i].Total
			}
		}
	case app.EventFailed:
		m.summary.failed++
		m.message = event.Message
		m.isError = true
	}
}

// SetSize sets the size of the terminal. It is used by the tests and by the
// probe program.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// ExitCode returns the exit code of the program.
func (m *Model) ExitCode() int { return m.exitCode }

// Close releases the resources of the model.
func (m *Model) Close() {}

// setTheme switches the color scheme and keeps the choice.
func (m *Model) setTheme(name Theme) {
	if name == m.theme {
		return
	}
	m.theme = name
	applyTheme(name)
	m.spinner.Style = theme.Accent
	if m.store != nil {
		_ = m.store.SetSetting(store.SettingTheme, string(name))
	}
}

// filteredFormats returns the formats that match the filter, grouped by their
// status.
// filteredFormats returns the formats that match the filter, in the order
// of the list: grouped by status, the newest format first in its group.
func (m *Model) filteredFormats() []orangeflix.Format {
	var result []orangeflix.Format
	for _, group := range m.formatGroups() {
		result = append(result, group.formats...)
	}
	return result
}

// filteredEpisodes returns the episodes that match the year filter. The
// newest episode comes first, so the new ones are at the top of the list.
// Die Reihenfolge folgt dem Datum, nicht der Nummer: die Nummern sind bei
// manchen Formaten (Saisons, Sonderfolgen) nicht aufsteigend, der Feed aber
// auch nicht von heute nach morgen geordnet - die Sortierung hält die Jahre
// zusammen und hält sie in einer Ordnung mit der Gruppenliste.
func (m *Model) filteredEpisodes() []orangeflix.Episode {
	year := m.episodeYear
	result := make([]orangeflix.Episode, 0, len(m.episodes))
	for i := len(m.episodes) - 1; i >= 0; i-- {
		episode := m.episodes[i]
		if year != "" {
			if episode.Date.IsZero() || episode.Date.Local().Format("2006") != year {
				continue
			}
		}
		result = append(result, episode)
	}
	sortEpisodesByDate(result)
	return result
}

// sortEpisodesByDate orders the episodes from the newest to the oldest.
// Episodes without a date land at the end; the number breaks a tie and keeps
// a stable order.
func sortEpisodesByDate(episodes []orangeflix.Episode) {
	slices.SortStableFunc(episodes, func(a, b orangeflix.Episode) int {
		aZero, bZero := a.Date.IsZero(), b.Date.IsZero()
		switch {
		case aZero && bZero:
		case aZero:
			return 1
		case bZero:
			return -1
		default:
			if !a.Date.Equal(b.Date) {
				if a.Date.After(b.Date) {
					return -1
				}
				return 1
			}
		}
		switch {
		case a.Number > b.Number:
			return -1
		case a.Number < b.Number:
			return 1
		}
		return 0
	})
}

// indexOfFormat returns the place of a format in a list, or 0 when the
// list does not know it.
func indexOfFormat(formats []orangeflix.Format, pid int) int {
	for index, format := range formats {
		if format.PID == pid {
			return index
		}
	}
	return 0
}

// libraryEntry returns the library entry of a format.
func (m *Model) libraryEntry(pid int) (store.FormatEntry, bool) {
	for _, entry := range m.library {
		if entry.PID == pid {
			return entry, true
		}
	}
	return store.FormatEntry{}, false
}

// tailLog returns the last lines of the protocol.
func (m *Model) tailLog(count int) []string {
	if len(m.log) <= count {
		return m.log
	}
	return m.log[len(m.log)-count:]
}

// setKnown stores the episodes of the library of one format.
func (m *Model) setKnown(pid int, episodes []store.KnownEpisode) {
	m.known = map[string]store.KnownEpisode{}
	counts := episodeCounts{}
	for _, episode := range episodes {
		m.known[episode.Identifier] = episode
		counts.total++
		if episode.DownloadedAt != nil {
			counts.loaded++
		}
	}
	m.knownCounts[pid] = counts
}

// visibleFormats returns the formats of the list. With the library filter
// only the remembered formats are shown.
func (m *Model) visibleFormats() []orangeflix.Format {
	formats := m.filteredFormats()
	if !m.libraryOnly {
		return formats
	}
	kept := make([]orangeflix.Format, 0, len(formats))
	for _, format := range formats {
		if _, ok := m.libraryEntry(format.PID); ok {
			kept = append(kept, format)
		}
	}
	return kept
}

// loadKnown reads the episodes of the library of one format.
func (m *Model) loadKnown(pid int) tea.Cmd {
	if m.store == nil {
		return nil
	}
	return func() tea.Msg {
		episodes, err := m.store.Episodes(pid)
		if err != nil {
			return KnownMsg{PID: pid}
		}
		return KnownMsg{PID: pid, Episodes: episodes}
	}
}

// loadClipInfo loads the available files of the marked episode.
func (m *Model) loadClipInfo() tea.Cmd {
	if m.clipInfo == nil {
		return nil
	}
	identifier := episodeIdentifier(m)
	if identifier == "" {
		return nil
	}
	m.clipLoading = true
	return func() tea.Msg {
		files, err := m.clipInfo(m.ctx, identifier)
		return ClipMsg{Identifier: identifier, Files: files, Err: err}
	}
}
