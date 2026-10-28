package tui

import (
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// The intro screen is a small port of the ASCII sharks of stackedlol and
// the shimmering wordmark of orangeflix.de: a school of darting fish swims
// through faint water, the iris of the logo of the brand turns above them,
// and the wordmark carries the animated brand gradient of the website.

// splashFrameTime is the duration of one frame of the intro.
const splashFrameTime = 80 * time.Millisecond

// splashGlyph is one cell of the intro scene with its color kind.
type splashGlyph struct {
	glyph rune
	kind  int
	// color overrides the color of the kind, when it is set.
	color string
	// background is the color behind the glyph; the logo puts its lower
	// pixel there.
	background string
}

// renderSplashGlyph renders one cell of the scene with its colors.
func renderSplashGlyph(cell splashGlyph) string {
	switch {
	case cell.kind == splashWater:
		return " "
	case cell.color != "":
		style := colorStyle(cell.color)
		if cell.background != "" {
			style = style.Background(lipgloss.Color(cell.background))
		}
		return style.Render(string(cell.glyph))
	}
	return splashStyles[cell.kind].Render(string(cell.glyph))
}

// The color kinds of the scene, tones of the deep water and the shark.
const (
	splashWater    = iota // the base of the water, an empty cell
	splashKindFish        // the silver dashes of the sardines
	splashSkin            // the bright skin of the shark
	splashEdge            // the shaded edge of the body
	splashFin             // the fins, darker than the skin
	splashRidge           // the dorsal ridge, the brightest line
	splashWake            // the cyan wake in the water
	splashIris            // the blades of the logo of the brand
)

// splashStyles maps the kinds to the colors of the scene.
var splashStyles = map[int]lipgloss.Style{
	splashKindFish: lipgloss.NewStyle().Foreground(lipgloss.Color("#a9c2d8")),
	splashSkin:     lipgloss.NewStyle().Foreground(lipgloss.Color("#c3d2e0")),
	splashEdge:     lipgloss.NewStyle().Foreground(lipgloss.Color("#7f93a8")),
	splashFin:      lipgloss.NewStyle().Foreground(lipgloss.Color("#64788c")),
	splashRidge:    lipgloss.NewStyle().Foreground(lipgloss.Color("#eaf2f9")),
	splashWake:     lipgloss.NewStyle().Foreground(lipgloss.Color("#22505e")),
}

// splashFishGlyphs cycles through the dashes of the swimming sardines.
var splashFishGlyphs = []rune{'·', '-', '~', '·', '-', ','}

// splashFishCount is the size of the school of the sardines.
const splashFishCount = 22

// splashFish is one fish of the school.
type splashFish struct {
	x, y   float64
	vx, vy float64
	glyph  int // the index of the glyph cycle
}

// splashSim carries the animation between two frames.
type splashSim struct {
	width  int
	height int
	fish   []splashFish
	frames int
	// iris is the logo of the brand in the middle of the water.
	iris  *irisLogo
	irisX int
	irisY int
}

// splashSpeed is the drift of the school in cells per second.
const splashSpeed = 9.0

// newSplashSim seeds the school in the middle of the water.
func newSplashSim(width, height int) *splashSim {
	sim := &splashSim{width: width, height: height}
	centerX, centerY := float64(width)/2, float64(height)/3
	sim.fish = make([]splashFish, splashFishCount)
	for index := range sim.fish {
		angle := 2 * math.Pi * float64(index) / splashFishCount
		radius := 2 + 5.5*math.Mod(float64(index)*2.399, 1)
		sim.fish[index] = splashFish{
			x:     centerX + radius*math.Cos(angle),
			y:     centerY + radius*math.Sin(angle)*0.55,
			vx:    -splashSpeed,
			glyph: index % len(splashFishGlyphs),
		}
	}
	sim.placeIris()
	return sim
}

// placeIris puts the logo of the brand in the middle of the water. The mark
// is square, so it needs half as many rows as columns.
func (sim *splashSim) placeIris() {
	// Das Logo ist das Bild der Szene: es nimmt die Höhe des Wassers bis auf
	// eine Zeile Rand oben und unten, die Fische schwimmen um es herum.
	height := clampInt(sim.height-2, 4, 18)
	// Eine Zeile trägt zwei Pixel und ist doppelt so hoch wie breit, darum
	// braucht das runde Logo doppelt so viele Spalten wie Zeilen.
	height = min(height, (sim.width-8)/2)
	height = max(height, 4)
	width := height * 2
	sim.iris = newIrisLogo(width, height)
	sim.irisX = (sim.width - width) / 2
	sim.irisY = max(0, (sim.height-height)/2)
}

// pushOutOfIris moves a point out of the logo, to the nearest side.
func (sim *splashSim) pushOutOfIris(point *splashFish) {
	if sim.iris == nil {
		return
	}
	left := float64(sim.irisX - 1)
	right := float64(sim.irisX + sim.iris.width + 1)
	top := float64(sim.irisY - 1)
	bottom := float64(sim.irisY + sim.iris.height + 1)
	if point.x < left || point.x > right || point.y < top || point.y > bottom {
		return
	}
	distances := []float64{
		math.Abs(point.x - left),
		math.Abs(right - point.x),
		math.Abs(point.y - top),
		math.Abs(bottom - point.y),
	}
	smallest := 0
	for index, distance := range distances {
		if distance < distances[smallest] {
			smallest = index
		}
	}
	switch smallest {
	case 0:
		point.x = left
	case 1:
		point.x = right
	case 2:
		point.y = top
	case 3:
		point.y = bottom
	}
}

// clampValue bounds a float.
func clampValue(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

// clampInt bounds an integer.
func clampInt(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

// step animates one frame: the school darts and mills, everything wraps at
// the edges.
func (sim *splashSim) step() {
	sim.frames++
	dt := splashFrameTime.Seconds()

	// The boids of the school: a weak cohesion holds them together, the
	// separation keeps them in their own cells.
	centerX, centerY := 0.0, 0.0
	for _, fish := range sim.fish {
		centerX += fish.x
		centerY += fish.y
	}
	centerX /= float64(len(sim.fish))
	centerY /= float64(len(sim.fish))
	for index := range sim.fish {
		fish := &sim.fish[index]
		fish.vx -= splashSpeed * 1.15 * dt // the school drifts leftwards
		fish.vx += (centerX - fish.x) * 0.22 * dt
		fish.vy += (centerY - fish.y) * 0.24 * dt
		for other := range sim.fish {
			if other == index {
				continue
			}
			deltaX := fish.x - sim.fish[other].x
			deltaY := fish.y - sim.fish[other].y
			if distance := math.Hypot(deltaX, deltaY); distance > 0.01 && distance < 2.6 {
				weight := (2.6 - distance) / 2.6
				fish.vx += deltaX / distance * weight * 26 * dt
				fish.vy += deltaY / distance * weight * 26 * dt
			}
		}
		fish.vy *= 0.86 // real fish cruise level
		splashClampSpeed(fish, 4, 30)
		fish.glyph = (fish.glyph + 1) % len(splashFishGlyphs)
		fish.x += fish.vx * dt
		fish.y += fish.vy * dt
		// Die Fische schwimmen um das Logo herum.
		sim.pushOutOfIris(fish)
		if fish.x < -1 {
			fish.x = float64(sim.width) + 1
		}
		if fish.x > float64(sim.width)+1 {
			fish.x = -1
		}
		fish.y = clampValue(fish.y, 0, float64(sim.height)-1)
	}
}

// splashClampSpeed keeps the speed of a fish in its bounds.
func splashClampSpeed(fish *splashFish, low, high float64) {
	speed := math.Hypot(fish.vx, fish.vy)
	if speed < 0.01 {
		return
	}
	if speed < low {
		fish.vx *= low / speed
		fish.vy *= low / speed
	}
	if speed > high {
		fish.vx *= high / speed
		fish.vy *= high / speed
	}
}

// paint draws the whole scene: the faint water, the sardines and the logo
// of the brand, and returns the rows with their colors.
func (sim *splashSim) paint() []string {
	rows := sim.height
	cells := make([][]splashGlyph, rows)
	for row := range cells {
		cells[row] = make([]splashGlyph, sim.width)
	}

	// Sparse wake dots: the agitation of the water, shifted every few
	// frames so the sea shimmers.
	offset := (sim.frames / 2) % 7
	for row := 0; row < rows; row++ {
		for col := 0; col < sim.width; col++ {
			if (col*17+row*11+offset*3)%71 == 0 {
				cells[row][col] = splashGlyph{glyph: '˙', kind: splashWake}
			}
		}
	}

	// The sardines.
	for index := range sim.fish {
		fish := &sim.fish[index]
		col, row := int(fish.x+0.5), int(fish.y+0.5)
		if col < 0 || col >= sim.width || row < 0 || row >= rows {
			continue
		}
		cells[row][col] = splashGlyph{glyph: splashFishGlyphs[fish.glyph], kind: splashKindFish}
	}

	// The logo of the brand floats in the middle of the water.
	sim.paintIris(cells)

	// The renderer groups runs of one kind, so the escape sequences stay
	// short and the rows keep their width.
	lines := make([]string, rows)
	for row := 0; row < rows; row++ {
		var builder strings.Builder
		for col := 0; col < sim.width; {
			if cells[row][col].kind == splashWater {
				builder.WriteString(" ")
				col++
				continue
			}
			if cells[row][col].color != "" {
				// A cell with its own color stands on its own.
				builder.WriteString(renderSplashGlyph(cells[row][col]))
				col++
				continue
			}
			kind := cells[row][col].kind
			for col < sim.width && cells[row][col].kind == kind && cells[row][col].color == "" {
				builder.WriteString(splashStyles[kind].Render(string(cells[row][col].glyph)))
				col++
			}
		}
		lines[row] = builder.String()
	}
	return lines
}

// paintIris puts the logo of the brand into the grid of the scene.
func (sim *splashSim) paintIris(cells [][]splashGlyph) {
	if sim.iris == nil {
		return
	}
	grid := sim.iris.paint(sim.frames)
	for row := range grid {
		target := sim.irisY + row
		if target < 0 || target >= len(cells) {
			continue
		}
		for col := range grid[row] {
			place := sim.irisX + col
			if place < 0 || place >= len(cells[target]) {
				continue
			}
			if grid[row][col].kind == splashWater {
				continue
			}
			cells[target][place] = grid[row][col]
		}
	}
}

// The block letters of the wordmark, five rows of block glyphs, in the
// spirit of the wordmark of the website. Only the letters of "ORANGEFLIX"
// are needed.
var splashLetters = map[rune][]string{
	'O': {" ████ ", "██  ██", "██  ██", "██  ██", " ████ "},
	'R': {"█████ ", "██  ██", "█████ ", "██  ██", "██  ██"},
	'A': {" ████ ", "██  ██", "██████", "██  ██", "██  ██"},
	'N': {"██  ██", "███ ██", "██████", "██ ███", "██  ██"},
	'G': {" ████ ", "██    ", "██ ███", "██  ██", " ████ "},
	'E': {"██████", "██    ", "█████ ", "██    ", "██████"},
	'F': {"██████", "██    ", "█████ ", "██    ", "██    "},
	'L': {"██    ", "██    ", "██    ", "██    ", "██████"},
	'I': {"██████", "  ██  ", "  ██  ", "  ██  ", "██████"},
	'X': {"██  ██", "██  ██", " ████ ", "██  ██", "██  ██"},
}

// splashWordmarkWidth is the width of the whole wordmark with the spaces
// between the letters.
func splashWordmarkWidth() int {
	return len([]rune("ORANGEFLIX"))*7 - 1
}

// splashWordmark builds the wordmark out of the block letters and walks the
// brand gradient over it, so the shimmer drifts like on the animated logo
// of the website.
func splashWordmark(frame int) []string {
	rows := 5
	result := make([][]string, rows)
	letters := []rune("ORANGEFLIX")
	for index, letter := range letters {
		glyphRows := splashLetters[letter]
		for row := 0; row < rows; row++ {
			result[row] = append(result[row], glyphRows[row])
			if index < len(letters)-1 {
				result[row] = append(result[row], " ")
			}
		}
	}
	shift := (frame / 2) % (splashWordmarkWidth() + 8)
	mark := make([]string, rows)
	for row := 0; row < rows; row++ {
		mark[row] = gradientText(strings.Join(result[row], ""), shift+row*2)
	}
	return mark
}

// splashPanel renders the intro screen with the scene and the wordmark. The
// scene takes the room that is left after the wordmark and the two lines
// of the text, so nothing is cut off.
func (m *Model) splashPanel(box rect) []string {
	// The two lines of the text at the bottom of the screen.
	const textLines = 2
	// The wordmark needs five rows and a blank line above it.
	markRows := 0
	if splashWordmarkWidth() <= box.width && box.height >= 16 {
		markRows = 6
	}
	sceneHeight := box.height - textLines - markRows
	if sceneHeight < 6 {
		sceneHeight = 6
	}
	if sceneHeight > box.height {
		sceneHeight = box.height
	}

	if m.splash == nil || m.splash.width != box.width || m.splash.height != sceneHeight {
		m.splash = newSplashSim(box.width, sceneHeight)
	}
	m.splash.step()

	content := m.splash.paint()

	// Der Schriftzug und der Text stehen mittig unter dem Logo.
	text := []string{}
	if markRows > 0 {
		text = append(text, "")
		// The wordmark stands under the scene; the small logo takes over
		// when the terminal is too narrow for the whole mark.
		text = append(text, splashWordmark(m.splash.frames)...)
	} else {
		text = append(text, gradientText(logoLine, m.splash.frames*2))
	}
	text = append(text,
		theme.Muted.Render("v"+m.version+" · Formate abfragen und Folgen laden"),
		theme.Muted.Render(m.spinner.View()+" starte … (Taste überspringt)"),
	)
	for _, line := range text {
		content = append(content, centerLine(line, box.width))
	}
	return content
}
