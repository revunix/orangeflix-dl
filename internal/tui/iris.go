package tui

import (
	"math"
	"strconv"
	"strings"
	"unicode"
)

// The logo of orangeflix.de is an iris: six blades, cut to a disc, and two
// leaves at the upper left. At rest the iris stands open and white; when it
// shuts, the blades turn into one another, take the orange of the brand and
// the leaves turn green, so the iris becomes an orange. This file draws
// that mark with the geometry and the timing of the SVG and the CSS of the
// website into the character grid of the terminal. Every cell holds two
// pixels, the upper and the lower half of a block glyph.

// The geometry of the logo in the units of the SVG of the website.
const (
	// irisViewMin and irisViewSize are the view box of the SVG.
	irisViewMin  = 220.0
	irisViewSize = 1060.0
	// irisCenterX and irisCenterY are the centre of the iris.
	irisCenterX = 811.15
	irisCenterY = 826.9
	// irisDiscRadius is the radius of the disc the blades are cut to.
	irisDiscRadius = 440.0
	// irisBladeCount is the number of the blades of the iris.
	irisBladeCount = 6
	// irisShut is the turn of every blade when the iris is shut; at rest
	// the blades stand at zero.
	irisShut = 38.15
	// irisPivotX and irisPivotY are the origin of the turn of a blade,
	// relative to the corner of the box around the blade.
	irisPivotX = 169.46
	irisPivotY = 269.00
	// irisLeafTurn is the turn of the leaves around their own centre.
	irisLeafTurn  = -9.0
	irisLeafTurnX = 648.68
	irisLeafTurnY = 550.46
)

// The colors of the logo. On a dark page the iris is white at rest; on a
// light page it is dark, so it stays visible.
const (
	irisOrange        = "#f19100"
	irisGreen         = "#009245"
	irisInkStrong     = "#f4f4f5"
	irisInkSoft       = "#e4e4e7"
	irisInkStrongLt   = "#27272a"
	irisInkSoftLt     = "#52525b"
	irisBodyOpacity   = 0.73
	irisLeafOpacity   = 0.56
	irisMinCoverage   = 0.12
	irisSamplesPerAxe = 4
)

// The timeline of the splash: the iris rests, shuts with the overshoot of
// the website, stays shut for a moment and opens again.
const (
	irisPeriod    = 3.4  // the length of one round in seconds
	irisRest      = 0.6  // the start of the shut
	irisTurnTime  = 0.72 // the turn of the blades
	irisColorTime = 0.36 // the change of the color of the blades
	irisLeafDelay = 0.22 // the leaves follow the blades
	irisLeafTime  = 0.42 // the change of the color of the leaves
	irisOrbitTime = 0.56 // the turn of the leaves
	irisOpen      = 2.6  // the start of the opening
	irisOpenTime  = 0.45 // the opening, a little slower than on the website
)

// irisPoint is a point in the coordinate system of the logo.
type irisPoint struct {
	x, y float64
}

// irisBladeSVG is the path of one blade on the website.
const irisBladeSVG = "M 641.69 271.17 L 1024.39 758.37 L 1205 307.96 A 600 600 0 0 0 641.69 271.17 Z"

// irisLeafSVG are the paths of the two leaves on the website.
var irisLeafSVG = []string{
	"M458.71,492.35c-2.72-20.72-2.66-41.94.33-63.2,1.86-13.21,4.8-26.1,8.76-38.57-52.23-57.75-125.71-96.49-209.45-103.24l-9.8-.79h0c-9.56,118.7,47.62,227.36,140.1,289.1,19.22-30.74,42.67-58.65,70.06-83.3Z",
	"M497.34,467.77c47-33.68,99.25-58.5,155.47-73.89,1.62-52.9-12.42-106.81-43.66-154.42h0s0,0,0,0c-81.44,53.45-124.07,143.82-120.31,234.55,2.81-2.11,5.64-4.19,8.5-6.25Z",
}

// The outlines of the shapes; they never change.
var (
	irisBlade  = parseSVGPath(irisBladeSVG)
	irisLeaves = func() [][]irisPoint {
		leaves := make([][]irisPoint, len(irisLeafSVG))
		for index, path := range irisLeafSVG {
			leaves[index] = parseSVGPath(path)
		}
		return leaves
	}()
	irisPivot = func() irisPoint {
		minX, minY := math.Inf(1), math.Inf(1)
		for _, point := range irisBlade {
			minX = math.Min(minX, point.x)
			minY = math.Min(minY, point.y)
		}
		return irisPoint{x: minX + irisPivotX, y: minY + irisPivotY}
	}()
)

// irisState is the look of the logo at one moment.
type irisState struct {
	// turn is the turn of the blades in degrees.
	turn float64
	// bladeMix runs from the ink of the rest to the orange of the shut iris.
	bladeMix float64
	// leafMix runs from the soft ink to the green of the leaves.
	leafMix float64
	// orbit is the turn of the leaves around the iris in degrees, scale
	// their growth.
	orbit, scale float64
}

// irisStateAt returns the look of the logo after the given seconds.
func irisStateAt(seconds float64) irisState {
	time := math.Mod(seconds, irisPeriod)
	shut := time - irisRest
	if time >= irisOpen {
		// Das Öffnen: alles kehrt gemeinsam in die Ruhe zurück.
		back := 1 - cubicBezier(0.22, 1, 0.36, 1, clampValue((time-irisOpen)/irisOpenTime, 0, 1))
		return irisState{turn: irisShut * back, bladeMix: back, leafMix: back, orbit: -4 * back, scale: 1 + 0.01*back}
	}
	if shut < 0 {
		return irisState{scale: 1}
	}
	orbit := cubicBezier(0.22, 1, 0.36, 1, clampValue((shut-irisLeafDelay)/irisOrbitTime, 0, 1))
	return irisState{
		turn:     irisShut * cubicBezier(0.34, 1.2, 0.64, 1, clampValue(shut/irisTurnTime, 0, 1)),
		bladeMix: clampValue(shut/irisColorTime, 0, 1),
		leafMix:  clampValue((shut-irisLeafDelay)/irisLeafTime, 0, 1),
		orbit:    -4 * orbit,
		scale:    1 + 0.01*orbit,
	}
}

// irisLogo is the logo in the character grid of the terminal.
type irisLogo struct {
	width  int
	height int
}

// newIrisLogo builds the logo for a box of the given size. The cells of a
// terminal are twice as high as they are wide and every cell carries two
// pixels, so a square logo needs half as many rows as columns.
func newIrisLogo(width, height int) *irisLogo {
	return &irisLogo{width: max(width, 4), height: max(height, 2)}
}

// irisSample is the color of one point of the logo with its opacity.
type irisSample struct {
	r, g, b, alpha float64
}

// paint draws the logo into a grid of cells. The cells carry the glyph and
// the colors, so the caller can put the mark into a bigger scene.
func (logo *irisLogo) paint(frame int) [][]splashGlyph {
	state := irisStateAt(float64(frame) * splashFrameTime.Seconds())
	pixels := logo.pixels(state)
	grid := make([][]splashGlyph, logo.height)
	for row := range grid {
		grid[row] = make([]splashGlyph, logo.width)
		for col := range grid[row] {
			grid[row][col] = irisCell(pixels[row*2][col], pixels[row*2+1][col])
		}
	}
	return grid
}

// irisCell puts two pixels into one cell: the upper half is the color of the
// glyph, the lower half the background.
func irisCell(top, bottom string) splashGlyph {
	switch {
	case top == "" && bottom == "":
		return splashGlyph{}
	case bottom == "":
		return splashGlyph{glyph: '▀', kind: splashIris, color: top}
	case top == "":
		return splashGlyph{glyph: '▄', kind: splashIris, color: bottom}
	case top == bottom:
		return splashGlyph{glyph: '█', kind: splashIris, color: top}
	}
	return splashGlyph{glyph: '▀', kind: splashIris, color: top, background: bottom}
}

// pixels renders the logo into a grid of pixels, two per cell. Every pixel
// is sampled a few times, so the edges are smooth; a pixel that the logo
// barely touches stays empty and the scene shows through.
func (logo *irisLogo) pixels(state irisState) [][]string {
	columns, rows := logo.width, logo.height*2
	backR, backG, backB := parseHexColor(irisPageColor())
	strong, soft := irisInkStrong, irisInkSoft
	if !isDark() {
		strong, soft = irisInkStrongLt, irisInkSoftLt
	}
	blade := irisColor(irisMix(strong, irisOrange, state.bladeMix))
	leaf := irisColor(irisMix(soft, irisGreen, state.leafMix))
	bodyAlpha := irisBodyOpacity + (1-irisBodyOpacity)*state.bladeMix
	leafAlpha := irisLeafOpacity + (1-irisLeafOpacity)*state.leafMix

	// Die Drehungen der Klingen und der Blätter werden einmal je Bild
	// gerechnet.
	turns := make([]float64, irisBladeCount)
	for index := range turns {
		turns[index] = float64(index) * 60 * math.Pi / 180
	}
	turn := state.turn * math.Pi / 180
	orbit := state.orbit * math.Pi / 180
	leafTurn := irisLeafTurn * math.Pi / 180

	result := make([][]string, rows)
	for row := range result {
		result[row] = make([]string, columns)
		for col := range result[row] {
			var sum irisSample
			for sy := 0; sy < irisSamplesPerAxe; sy++ {
				for sx := 0; sx < irisSamplesPerAxe; sx++ {
					point := irisPoint{
						x: irisViewMin + (float64(col)+(float64(sx)+0.5)/irisSamplesPerAxe)/float64(columns)*irisViewSize,
						y: irisViewMin + (float64(row)+(float64(sy)+0.5)/irisSamplesPerAxe)/float64(rows)*irisViewSize,
					}
					sample := irisSample{}
					if irisInBody(point, turns, turn) {
						sample = irisOver(sample, blade, bodyAlpha)
					}
					if irisInLeaf(point, orbit, state.scale, leafTurn) {
						sample = irisOver(sample, leaf, leafAlpha)
					}
					sum.r += sample.r
					sum.g += sample.g
					sum.b += sample.b
					sum.alpha += sample.alpha
				}
			}
			count := float64(irisSamplesPerAxe * irisSamplesPerAxe)
			alpha := sum.alpha / count
			if alpha < irisMinCoverage {
				continue
			}
			// Die Farben sind mit ihrer Deckkraft gewichtet; der Rest ist
			// die Seite dahinter.
			result[row][col] = formatHexColor(
				clampByte(sum.r/count+float64(backR)*(1-alpha)),
				clampByte(sum.g/count+float64(backG)*(1-alpha)),
				clampByte(sum.b/count+float64(backB)*(1-alpha)),
			)
		}
	}
	return result
}

// irisInBody tells whether a point lies on one of the blades. The blades
// are cut to the disc of the iris.
func irisInBody(point irisPoint, turns []float64, turn float64) bool {
	if math.Hypot(point.x-irisCenterX, point.y-irisCenterY) > irisDiscRadius {
		return false
	}
	center := irisPoint{x: irisCenterX, y: irisCenterY}
	for _, group := range turns {
		// Erst die Drehung der Gruppe um die Mitte zurück, dann die Drehung
		// der Klinge um ihren eigenen Punkt.
		local := irisRotateAround(point, center, -group)
		local = irisRotateAround(local, irisPivot, -turn)
		if irisInsidePolygon(local, irisBlade) {
			return true
		}
	}
	return false
}

// irisInLeaf tells whether a point lies on one of the leaves. The leaves
// turn and grow a little around the iris when it shuts.
func irisInLeaf(point irisPoint, orbit, scale, leafTurn float64) bool {
	center := irisPoint{x: irisCenterX, y: irisCenterY}
	local := irisRotateAround(point, center, -orbit)
	local = irisPoint{x: center.x + (local.x-center.x)/scale, y: center.y + (local.y-center.y)/scale}
	local = irisRotateAround(local, irisPoint{x: irisLeafTurnX, y: irisLeafTurnY}, -leafTurn)
	for _, leaf := range irisLeaves {
		if irisInsidePolygon(local, leaf) {
			return true
		}
	}
	return false
}

// irisColor turns a color into its channels, once per frame.
func irisColor(hex string) irisSample {
	r, g, b := parseHexColor(hex)
	return irisSample{r: float64(r), g: float64(g), b: float64(b), alpha: 1}
}

// irisOver lays a color with its opacity over a sample; the colors are kept
// weighted with their opacity.
func irisOver(sample, color irisSample, alpha float64) irisSample {
	return irisSample{
		r:     color.r*alpha + sample.r*(1-alpha),
		g:     color.g*alpha + sample.g*(1-alpha),
		b:     color.b*alpha + sample.b*(1-alpha),
		alpha: alpha + sample.alpha*(1-alpha),
	}
}

// irisMix mixes two colors; amount runs from 0 to 1.
func irisMix(from, to string, amount float64) string {
	fromR, fromG, fromB := parseHexColor(from)
	toR, toG, toB := parseHexColor(to)
	return formatHexColor(
		clampByte(float64(fromR)+(float64(toR)-float64(fromR))*amount),
		clampByte(float64(fromG)+(float64(toG)-float64(fromG))*amount),
		clampByte(float64(fromB)+(float64(toB)-float64(fromB))*amount),
	)
}

// irisPageColor is the color of the page behind the logo.
func irisPageColor() string {
	if isDark() {
		return colorBackground.Dark
	}
	return colorBackground.Light
}

// clampByte rounds a channel of a color into its range.
func clampByte(value float64) int {
	return clampInt(int(math.Round(value)), 0, 255)
}

// irisRotateAround turns a point around a centre. The y axis points down,
// so a positive angle turns clockwise, like in SVG and CSS.
func irisRotateAround(point, center irisPoint, angle float64) irisPoint {
	sin, cos := math.Sincos(angle)
	dx, dy := point.x-center.x, point.y-center.y
	return irisPoint{
		x: center.x + dx*cos - dy*sin,
		y: center.y + dx*sin + dy*cos,
	}
}

// irisInsidePolygon tells whether a point lies inside a polygon.
func irisInsidePolygon(point irisPoint, polygon []irisPoint) bool {
	inside := false
	for index, current := range polygon {
		next := polygon[(index+1)%len(polygon)]
		if (current.y > point.y) != (next.y > point.y) {
			cross := (next.x-current.x)*(point.y-current.y)/(next.y-current.y) + current.x
			if point.x < cross {
				inside = !inside
			}
		}
	}
	return inside
}

// cubicBezier is the easing of CSS: the curve runs from (0,0) to (1,1)
// through the two control points; the result is the progress at the time.
func cubicBezier(x1, y1, x2, y2, time float64) float64 {
	if time <= 0 {
		return 0
	}
	if time >= 1 {
		return 1
	}
	curve := func(a, b, t float64) float64 {
		return 3*a*t*(1-t)*(1-t) + 3*b*t*t*(1-t) + t*t*t
	}
	// Die Zeit liegt auf der x-Achse; der Parameter dazu wird durch Halbieren
	// gesucht, die Kurve steigt auf ihr stetig.
	low, high := 0.0, 1.0
	for range 40 {
		middle := (low + high) / 2
		if curve(x1, x2, middle) < time {
			low = middle
		} else {
			high = middle
		}
	}
	return curve(y1, y2, (low+high)/2)
}

// parseSVGPath turns a path of an SVG into a polygon. It knows the commands
// the logo uses: moves, lines, cubic curves and arcs, absolute and relative.
func parseSVGPath(path string) []irisPoint {
	tokens := svgTokens(path)
	var polygon []irisPoint
	var current, start, control irisPoint
	var command rune
	index := 0
	number := func() float64 {
		value, _ := strconv.ParseFloat(tokens[index], 64)
		index++
		return value
	}
	cubic := func(c1, c2, end irisPoint) {
		from := current
		const steps = 12
		for step := 1; step <= steps; step++ {
			t := float64(step) / steps
			u := 1 - t
			polygon = append(polygon, irisPoint{
				x: u*u*u*from.x + 3*u*u*t*c1.x + 3*u*t*t*c2.x + t*t*t*end.x,
				y: u*u*u*from.y + 3*u*u*t*c1.y + 3*u*t*t*c2.y + t*t*t*end.y,
			})
		}
		control = c2
		current = end
	}
	for index < len(tokens) {
		if letter := []rune(tokens[index])[0]; unicode.IsLetter(letter) {
			command = letter
			index++
		}
		relative := unicode.IsLower(command)
		offset := func(point irisPoint) irisPoint {
			if relative {
				return irisPoint{x: current.x + point.x, y: current.y + point.y}
			}
			return point
		}
		switch unicode.ToUpper(command) {
		case 'M':
			current = offset(irisPoint{x: number(), y: number()})
			start = current
			control = current
			polygon = append(polygon, current)
			// Weitere Paare nach einem Move sind Linien.
			if relative {
				command = 'l'
			} else {
				command = 'L'
			}
		case 'L':
			current = offset(irisPoint{x: number(), y: number()})
			control = current
			polygon = append(polygon, current)
		case 'H':
			x := number()
			if relative {
				x += current.x
			}
			current = irisPoint{x: x, y: current.y}
			control = current
			polygon = append(polygon, current)
		case 'V':
			y := number()
			if relative {
				y += current.y
			}
			current = irisPoint{x: current.x, y: y}
			control = current
			polygon = append(polygon, current)
		case 'C':
			c1 := offset(irisPoint{x: number(), y: number()})
			c2 := offset(irisPoint{x: number(), y: number()})
			end := offset(irisPoint{x: number(), y: number()})
			cubic(c1, c2, end)
		case 'S':
			// Der erste Kontrollpunkt ist der gespiegelte letzte.
			c1 := irisPoint{x: 2*current.x - control.x, y: 2*current.y - control.y}
			c2 := offset(irisPoint{x: number(), y: number()})
			end := offset(irisPoint{x: number(), y: number()})
			cubic(c1, c2, end)
		case 'A':
			rx, ry := number(), number()
			_ = number() // Die Drehung der Ellipse; das Logo hat Kreise.
			large, sweep := number() != 0, number() != 0
			end := offset(irisPoint{x: number(), y: number()})
			polygon = append(polygon, svgArc(current, end, math.Max(rx, ry), large, sweep)...)
			current = end
			control = current
		case 'Z':
			current = start
			control = current
		default:
			index++
		}
	}
	return polygon
}

// svgTokens splits a path into its letters and numbers. A number ends at a
// sign or at a second point, like in ".33-63.2".
func svgTokens(path string) []string {
	var tokens []string
	var builder strings.Builder
	flush := func() {
		if builder.Len() > 0 {
			tokens = append(tokens, builder.String())
			builder.Reset()
		}
	}
	dot := false
	for _, char := range path {
		switch {
		case unicode.IsLetter(char) && char != 'e':
			flush()
			tokens = append(tokens, string(char))
			dot = false
		case char == '-' || char == '+':
			flush()
			builder.WriteRune(char)
			dot = false
		case char == '.':
			if dot {
				flush()
			}
			builder.WriteRune(char)
			dot = true
		case unicode.IsDigit(char):
			builder.WriteRune(char)
		default:
			flush()
			dot = false
		}
	}
	flush()
	return tokens
}

// svgArc samples a circular arc of an SVG path from one point to another.
// The centre follows the rules of the SVG standard for the flags.
func svgArc(from, to irisPoint, radius float64, large, sweep bool) []irisPoint {
	dx, dy := to.x-from.x, to.y-from.y
	chord := math.Hypot(dx, dy)
	if chord < 1e-9 {
		return nil
	}
	radius = math.Max(radius, chord/2)
	middle := irisPoint{x: (from.x + to.x) / 2, y: (from.y + to.y) / 2}
	height := math.Sqrt(math.Max(0, radius*radius-chord*chord/4))
	sign := 1.0
	if large == sweep {
		sign = -1
	}
	// Die Senkrechte auf der Sehne, nach den Regeln des Standards gedreht.
	center := irisPoint{x: middle.x - sign*height*dy/chord, y: middle.y + sign*height*dx/chord}
	begin := math.Atan2(from.y-center.y, from.x-center.x)
	end := math.Atan2(to.y-center.y, to.x-center.x)
	delta := end - begin
	if sweep && delta < 0 {
		delta += 2 * math.Pi
	}
	if !sweep && delta > 0 {
		delta -= 2 * math.Pi
	}
	const steps = 24
	points := make([]irisPoint, 0, steps)
	for step := 1; step <= steps; step++ {
		angle := begin + delta*float64(step)/steps
		points = append(points, irisPoint{x: center.x + radius*math.Cos(angle), y: center.y + radius*math.Sin(angle)})
	}
	return points
}

// irisLines renders the logo on its own, for the tests.
func (logo *irisLogo) irisLines(frame int) []string {
	grid := logo.paint(frame)
	lines := make([]string, len(grid))
	for row, cells := range grid {
		var builder strings.Builder
		for _, cell := range cells {
			builder.WriteString(renderSplashGlyph(cell))
		}
		lines[row] = builder.String()
	}
	return lines
}
