package tui

import (
	"sync/atomic"
)

// The small logo in the head line: a leaf and the fruit, in the colors the
// iris of the website has at that moment, so the mark in the head turns
// orange together with the big one.
var irisFrameCounter atomic.Uint64

// irisMiniMark returns the small logo of the head line.
func irisMiniMark() string {
	frame := int(irisFrameCounter.Load())
	state := irisStateAt(float64(frame) * splashFrameTime.Seconds())
	strong, soft := irisInkStrong, irisInkSoft
	if !isDark() {
		strong, soft = irisInkStrongLt, irisInkSoftLt
	}
	leaf := colorStyle(irisMix(soft, irisGreen, state.leafMix)).Render("❛")
	fruit := colorStyle(irisMix(strong, irisOrange, state.bladeMix)).Render("●")
	return leaf + fruit
}
