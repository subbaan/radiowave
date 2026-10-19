package main

import (
	"math"
	"strings"
	"time"
)

type frequencyOrder int

const (
	frequencyCentered frequencyOrder = iota
	frequencyLinear
)

func (o frequencyOrder) String() string {
	if o == frequencyLinear {
		return "linear"
	}
	return "centered"
}

func (o frequencyOrder) Label() string {
	if o == frequencyLinear {
		return "Bass left → highs right"
	}
	return "Bass centre → highs outward"
}

func parseFrequencyOrder(s string) frequencyOrder {
	if s == "linear" {
		return frequencyLinear
	}
	return frequencyCentered
}

func (a *App) spectrumLevels(n int) []float64 {
	levels := a.visualizer.Levels(n)
	if a.vizOrder == frequencyCentered {
		return centerOutLevels(levels)
	}
	return levels
}

// Effects use a bounded character grid, theme colours and the existing 12.5 Hz
// redraw. Bass drives speed/size, mids distort shapes and treble adds detail;
// kicks flash Plasma and send a shockwave ring out of the Tunnel. Each band is
// scaled against its running peak, as the pixel styles' normalise does, so
// quiet stations still swing the whole range.
// The input remains low-to-high here: frequency order only affects spectra.
func (a *App) reactiveEffectRows(levels []float64, rows, cols int, now time.Time) []string {
	var bands [3]float64
	var counts [3]int
	for i, v := range levels {
		band := min(2, i*3/len(levels))
		bands[band] += clamp01(v)
		counts[band]++
	}
	for i := range bands {
		bands[i] /= float64(max(1, counts[i]))
	}
	dt := 0.0
	if !a.effectLast.IsZero() {
		dt = math.Max(0, math.Min(0.2, now.Sub(a.effectLast).Seconds()))
	}
	a.effectLast = now
	fall := math.Exp(-dt * 0.25)
	top := 0.0
	for i, v := range bands {
		a.effectPeak[i] = math.Max(v, a.effectPeak[i]*fall)
		top = math.Max(top, a.effectPeak[i])
	}
	for i, v := range bands {
		bands[i] = clamp01(v / math.Max(a.effectPeak[i], 0.4*top+0.02))
	}
	kick := 0.0
	if len(levels) > 0 {
		kick = bandMean(levels, 0, 1.0/16)
	}
	a.effectBeat.trackBeat(kick, dt)
	beat := a.effectBeat.beat
	bass, mid, high := bands[0], bands[1], bands[2]
	energy := (bass + mid + high) / 3
	speed := 0.3 + 2.2*bass*bass + 3*beat
	switch a.vizStyle {
	case styleRibbons:
		speed = 0.4 + 2.5*bass + 0.8*high + 1.5*beat
	case styleTunnel:
		speed = 0.3 + 1.5*bass*bass + 2*beat
	}
	a.effectPhase += dt * speed
	t := a.effectPhase
	// The Tunnel's kick ring travels out from the centre over about half a
	// second; beat = e^-6t gives the time since the kick.
	ring := 0.1 + 2.2*-math.Log(math.Max(beat, 1e-3))/6
	zoom := 1 - 0.3*bass*bass - 0.15*beat
	w, h := min(cols, 160), min(rows, 60)
	palette := []string{fgMuted, purple, cyanish, green, orange, magenta, red, fgText}
	glyphs := []rune(" .:-=+*#")
	grid := make([][]int, h)
	for y := 0; y < h; y++ {
		grid[y] = make([]int, w)
		fy := 2*float64(y)/float64(max(1, h-1)) - 1
		for x := 0; x < w; x++ {
			fx := 2*float64(x)/float64(max(1, w-1)) - 1
			var v float64
			switch a.vizStyle {
			case stylePlasma:
				// Bass zooms in, mids bend the diagonal wave, treble
				// ripples fine detail over the top.
				px, py := fx*zoom, fy*zoom
				v = (math.Sin(px*5+t) + math.Sin(py*6-t*1.3) + math.Sin((px+py)*(3+mid*5)+t*0.7) + 3) / 6
				v += 0.25 * high * math.Sin(px*19-t*3) * math.Sin(py*13+t*2)
				v += 0.35 * beat
			case styleTunnel:
				r := math.Hypot(fx, fy)
				angle := math.Atan2(fy, fx)
				// Bass widens the rings, mids twist them, treble sharpens
				// the spokes.
				v = (1 + math.Sin(r*(18-8*bass)-t*7+math.Sin(angle*5+t)*mid*2.5)) / 2
				v *= 1 - 0.7*high + 0.7*high*math.Pow((1+math.Cos(angle*8+t))/2, 2)
				v *= 0.5 + 0.8*r
				if d := (r - ring) / 0.15; beat > 0.05 {
					v += 1.2 * math.Sqrt(beat) * math.Exp(-d*d)
				}
			case styleRibbons:
				for ribbon := 0; ribbon < 3; ribbon++ {
					// Keep the full-height sweep, with larger bends and a
					// separate frequency-band displacement for each ribbon.
					bend := 0.65 * high * math.Sin(fx*7-t)
					centre := math.Sin(fx*(3+mid*4) + t + float64(ribbon)*2.1 + bands[ribbon]*2.4 + bend)
					v = math.Max(v, 1-math.Abs(fy-centre)/(0.16+0.32*mid+0.12*high))
				}
			}
			grid[y][x] = int(clamp01(v*(0.2+0.8*energy)) * 7)
		}
	}
	out := make([]string, rows)
	for y := range out {
		var b strings.Builder
		last := -1
		for x := 0; x < cols; x++ {
			c := grid[y*h/rows][x*w/cols]
			if c != last {
				b.WriteString(palette[c])
				last = c
			}
			b.WriteRune(glyphs[c])
		}
		b.WriteString(reset)
		out[y] = b.String()
	}
	return out
}
