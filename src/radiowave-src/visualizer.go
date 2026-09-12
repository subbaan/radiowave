package main

import (
	"math"
	"time"
)

// Visualizer is intentionally minimal so a real audio-analysis backend (e.g.
// CAVA raw output) can be dropped in later without touching renderer code.
// Start/Stop exist now so the lifecycle contract is settled before that lands.
type Visualizer interface {
	Start() error
	Stop()
	Levels(n int) []float64
	Name() string
	// Demo reports whether Levels() is synthetic rather than real audio
	// analysis. The renderer must never present demo data as if it were
	// real, so every implementation has to answer this explicitly.
	Demo() bool
}

// DemoVisualizer deliberately does not pretend to analyse audio. It gives the
// renderer a stable visualizer contract until a CAVA raw-output backend lands.
// Its Name() is always shown next to a "demo" label in the UI.
type DemoVisualizer struct {
	label   string
	seed    float64
	started time.Time
}

func NewDemoVisualizer() *DemoVisualizer {
	return &DemoVisualizer{label: "sine sweep", seed: 1.0, started: time.Now()}
}

// NewPulseDemoVisualizer is a second demo waveform so visualizer cycling
// (v/V) has something to cycle through before real backends exist.
func NewPulseDemoVisualizer() *DemoVisualizer {
	return &DemoVisualizer{label: "pulse bands", seed: 2.3, started: time.Now()}
}

func (v *DemoVisualizer) Start() error { v.started = time.Now(); return nil }
func (v *DemoVisualizer) Stop()        {}
func (v *DemoVisualizer) Name() string { return v.label }
func (v *DemoVisualizer) Demo() bool   { return true }

func (v *DemoVisualizer) Levels(n int) []float64 {
	if n < 1 {
		return nil
	}
	t := time.Since(v.started).Seconds()
	out := make([]float64, n)
	for i := range out {
		x := float64(i) / float64(n)
		a := 0.48 + 0.28*math.Sin(v.seed*t*2.2+x*15.0)
		b := 0.16 * math.Sin(v.seed*t*3.7+x*31.0)
		c := 0.08 * math.Sin(v.seed*t*1.1+x*7.0)
		out[i] = clamp01(a + b + c)
	}
	return out
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
