package main

import "math"

// stepRatio is how much the finger distance must grow (or shrink by its inverse) for one zoom step.
const stepRatio = 1.2

type zoomStep int

const (
	zoomNone zoomStep = iota
	zoomIn
	zoomOut
)

type contact struct {
	id   uint32
	x, y float64
}

// pinchState is the current two-finger pair, the distance baseline for the next zoom step
// (zero means none) and the pair's positions in the previous frame.
type pinchState struct {
	pair     [2]uint32
	baseline float64
	last     [2]contact
}

// nextPinch is pure: given the previous state and the contacts now down, it returns the
// zoom step to send (if any) and the new state. Anything but exactly two contacts ends the pinch.
// Frames where the fingers travel together more than they spread or squeeze (a two-finger
// scroll) reset the baseline, so slow spacing drift while scrolling never adds up to a zoom.
func nextPinch(state pinchState, contacts []contact) (zoomStep, pinchState) {
	if len(contacts) != 2 {
		return zoomNone, pinchState{}
	}
	a, b := contacts[0], contacts[1]
	if a.id > b.id {
		a, b = b, a
	}
	pair := [2]uint32{a.id, b.id}
	dist := math.Hypot(a.x-b.x, a.y-b.y)
	fresh := pinchState{pair: pair, baseline: dist, last: [2]contact{a, b}}
	if state.baseline == 0 || state.pair != pair || dist == 0 {
		return zoomNone, fresh
	}
	pa, pb := state.last[0], state.last[1]
	spread := math.Abs(dist - math.Hypot(pa.x-pb.x, pa.y-pb.y))
	travel := math.Hypot((a.x+b.x-pa.x-pb.x)/2, (a.y+b.y-pa.y-pb.y)/2)
	if travel > spread {
		return zoomNone, fresh
	}
	ratio := dist / state.baseline
	switch {
	case ratio >= stepRatio:
		return zoomIn, fresh
	case ratio <= 1/stepRatio:
		return zoomOut, fresh
	default:
		return zoomNone, pinchState{pair: pair, baseline: state.baseline, last: [2]contact{a, b}}
	}
}
