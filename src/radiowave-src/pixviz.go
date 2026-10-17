package main

import (
	"math"
	"math/rand/v2"
	"os"
	"strings"
	"time"
)

// vizRect is where the cell renderer reserved the visualizer, in 1-based
// terminal cells. A zero rect means "no visualizer on screen".
type vizRect struct{ top, left, rows, cols int }

type pixShape struct {
	rect         vizRect
	cellW, cellH int
}

// pixCanvasBudget caps canvas pixels per frame (~1800x900 screen pixels
// at 2x). Frame cost is linear in it.
const pixCanvasBudget = 900 * 450

func (r vizRect) valid() bool { return r.rows > 0 && r.cols > 0 }

// pixState carries everything the pixel styles keep between frames. The
// styles are chosen to stay lean at 60 fps on a half-resolution canvas:
// Neon touches only the pixels its bars cover plus one max-blend pass,
// Nebula is one bilinear warp pass plus a few hundred soft dots, and
// Spectrogram is a memmove plus a column or two of new pixels.
type pixState struct {
	cv, back pixCanvas
	style    vizStyle
	last     time.Time

	levels, peaks, peakVel []float64

	bassAvg, beat, beatWait float64
	spin, ripple            float64

	sparks []spark

	column []float64 // Spectrogram: the previous column's levels
	scroll float64   // Spectrogram: fractional columns not yet scrolled
}

// spark is one beat-burst particle, in canvas pixels.
type spark struct {
	x, y, vx, vy, life, maxLife float32
	col                         rgb
}

// maxSparks bounds the particle cost however dense the beats get.
const maxSparks = 600

func (p *pixState) reset() {
	p.cv.clear()
	p.back.clear()
	p.levels, p.peaks, p.peakVel = nil, nil, nil
	p.beat, p.beatWait = 0, 0
	p.sparks = p.sparks[:0]
	p.column, p.scroll = nil, 0
}

// smoothLevels eases p.levels towards target: fast attack, slower release,
// independent of frame rate.
func (p *pixState) smoothLevels(target []float64, dt float64) {
	if len(p.levels) != len(target) {
		p.levels = append([]float64(nil), target...)
		p.peaks = make([]float64, len(target))
		p.peakVel = make([]float64, len(target))
		return
	}
	up, down := 1-math.Exp(-dt*35), 1-math.Exp(-dt*9)
	for i, t := range target {
		k := down
		if t > p.levels[i] {
			k = up
		}
		p.levels[i] += (t - p.levels[i]) * k
	}
}

// trackBeat flags a beat when bass jumps well above its running average,
// and reports whether one started this frame.
func (p *pixState) trackBeat(bass, dt float64) bool {
	p.bassAvg += (bass - p.bassAvg) * (1 - math.Exp(-dt*2))
	p.beat *= math.Exp(-dt * 6)
	p.beatWait -= dt
	if p.beatWait <= 0 && bass > p.bassAvg*1.3+0.06 {
		p.beat = 1
		p.beatWait = 0.18
		return true
	}
	return false
}

func (p *pixState) addSpark(s spark) {
	if len(p.sparks) < maxSparks {
		s.maxLife = s.life
		p.sparks = append(p.sparks, s)
	}
}

// stepSparks moves, fades and draws the particles into c. Sparks bounce
// off floor (canvas y) with most of their energy lost.
func (p *pixState) stepSparks(c *pixCanvas, dt, gravity, floor float32) {
	drag := float32(math.Exp(-float64(dt) * 0.8))
	live := p.sparks[:0]
	for _, s := range p.sparks {
		s.life -= dt
		if s.life <= 0 {
			continue
		}
		s.vy += gravity * dt
		s.vx *= drag
		s.vy *= drag
		s.x += s.vx * dt
		s.y += s.vy * dt
		if s.y > floor && s.vy > 0 {
			s.y, s.vy, s.vx = floor, -s.vy*0.3, s.vx*0.7
		}
		if s.x < -4 || s.x > float32(c.w)+4 || s.y < -4 || s.y > float32(c.h)+4 {
			continue
		}
		c.splat(s.x, s.y, 1.3, s.col.scale(s.life/s.maxLife))
		live = append(live, s)
	}
	p.sparks = live
}

func bandMean(levels []float64, from, to float64) float64 {
	i0, i1 := int(from*float64(len(levels))), int(to*float64(len(levels)))
	i1 = max(i0+1, min(len(levels), i1))
	var s float64
	for _, v := range levels[i0:i1] {
		s += v
	}
	return s / float64(i1-i0)
}

func (p *pixState) render(a *App, w, h int, now time.Time) {
	dt := 1.0 / 60
	if !p.last.IsZero() {
		dt = math.Max(0.001, math.Min(0.1, now.Sub(p.last).Seconds()))
	}
	p.last = now
	style := a.activeVizStyle()
	resized := p.cv.resize(w, h)
	p.back.resize(w, h)
	if resized || style != p.style {
		p.style = style
		p.reset()
	}
	switch style {
	case styleNebula:
		p.renderNebula(a, dt)
	case styleSpectrogram:
		p.renderSpectrogram(a, dt)
	default:
		p.renderNeon(a, dt)
	}
}

// --- Neon -------------------------------------------------------------

// renderNeon draws gradient bars with a soft side glow, a hot tip, falling
// peak caps and a rippling floor reflection, and spits sparks from the
// tallest bars on each beat. The frame is built additively in the scratch
// canvas, then max-blended over the decaying previous frame: falling bars
// and sparks leave a short phosphor trail without additive blow-out.
func (p *pixState) renderNeon(a *App, dt float64) {
	cv, fr := &p.cv, &p.back
	w, h := cv.w, cv.h
	n := max(8, min(128, w/6))
	p.smoothLevels(a.spectrumLevels(n), dt)
	bass := bandMean(a.visualizer.Levels(32), 0, 0.15)
	hit := p.trackBeat(bass, dt)

	pal := activePalette
	stops := []rgb{hexRGB(pal.Info), hexRGB(pal.Secondary), hexRGB(pal.Accent)}
	capCol := hexRGB(pal.Foreground)
	floor := hexRGB(pal.Border)

	base := h * 4 / 5
	maxH := float64(max(1, base-3))
	pitch := float64(w) / float64(n)
	barW := max(1, int(pitch*0.6))
	lift := float32(0.85 + 0.25*p.beat)

	// Ripples: each reflection row is shifted sideways by a travelling wave
	// whose amplitude grows with depth and with the bass, and shimmers.
	depth := float32(h - base)
	p.ripple += dt * (4 + 6*bass)
	amp := 0.6 + 3*bass + 3*p.beat
	rowOff := make([]int, h-base)
	shimmer := make([]float32, h-base)
	for d := range rowOff {
		k := float64(d) / float64(len(rowOff))
		rowOff[d] = int(math.Round(amp * k * math.Sin(float64(d)*0.9-p.ripple)))
		shimmer[d] = float32(0.75 + 0.25*math.Sin(float64(d)*0.45-p.ripple*1.3))
	}

	tallest := 0.0
	for _, lvl := range p.levels {
		tallest = math.Max(tallest, lvl)
	}

	fr.clear()
	for i, lvl := range p.levels {
		lvl = clamp01(lvl)
		// Peaks hold at the top and fall under gravity.
		if lvl >= p.peaks[i] {
			p.peaks[i], p.peakVel[i] = lvl, 0
		} else {
			p.peakVel[i] += dt * 2.2
			p.peaks[i] = math.Max(lvl, p.peaks[i]-p.peakVel[i]*dt)
		}
		x0 := int(float64(i)*pitch + (pitch-float64(barW))/2)
		x1 := x0 + barW
		top := base - int(lvl*maxH)
		for y := top; y < base; y++ {
			t := float32(base-y) / float32(maxH)
			col := gradient(stops, t)
			fr.hspan(x0, x1, y, col.scale(lift))
			fr.hspan(x0-1, x0, y, col.scale(0.22))
			fr.hspan(x1, x1+1, y, col.scale(0.22))
			fr.hspan(x0-2, x0-1, y, col.scale(0.07))
			fr.hspan(x1+1, x1+2, y, col.scale(0.07))
		}
		if lvl > 0.02 {
			tip := gradient(stops, float32(lvl))
			fr.splat(float32(x0+x1)/2, float32(top), float32(barW)*0.9, tip.scale(float32(0.5*lvl)))
			if hit && lvl > 0.15 && lvl > tallest*0.6 {
				for k := 0; k < 3; k++ {
					p.addSpark(spark{
						x:    float32(x0) + rand.Float32()*float32(barW),
						y:    float32(top),
						vx:   (rand.Float32() - 0.5) * float32(h) * 0.6,
						vy:   -float32(h) * (0.2 + (0.4+0.6*rand.Float32())*float32(lvl)),
						life: 0.5 + 0.7*rand.Float32(),
						col:  tip.scale(1.4),
					})
				}
			}
		}
		// Reflection: a third of the bar, fading towards the bottom edge.
		for y := base + 1; y < h && y-base < int(lvl*maxH/3)+1; y++ {
			d := y - base
			f := 1 - float32(d)/depth
			col := gradient(stops, float32(d)/float32(maxH))
			fr.hspan(x0+rowOff[d], x1+rowOff[d], y, col.scale(0.2*f*f*shimmer[d]))
		}
		py := base - 2 - int(p.peaks[i]*maxH)
		fr.hspan(x0, x1, py, capCol)
		fr.hspan(x0, x1, py-1, capCol.scale(0.25))
		fr.hspan(x0, x1, py+1, capCol.scale(0.25))
	}
	fr.hspan(0, w, base, floor.scale(0.5))
	p.stepSparks(fr, float32(dt), float32(h)*1.6, float32(base))

	keep := float32(math.Exp(-dt * 12))
	for i, v := range fr.pix {
		old := cv.pix[i] * keep
		if v > old {
			old = v
		}
		cv.pix[i] = old
	}
}

// --- Nebula -----------------------------------------------------------

// renderNebula is a Milkdrop-style feedback loop: the previous frame is
// zoomed outwards, rotated a little and decayed, then a spectrum ring is
// drawn on top. The ring's trail becomes the flowing nebula. Bass speeds the
// zoom and swells the ring, mids drive the swirl, beats flash the core.
func (p *pixState) renderNebula(a *App, dt float64) {
	raw := a.visualizer.Levels(48)
	p.smoothLevels(raw, dt)
	bass := bandMean(p.levels, 0, 0.15)
	mid := bandMean(p.levels, 0.15, 0.5)
	hit := p.trackBeat(bandMean(raw, 0, 0.15), dt)
	p.spin += dt * (0.12 + 0.6*mid)

	zoom := float32(1 - dt*(0.35+1.6*bass))
	rot := float32(dt * (0.08 + 0.5*mid) * math.Sin(p.spin*0.7))
	decay := float32(math.Exp(-dt * 2.8))
	p.back.warpFrom(&p.cv, zoom, rot, decay)
	p.cv, p.back = p.back, p.cv
	cv := &p.cv

	pal := activePalette
	stops := []rgb{hexRGB(pal.Info), hexRGB(pal.Secondary), hexRGB(pal.Accent), hexRGB(pal.Warning)}
	cx, cy := float64(cv.w)/2, float64(cv.h)/2
	span := float64(min(cv.w, cv.h))
	baseR := span * (0.16 + 0.05*p.beat)
	amp := span * 0.26

	// Mirror the band index around the ring so it is symmetric, bass at
	// the top and treble meeting at the bottom.
	m := len(p.levels)
	pts := 2 * m
	point := func(k int) (float32, float32, int) {
		j := k % pts
		if j >= m {
			j = pts - 1 - j
		}
		ang := -math.Pi/2 + 2*math.Pi*float64(k)/float64(pts) + p.spin*0.3
		r := baseR + clamp01(p.levels[j])*amp
		return float32(cx + r*math.Cos(ang)), float32(cy + r*math.Sin(ang)), j
	}
	px, py, _ := point(0)
	for k := 1; k <= pts; k++ {
		x, y, j := point(k)
		col := gradient(stops, float32(j)/float32(m-1))
		cv.line(px, py, x, y, 1.4, col.scale(0.32))
		px, py = x, y
	}
	if p.beat > 0.05 {
		core := hexRGB(pal.Accent).scale(float32(0.35 * p.beat))
		cv.splat(float32(cx), float32(cy), float32(baseR*0.8), core)
	}
	// Beat sparks fly outwards from the ring; the feedback warp smears
	// them into comet streaks on the following frames.
	if hit {
		for i := 0; i < 48; i++ {
			k := rand.IntN(pts)
			x, y, j := point(k)
			dx, dy := float64(x)-cx, float64(y)-cy
			d := math.Max(1, math.Hypot(dx, dy))
			speed := span * (0.35 + 0.8*rand.Float64())
			p.addSpark(spark{
				x: x, y: y,
				vx:   float32(dx / d * speed),
				vy:   float32(dy / d * speed),
				life: 0.5 + 0.8*rand.Float32(),
				col:  gradient(stops, float32(j)/float32(m-1)).scale(0.9),
			})
		}
	}
	p.stepSparks(cv, float32(dt), 0, float32(cv.h)*2)
}

// --- Spectrogram ------------------------------------------------------

// renderSpectrogram scrolls the canvas left and paints the newest columns
// at the right edge: time runs right to left, bass at the bottom, loudness
// mapped through the palette with quiet bins left transparent. New columns
// interpolate from the previous levels so the scroll stays smooth at any
// frame rate. Cost is one memmove of the canvas plus h pixels per column.
func (p *pixState) renderSpectrogram(a *App, dt float64) {
	cv := &p.cv
	w, h := cv.w, cv.h
	m := max(2, min(cavaBars, h))
	raw := a.visualizer.Levels(m)
	if len(p.column) != m {
		p.column = append([]float64(nil), raw...)
	}
	// About eight seconds of history across the panel.
	p.scroll += dt * float64(w) / 8
	n := min(w, int(p.scroll))
	if n == 0 {
		return
	}
	p.scroll -= float64(n)
	stride := w * 3
	for y := 0; y < h; y++ {
		row := cv.pix[y*stride : y*stride+stride]
		copy(row, row[n*3:])
	}

	pal := activePalette
	stops := []rgb{hexRGB(pal.Secondary), hexRGB(pal.Accent), hexRGB(pal.Warning), hexRGB(pal.Foreground)}
	for c := 0; c < n; c++ {
		t := float64(c+1) / float64(n)
		x := w - n + c
		for y := 0; y < h; y++ {
			fi := float64(h-1-y) / float64(max(1, h-1)) * float64(m-1)
			i := int(fi)
			f := fi - float64(i)
			j := min(m-1, i+1)
			cur := raw[i]*(1-f) + raw[j]*f
			prev := p.column[i]*(1-f) + p.column[j]*f
			v := clamp01(prev + (cur-prev)*t)
			col := gradient(stops, float32(v)).scale(float32(1.2 * math.Pow(v, 1.4)))
			k := (y*w + x) * 3
			cv.pix[k], cv.pix[k+1], cv.pix[k+2] = col.r, col.g, col.b
		}
	}
	copy(p.column, raw)
}

// --- App glue ---------------------------------------------------------

// initGfx probes the terminal and enables pixel styles when it can. tmux
// is skipped unless a transport is forced: it needs passthrough setup and
// cannot place images by cell without it.
func (a *App) initGfx(want string) {
	if want == "off" || (want == "auto" && os.Getenv("TMUX") != "") {
		return
	}
	a.gfxProbe = probeGraphics()
	if tx := chooseTransport(want, a.gfxProbe); tx != gfxOff {
		a.gfx = &kittyGfx{tx: tx}
		a.updateCellSize()
	}
}

func (a *App) closeGfx() {
	if a.gfx == nil {
		return
	}
	os.Stdout.WriteString(a.gfx.hide())
	a.gfx.cleanup()
}

func (a *App) updateCellSize() {
	if a.gfx != nil {
		a.cellW, a.cellH = cellPixels(a.width, a.height, a.gfxProbe)
	}
}

func (a *App) vizStyleAvailable(s vizStyle) bool { return a.gfx != nil || !s.pixel() }

// activeVizStyle is the style actually drawn: a remembered pixel style
// falls back to Classic in a terminal without graphics.
func (a *App) activeVizStyle() vizStyle {
	if !a.vizStyleAvailable(a.vizStyle) {
		return styleClassic
	}
	return a.vizStyle
}

func (a *App) pixelStyleActive() bool { return a.gfx != nil && a.activeVizStyle().pixel() }

// draw paints the whole UI. With graphics, the full-screen clear is
// swapped for whole-line erases: kitty drops images on ED 2, and keeping the
// image alive means a cell redraw only needs a new frame when the
// rectangle moves. Cells and image go out in one synchronized update.
func (a *App) draw() {
	if a.gfx == nil {
		os.Stdout.WriteString(renderApp(a))
		return
	}
	// Erase each whole line before drawing it, never after: with autowrap
	// off, an erase after a full-width line would eat its last cell.
	frame := strings.Replace(renderApp(a), "\x1b[2J", "\x1b[2K", 1)
	frame = strings.ReplaceAll(frame, "\n", "\n\x1b[2K")
	img := ""
	switch {
	case !a.pixRect.valid():
		img = a.gfx.hide()
	case !a.gfx.placed || a.gfx.shown != a.pixShape():
		img = a.pixelFrame(time.Now())
	}
	os.Stdout.WriteString("\x1b[?2026h" + frame + img + "\x1b[?2026l")
}

// pixShape identifies what the current image must cover; a change means
// the image on screen is stale.
func (a *App) pixShape() pixShape {
	return pixShape{rect: a.pixRect, cellW: a.cellW, cellH: a.cellH}
}

// drawPixels advances only the image, between cell redraws.
func (a *App) drawPixels() {
	if a.gfx == nil || !a.pixRect.valid() || a.page != pagePlayer {
		return
	}
	if esc := a.pixelFrame(time.Now()); esc != "" {
		os.Stdout.WriteString("\x1b7\x1b[?2026h" + esc + "\x1b[?2026l\x1b8")
	}
}

// pixelFrame renders the active pixel style into a.pixRect and returns the
// escapes presenting it, or removing a stale image when none is wanted.
func (a *App) pixelFrame(now time.Time) string {
	r := a.pixRect
	if !r.valid() || a.cellW <= 0 || a.cellH <= 0 {
		return a.gfx.hide()
	}
	outW, outH := r.cols*a.cellW, r.rows*a.cellH
	s := max(1, a.pixScale)
	// Bound the per-frame cost: on very large panels, use bigger pixels
	// rather than letting the canvas (and CPU time) grow without limit.
	for s < 8 && (outW/s)*(outH/s) > pixCanvasBudget {
		s++
	}
	a.pix.render(a, (outW+s-1)/s, (outH+s-1)/s, now)
	esc, err := a.gfx.frame(&a.pix.cv, s, outW, outH, r)
	if err != nil && a.gfx.tx == gfxSHM && a.gfxProbe.direct {
		// e.g. /dev/shm unavailable: keep going the slow way.
		a.gfx.tx = gfxDirect
		esc, err = a.gfx.frame(&a.pix.cv, s, outW, outH, r)
	}
	if err == nil {
		a.gfx.shown = a.pixShape()
	}
	return esc
}
