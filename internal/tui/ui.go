package tui

import (
	"context"

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
	// Watch plays the episodes with mpv. It returns the command that plays
	// them.
	Watch func(context.Context, []orangeflix.Episode) (*app.Player, error)
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
	model := NewModel(ctx, opts)
	program := tea.NewProgram(model, tea.WithAltScreen())
	model.program = program
	return &UI{model: model, program: program}
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
