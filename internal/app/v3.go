package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/revunix/orangeflix-dl/internal/orangeflix"
)

// V3 is the client of the version 3 API. It is optional: when it is missing
// the program uses the legacy API.
type V3 struct {
	Client *orangeflix.V3Client
	// TokenStore keeps the access token between the runs.
	TokenStore TokenStore
	// TotpPrompt asks the user for the code of the second factor. It is set
	// by the interface when it is able to ask.
	TotpPrompt func(context.Context) (string, error)

	// totpCode is the code of the interface for the next login.
	totpCode string
	totpMu   sync.Mutex
}

// TokenStore keeps the access token.
type TokenStore interface {
	GetToken() (string, error)
	SetToken(token string) error
}

// EnsureV3 makes sure that the version 3 client has a usable token. The
// token of the last run is reused when it is still valid.
func (a *App) EnsureV3(ctx context.Context) error {
	if a.V3 == nil {
		return errors.New("the version 3 API is not available")
	}
	client := a.V3.Client
	if client.AccessToken == "" && a.V3.TokenStore != nil {
		if token, err := a.V3.TokenStore.GetToken(); err == nil && token != "" {
			client.AccessToken = token
		}
	}
	if err := client.EnsureToken(ctx); err != nil {
		var totp *orangeflix.TotpRequiredError
		if !errors.As(err, &totp) {
			a.logf("Der Token konnte nicht geholt werden: %s", err.Error())
			return err
		}
		code, promptErr := a.totpCode(ctx, totp.Token)
		if promptErr != nil {
			return promptErr
		}
		if code == "" {
			return fmt.Errorf("%w: einen TOTP-Code mit --totp angeben, ORANGEFLIX_TOTP setzen oder im Anmeldefenster eintragen", ErrTotpRequired)
		}
		if err := a.V3.Client.CompleteTotpLogin(ctx, totp.Token, code); err != nil {
			a.logf("Die Anmeldung mit dem zweiten Faktor ist fehlgeschlagen: %s", err.Error())
			return fmt.Errorf("%w: %w", ErrTotpRejected, err)
		}
		a.logf("Anmeldung mit dem zweiten Faktor abgeschlossen.")
	}
	if a.V3.TokenStore != nil && client.AccessToken != "" {
		if err := a.V3.TokenStore.SetToken(client.AccessToken); err != nil {
			a.logf("Der Token konnte nicht gespeichert werden: %s", err.Error())
		}
	}
	return nil
}

// Profile returns the account with the state of the subscription.
func (a *App) Profile(ctx context.Context) (*orangeflix.V3Profile, error) {
	if a.V3 == nil {
		return nil, errors.New("the version 3 API is not available")
	}
	if err := a.EnsureV3(ctx); err != nil {
		return nil, err
	}
	return a.V3.Client.Profile(ctx)
}

// NewEpisodeDays is the age in days in which an episode counts as new. The
// age is measured against the current date.
const NewEpisodeDays = 7

// Formats returns the catalog of the formats. The live catalog of the
// version 3 API is merged into the catalog of the program: the API brings
// the fresh dates and the numbers of the episodes, the program knows the
// formats that only appear from time to time.
func (a *App) Formats(ctx context.Context) ([]orangeflix.Format, error) {
	opened := a.openedFormats()
	formats := a.markStaticCatalog(opened)

	if a.V3 != nil {
		magazines, err := a.V3.Client.Magazines(ctx)
		if err == nil && len(magazines) > 0 {
			place := map[int]int{}
			for index, format := range formats {
				place[format.PID] = index
			}
			for _, magazine := range magazines {
				format := magazineFormat(magazine)
				if index, ok := place[magazine.PID]; ok {
					// Der Katalog im Programm kennt Slug, Status und
					// Beschreibung, die API bringt das Datum.
					known := formats[index]
					if known.Slug != "" {
						format.Slug = known.Slug
					}
					if known.Status != "" {
						format.Status = known.Status
					}
					format.Description = known.Description
					if format.LastEpisodeTime.IsZero() {
						format.LastEpisode = known.LastEpisode
						format.LastEpisodeTime = known.LastEpisodeTime
					}
					formats[index] = format
					continue
				}
				formats = append(formats, format)
			}
			return markFormats(formats, opened), nil
		} else if err != nil {
			a.logf("Der Live-Katalog ist nicht erreichbar (%s), nutze den Katalog im Programm.", err.Error())
		}
	}
	return markFormats(formats, opened), nil
}

// markFormats adds the state of the library and the age of the last episode
// to the formats of the list.
func markFormats(formats []orangeflix.Format, opened map[int]bool) []orangeflix.Format {
	now := time.Now()
	for index := range formats {
		formats[index].Opened = opened[formats[index].PID]
		formats[index].NewEpisode = isNewEpisode(formats[index].LastEpisodeTime, now)
	}
	return formats
}

// magazineFormat builds a format of the live catalog.
func magazineFormat(magazine orangeflix.V3Magazine) orangeflix.Format {
	// Die API kennt nur aktiv und archiviert.
	status := orangeflix.StatusArchived
	if magazine.Active {
		status = orangeflix.StatusActive
	}
	format := orangeflix.Format{
		PID:       magazine.PID,
		Slug:      formatSlug(magazine),
		Title:     magazine.Title,
		Status:    status,
		ClipCount: magazine.ClipCount,
		Logo:      magazine.Artwork.Logo,
		Original:  magazine.Original,
		Handle:    magazine.Handle,
	}
	if moment, ok := magazineLastEpisode(magazine); ok {
		format.LastEpisodeTime = moment
		format.LastEpisode = moment.Local().Format("2006-01-02")
	}
	return format
}

// markStaticCatalog adds the state of the library to the catalog that is
// built into the program.
func (a *App) markStaticCatalog(opened map[int]bool) []orangeflix.Format {
	formats := make([]orangeflix.Format, 0, len(orangeflix.Catalog))
	for _, format := range orangeflix.Catalog {
		format.Opened = opened[format.PID]
		format.NewEpisode = isNewEpisode(format.LastEpisodeTime, time.Now())
		formats = append(formats, format)
	}
	return formats
}

// openedFormats returns the formats the user has already been in.
func (a *App) openedFormats() map[int]bool {
	if a.Store == nil {
		return nil
	}
	opened, err := a.Store.OpenedFormats()
	if err != nil {
		return nil
	}
	return opened
}

// magazineLastEpisode returns the date of the most recent episode of a
// format. The live catalog carries it as a timestamp.
func magazineLastEpisode(magazine orangeflix.V3Magazine) (time.Time, bool) {
	value := magazine.TimeRange.Max
	if value <= 0 {
		return time.Time{}, false
	}
	// Die API liefert die Zeit in Sekunden, manche Werte kommen in
	// Millisekunden.
	if value > 1_000_000_000_000 {
		value /= 1000
	}
	moment := time.Unix(value, 0)
	// Nur Werte, die wie ein Datum aussehen, werden genommen.
	if moment.Year() < 2000 || moment.After(time.Now().AddDate(1, 0, 0)) {
		return time.Time{}, false
	}
	return moment, true
}

// isNewEpisode reports whether an episode is new. The age is measured
// against the current date: an episode of the last days counts as new.
func isNewEpisode(moment time.Time, now time.Time) bool {
	if moment.IsZero() {
		return false
	}
	age := now.Sub(moment)
	if age < 0 {
		// Eine Folge in der Zukunft zählt auch als neu.
		return true
	}
	return age <= time.Duration(NewEpisodeDays)*24*time.Hour
}

// formatSlug returns the name of a format for the command line. The catalog
// of the program has a speaking name for every format; a new format gets a
// name from its title.
func formatSlug(magazine orangeflix.V3Magazine) string {
	if format, ok := orangeflix.FormatByPID(magazine.PID); ok {
		return format.Slug
	}
	slug := strings.ToLower(magazine.Title)
	var b strings.Builder
	for _, r := range slug {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == 'ä' || r == 'ö' || r == 'ü':
			b.WriteRune('a')
		case r == 'ß':
			b.WriteRune('s')
		case r == ' ' || r == '-' || r == '_':
			b.WriteRune('-')
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "" {
		return fmt.Sprintf("format-%d", magazine.PID)
	}
	return name
}

// FormatByID returns one format of the live catalog.
func (a *App) FormatByID(ctx context.Context, pid int) (orangeflix.Format, error) {
	formats, err := a.Formats(ctx)
	if err != nil {
		return orangeflix.Format{}, err
	}
	for _, format := range formats {
		if format.PID == pid {
			return format, nil
		}
	}
	return orangeflix.Format{}, fmt.Errorf("unknown format: %d", pid)
}

// Episodes returns the episodes of a format. The version 3 API delivers
// fifty episodes per request, the legacy feed only ten.
func (a *App) EpisodesV3(ctx context.Context, pid int) ([]orangeflix.Episode, error) {
	if a.V3 == nil {
		return nil, errors.New("the version 3 API is not available")
	}
	if err := a.EnsureV3(ctx); err != nil {
		return nil, err
	}
	const pageSize = 50
	var all []orangeflix.Episode
	for offset := 0; offset < 5000; offset += pageSize {
		clips, err := a.V3.Client.Feed(ctx, pid, offset, pageSize)
		if err != nil {
			return nil, err
		}
		if len(clips) == 0 {
			break
		}
		for _, clip := range clips {
			all = append(all, clip.Episode())
		}
		if len(clips) < pageSize {
			break
		}
	}
	// Die neuesten Folgen kommen zuerst, die Liste wird wie im alten Feed
	// aufsteigend sortiert.
	sortEpisodesByNumber(all)
	return all, nil
}

// Search looks for clips in the whole catalog. The search covers the titles,
// the descriptions and the transcripts.
func (a *App) Search(ctx context.Context, query string, limit int) ([]orangeflix.Episode, error) {
	if a.V3 == nil {
		return nil, errors.New("the version 3 API is not available")
	}
	if err := a.EnsureV3(ctx); err != nil {
		return nil, err
	}
	results, err := a.V3.Client.Search(ctx, query, limit, 0)
	if err != nil {
		a.logf("Die Suche ist fehlgeschlagen: %s", err.Error())
		return nil, err
	}
	episodes := make([]orangeflix.Episode, 0, len(results))
	for _, result := range results {
		episode := orangeflix.Episode{
			Identifier:  result.ID,
			PID:         result.MagazineID,
			Number:      result.SeqNr,
			Title:       result.Title,
			FormatName:  result.ProjectTitle,
			Description: result.ShortDescription,
			Duration:    result.Duration,
			Thumb:       result.Image,
		}
		episodes = append(episodes, episode)
	}
	return episodes, nil
}

// sortEpisodesByNumber sorts the episodes by their number, the oldest first.
func sortEpisodesByNumber(episodes []orangeflix.Episode) {
	for i := 1; i < len(episodes); i++ {
		for j := i; j > 0 && episodes[j].Number < episodes[j-1].Number; j-- {
			episodes[j], episodes[j-1] = episodes[j-1], episodes[j]
		}
	}
}

// ProfileSummary returns the state of the subscription for the interface.
func (a *App) ProfileSummary(ctx context.Context) (*ProfileSummary, error) {
	profile, err := a.Profile(ctx)
	if err != nil {
		return nil, err
	}
	summary := &ProfileSummary{
		HasAccess:  profile.Subscription.HasAccess,
		Tier:       profile.Subscription.Tier,
		AccessType: profile.Subscription.AccessType,
		Lifetime:   profile.Subscription.Lifetime,
	}
	if profile.Subscription.ExpiresAt > 0 {
		summary.ExpiresAt = time.Unix(profile.Subscription.ExpiresAt, 0)
	}
	summary.Description = summary.Summarize()
	return summary, nil
}

// ProfileSummary describes the subscription for the user interface.
type ProfileSummary struct {
	// HasAccess tells whether the account may watch the premium content.
	HasAccess bool
	// Tier is the level of the subscription.
	Tier int
	// AccessType names the kind of the access ("subscription").
	AccessType string
	// ExpiresAt is the time the subscription runs out.
	ExpiresAt time.Time
	// Lifetime tells whether the subscription never runs out.
	Lifetime bool
	// Description is a short text for the user interface.
	Description string
}

// DaysLeft is the number of days until the subscription runs out; the
// maximum of int is returned when it never runs out or the date is unknown.
func (p *ProfileSummary) DaysLeft() int {
	if p.Lifetime || p.ExpiresAt.IsZero() {
		return int(^uint(0) >> 1)
	}
	return int(time.Until(p.ExpiresAt).Hours() / 24)
}

// ExpiryWarning is the warning text for the user when the subscription runs
// out soon, or an empty string when it does not.
func (p *ProfileSummary) ExpiryWarning() string {
	if p.Lifetime || p.ExpiresAt.IsZero() {
		return ""
	}
	days := int(time.Until(p.ExpiresAt).Hours() / 24)
	switch {
	case days < 0:
		return "Hinweis: Das Abo ist abgelaufen (" + p.ExpiresAt.Local().Format("02.01.2006") + "); vielleicht ist deshalb keine Folge ladbar."
	case days <= 14:
		return fmt.Sprintf("Hinweis: Das Abo läuft in %d Tag(en) ab (%s).", days, p.ExpiresAt.Local().Format("02.01.2006"))
	}
	return ""
}

// Summarize turns the profile into a short text.
func (p *ProfileSummary) Summarize() string {
	if !p.HasAccess {
		return "kein Zugang"
	}
	text := p.AccessType
	if text == "" {
		text = "Zugang"
	}
	if !p.ExpiresAt.IsZero() {
		text += " bis " + p.ExpiresAt.Local().Format("02.01.2006")
	}
	if p.Tier > 0 {
		text += fmt.Sprintf(" (Stufe %d)", p.Tier)
	}
	return text
}
