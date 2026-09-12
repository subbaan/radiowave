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
// redraw. Bass drives speed/size, mids distort shapes and treble adds detail.
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
	bass, mid, high := bands[0], bands[1], bands[2]
	energy := (bass + mid + high) / 3
	dt := 0.0
	if !a.effectLast.IsZero() {
		dt = math.Max(0, math.Min(0.2, now.Sub(a.effectLast).Seconds()))
	}
	a.effectLast = now
	speed := 0.4 + 2.5*bass
	if a.vizStyle == styleRibbons {
		speed = 0.4 + 5*bass + 1.2*high
	}
	a.effectPhase += dt * speed
	t := a.effectPhase
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
				v = (math.Sin(fx*5+t) + math.Sin(fy*6-t*1.3) + math.Sin((fx+fy)*(3+mid*3)+t*0.7) + 3) / 6
			case styleTunnel:
				r := math.Hypot(fx, fy)
				angle := math.Atan2(fy, fx)
				v = (1 + math.Sin(r*(18-6*bass)-t*7+math.Sin(angle*5+t)*mid)) / 2
				v *= 0.6 + 0.4*math.Pow((1+math.Cos(angle*8+t))/2, 2)
			case styleRibbons:
				for ribbon := 0; ribbon < 3; ribbon++ {
					// Keep the full-height sweep, with larger bends and a
					// separate frequency-band displacement for each ribbon.
					bend := 0.65 * high * math.Sin(fx*7-t)
					centre := math.Sin(fx*(3+mid*4) + t + float64(ribbon)*2.1 + bands[ribbon]*2.4 + bend)
					v = math.Max(v, 1-math.Abs(fy-centre)/(0.16+0.32*mid+0.12*high))
				}
			}
			grid[y][x] = int(clamp01(v*(0.25+0.75*energy)) * 7)
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
