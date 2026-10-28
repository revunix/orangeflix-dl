package tui

import (
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Theme names the color scheme.
type Theme string

// The available themes.
const (
	ThemeDark  Theme = "dark"
	ThemeLight Theme = "light"
	// ThemeAuto follows the background of the terminal.
	ThemeAuto Theme = "auto"
)

// The colors of the interface. They follow the palette of orangeflix.de:
// a warm orange brand color on a near black surface, with a light scheme for
// bright terminals.
var (
	// colorBackground paints the surface of the interface.
	colorBackground = lipgloss.AdaptiveColor{Light: "#f8fafc", Dark: "#09090b"}
	// colorSurface is the color of the cards.
	colorSurface = lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#18181b"}
	// colorText is the color of the normal text.
	colorText = lipgloss.AdaptiveColor{Light: "#18181b", Dark: "#fafafa"}
	// The colors of the boxes. The list is the box the keys work in, its
	// frame carries the brand color; the other boxes stand back in a quiet
	// grey, so the eye finds the list at once.
	colorBoxList   = lipgloss.AdaptiveColor{Light: "#c27400", Dark: "#f19100"}
	colorBoxDetail = lipgloss.AdaptiveColor{Light: "#d4d4d8", Dark: "#3f3f46"}
	colorBoxLog    = lipgloss.AdaptiveColor{Light: "#d4d4d8", Dark: "#3f3f46"}
	colorBoxInfo   = lipgloss.AdaptiveColor{Light: "#d4d4d8", Dark: "#3f3f46"}
	// colorSelected is the background of the marked line.
	colorSelected = lipgloss.AdaptiveColor{Light: "#fff1dc", Dark: "#2a1f10"}
	// colorSelectedFg is the color of the text on the marked line.
	colorSelectedFg = lipgloss.AdaptiveColor{Light: "#18181b", Dark: "#fafafa"}
	// colorDivLine is the color of the separators. Terminals know no
	// opacity, so the greys are solid colors.
	colorDivLine = lipgloss.AdaptiveColor{Light: "#e4e4e7", Dark: "#27272a"}
	// colorScrim is the color of the page behind an open window: the text
	// stays readable, but steps back.
	colorScrim = lipgloss.AdaptiveColor{Light: "#a1a1aa", Dark: "#52525b"}
	// colorWindowSelected is the background of the marked line in a window;
	// the brand color shines a little through the raised surface.
	colorWindowSelected = lipgloss.AdaptiveColor{Light: "#ffedd5", Dark: "#3f311f"}
	// colorSurfaceRaised is the color of the window over the page.
	colorSurfaceRaised = lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#202024"}
	// colorMuted is the color of the secondary text.
	colorMuted = lipgloss.AdaptiveColor{Light: "#52525b", Dark: "#a1a1aa"}
	// colorSubtle is the color of the hints.
	colorSubtle = lipgloss.AdaptiveColor{Light: "#71717a", Dark: "#71717a"}
	// colorAccent is the brand color.
	colorAccent = lipgloss.AdaptiveColor{Light: "#9a5b00", Dark: "#f5a019"}
	// colorAccentSoft is the light brand color.
	colorAccentSoft = lipgloss.AdaptiveColor{Light: "#8a5200", Dark: "#fed7aa"}
	// colorBorder is the color of the separators.
	colorBorder = lipgloss.AdaptiveColor{Light: "#e4e4e7", Dark: "#27272a"}
	// colorBorderStrong is the color of the stronger separators.
	colorBorderStrong = lipgloss.AdaptiveColor{Light: "#d4d4d8", Dark: "#3f3f46"}
	// colorControl is the color of the keycaps.
	colorControl = lipgloss.AdaptiveColor{Light: "#e4e4e7", Dark: "#27272a"}
	// colorControlText is the text on the keycaps.
	colorControlText = lipgloss.AdaptiveColor{Light: "#52525b", Dark: "#c8cdd6"}
	// colorSuccess is the color of the finished state.
	colorSuccess = lipgloss.AdaptiveColor{Light: "#166534", Dark: "#bbf7d0"}
	// colorWarning is the color of the new state.
	colorWarning = lipgloss.AdaptiveColor{Light: "#9a5b00", Dark: "#fdba74"}
	// colorDanger is the color of the failed state.
	colorDanger = lipgloss.AdaptiveColor{Light: "#b91c1c", Dark: "#fecaca"}
)

// The gradient of the program name. It uses the brand gradient of the site.
const (
	colorGradientStartLight = "#c27400"
	colorGradientMidLight   = "#a86200"
	colorGradientEndLight   = "#7a4800"
	colorGradientStartDark  = "#fed7aa"
	colorGradientMidDark    = "#f19100"
	colorGradientEndDark    = "#d67f00"
)

// themeStyles holds the styles of the active color scheme.
type themeStyles struct {
	// Background paints the surface of the interface.
	Background lipgloss.Style
	// Surface is the color of the cards.
	Surface lipgloss.Style
	// SurfaceRaised is the color of the window that floats over the page.
	SurfaceRaised lipgloss.Style
	// Window is the background of the window that floats over the page.
	Window lipgloss.Style
	// WindowEdge draws the upper and lower edge of a window in its color.
	WindowEdge lipgloss.Style
	// WindowSelected is the background of the marked line in a window.
	WindowSelected lipgloss.Style
	// Scrim is the color of the page behind an open window.
	Scrim lipgloss.Style
	// Panel is the background of the boxes on the page.
	Panel lipgloss.Style
	// Bar is the background of the message bar in the footer.
	Bar lipgloss.Style
	// BoxList is the frame of the list.
	BoxList lipgloss.Style
	// BoxDetail is the frame of the details.
	BoxDetail lipgloss.Style
	// BoxLog is the frame of the log.
	BoxLog lipgloss.Style
	// BoxInfo is the frame of the account.
	BoxInfo lipgloss.Style
	// Selected is the background of the marked line.
	Selected lipgloss.Style
	// SelectedFg is the color of the text on the marked line.
	SelectedFg lipgloss.Style
	// Divider is the color of the separators.
	Divider lipgloss.Style
	// Title is the color of the headings.
	Title lipgloss.Style
	// Text is the color of the normal text.
	Text lipgloss.Style
	// Muted is the color of the secondary text.
	Muted lipgloss.Style
	// Subtle is the color of the hints.
	Subtle lipgloss.Style
	// Accent is the brand color.
	Accent lipgloss.Style
	// AccentBold is the brand color for the keys of a table.
	AccentBold lipgloss.Style
	// AccentSoft is the light brand color.
	AccentSoft lipgloss.Style
	// Badge is the color of the keycaps.
	Badge lipgloss.Style
	// Chip is the color of the inactive titles in the head.
	Chip lipgloss.Style
	// KeyLabel is the color of the text behind a keycap.
	KeyLabel lipgloss.Style
	// Success is the color of the finished state.
	Success lipgloss.Style
	// Warning is the color of the new state.
	Warning lipgloss.Style
	// Danger is the color of the failed state.
	Danger lipgloss.Style
	// Border is the color of the separators.
	Border lipgloss.Style
	// BorderStrong is the color of the stronger separators.
	BorderStrong lipgloss.Style
	// Row is the color of a line of a list.
	Row lipgloss.Style
	// RowActive is the color of the marked line of a list.
	RowActive lipgloss.Style
}

// theme holds the styles of the active color scheme.
var theme = newTheme(ThemeDark)

// darkScheme remembers whether the dark scheme is in use.
var darkScheme = true

// colorOf returns the color of the active scheme.
func colorOf(color lipgloss.AdaptiveColor) lipgloss.TerminalColor {
	if darkScheme {
		return lipgloss.Color(color.Dark)
	}
	return lipgloss.Color(color.Light)
}

// newTheme builds the styles of a color scheme.
func newTheme(name Theme) themeStyles {
	return themeStyles{
		Background:    lipgloss.NewStyle().Background(colorOf(colorBackground)),
		Surface:       lipgloss.NewStyle().Foreground(colorOf(colorSurface)),
		SurfaceRaised: lipgloss.NewStyle().Foreground(colorOf(colorSurfaceRaised)),
		// Auch das Fenster bringt seine eigene Vordergrundfarbe mit.
		WindowEdge:     lipgloss.NewStyle().Foreground(colorOf(colorSurfaceRaised)),
		WindowSelected: lipgloss.NewStyle().Background(colorOf(colorWindowSelected)),
		Scrim:          lipgloss.NewStyle().Foreground(colorOf(colorScrim)),
		Window: lipgloss.NewStyle().
			Foreground(colorOf(colorText)).
			Background(colorOf(colorSurfaceRaised)),
		// Der Kasten bringt Vorder- und Hintergrund mit: so bleibt der Text
		// lesbar, auch wenn das Terminal eine andere Farbe hat. Der
		// Hintergrund ist der der Seite, die Kästen stehen also auf
		// derselben schwarzen Fläche wie der Rest.
		Panel: lipgloss.NewStyle().
			Foreground(colorOf(colorText)).
			Background(colorOf(colorBackground)),
		// Die Meldungsleiste liegt auf einer eigenen Fläche, damit sie sich
		// von der Seite abhebt.
		Bar: lipgloss.NewStyle().
			Foreground(colorOf(colorText)).
			Background(colorOf(colorSurface)),
		BoxList:    lipgloss.NewStyle().Foreground(colorOf(colorBoxList)),
		BoxDetail:  lipgloss.NewStyle().Foreground(colorOf(colorBoxDetail)),
		BoxLog:     lipgloss.NewStyle().Foreground(colorOf(colorBoxLog)),
		BoxInfo:    lipgloss.NewStyle().Foreground(colorOf(colorBoxInfo)),
		Selected:   lipgloss.NewStyle().Background(colorOf(colorSelected)),
		SelectedFg: lipgloss.NewStyle().Foreground(colorOf(colorSelectedFg)),
		Divider:    lipgloss.NewStyle().Foreground(colorOf(colorDivLine)),
		Title:      lipgloss.NewStyle().Foreground(colorOf(colorText)).Bold(true),
		Text:       lipgloss.NewStyle().Foreground(colorOf(colorText)),
		Muted:      lipgloss.NewStyle().Foreground(colorOf(colorMuted)),
		Subtle:     lipgloss.NewStyle().Foreground(colorOf(colorSubtle)),
		Accent:     lipgloss.NewStyle().Foreground(colorOf(colorAccent)),
		AccentBold: lipgloss.NewStyle().Foreground(colorOf(colorAccent)).Bold(true),
		AccentSoft: lipgloss.NewStyle().Foreground(colorOf(colorAccentSoft)),
		Badge: lipgloss.NewStyle().
			Foreground(colorOf(colorControlText)).
			Background(colorOf(colorControl)),
		Chip:         lipgloss.NewStyle().Foreground(colorOf(colorMuted)),
		KeyLabel:     lipgloss.NewStyle().Foreground(colorOf(colorMuted)),
		Success:      lipgloss.NewStyle().Foreground(colorOf(colorSuccess)),
		Warning:      lipgloss.NewStyle().Foreground(colorOf(colorWarning)),
		Danger:       lipgloss.NewStyle().Foreground(colorOf(colorDanger)),
		Border:       lipgloss.NewStyle().Foreground(colorOf(colorBorder)),
		BorderStrong: lipgloss.NewStyle().Foreground(colorOf(colorBorderStrong)),
		Row:          lipgloss.NewStyle().Foreground(colorOf(colorText)),
		RowActive:    lipgloss.NewStyle().Foreground(colorOf(colorAccent)).Bold(true),
	}
}

// isDark reports whether the dark scheme is in use.
func isDark() bool { return darkScheme }

// resolveTheme returns the scheme a name stands for. The automatic scheme
// follows the background of the terminal.
func resolveTheme(name Theme) Theme {
	if name != ThemeAuto {
		return name
	}
	if detectDarkBackground() {
		return ThemeDark
	}
	return ThemeLight
}

// detectDarkBackground guesses the background of the terminal from the
// environment. The COLORFGBG variable holds the colors of the terminal and is
// set by many of them.
//
// The terminal is deliberately not asked with an OSC query: a terminal that
// does not answer would hold the start of the program up for seconds.
func detectDarkBackground() bool {
	value := os.Getenv("COLORFGBG")
	if value == "" {
		return true
	}
	parts := strings.Split(value, ";")
	if len(parts) < 2 {
		return true
	}
	background, err := strconv.Atoi(strings.TrimSpace(parts[len(parts)-1]))
	if err != nil {
		return true
	}
	// The colors 0 to 6 are dark, 7 to 15 are light.
	return background <= 6
}

// applyTheme switches the interface to a color scheme.
func applyTheme(name Theme) {
	resolved := resolveTheme(name)
	darkScheme = resolved != ThemeLight
	theme = newTheme(resolved)
}

// backgroundSequence returns the escape sequence that starts a line with the
// surface color of the interface. The scheme always brings its own
// background, so the view looks the same in every terminal.
func backgroundSequence() string {
	return sequenceOf(theme.Background)
}

// windowBackgroundSequence returns the escape sequence of the window
// surface. A window carries its own background, so the page behind it does
// not break through after every reset.
func windowBackgroundSequence() string {
	return sequenceOf(theme.Window)
}

// sequenceOf returns the escape sequence that a style produces.
func sequenceOf(style lipgloss.Style) string {
	rendered := style.Render(" ")
	index := strings.Index(rendered, "m")
	if index <= 0 {
		return ""
	}
	return rendered[:index+1]
}
