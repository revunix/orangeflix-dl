package tui

import (
	"context"
	"io"
	"log"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/revunix/orangeflix-dl/internal/app"
	"github.com/revunix/orangeflix-dl/internal/config"
	"github.com/revunix/orangeflix-dl/internal/orangeflix"
	"github.com/revunix/orangeflix-dl/internal/store"
)

// Options configures the interface.
type Options struct {
	// Context is the context of the program.
	Context context.Context
	// Config holds the resolved command line.
	Config *config.Options
	// Store is the library.
	Store *store.Store
	// Version is the version of the program.
	Version string
	// HasToken tells that a valid access token of a former run is there. The
	// start screen is then not needed.
	HasToken bool

	// Run starts a download.
	Run func(context.Context) error
	// Check looks for new episodes.
	Check func(context.Context) ([]app.CheckResult, error)
	// DownloadNew loads the new episodes of a format.
	DownloadNew func(context.Context, store.FormatEntry) error
	// AddFormat stores a format in the library.
	AddFormat func(context.Context, string) (store.FormatEntry, error)
	// Episodes loads the episodes of a format.
	Episodes func(context.Context, int) ([]orangeflix.Episode, error)
	// ClipInfo loads the available files of one episode.
	ClipInfo func(context.Context, string) ([]orangeflix.File, error)
	// Formats returns the catalog of the formats.
	Formats func(context.Context) ([]orangeflix.Format, error)
	// Profile returns the state of the subscription.
	Profile func(context.Context) (*app.ProfileSummary, error)
	// Search looks for clips in the whole catalog.
	Search func(context.Context, string, int) ([]orangeflix.Episode, error)
	// Watch plays the episodes with mpv. The player runs in the background,
	// so the interface stays usable.
	Watch func(context.Context, []orangeflix.Episode) error
	// StopWatch stops the running playback when the interface closes.
	StopWatch func()
	// Recents loads the recently watched clips.
	Recents func(context.Context, int) ([]orangeflix.Episode, error)
	// Bookmarks loads the bookmarked clips.
	Bookmarks func(context.Context, int) ([]orangeflix.Episode, error)
	// SetBookmark adds or removes a clip from the list.
	SetBookmark func(context.Context, string, bool) error
	// SetWatched marks a clip as watched or not watched on the account.
	SetWatched func(context.Context, orangeflix.Episode, bool) error
	// DownloadEpisode loads one episode from a clip list.
	DownloadEpisode func(context.Context, orangeflix.Episode) error
	// LatestEpisode returns the newest episode of a format without loading
	// the whole list.
	LatestEpisode func(context.Context, int) (orangeflix.Episode, error)
	// Calendar loads the upcoming calendar entries.
	Calendar func(context.Context, int) ([]orangeflix.V3CalendarEntry, error)
	// Live loads the live state.
	Live func(context.Context) (*orangeflix.V3LiveInfo, error)
	// PlayLive starts a live stream or replay.
	PlayLive func(context.Context, *orangeflix.V3LiveInfo, string) error
	// OpenChat connects to the live chat.
	OpenChat func(context.Context) error
	// SendChat writes a message to the live chat.
	SendChat func(context.Context, string) error
	// SendStudio writes a message to the studio of the show.
	SendStudio func(context.Context, string) error
	// ReplyChat writes a message that answers another one.
	ReplyChat func(context.Context, string, string) error
	// ReactChat sets or removes a reaction on a chat message.
	ReactChat func(context.Context, string, string, bool) error
	// ChatReactions reads the reactions of chat messages.
	ChatReactions func(context.Context, []string) ([]app.ChatReaction, map[string]string, error)
	// ChatAvatar loads the picture of a chat user.
	ChatAvatar func(context.Context, string) ([]byte, error)
	// CloseChat disconnects the live chat.
	CloseChat func()
	// Similar loads clips related to an episode.
	Similar func(context.Context, string, int) ([]orangeflix.Episode, error)
	// Host loads the page of a guest.
	Host func(context.Context, string) (*orangeflix.V3HostResponse, error)
	// Announcements loads the active announcements.
	Announcements func(context.Context, string, int) ([]orangeflix.V3Announcement, error)
	// SetQuality takes over the quality for the next download.
	SetQuality func(string)
	// SetDownloadsDir takes over the folder for the downloads.
	SetDownloadsDir func(string)
	// SetSelection hands the marked episodes to the application.
	SetSelection func([]string)
	// SetCredentials hands the credentials to the application.
	SetCredentials func(user, password string)
	// SetTotp hands the code of the second factor for the next login to the
	// application.
	SetTotp func(code string)
}

// UI is the interactive front end.
type UI struct {
	model   *Model
	program *tea.Program
}

// NewUI builds the interactive front end.
func NewUI(opts Options) *UI {
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	// Die Avatare lassen sich abschalten, etwa bei langsamer Verbindung.
	if !switchedOn(os.Getenv("ORANGEFLIX_AVATARS")) {
		opts.ChatAvatar = nil
	}
	model := NewModel(ctx, opts)
	// Solange die Oberfläche den Bildschirm hat, darf keine Bibliothek über
	// das Standard-log hineinschreiben: jede Zeile zerreißt das Bild.
	log.SetOutput(io.Discard)
	programOptions := []tea.ProgramOption{tea.WithAltScreen()}
	// Die Maus lässt sich abschalten: solange das Programm sie liest,
	// markiert das Terminal Text nur mit gedrückter Umschalttaste.
	if switchedOn(os.Getenv("ORANGEFLIX_MOUSE")) {
		// Alle Bewegungen, auch ohne gedrückte Taste: sonst gäbe es kein
		// Hervorheben unter dem Zeiger.
		programOptions = append(programOptions, tea.WithMouseAllMotion())
	}
	program := tea.NewProgram(model, programOptions...)
	model.program = program
	return &UI{model: model, program: program}
}

// switchedOn reads a switch like ORANGEFLIX_MOUSE or ORANGEFLIX_AVATARS:
// "0", "off", "no", "false", "aus" and "nein" turn it off, everything else
// keeps it on.
func switchedOn(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "0", "off", "no", "false", "aus", "nein":
		return false
	}
	return true
}

// Program returns the tea program.
func (u *UI) Program() *tea.Program { return u.program }

// Event reports a state change of the application.
func (u *UI) Event(event app.Event) {
	if u.program == nil {
		return
	}
	u.program.Send(EventMsg{Event: event})
}

// SelectEpisodes returns the episodes of a format. The interface marks the
// episodes itself, so every episode is selected here.
func (u *UI) SelectEpisodes(ctx context.Context, format orangeflix.Format, episodes []orangeflix.Episode) ([]string, error) {
	identifiers := make([]string, 0, len(episodes))
	for _, episode := range episodes {
		identifiers = append(identifiers, episode.Identifier)
	}
	return identifiers, nil
}
