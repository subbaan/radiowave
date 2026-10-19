package main

import (
	"strings"
	"time"
)

// vizStyle selects how the active visualizer's Levels() data is drawn. This
// is deliberately independent of Visualizer (the data source): the same
// cava/demo levels can be presented as classic bars, mirrored, VU meters,
// and so on. v/V cycle this, not the data source — the data source (cava
// vs. the demo fallback) is chosen automatically at startup.
type vizStyle int

const (
	styleClassic vizStyle = iota
	styleMirrored
	styleBlocks
	stylePeaks
	styleVU
	styleOscilloscope
	styleWaterfall
	stylePlasma
	styleTunnel
	styleRibbons
	styleNeon
	styleNebula
	styleSpectrogram
	styleFire
	styleWarp
	styleWormhole
	styleCopper
	styleRidgeline
	styleHorizon
	styleLava
	styleLED
	vizStyleCount // sentinel — keep last
)

// pixel reports whether the style is drawn as a kitty graphics image rather
// than character cells. Pixel styles are skipped when graphics are off; they
// are kept together from styleNeon to the end of the list.
func (s vizStyle) pixel() bool { return s >= styleNeon && s < vizStyleCount }

var vizStyleLabels = [vizStyleCount]string{
	styleClassic:      "Classic",
	styleMirrored:     "Mirrored",
	styleBlocks:       "Blocks",
	stylePeaks:        "Peaks",
	styleVU:           "VU",
	styleOscilloscope: "Oscilloscope",
	styleWaterfall:    "Waterfall",
	stylePlasma:       "Plasma",
	styleTunnel:       "Tunnel",
	styleRibbons:      "Ribbons",
	styleNeon:         "Neon",
	styleNebula:       "Nebula",
	styleSpectrogram:  "Spectrogram",
	styleFire:         "Fire",
	styleWarp:         "Warp",
	styleWormhole:     "Wormhole",
	styleCopper:       "Copper",
	styleRidgeline:    "Ridgeline",
	styleHorizon:      "Horizon",
	styleLava:         "Lava",
	styleLED:          "LED",
}

func (s vizStyle) Label() string {
	if s < 0 || s >= vizStyleCount {
		s = styleClassic
	}
	return vizStyleLabels[s]
}

func (s vizStyle) String() string { return strings.ToLower(s.Label()) }

func parseVizStyle(s string) vizStyle {
	for i, l := range vizStyleLabels {
		if strings.EqualFold(l, s) {
			return vizStyle(i)
		}
	}
	return styleClassic
}

// renderVisualizer produces `rows` terminal lines of width `cols` for the
// active style, pulling live levels from the active data source. Peak-hold
// and waterfall history live on App (not as locals here) so they stay
// continuous across layout switches instead of resetting every frame.
func (a *App) renderVisualizer(rows, cols int) []string {
	if rows < 1 {
		rows = 1
	}
	cols = max(1, cols)
	switch a.activeVizStyle() {
	case stylePlasma, styleTunnel, styleRibbons:
		return a.reactiveEffectRows(a.visualizer.Levels(48), rows, cols, time.Now())
	case styleVU:
		levels := a.spectrumLevels(vuMeterCount(cols))
		return vuMeterRows(levels, rows, cols)
	case styleWaterfall:
		levels := a.spectrumLevels(max(12, cols-2))
		a.pushWaterfallFrame(levels, rows)
		return waterfallRows(a.vizHistory, rows, cols)
	case stylePeaks:
		levels := a.spectrumLevels(max(1, min(96, cols/2)))
		a.trackPeaks(levels)
		return peakSpectrumRows(levels, a.vizPeaks, rows, cols)
	case styleMirrored:
		levels := a.spectrumLevels(max(1, min(96, cols/2)))
		top := verticalSpectrum(levels, (rows+1)/2, cols)
		out := append([]string(nil), top...)
		for y := rows/2 - 1; y >= 0; y-- {
			out = append(out, top[y])
		}
		return out
	case styleBlocks:
		levels := a.spectrumLevels(max(1, min(96, cols/2)))
		return verticalSpectrum(chunkyLevels(levels, 3), rows, cols)
	case styleOscilloscope:
		levels := a.spectrumLevels(cols)
		return oscilloscopeRows(levels, rows, cols)
	default:
		levels := a.spectrumLevels(max(1, min(96, cols/2)))
		return verticalSpectrum(levels, rows, cols)
	}
}

func (a *App) trackPeaks(levels []float64) {
	if len(a.vizPeaks) != len(levels) {
		a.vizPeaks = append([]float64(nil), levels...)
		return
	}
	const decay = 0.025
	for i, v := range levels {
		a.vizPeaks[i] -= decay
		if a.vizPeaks[i] < v {
			a.vizPeaks[i] = v
		}
		if a.vizPeaks[i] < 0 {
			a.vizPeaks[i] = 0
		}
	}
}

func (a *App) pushWaterfallFrame(levels []float64, maxRows int) {
	frame := append([]float64(nil), levels...)
	a.vizHistory = append([][]float64{frame}, a.vizHistory...)
	if len(a.vizHistory) > maxRows {
		a.vizHistory = a.vizHistory[:maxRows]
	}
}

// centerOutLevels reorders a left-to-right, low-to-high spectrum so the
// lowest frequencies sit in the middle and expand outward — the same
// arrangement cava's own "stereo" output mode uses, replicated here as a
// pure display transform so it works for any data source, demo included.
func centerOutLevels(levels []float64) []float64 {
	n := len(levels)
	out := make([]float64, n)
	left, right := n/2-1, n/2
	for i := 0; i < n; i++ {
		v := levels[i]
		if i%2 == 0 {
			if right >= n {
				break
			}
			out[right] = v
			right++
		} else {
			if left < 0 {
				break
			}
			out[left] = v
			left--
		}
	}
	return out
}

// chunkyLevels flattens every `group` adjacent bars to their shared max, so
// verticalSpectrum draws wide, blocky columns instead of thin ones.
func chunkyLevels(levels []float64, group int) []float64 {
	if group < 2 || len(levels) == 0 {
		return levels
	}
	out := make([]float64, len(levels))
	for i := 0; i < len(levels); i += group {
		end := i + group
		if end > len(levels) {
			end = len(levels)
		}
		m := 0.0
		for _, v := range levels[i:end] {
			if v > m {
				m = v
			}
		}
		for j := i; j < end; j++ {
			out[j] = m
		}
	}
	return out
}

// peakSpectrumRows draws the classic bar spectrum plus a slowly-decaying
// peak marker above each column, like a hardware spectrum analyser.
func peakSpectrumRows(levels, peaks []float64, height, width int) []string {
	if height < 1 {
		return nil
	}
	if len(levels) < 1 {
		return make([]string, height)
	}
	maxBars := width / 2
	if maxBars < 1 {
		maxBars = 1
	}
	if len(levels) > maxBars {
		levels = levels[:maxBars]
	}
	colors := []string{magenta, green, orange, red, purple}
	rows := make([]string, height)
	for row := 0; row < height; row++ {
		threshold := float64(height-row) / float64(height)
		var b strings.Builder
		for i, v := range levels {
			peak := 0.0
			if i < len(peaks) {
				peak = peaks[i]
			}
			peakRow := int((1 - peak) * float64(height))
			switch {
			case v >= threshold:
				b.WriteString(colors[(i/7)%len(colors)])
				b.WriteRune('█')
			case row == peakRow:
				b.WriteString(fgText)
				b.WriteRune('‾')
			default:
				b.WriteRune(' ')
			}
			b.WriteString(reset)
			b.WriteRune(' ')
		}
		rows[row] = b.String()
	}
	return rows
}

func vuMeterCount(cols int) int {
	n := cols / 5
	if n < 2 {
		n = 2
	}
	if n > 12 {
		n = 12
	}
	return n
}

// vuMeterRows draws a handful of large, chunky level meters with fixed
// green/amber/red zones by row (like a real VU meter's markings), rather
// than colouring by the live value the way the spectrum styles do. Meter
// width stretches to fill `width` — a wider terminal gets wider meters,
// not more of them, keeping the classic few-big-meters look at any size.
func vuMeterRows(levels []float64, height, width int) []string {
	if height < 1 {
		return nil
	}
	n := len(levels)
	if n == 0 {
		return make([]string, height)
	}
	const gap = 1
	barWidth := (width - n*gap) / n
	if barWidth < 1 {
		barWidth = 1
	}
	rows := make([]string, height)
	for row := 0; row < height; row++ {
		frac := float64(height-row) / float64(height)
		color := green
		switch {
		case frac > 0.85:
			color = red
		case frac > 0.6:
			color = orange
		}
		var b strings.Builder
		for _, v := range levels {
			lit := v >= frac
			for x := 0; x < barWidth; x++ {
				if lit {
					b.WriteString(color)
					b.WriteRune('█')
				} else {
					b.WriteRune(' ')
				}
			}
			b.WriteString(reset)
			for g := 0; g < gap; g++ {
				b.WriteRune(' ')
			}
		}
		rows[row] = b.String()
	}
	return rows
}

// oscilloscopeRows plots levels as a connected line trace rather than
// bottom-anchored bars — a creative liberty since the data is frequency
// magnitude, not a raw waveform, but it reads as scope-like motion.
func oscilloscopeRows(levels []float64, height, width int) []string {
	if height < 1 {
		return nil
	}
	// One visible character per data point (no trailing gap column below),
	// so the rendered width matches `width` exactly instead of doubling it.
	if width > 0 && len(levels) > width {
		levels = levels[:width]
	}
	if len(levels) == 0 {
		return make([]string, height)
	}
	rowFor := func(v float64) int {
		r := int((1 - v) * float64(height-1))
		if r < 0 {
			r = 0
		}
		if r >= height {
			r = height - 1
		}
		return r
	}
	lit := make([][]bool, height)
	for i := range lit {
		lit[i] = make([]bool, len(levels))
	}
	prev := rowFor(levels[0])
	lit[prev][0] = true
	for i := 1; i < len(levels); i++ {
		r := rowFor(levels[i])
		lo, hi := r, prev
		if lo > hi {
			lo, hi = hi, lo
		}
		for y := lo; y <= hi; y++ {
			lit[y][i] = true
		}
		prev = r
	}
	colors := []string{cyanish, green, magenta}
	rows := make([]string, height)
	for y := 0; y < height; y++ {
		var b strings.Builder
		for x, on := range lit[y] {
			if on {
				b.WriteString(colors[(x/9)%len(colors)])
				b.WriteRune('●')
			} else {
				b.WriteRune(' ')
			}
		}
		b.WriteString(reset)
		rows[y] = b.String()
	}
	return rows
}

// waterfallRows renders scrolling history as a density-glyph spectrogram —
// newest frame on top, each older frame shifted down one row.
func waterfallRows(history [][]float64, height, width int) []string {
	rows := make([]string, height)
	glyphs := []rune(" .:-=+*#%@")
	colors := []string{fgMuted, cyanish, green, orange, magenta, red}
	if width < 1 {
		width = 1
	}
	for y := 0; y < height; y++ {
		if y >= len(history) || len(history[y]) == 0 {
			rows[y] = ""
			continue
		}
		frame := history[y]
		var b strings.Builder
		for x := 0; x < width; x++ {
			idx := x * len(frame) / width
			if idx >= len(frame) {
				idx = len(frame) - 1
			}
			v := clamp01(frame[idx])
			gi := int(v * float64(len(glyphs)-1))
			ci := int(v * float64(len(colors)-1))
			b.WriteString(colors[ci])
			b.WriteRune(glyphs[gi])
		}
		b.WriteString(reset)
		rows[y] = b.String()
	}
	return rows
}
