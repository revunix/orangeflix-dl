package tui

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	_ "image/gif"  // GIF avatars
	_ "image/jpeg" // JPEG avatars
	_ "image/png"  // PNG avatars
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// The avatar is drawn out of half blocks: every cell carries two pixels on
// top of each other, so the cells give square pixels.
const (
	// avatarCols and avatarRows are the cells of an avatar.
	avatarCols = 4
	avatarRows = 2
	// avatarPixels is its side in pixels.
	avatarPixels = 4
	// avatarRoom is the room in front of a bubble: the avatar and a gap.
	avatarRoom = avatarCols + 1
	// avatarBatch is how many pictures load at once.
	avatarBatch = 6
)

// avatarPicture is the downscaled picture of a chat user.
type avatarPicture struct {
	pixels [avatarPixels][avatarPixels]color.RGBA
}

// AvatarMsg carries a loaded picture.
type AvatarMsg struct {
	Name string
	Data []byte
	Err  error
}

// avatarMasked tells which pixels the round shape leaves out: the corners.
func avatarMasked(x, y int) bool {
	last := avatarPixels - 1
	return (x == 0 || x == last) && (y == 0 || y == last)
}

// errDefaultAvatar tells that the server sent its picture for users
// without one.
var errDefaultAvatar = fmt.Errorf("default avatar")

// isDefaultAvatar tells the picture of the server for users without one:
// a PNG of 45 by 45 pixels, where the real ones are JPEGs of 50 pixels.
// The dot in the color of the name says more than a grey figure.
func isDefaultAvatar(format string, bounds image.Rectangle) bool {
	return format == "png" && bounds.Dx() == 45 && bounds.Dy() == 45
}

// decodeAvatar reads a picture and scales it down: every pixel of the
// avatar is the mean of its area of the picture.
func decodeAvatar(data []byte) (*avatarPicture, error) {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	if isDefaultAvatar(format, bounds) {
		return nil, errDefaultAvatar
	}
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return nil, fmt.Errorf("empty picture")
	}
	picture := &avatarPicture{}
	for py := 0; py < avatarPixels; py++ {
		y0 := bounds.Min.Y + py*bounds.Dy()/avatarPixels
		y1 := max(y0+1, bounds.Min.Y+(py+1)*bounds.Dy()/avatarPixels)
		for px := 0; px < avatarPixels; px++ {
			x0 := bounds.Min.X + px*bounds.Dx()/avatarPixels
			x1 := max(x0+1, bounds.Min.X+(px+1)*bounds.Dx()/avatarPixels)
			var r, g, b, n uint64
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					cr, cg, cb, _ := img.At(x, y).RGBA()
					r, g, b, n = r+uint64(cr), g+uint64(cg), b+uint64(cb), n+1
				}
			}
			picture.pixels[py][px] = color.RGBA{R: uint8(r / n >> 8), G: uint8(g / n >> 8), B: uint8(b / n >> 8), A: 255}
		}
	}
	return picture, nil
}

// fallbackAvatar is the round dot in the color of the name, shown while
// the picture loads or when it is missing.
func fallbackAvatar(name string) *avatarPicture {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(name))
	tone := chatNameColors[hash.Sum32()%uint32(len(chatNameColors))]
	hex := tone.Dark
	if !isDark() {
		hex = tone.Light
	}
	fill := parseHex(hex)
	picture := &avatarPicture{}
	for y := range picture.pixels {
		for x := range picture.pixels[y] {
			picture.pixels[y][x] = fill
		}
	}
	return picture
}

// parseHex reads a color like "#7dd3fc".
func parseHex(hex string) color.RGBA {
	var r, g, b uint8
	_, _ = fmt.Sscanf(strings.TrimPrefix(hex, "#"), "%02x%02x%02x", &r, &g, &b)
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

// hexOf writes a color for lipgloss.
func hexOf(c color.RGBA) lipgloss.Color {
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B))
}

// rows draws the avatar: two lines of half blocks. A cell with a pixel
// left out shows the page behind it.
func (p *avatarPicture) rows() []string {
	lines := make([]string, avatarRows)
	for row := 0; row < avatarRows; row++ {
		var b strings.Builder
		for x := 0; x < avatarCols; x++ {
			top, bottom := 2*row, 2*row+1
			topOff, bottomOff := avatarMasked(x, top), avatarMasked(x, bottom)
			switch {
			case topOff && bottomOff:
				b.WriteString(" ")
			case topOff:
				b.WriteString(lipgloss.NewStyle().Foreground(hexOf(p.pixels[bottom][x])).Render("▄"))
			case bottomOff:
				b.WriteString(lipgloss.NewStyle().Foreground(hexOf(p.pixels[top][x])).Render("▀"))
			default:
				b.WriteString(lipgloss.NewStyle().
					Foreground(hexOf(p.pixels[top][x])).
					Background(hexOf(p.pixels[bottom][x])).
					Render("▀"))
			}
		}
		lines[row] = b.String()
	}
	return lines
}

// avatarsShown tells whether the chat draws avatars: they need colors and
// a way to load them.
func (m *Model) avatarsShown() bool {
	return m.chatAvatarFunc != nil && sequenceOf(theme.Bubble) != ""
}

// avatarName is the name the picture of a message belongs to. A message of
// the account that is still on its way only says "Du"; its name is the one
// the server sent with the own messages.
func (m *Model) avatarName(message chatLine) string {
	if message.self && (message.from == "" || message.from == "Du") {
		return m.chatSelfName
	}
	return message.from
}

// avatarRows returns the drawn avatar of a sender.
func (m *Model) avatarRows(name string) []string {
	if picture, ok := m.avatars[name]; ok && picture != nil {
		return picture.rows()
	}
	return fallbackAvatar(name).rows()
}

// avatarCmds loads the pictures of the senders that are not known yet, a
// few at a time; the next event asks for the rest.
func (m *Model) avatarCmds() tea.Cmd {
	if m.chatAvatarFunc == nil {
		return nil
	}
	if m.avatars == nil {
		m.avatars = map[string]*avatarPicture{}
	}
	fetch := m.chatAvatarFunc
	ctx := m.ctx
	cmds := []tea.Cmd{}
	for index := len(m.chatMessages) - 1; index >= 0 && len(cmds) < avatarBatch; index-- {
		message := m.chatMessages[index]
		name := m.avatarName(message)
		if message.system || name == "" {
			continue
		}
		if _, known := m.avatars[name]; known {
			continue
		}
		// Ein Eintrag ohne Bild heißt: es wird geladen oder fehlt.
		m.avatars[name] = nil
		cmds = append(cmds, func() tea.Msg {
			data, err := fetch(ctx, name)
			return AvatarMsg{Name: name, Data: data, Err: err}
		})
	}
	return tea.Batch(cmds...)
}

// handleAvatar keeps a loaded picture. A picture that cannot be read keeps
// the dot of the name.
func (m *Model) handleAvatar(message AvatarMsg) (tea.Model, tea.Cmd) {
	if m.avatars == nil {
		m.avatars = map[string]*avatarPicture{}
	}
	if message.Err != nil {
		return m, m.avatarCmds()
	}
	if picture, err := decodeAvatar(message.Data); err == nil {
		m.avatars[message.Name] = picture
	}
	return m, m.avatarCmds()
}
