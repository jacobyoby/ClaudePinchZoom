package main

import "testing"

func run(frames [][]contact) []zoomStep {
	state := pinchState{}
	steps := []zoomStep{}
	for _, f := range frames {
		var step zoomStep
		step, state = nextPinch(state, f)
		steps = append(steps, step)
	}
	return steps
}

func TestPinchSpreadAndSqueeze(t *testing.T) {
	frames := [][]contact{}
	for _, gap := range []float64{100, 110, 125, 150, 150, 120, 100} {
		frames = append(frames, []contact{{id: 3, x: 0, y: 0}, {id: 7, x: gap, y: 0}})
	}
	frames = append(frames, frames[0][:1])
	want := []zoomStep{zoomNone, zoomNone, zoomIn, zoomIn, zoomNone, zoomOut, zoomOut, zoomNone}
	got := run(frames)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("step %d = %d, want %d (all %v)", i, got[i], want[i], got)
		}
	}
}

func TestScrollWithDriftingSpacingDoesNotZoom(t *testing.T) {
	frames := [][]contact{}
	// Fingers slide down 20 units per frame while their spacing drifts from 100 to 160.
	for i := 0; i <= 20; i++ {
		y := float64(i * 20)
		gap := 100 + float64(i*3)
		frames = append(frames, []contact{{id: 1, x: 0, y: y}, {id: 2, x: gap, y: y}})
	}
	for i, step := range run(frames) {
		if step != zoomNone {
			t.Fatalf("frame %d zoomed (%d) during a two-finger scroll", i, step)
		}
	}
}
