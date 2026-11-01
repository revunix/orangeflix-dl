package orangeflix

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
)

// V3Pagination is one page of a list.
type V3Pagination struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
	Count  int `json:"count"`
	Total  int `json:"total"`
}

// V3SimilarClip is one entry of the similar-clips rail.
type V3SimilarClip struct {
	Type            string `json:"type"`
	ID              string `json:"id"`
	MagazineID      int    `json:"magazineId"`
	CategoryID      int    `json:"categoryId"`
	Time            int64  `json:"time"`
	SeqNr           int    `json:"seqNr"`
	Title           string `json:"title"`
	ShortDesc       string `json:"shortdesc"`
	ProjectTitle    string `json:"projectTitle"`
	Image           string `json:"image"`
	Duration        string `json:"duration"`
	DurationSeconds int    `json:"durationSeconds"`
	Source          string `json:"source"`
}

// Episode converts a similar clip into an episode.
func (c V3SimilarClip) Episode() Episode {
	return Episode{
		Identifier:      c.ID,
		PID:             c.MagazineID,
		Number:          c.SeqNr,
		Title:           c.Title,
		FormatName:      c.ProjectTitle,
		Description:     c.ShortDesc,
		Duration:        c.Duration,
		DurationSeconds: c.DurationSeconds,
		Thumb:           c.Image,
	}
}

// V3ClipDetail is the detailed answer of the clip endpoint.
type V3ClipDetail struct {
	Clip    V3Clip          `json:"clip"`
	Similar json.RawMessage `json:"similar"`
}

// V3CalendarEntry is one entry of the calendar.
type V3CalendarEntry struct {
	ID          string           `json:"id"`
	URL         string           `json:"url"`
	Timestamp   int64            `json:"timestamp"`
	Premium     bool             `json:"premium"`
	DateTime    string           `json:"datetime"`
	Weekday     string           `json:"weekday"`
	Date        string           `json:"date"`
	Time        string           `json:"time"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	PID         int              `json:"pid"`
	Image       string           `json:"image"`
	ImageDark   string           `json:"imageDark"`
	Type        string           `json:"type"`
	Live        *V3CalendarLive  `json:"live"`
	Event       *V3CalendarEvent `json:"event"`
}

// V3CalendarLive is the live part of a calendar entry.
type V3CalendarLive struct {
	ID          string `json:"id"`
	End         int64  `json:"end"`
	Description string `json:"description"`
	HasAudience bool   `json:"hasAudience"`
}

// V3CalendarEvent is the event part of a calendar entry.
type V3CalendarEvent struct {
	ID          int    `json:"id"`
	SoldOut     bool   `json:"soldOut"`
	HasCost     bool   `json:"hasCost"`
	Description string `json:"description"`
	Address     string `json:"address"`
	Tickets     string `json:"tickets"`
	HasAudience bool   `json:"hasAudience"`
}

// V3LiveInfo is the answer of the live endpoint.
type V3LiveInfo struct {
	OK             bool              `json:"ok"`
	IsLive         bool              `json:"isLive"`
	LastWatchedAt  int64             `json:"lastWatchedAt"`
	LastWatchedVia string            `json:"lastWatchedVia"`
	IsPending      bool              `json:"isPending"`
	CanAccess      *bool             `json:"canAccess"`
	URL            map[string]string `json:"url"`
	Replay         *V3LiveReplay     `json:"replay"`
	Show           *V3LiveShow       `json:"show"`
	Scheduled      *V3LiveScheduled  `json:"scheduled"`
	Message        string            `json:"message"`
}

// HasAccess reports whether the account may watch the active stream.
func (l V3LiveInfo) HasAccess() bool {
	return l.CanAccess != nil && *l.CanAccess
}

// V3LiveShow is the current live show.
type V3LiveShow struct {
	ID          string `json:"id"`
	PID         int    `json:"pid"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Begin       int64  `json:"begin"`
	End         int64  `json:"end"`
	Chat        bool   `json:"chat"`
	IsLive      bool   `json:"isLive"`
}

// V3LiveReplay is a possible replay of the latest show.
type V3LiveReplay struct {
	Possible       bool              `json:"possible"`
	ID             string            `json:"id"`
	CanAccess      bool              `json:"canAccess"`
	PID            int               `json:"pid"`
	ProjectTitle   string            `json:"projectTitle"`
	Title          string            `json:"title"`
	Description    string            `json:"description"`
	Begin          int64             `json:"begin"`
	End            int64             `json:"end"`
	BeginFormatted string            `json:"beginFormatted"`
	ClipID         string            `json:"clipId"`
	URL            map[string]string `json:"url"`
}

// V3LiveScheduled is the next scheduled show.
type V3LiveScheduled struct {
	ID             string `json:"id"`
	PID            int    `json:"pid"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	Begin          int64  `json:"begin"`
	End            int64  `json:"end"`
	BeginFormatted string `json:"beginFormatted"`
	StartsIn       int64  `json:"startsIn"`
	Chat           bool   `json:"chat"`
}

// V3LiveProgressRequest records a live or replay position.
type V3LiveProgressRequest struct {
	Mode      string `json:"mode,omitempty"`
	BehindSec int    `json:"behindSec,omitempty"`
	WatchedAt int64  `json:"watchedAt,omitempty"`
}

// V3LiveProgressResponse is the answer of a live progress ping.
type V3LiveProgressResponse struct {
	OK        bool  `json:"ok"`
	Recorded  bool  `json:"recorded"`
	WatchedAt int64 `json:"watchedAt"`
}

// V3Announcement is an alert or a promotional campaign.
type V3Announcement struct {
	Type             string `json:"type"`
	ID               string `json:"id"`
	Title            string `json:"title"`
	BodyMarkdown     string `json:"bodyMarkdown"`
	BodyHTML         string `json:"bodyHtml"`
	URL              string `json:"url"`
	External         bool   `json:"external"`
	ExpiresAt        int64  `json:"expiresAt"`
	Date             int64  `json:"date"`
	DateTime         string `json:"datetime"`
	TimeDescription  string `json:"timeDescription"`
	ShortDescription string `json:"shortDescription"`
	Description      string `json:"description"`
	MagazineID       int    `json:"magazineId"`
	Image            string `json:"image"`
	CoverImage       string `json:"coverImage"`
	Label            string `json:"label"`
	CTALabel         string `json:"ctaLabel"`
	Priority         int    `json:"priority"`
}

// V3Host is a guest or host of a format.
type V3Host struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	NameURL     string            `json:"nameUrl"`
	Description string            `json:"description"`
	Image       string            `json:"image"`
	Social      map[string]string `json:"social"`
	Clip        string            `json:"clip"`
	ClipCount   int               `json:"clipCount"`
}

// V3HostVideo is a short video of a host page.
type V3HostVideo struct {
	Index    int    `json:"index"`
	Label    string `json:"label"`
	HLS      string `json:"hls"`
	MP4      string `json:"mp4"`
	ClipName string `json:"clipName"`
}

// V3HostResponse is the answer of the host endpoint.
type V3HostResponse struct {
	OK         bool          `json:"ok"`
	Host       V3Host        `json:"host"`
	NameURL    string        `json:"nameUrl"`
	HeaderText string        `json:"headerText"`
	HeaderLink string        `json:"headerLink"`
	Recents    []V3Clip      `json:"recents"`
	HasRecents bool          `json:"hasRecents"`
	Pagination V3Pagination  `json:"pagination"`
	Videos     []V3HostVideo `json:"videos"`
	Magazines  []V3Magazine  `json:"magazines"`
}

// V3ProgressEntry is one entry of the watch progress list.
type V3ProgressEntry struct {
	ID           string `json:"id"`
	Percentage   int    `json:"percentage"`
	HasStarted   bool   `json:"hasStarted"`
	LastAccessed string `json:"lastAccessed"`
}

// V3Download is the answer of the official download endpoint.
type V3Download struct {
	OK   bool   `json:"ok"`
	URL  string `json:"url"`
	Type int    `json:"type"`
}

// V3ChapterCategory is one chapter category.
type V3ChapterCategory struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Color string `json:"color"`
}

// V3ChapterSuggestionContext is the state for a chapter suggestion.
type V3ChapterSuggestionContext struct {
	OK               bool            `json:"ok"`
	CanSuggest       bool            `json:"canSuggest"`
	LastOutcome      json.RawMessage `json:"lastOutcome"`
	ChaptersRevision int             `json:"chaptersRevision"`
	Suggestion       json.RawMessage `json:"suggestion"`
	OtherEdges       json.RawMessage `json:"otherEdges"`
	OtherCount       int             `json:"otherCount"`
	BusyUntil        int64           `json:"busyUntil"`
}

// V3ChatMessage is one message of a chat replay.
type V3ChatMessage struct {
	Username  string  `json:"username"`
	Message   string  `json:"message"`
	Time      int64   `json:"time"`
	VideoTime float64 `json:"videoTime"`
}

// V3ChatReplay is the chat replay of a clip.
type V3ChatReplay struct {
	OK       bool            `json:"ok"`
	Messages []V3ChatMessage `json:"messages"`
	Range    struct {
		Start float64 `json:"start"`
		End   float64 `json:"end"`
	} `json:"range"`
}

// V3Token is a short-lived JWT for an external service.
type V3Token struct {
	OK   bool            `json:"ok"`
	JWT  string          `json:"jwt"`
	JWTD json.RawMessage `json:"jwtd"`
}

// V3NotificationEventPreference is the preference of one notification event.
type V3NotificationEventPreference struct {
	Enabled  bool            `json:"enabled"`
	Channels map[string]bool `json:"channels"`
}

// V3NotificationEvent is one event of the notification catalog.
type V3NotificationEvent struct {
	Type         string                        `json:"type"`
	Category     string                        `json:"category"`
	Configurable bool                          `json:"configurable"`
	Channels     []string                      `json:"channels"`
	Defaults     V3NotificationEventPreference `json:"defaults"`
	Scope        string                        `json:"scope"`
	Title        string                        `json:"title"`
	Description  string                        `json:"description"`
}

// V3NotificationCatalog is the notification event catalog.
type V3NotificationCatalog struct {
	OK       bool                  `json:"ok"`
	Channels []string              `json:"channels"`
	Events   []V3NotificationEvent `json:"events"`
}

// V3NotificationChannelPreference is the preference of one channel.
type V3NotificationChannelPreference struct {
	Enabled bool `json:"enabled"`
}

// V3NotificationPreferenceSet is the effective notification preferences.
type V3NotificationPreferenceSet struct {
	Channels      map[string]V3NotificationChannelPreference `json:"channels"`
	Events        map[string]V3NotificationEventPreference   `json:"events"`
	Subscriptions json.RawMessage                            `json:"subscriptions"`
}

// V3NotificationPreferences is the answer of the notification preferences.
type V3NotificationPreferences struct {
	OK          bool                        `json:"ok"`
	Preferences V3NotificationPreferenceSet `json:"preferences"`
}

// V3NotificationPreferencesUpdate is a partial notification update.
type V3NotificationPreferencesUpdate struct {
	Channels      map[string]V3NotificationChannelPreference `json:"channels,omitempty"`
	Events        map[string]V3NotificationEventUpdate       `json:"events,omitempty"`
	Subscriptions json.RawMessage                            `json:"subscriptions,omitempty"`
}

// V3NotificationEventUpdate updates one notification event.
type V3NotificationEventUpdate struct {
	Enabled  *bool           `json:"enabled,omitempty"`
	Channels map[string]bool `json:"channels,omitempty"`
}

// V3NotificationDevices is the list of registered push devices.
type V3NotificationDevices struct {
	OK      bool              `json:"ok"`
	Count   int               `json:"count"`
	Devices []json.RawMessage `json:"devices"`
	Push    json.RawMessage   `json:"push"`
}

// V3SharingLimit is the sharing quota of the account.
type V3SharingLimit struct {
	OK                 bool `json:"ok"`
	Eligible           bool `json:"eligible"`
	Tier               int  `json:"tier"`
	RequiredTier       int  `json:"requiredTier"`
	MonthlyLimit       int  `json:"monthlyLimit"`
	Used               int  `json:"used"`
	Remaining          int  `json:"remaining"`
	AccessLimitPerLink int  `json:"accessLimitPerLink"`
}

// V3ShareLink is one sharing link.
type V3ShareLink struct {
	Code        string `json:"code"`
	ClipID      string `json:"clipId"`
	URL         string `json:"url"`
	AccessLimit int    `json:"accessLimit"`
	Remaining   int    `json:"remaining"`
	Accessed    int    `json:"accessed"`
	Unlimited   bool   `json:"unlimited"`
	Created     int64  `json:"created"`
}

// V3ShareValidation is the answer of the sharing validation.
type V3ShareValidation struct {
	OK        bool   `json:"ok"`
	Valid     bool   `json:"valid"`
	Remaining int    `json:"remaining"`
	Reason    string `json:"reason"`
}

// V3SessionClient is the self-reported client of a session.
type V3SessionClient struct {
	ID         string `json:"id"`
	Platform   string `json:"platform"`
	DeviceType string `json:"deviceType"`
}

// V3Session is one active session of the account.
type V3Session struct {
	ID              string           `json:"id"`
	CreatedAt       int64            `json:"createdAt"`
	LastRefreshedAt int64            `json:"lastRefreshedAt"`
	DeviceName      string           `json:"deviceName"`
	Client          *V3SessionClient `json:"client"`
	UserAgent       string           `json:"userAgent"`
	IP              string           `json:"ip"`
	LastIP          string           `json:"lastIp"`
	LastSeenAt      int64            `json:"lastSeenAt"`
	Extended        bool             `json:"extended"`
	Current         bool             `json:"current"`
}

// V3UserNotificationPreference is the account-wide notification setting.
type V3UserNotificationPreference struct {
	Method     int    `json:"method"`
	MethodName string `json:"methodName"`
	Enabled    bool   `json:"enabled"`
}

// V3UserPreferenceSet is the preference part of the user profile.
type V3UserPreferenceSet struct {
	FeedStyle       int                          `json:"feedStyle"`
	LegacyPlayer    bool                         `json:"legacyPlayer"`
	ExclusionFilter json.RawMessage              `json:"exclusionFilter"`
	Notifications   V3UserNotificationPreference `json:"notifications"`
}

// V3OptionalInfo tells which optional personal data is stored.
type V3OptionalInfo struct {
	AddressCollected  bool `json:"addressCollected"`
	BirthdayCollected bool `json:"birthdayCollected"`
	AbonameCollected  bool `json:"abonameCollected"`
	CreditsCollected  bool `json:"creditsCollected"`
}

// V3Privacy is the state of the legal agreements.
type V3Privacy struct {
	AcceptedPrivacy1 bool `json:"acceptedPrivacy1"`
	AcceptedPrivacy2 bool `json:"acceptedPrivacy2"`
	AcceptedAGB2     bool `json:"acceptedAgb2"`
}

// V3UserPreferences is the answer of the preferences endpoint.
type V3UserPreferences struct {
	OK           bool                `json:"ok"`
	Preferences  V3UserPreferenceSet `json:"preferences"`
	OptionalInfo V3OptionalInfo      `json:"optionalInfo"`
	Privacy      V3Privacy           `json:"privacy"`
}

// V3UserPreferencesUpdate is a partial update of the preferences.
type V3UserPreferencesUpdate struct {
	FeedStyle       *int            `json:"feedStyle,omitempty"`
	LegacyPlayer    *bool           `json:"legacyPlayer,omitempty"`
	Name            *string         `json:"name,omitempty"`
	Chatname        *string         `json:"chatname,omitempty"`
	ExclusionFilter json.RawMessage `json:"exclusionFilter,omitempty"`
	Notifications   json.RawMessage `json:"notifications,omitempty"`
}

// V3FeedPreferencesUpdate is a partial update of the feed preferences.
type V3FeedPreferencesUpdate struct {
	FeedStyle       *int            `json:"feedStyle,omitempty"`
	ExclusionFilter json.RawMessage `json:"exclusionFilter,omitempty"`
	Exclude         json.RawMessage `json:"exclude,omitempty"`
}

// V3Address is the address of the account.
type V3Address struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	Address2 string `json:"address2,omitempty"`
	ZIP      string `json:"zip"`
	City     string `json:"city"`
	Country  string `json:"country"`
}

// V3Birthday is the birthday of the account.
type V3Birthday struct {
	Day   int `json:"day"`
	Month int `json:"month"`
	Year  int `json:"year,omitempty"`
}

// V3Credits is a credits mention.
type V3Credits struct {
	Name string `json:"name"`
	Mag  string `json:"mag"`
}

// V3Meta is the public project metadata.
type V3Meta struct {
	OK                 bool  `json:"ok"`
	TotalClipDurations int64 `json:"totalClipDurations"`
}

// V3Health is the health answer of the API.
type V3Health struct {
	OK        bool   `json:"ok"`
	Version   string `json:"version"`
	Timestamp int64  `json:"timestamp"`
}

// V3ServerTime is the server time.
type V3ServerTime struct {
	OK        bool   `json:"ok"`
	Timestamp int64  `json:"timestamp"`
	ISO       string `json:"iso"`
	Formatted string `json:"formatted"`
}

// The private answers of the endpoints that wrap their data in a named field.
type (
	v3RecentsResponse struct {
		OK    bool     `json:"ok"`
		Clips []V3Clip `json:"clips"`
	}
	v3CalendarResponse struct {
		OK      bool              `json:"ok"`
		Count   int               `json:"count"`
		Entries []V3CalendarEntry `json:"entries"`
	}
	v3AnnouncementsResponse struct {
		OK            bool             `json:"ok"`
		Count         int              `json:"count"`
		Announcements []V3Announcement `json:"announcements"`
	}
	v3SimilarResponse struct {
		OK      bool            `json:"ok"`
		ID      string          `json:"id"`
		Count   int             `json:"count"`
		Sources []string        `json:"sources"`
		Clips   []V3SimilarClip `json:"clips"`
	}
	v3ProgressResponse struct {
		OK       bool              `json:"ok"`
		Progress []V3ProgressEntry `json:"progress"`
	}
	v3BookmarkStatusResponse struct {
		OK           bool   `json:"ok"`
		ClipID       string `json:"clipId"`
		IsBookmarked bool   `json:"isBookmarked"`
	}
	v3BookmarkBatchResponse struct {
		OK        bool            `json:"ok"`
		Bookmarks map[string]bool `json:"bookmarks"`
	}
	v3MagazineResponse struct {
		OK       bool       `json:"ok"`
		Magazine V3Magazine `json:"magazine"`
	}
	v3SharingLimitResponse struct {
		OK      bool           `json:"ok"`
		Sharing V3SharingLimit `json:"sharing"`
	}
	v3ShareLinkResponse struct {
		OK   bool        `json:"ok"`
		Link V3ShareLink `json:"link"`
	}
	v3ShareLinksResponse struct {
		OK    bool          `json:"ok"`
		Links []V3ShareLink `json:"links"`
	}
	v3ResolveShareResponse struct {
		OK     bool   `json:"ok"`
		ClipID string `json:"clipId"`
	}
	v3SubscriptionResponse struct {
		OK           bool           `json:"ok"`
		Subscription V3Subscription `json:"subscription"`
	}
	v3FeedSeenResponse struct {
		OK         bool  `json:"ok"`
		FeedSeenAt int64 `json:"feedSeenAt"`
	}
	v3SessionsResponse struct {
		OK       bool        `json:"ok"`
		Sessions []V3Session `json:"sessions"`
	}
	v3TotalDurationsResponse struct {
		OK                 bool  `json:"ok"`
		TotalClipDurations int64 `json:"totalClipDurations"`
	}
	v3ChapterCategoriesResponse struct {
		OK         bool                `json:"ok"`
		Categories []V3ChapterCategory `json:"categories"`
	}
)

// Logout ends the session of the access token.
func (c *V3Client) Logout(ctx context.Context) error {
	return c.request(ctx, http.MethodPost, "auth/logout", nil, nil, nil)
}

// Handoff redeems a legacy website login handoff.
func (c *V3Client) Handoff(ctx context.Context, code, state string) error {
	var answer v3LoginResponse
	if err := c.request(ctx, http.MethodPost, "auth/handoff/redeem", nil, map[string]string{
		"code": code, "state": state,
	}, &answer); err != nil {
		return err
	}
	if answer.Auth.AccessToken == "" {
		return errors.New("the handoff failed: no access token in the answer")
	}
	c.setFromAuth(answer.Auth.AccessToken, answer.Auth.RefreshToken, answer.Auth.ExpiresIn, answer.Auth.ExpiresAt)
	return nil
}

// BookmarkStatus reports whether a clip is on the list.
func (c *V3Client) BookmarkStatus(ctx context.Context, clipID string) (bool, error) {
	var answer v3BookmarkStatusResponse
	if err := c.request(ctx, http.MethodGet, "bookmarks/"+url.PathEscape(clipID), nil, nil, &answer); err != nil {
		return false, err
	}
	return answer.IsBookmarked, nil
}

// AddBookmark puts a clip on the list.
func (c *V3Client) AddBookmark(ctx context.Context, clipID string) error {
	return c.request(ctx, http.MethodPost, "bookmarks/"+url.PathEscape(clipID), nil, nil, nil)
}

// RemoveBookmark removes a clip from the list.
func (c *V3Client) RemoveBookmark(ctx context.Context, clipID string) error {
	return c.request(ctx, http.MethodDelete, "bookmarks/"+url.PathEscape(clipID), nil, nil, nil)
}

// BookmarkStatuses checks several clips at once.
func (c *V3Client) BookmarkStatuses(ctx context.Context, ids []string) (map[string]bool, error) {
	var answer v3BookmarkBatchResponse
	if err := c.request(ctx, http.MethodPost, "bookmarks/batch-check", nil, map[string]any{
		"clipIds": ids,
	}, &answer); err != nil {
		return nil, err
	}
	return answer.Bookmarks, nil
}

// Recents returns the clips the account watched most recently.
func (c *V3Client) Recents(ctx context.Context, limit int) ([]V3Clip, error) {
	if limit <= 0 {
		limit = 50
	}
	var answer v3RecentsResponse
	if err := c.request(ctx, http.MethodGet, "content/recents", url.Values{"limit": {strconv.Itoa(limit)}}, nil, &answer); err != nil {
		return nil, err
	}
	return answer.Clips, nil
}

// Clip returns the details of one clip.
func (c *V3Client) Clip(ctx context.Context, identifier, share string) (*V3ClipDetail, error) {
	query := url.Values{}
	if share != "" {
		query.Set("share", share)
	}
	var answer V3ClipDetail
	if err := c.request(ctx, http.MethodGet, "content/clips/"+url.PathEscape(identifier), query, nil, &answer); err != nil {
		return nil, err
	}
	return &answer, nil
}

// SimilarClips returns clips related to a clip.
func (c *V3Client) SimilarClips(ctx context.Context, clipID string, limit int) ([]V3SimilarClip, error) {
	if limit <= 0 {
		limit = 10
	}
	var answer v3SimilarResponse
	if err := c.request(ctx, http.MethodGet, "content/clips/"+url.PathEscape(clipID)+"/similar",
		url.Values{"limit": {strconv.Itoa(limit)}}, nil, &answer); err != nil {
		return nil, err
	}
	return answer.Clips, nil
}

// Magazine returns one magazine of the catalog.
func (c *V3Client) Magazine(ctx context.Context, pid int) (*V3Magazine, error) {
	var answer v3MagazineResponse
	if err := c.request(ctx, http.MethodGet, "content/magazines/"+strconv.Itoa(pid), nil, nil, &answer); err != nil {
		return nil, err
	}
	return &answer.Magazine, nil
}

// Host returns the page of a guest or host.
func (c *V3Client) Host(ctx context.Context, identifier string, offset, limit int) (*V3HostResponse, error) {
	if limit <= 0 {
		limit = 50
	}
	query := url.Values{}
	query.Set("offset", strconv.Itoa(offset))
	query.Set("limit", strconv.Itoa(limit))
	var answer V3HostResponse
	if err := c.request(ctx, http.MethodGet, "content/hosts/"+url.PathEscape(identifier), query, nil, &answer); err != nil {
		return nil, err
	}
	return &answer, nil
}

// ChapterCategories returns the chapter categories.
func (c *V3Client) ChapterCategories(ctx context.Context, magazineID int) ([]V3ChapterCategory, error) {
	query := url.Values{}
	if magazineID > 0 {
		query.Set("magazineId", strconv.Itoa(magazineID))
	}
	var answer v3ChapterCategoriesResponse
	if err := c.request(ctx, http.MethodGet, "content/chapter-categories", query, nil, &answer); err != nil {
		return nil, err
	}
	return answer.Categories, nil
}

// ChapterSuggestionContext returns the state for a chapter suggestion.
func (c *V3Client) ChapterSuggestionContext(ctx context.Context, clipID string) (*V3ChapterSuggestionContext, error) {
	var answer V3ChapterSuggestionContext
	if err := c.request(ctx, http.MethodGet, "content/clips/"+url.PathEscape(clipID)+"/chapter-suggestions", nil, nil, &answer); err != nil {
		return nil, err
	}
	return &answer, nil
}

// SubmitChapterSuggestion submits a complete set of chapters. The request
// body is raw because the API does not publish its schema.
func (c *V3Client) SubmitChapterSuggestion(ctx context.Context, clipID string, body json.RawMessage) error {
	if len(body) == 0 {
		body = json.RawMessage("{}")
	}
	return c.request(ctx, http.MethodPost, "content/clips/"+url.PathEscape(clipID)+"/chapter-suggestions", nil, body, nil)
}

// ChatToken returns a short-lived chat token.
func (c *V3Client) ChatToken(ctx context.Context) (*V3Token, error) {
	var answer V3Token
	if err := c.request(ctx, http.MethodGet, "chat/token", nil, nil, &answer); err != nil {
		return nil, err
	}
	return &answer, nil
}

// ChatReplay returns the chat replay of a clip.
func (c *V3Client) ChatReplay(ctx context.Context, clipID string, start, end float64) (*V3ChatReplay, error) {
	if end <= 0 {
		return nil, errors.New("the chat replay needs an end")
	}
	query := url.Values{}
	query.Set("end", strconv.FormatFloat(end, 'f', -1, 64))
	if start > 0 {
		query.Set("start", strconv.FormatFloat(start, 'f', -1, 64))
	}
	var answer V3ChatReplay
	if err := c.request(ctx, http.MethodGet, "chat/"+url.PathEscape(clipID)+"/replay.json", query, nil, &answer); err != nil {
		return nil, err
	}
	return &answer, nil
}

// Calendar returns the upcoming calendar entries.
func (c *V3Client) Calendar(ctx context.Context, limit int) ([]V3CalendarEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	var answer v3CalendarResponse
	if err := c.request(ctx, http.MethodGet, "calendar", url.Values{"limit": {strconv.Itoa(limit)}}, nil, &answer); err != nil {
		return nil, err
	}
	return answer.Entries, nil
}

// Announcements returns the active alerts and campaigns.
func (c *V3Client) Announcements(ctx context.Context, surface string, limit int) ([]V3Announcement, error) {
	if surface == "" {
		surface = "overview"
	}
	if limit <= 0 {
		limit = 10
	}
	query := url.Values{"surface": {surface}, "limit": {strconv.Itoa(limit)}}
	var answer v3AnnouncementsResponse
	if err := c.request(ctx, http.MethodGet, "announcements", query, nil, &answer); err != nil {
		return nil, err
	}
	return answer.Announcements, nil
}

// PlaybackProgress returns the watch progress of all started clips.
func (c *V3Client) PlaybackProgress(ctx context.Context) ([]V3ProgressEntry, error) {
	var answer v3ProgressResponse
	if err := c.request(ctx, http.MethodGet, "playback/progress", nil, nil, &answer); err != nil {
		return nil, err
	}
	return answer.Progress, nil
}

// PlaybackDownloadURL resolves one download file to its short-lived URL.
func (c *V3Client) PlaybackDownloadURL(ctx context.Context, clipID, file string) (string, error) {
	if file == "" {
		return "", errors.New("the download needs a file")
	}
	var answer V3Download
	if err := c.request(ctx, http.MethodGet, "playback/"+url.PathEscape(clipID)+"/download",
		url.Values{"file": {file}}, nil, &answer); err != nil {
		return "", err
	}
	if answer.URL == "" {
		return "", errors.New("the download answer has no URL")
	}
	return answer.URL, nil
}

// LiveActive returns the active, replay or upcoming livestream.
func (c *V3Client) LiveActive(ctx context.Context) (*V3LiveInfo, error) {
	var answer V3LiveInfo
	if err := c.request(ctx, http.MethodGet, "live/active", nil, nil, &answer); err != nil {
		return nil, err
	}
	return &answer, nil
}

// RecordLiveProgress stores the live or replay position.
func (c *V3Client) RecordLiveProgress(ctx context.Context, request V3LiveProgressRequest) (*V3LiveProgressResponse, error) {
	if request.Mode == "" {
		request.Mode = "live"
	}
	if request.Mode != "live" && request.Mode != "replay" {
		return nil, errors.New("unknown live mode")
	}
	var answer V3LiveProgressResponse
	if err := c.request(ctx, http.MethodPost, "live/progress", nil, request, &answer); err != nil {
		return nil, err
	}
	return &answer, nil
}

// NotificationEvents returns the notification event catalog.
func (c *V3Client) NotificationEvents(ctx context.Context) (*V3NotificationCatalog, error) {
	var answer V3NotificationCatalog
	if err := c.request(ctx, http.MethodGet, "notifications/events", nil, nil, &answer); err != nil {
		return nil, err
	}
	return &answer, nil
}

// NotificationPreferences returns the effective notification preferences.
func (c *V3Client) NotificationPreferences(ctx context.Context) (*V3NotificationPreferences, error) {
	var answer V3NotificationPreferences
	if err := c.request(ctx, http.MethodGet, "notifications/preferences", nil, nil, &answer); err != nil {
		return nil, err
	}
	return &answer, nil
}

// UpdateNotificationPreferences updates the notification preferences.
func (c *V3Client) UpdateNotificationPreferences(ctx context.Context, update V3NotificationPreferencesUpdate) (json.RawMessage, error) {
	var answer json.RawMessage
	if err := c.request(ctx, http.MethodPatch, "notifications/preferences", nil, update, &answer); err != nil {
		return nil, err
	}
	return answer, nil
}

// UpdateNotificationEvent updates one notification event.
func (c *V3Client) UpdateNotificationEvent(ctx context.Context, eventType string, update V3NotificationEventUpdate) (json.RawMessage, error) {
	var answer json.RawMessage
	if err := c.request(ctx, http.MethodPatch, "notifications/preferences/events/"+url.PathEscape(eventType), nil, update, &answer); err != nil {
		return nil, err
	}
	return answer, nil
}

// NotificationDevices returns the registered push devices.
func (c *V3Client) NotificationDevices(ctx context.Context) (*V3NotificationDevices, error) {
	var answer V3NotificationDevices
	if err := c.request(ctx, http.MethodGet, "notifications/devices", nil, nil, &answer); err != nil {
		return nil, err
	}
	return &answer, nil
}

// DeleteNotificationDevice removes a registered push device.
func (c *V3Client) DeleteNotificationDevice(ctx context.Context, deviceID string) error {
	return c.request(ctx, http.MethodDelete, "notifications/devices/"+url.PathEscape(deviceID), nil, nil, nil)
}

// ResubscribeNotificationEmail re-enables email delivery.
func (c *V3Client) ResubscribeNotificationEmail(ctx context.Context) error {
	return c.request(ctx, http.MethodPost, "notifications/email/resubscribe", nil, nil, nil)
}

// SendTestNotification sends a test notification to the account.
func (c *V3Client) SendTestNotification(ctx context.Context) (json.RawMessage, error) {
	var answer json.RawMessage
	if err := c.request(ctx, http.MethodPost, "notifications/test", nil, nil, &answer); err != nil {
		return nil, err
	}
	return answer, nil
}

// SharingLimit returns the sharing quota of the account.
func (c *V3Client) SharingLimit(ctx context.Context) (*V3SharingLimit, error) {
	var answer v3SharingLimitResponse
	if err := c.request(ctx, http.MethodGet, "sharing/limit", nil, nil, &answer); err != nil {
		return nil, err
	}
	return &answer.Sharing, nil
}

// CreateShare creates a sharing link for a clip.
func (c *V3Client) CreateShare(ctx context.Context, clipID string) (*V3ShareLink, error) {
	var answer v3ShareLinkResponse
	if err := c.request(ctx, http.MethodPost, "sharing/"+url.PathEscape(clipID), nil, nil, &answer); err != nil {
		return nil, err
	}
	return &answer.Link, nil
}

// ValidateShare validates a sharing code.
func (c *V3Client) ValidateShare(ctx context.Context, clipID, code string) (*V3ShareValidation, error) {
	var answer V3ShareValidation
	path := "sharing/validate/" + url.PathEscape(clipID) + "/" + url.PathEscape(code)
	if err := c.request(ctx, http.MethodGet, path, nil, nil, &answer); err != nil {
		return nil, err
	}
	return &answer, nil
}

// ShareLinks returns the sharing links of the account.
func (c *V3Client) ShareLinks(ctx context.Context) ([]V3ShareLink, error) {
	var answer v3ShareLinksResponse
	if err := c.request(ctx, http.MethodGet, "sharing/links", nil, nil, &answer); err != nil {
		return nil, err
	}
	return answer.Links, nil
}

// RevokeShare deletes a sharing link.
func (c *V3Client) RevokeShare(ctx context.Context, code string) error {
	return c.request(ctx, http.MethodDelete, "sharing/"+url.PathEscape(code), nil, nil, nil)
}

// ResolveShare resolves a sharing code to its clip.
func (c *V3Client) ResolveShare(ctx context.Context, code string) (string, error) {
	var answer v3ResolveShareResponse
	if err := c.request(ctx, http.MethodGet, "sharing/resolve/"+url.PathEscape(code), nil, nil, &answer); err != nil {
		return "", err
	}
	return answer.ClipID, nil
}

// Subscription returns the subscription status.
func (c *V3Client) Subscription(ctx context.Context) (*V3Subscription, error) {
	var answer v3SubscriptionResponse
	if err := c.request(ctx, http.MethodGet, "user/subscription", nil, nil, &answer); err != nil {
		return nil, err
	}
	return &answer.Subscription, nil
}

// FeedSeen advances the feed seen marker.
func (c *V3Client) FeedSeen(ctx context.Context) (int64, error) {
	var answer v3FeedSeenResponse
	if err := c.request(ctx, http.MethodPost, "me/feed/seen", nil, nil, &answer); err != nil {
		return 0, err
	}
	return answer.FeedSeenAt, nil
}

// UserPreferences returns the preferences of the account.
func (c *V3Client) UserPreferences(ctx context.Context) (*V3UserPreferences, error) {
	var answer V3UserPreferences
	if err := c.request(ctx, http.MethodGet, "user/preferences", nil, nil, &answer); err != nil {
		return nil, err
	}
	return &answer, nil
}

// UpdateUserPreferences updates the preferences of the account.
func (c *V3Client) UpdateUserPreferences(ctx context.Context, update V3UserPreferencesUpdate) error {
	return c.request(ctx, http.MethodPatch, "user/preferences", nil, update, nil)
}

// UpdateFeedPreferences updates the feed preferences.
func (c *V3Client) UpdateFeedPreferences(ctx context.Context, update V3FeedPreferencesUpdate) error {
	return c.request(ctx, http.MethodPatch, "user/preferences/feed", nil, update, nil)
}

// SaveAddress stores the address of the account.
func (c *V3Client) SaveAddress(ctx context.Context, address V3Address) error {
	return c.request(ctx, http.MethodPut, "user/address", nil, address, nil)
}

// DeleteAddress removes the address of the account.
func (c *V3Client) DeleteAddress(ctx context.Context) error {
	return c.request(ctx, http.MethodDelete, "user/address", nil, nil, nil)
}

// SaveBirthday stores the birthday of the account.
func (c *V3Client) SaveBirthday(ctx context.Context, birthday V3Birthday) error {
	return c.request(ctx, http.MethodPut, "user/birthday", nil, birthday, nil)
}

// DeleteBirthday removes the birthday of the account.
func (c *V3Client) DeleteBirthday(ctx context.Context) error {
	return c.request(ctx, http.MethodDelete, "user/birthday", nil, nil, nil)
}

// SaveCredits stores a credits mention.
func (c *V3Client) SaveCredits(ctx context.Context, credits V3Credits) error {
	return c.request(ctx, http.MethodPut, "user/credits", nil, credits, nil)
}

// DeleteCredits removes a credits mention.
func (c *V3Client) DeleteCredits(ctx context.Context) error {
	return c.request(ctx, http.MethodDelete, "user/credits", nil, nil, nil)
}

// SessionRecoverySessions lists the sessions with a recovery token.
func (c *V3Client) SessionRecoverySessions(ctx context.Context, token string) ([]V3Session, error) {
	var answer v3SessionsResponse
	if err := c.requestWithToken(ctx, token, http.MethodGet, "auth/session-recovery/sessions", nil, nil, &answer); err != nil {
		return nil, err
	}
	return answer.Sessions, nil
}

// RevokeSession revokes a session with a recovery token.
func (c *V3Client) RevokeSession(ctx context.Context, token, sessionID string) error {
	return c.requestWithToken(ctx, token, http.MethodDelete, "auth/session-recovery/sessions/"+url.PathEscape(sessionID), nil, nil, nil)
}

// Meta returns the public project metadata.
func (c *V3Client) Meta(ctx context.Context) (*V3Meta, error) {
	var answer V3Meta
	if err := c.request(ctx, http.MethodGet, "meta", nil, nil, &answer); err != nil {
		return nil, err
	}
	return &answer, nil
}

// TotalClipDurations returns the total duration of all clips.
func (c *V3Client) TotalClipDurations(ctx context.Context) (int64, error) {
	var answer v3TotalDurationsResponse
	if err := c.request(ctx, http.MethodGet, "meta/totalClipDurations", nil, nil, &answer); err != nil {
		return 0, err
	}
	return answer.TotalClipDurations, nil
}

// Health checks the API.
func (c *V3Client) Health(ctx context.Context) (*V3Health, error) {
	var answer V3Health
	if err := c.request(ctx, http.MethodGet, "health", nil, nil, &answer); err != nil {
		return nil, err
	}
	return &answer, nil
}

// ServerTime returns the current server time.
func (c *V3Client) ServerTime(ctx context.Context) (*V3ServerTime, error) {
	var answer V3ServerTime
	if err := c.request(ctx, http.MethodGet, "time", nil, nil, &answer); err != nil {
		return nil, err
	}
	return &answer, nil
}

// VotingToken returns a short-lived voting token.
func (c *V3Client) VotingToken(ctx context.Context) (*V3Token, error) {
	var answer V3Token
	if err := c.request(ctx, http.MethodGet, "voting/token", nil, nil, &answer); err != nil {
		return nil, err
	}
	return &answer, nil
}

// AcceptAGB accepts the current terms.
func (c *V3Client) AcceptAGB(ctx context.Context) error {
	return c.request(ctx, http.MethodPost, "legal/agb/accept", nil, nil, nil)
}

// AcceptPrivacy accepts the current privacy policy.
func (c *V3Client) AcceptPrivacy(ctx context.Context) error {
	return c.request(ctx, http.MethodPost, "legal/privacy/accept", nil, nil, nil)
}
