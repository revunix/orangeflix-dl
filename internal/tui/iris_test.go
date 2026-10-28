package tui

import (
	"math"
	"strings"
	"testing"
)

// irisPixelAt returns the pixel of the logo at a point of the SVG.
func irisPixelAt(pixels [][]string, x, y float64) string {
	col := int((x - irisViewMin) / irisViewSize * float64(len(pixels[0])))
	row := int((y - irisViewMin) / irisViewSize * float64(len(pixels)))
	return pixels[row][col]
}

// TestIrisRestsOpenAndWhite checks the logo at rest, like in the head of the
// website: the iris stands open with a hole in the middle, the blades are
// white.
func TestIrisRestsOpenAndWhite(t *testing.T) {
	applyTheme(ThemeDark)
	pixels := newIrisLogo(80, 40).pixels(irisStateAt(0))
	if hole := irisPixelAt(pixels, irisCenterX, irisCenterY); hole != "" {
		t.Errorf("the middle of the open iris is %q, want empty", hole)
	}
	// Ein Punkt am Rand der Scheibe liegt auf einer Klinge.
	blade := irisPixelAt(pixels, irisCenterX, irisCenterY+irisDiscRadius*0.8)
	if blade == "" {
		t.Fatal("the blades are missing")
	}
	if r, g, b := parseHexColor(blade); r < 150 || r != g || g < b-5 {
		t.Errorf("the blade at rest is %q, want a light grey", blade)
	}
}

// TestIrisShutsIntoAnOrange checks the shut iris: the blades close the disc
// in the orange of the brand and the leaves turn green.
func TestIrisShutsIntoAnOrange(t *testing.T) {
	applyTheme(ThemeDark)
	pixels := newIrisLogo(80, 40).pixels(irisStateAt(irisRest + 1.5))
	if middle := irisPixelAt(pixels, irisCenterX, irisCenterY); !strings.EqualFold(middle, irisOrange) {
		t.Errorf("the middle of the shut iris is %q, want %s", middle, irisOrange)
	}
	// Die Scheibe ist fast ganz geschlossen.
	inside, orange := 0, 0
	for row := range pixels {
		for col := range pixels[row] {
			x := irisViewMin + (float64(col)+0.5)/float64(len(pixels[row]))*irisViewSize
			y := irisViewMin + (float64(row)+0.5)/float64(len(pixels))*irisViewSize
			if math.Hypot(x-irisCenterX, y-irisCenterY) > irisDiscRadius*0.9 {
				continue
			}
			inside++
			if strings.EqualFold(pixels[row][col], irisOrange) {
				orange++
			}
		}
	}
	if orange < inside*95/100 {
		t.Errorf("only %d of %d pixels of the disc are orange", orange, inside)
	}
	// Die Blätter oben links sind grün.
	if leaf := irisPixelAt(pixels, 560, 380); !strings.EqualFold(leaf, irisGreen) {
		t.Errorf("the leaf is %q, want %s", leaf, irisGreen)
	}
}

// TestIrisOvershootsLikeTheWebsite checks the easing of the turn: the
// blades swing a little past the shut position before they settle.
func TestIrisOvershootsLikeTheWebsite(t *testing.T) {
	widest := 0.0
	for step := 0; step <= 100; step++ {
		widest = math.Max(widest, irisStateAt(irisRest+irisTurnTime*float64(step)/100).turn)
	}
	if widest <= irisShut {
		t.Errorf("the blades turn at most %.2f°, want more than %.2f°", widest, irisShut)
	}
	if turn := irisStateAt(irisOpen + irisOpenTime).turn; math.Abs(turn) > 0.01 {
		t.Errorf("the iris does not open again: %.2f°", turn)
	}
}

// TestSVGPathsOfTheLogo checks the reading of the paths of the website.
func TestSVGPathsOfTheLogo(t *testing.T) {
	if len(irisBlade) < 10 {
		t.Fatalf("the blade has %d points, the arc is missing", len(irisBlade))
	}
	// Der Drehpunkt der Klingen liegt senkrecht über der Mitte der Iris.
	if math.Abs(irisPivot.x-irisCenterX) > 0.01 {
		t.Errorf("pivot = %+v", irisPivot)
	}
	for index, leaf := range irisLeaves {
		if len(leaf) < 20 {
			t.Errorf("leaf %d has %d points", index, len(leaf))
		}
		for _, point := range leaf {
			if point.x < irisViewMin || point.y < irisViewMin || point.x > 700 || point.y > 700 {
				t.Fatalf("leaf %d leaves the upper left: %+v", index, point)
			}
		}
	}
}

// TestIrisMovesWithTheTimeline checks that the logo changes between rest
// and shut.
func TestIrisMovesWithTheTimeline(t *testing.T) {
	logo := newIrisLogo(40, 20)
	rest := strings.Join(logo.irisLines(0), "\n")
	shut := strings.Join(logo.irisLines(int((irisRest+1.5)/splashFrameTime.Seconds())), "\n")
	if rest == shut {
		t.Error("the logo does not move")
	}
}

// TestIrisMiniMarkFitsTheHeadLine checks the small logo of the head line:
// it stays short, so the breadcrumb keeps its place.
func TestIrisMiniMarkFitsTheHeadLine(t *testing.T) {
	mark := stripANSI(irisMiniMark())
	if len([]rune(mark)) != 2 {
		t.Errorf("the small logo is %q", mark)
	}
}

// TestSplashShowsTheLogo checks that the intro screen carries the logo of
// the brand in the water, drawn with half blocks.
func TestSplashShowsTheLogo(t *testing.T) {
	sim := newSplashSim(100, 24)
	if sim.iris == nil {
		t.Fatal("the scene has no logo")
	}
	text := strings.Join(sim.paint(), "\n")
	if !strings.ContainsAny(text, "▀▄█") {
		t.Error("the scene does not show the logo")
	}
	if sim.iris.width != sim.iris.height*2 {
		t.Errorf("the logo is %dx%d, want twice as wide as high", sim.iris.width, sim.iris.height)
	}
}
