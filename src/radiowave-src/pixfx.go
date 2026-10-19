package main

import (
	"math"
	"math/rand/v2"
	"slices"
)

// Pixel styles borrowed from the demoscene and from Winamp-era plugins.
// Each keeps to one cheap pass over the canvas at most: Fire and Wormhole
// are per-pixel lookups, Lava is the only one that sums a few terms per
// pixel, and the rest touch just the pixels they draw.

// star is one Warp particle in camera space: x and y across the view, z the
// distance (1 at the far plane, 0 at the camera). Horizon's sky reuses it
// with x and y as fractions of the sky and z unused.
type star struct {
	x, y, z float32
	band    int
}

// updatePeaks lets each peak cap hold at its bar's top, then fall under
// gravity.
func (p *pixState) updatePeaks(dt float64) {
	for i, lvl := range p.levels {
		lvl = clamp01(lvl)
		if lvl >= p.peaks[i] {
			p.peaks[i], p.peakVel[i] = lvl, 0
		} else {
			p.peakVel[i] += dt * 2.2
			p.peaks[i] = math.Max(lvl, p.peaks[i]-p.peakVel[i]*dt)
		}
	}
}

// normalise fills p.norm with p.levels scaled against each band's running
// peak (instant attack, about a four-second fall), so quiet material such
// as speech still swings across the whole range. A band's peak is floored at
// a share of the loudest band's, so a band that is nearly silent is lifted
// a few times at most rather than amplifying noise to full scale.
//
// It also fills p.pulse, which stretches each band between its recent floor
// (instant fall, about a two-second rise) and its peak. Dense music keeps
// p.norm between about 0.75 and 1, so p.pulse is what shows each hit; its
// range is floored at a third of the peak so a steady band doesn't
// magnify its own jitter.
func (p *pixState) normalise(dt float64) {
	n := len(p.levels)
	if len(p.bandPeak) != n {
		p.bandPeak, p.norm = make([]float64, n), make([]float64, n)
		p.bandLow, p.pulse = slices.Clone(p.levels), make([]float64, n)
	}
	fall := math.Exp(-dt * 0.25)
	rise := 1 - math.Exp(-dt*0.5)
	p.top = 0
	for i, v := range p.levels {
		p.bandPeak[i] = math.Max(v, p.bandPeak[i]*fall)
		p.bandLow[i] = math.Min(v, p.bandLow[i]+(v-p.bandLow[i])*rise)
		p.top = math.Max(p.top, p.bandPeak[i])
	}
	floor := 0.4*p.top + 0.02
	for i, v := range p.levels {
		peak := math.Max(p.bandPeak[i], floor)
		p.norm[i] = clamp01(v / peak)
		p.pulse[i] = clamp01((v - p.bandLow[i]) / math.Max(peak-p.bandLow[i], peak/3))
	}
}

// levelAt, normAt and pulseAt sample p.levels, p.norm and p.pulse at t
// in [0,1].
func (p *pixState) levelAt(t float64) float64 { return sampleAt(p.levels, t) }
func (p *pixState) normAt(t float64) float64  { return sampleAt(p.norm, t) }
func (p *pixState) pulseAt(t float64) float64 { return sampleAt(p.pulse, t) }

// sampleAt samples v at t in [0,1] with linear interpolation.
func sampleAt(v []float64, t float64) float64 {
	n := len(v)
	if n == 0 {
		return 0
	}
	f := math.Max(0, math.Min(float64(n-1), t*float64(n-1)))
	i := int(f)
	j := min(n-1, i+1)
	return clamp01(v[i] + (v[j]-v[i])*(f-float64(i)))
}

// --- Fire -------------------------------------------------------------

// renderFire is the classic demoscene fire: a heat field at half the canvas
// resolution, seeded along the bottom by the spectrum and rising a cell per
// step while each cell averages the ones below it and cools at random. Beats
// throw embers. The field is coloured through a theme-built heat ramp.
func (p *pixState) renderFire(a *App, dt float64) {
	cv := &p.cv
	w, h := cv.w, cv.h
	fw, fh := (w+1)/2, (h+1)/2
	if len(p.heat) != fw*fh {
		p.heat = make([]float32, fw*fh)
	}
	n := max(8, min(64, fw/4))
	p.smoothLevels(a.spectrumLevels(n), dt)
	hit := p.trackBeat(kickLevel(a.visualizer), dt)

	// A fixed 90 steps a second keeps flame height independent of the frame
	// rate; cooling is scaled so a full-level column reaches the top.
	p.tick += dt * 90
	steps := min(4, int(p.tick))
	p.tick -= math.Floor(p.tick)
	cool := float32(2.6 / float64(fh))
	boost := float32(0.25 * p.beat)
	for s := 0; s < steps; s++ {
		for y := max(0, fh-2); y < fh; y++ {
			row := p.heat[y*fw : y*fw+fw]
			for x := range row {
				lvl := float32(p.levelAt((float64(x) + 0.5) / float64(fw)))
				row[x] = (1.2*lvl + boost) * (0.55 + 0.6*rand.Float32())
			}
		}
		for y := 0; y < fh-2; y++ {
			row := p.heat[y*fw : y*fw+fw]
			b1 := p.heat[(y+1)*fw : (y+1)*fw+fw]
			b2 := p.heat[(y+2)*fw : (y+2)*fw+fw]
			for x := range row {
				l, r := max(0, x-1), min(fw-1, x+1)
				v := (b1[l]+b1[x]+b1[r]+b2[x])*0.25 - rand.Float32()*cool
				row[x] = clampf(v, 0, v)
			}
		}
	}

	pal := activePalette
	stops := []rgb{hexRGB(pal.Error), hexRGB(pal.Warning), hexRGB(pal.Foreground)}
	var lut [256]rgb
	for i := range lut {
		t := float32(i) / 255
		k := clampf(t*2.2, 0, 1)
		lut[i] = gradient(stops, t).scale(k * k * 1.3)
	}
	cv.drawField(p.heat, fw, fh, &lut)

	if hit {
		ember := hexRGB(pal.Warning).scale(1.3)
		for i := 0; i < 40; i++ {
			x := rand.Float64()
			if p.levelAt(x) < 0.25 {
				continue
			}
			p.addSpark(spark{
				x:    float32(x * float64(w)),
				y:    float32(h) * (1 - 0.35*rand.Float32()),
				vx:   (rand.Float32() - 0.5) * float32(h) * 0.25,
				vy:   -float32(h) * (0.3 + 0.5*rand.Float32()),
				life: 0.6 + 0.9*rand.Float32(),
				col:  ember,
			})
		}
	}
	p.stepSparks(cv, float32(dt), -float32(h)*0.3, float32(h)*2)
}

// --- Warp -------------------------------------------------------------

// renderWarp flies through a starfield. The spectrum is laid around the
// view (bass at the bottom, treble at the top, mirrored left and right) and
// each star burns as brightly as the band in its direction, so loud bands
// show as bright sectors of the field. Overall loudness sets the speed,
// beats kick into hyperspace streaks, and the mids roll the view.
func (p *pixState) renderWarp(a *App, dt float64) {
	cv := &p.cv
	w, h := cv.w, cv.h
	const m = 24
	p.smoothLevels(a.visualizer.Levels(m), dt)
	p.normalise(dt)
	energy := bandMean(p.norm, 0, 1)
	mid := bandMean(p.norm, 0.2, 0.6)
	hit := p.trackBeat(kickLevel(a.visualizer), dt)
	aspect := float32(w) / float32(max(1, h))
	respawn := func(s *star, z float32) {
		s.x = (rand.Float32()*2 - 1) * aspect
		s.y = rand.Float32()*2 - 1
		s.z = z
	}
	if len(p.stars) == 0 {
		p.stars = make([]star, 900)
		for i := range p.stars {
			respawn(&p.stars[i], 0.05+0.95*rand.Float32())
		}
	}
	speed := float32(0.1 + 1.6*energy*energy + 3*p.beat)
	streak := speed * (0.05 + 0.15*float32(p.beat))
	p.spin += dt * (mid - 0.4) * 0.8
	sin, cos := float32(math.Sin(p.spin)), float32(math.Cos(p.spin))

	cv.fade(float32(math.Exp(-dt * 20)))
	pal := activePalette
	stops := []rgb{hexRGB(pal.Info), hexRGB(pal.Secondary), hexRGB(pal.Accent), hexRGB(pal.Warning)}
	white := hexRGB(pal.Foreground)
	cx, cy, focal := float32(w)/2, float32(h)/2, float32(h)/2
	project := func(s *star, z float32) (float32, float32) {
		x, y := s.x*cos-s.y*sin, s.x*sin+s.y*cos
		return cx + x/z*focal, cy + y/z*focal
	}
	for i := range p.stars {
		s := &p.stars[i]
		s.z -= speed * float32(dt)
		if s.z < 0.02 {
			respawn(s, 1)
			continue
		}
		x1, y1 := project(s, s.z)
		if x1 < -2 || y1 < -2 || x1 > float32(w)+2 || y1 > float32(h)+2 {
			respawn(s, 0.9+0.1*rand.Float32())
			continue
		}
		x0, y0 := project(s, clampf(s.z+streak, 0, 1))
		// Cap streak length so hyperspace stays a few hundred splats.
		if d := float32(math.Hypot(float64(x1-x0), float64(y1-y0))); d > 60 {
			x0, y0 = x1+(x0-x1)*60/d, y1+(y0-y1)*60/d
		}
		// The band follows the star's direction on screen, so the spectrum
		// stays put while the stars roll through it.
		t := math.Abs(math.Atan2(float64(x1-cx), float64(y1-cy))) / math.Pi
		band := min(m-1, int(t*m))
		lvl := float32(p.norm[band])
		near := 1 - s.z
		col := lerpRGB(gradient(stops, float32(band)/(m-1)), white, 0.2+0.5*float32(p.beat))
		cv.line(x0, y0, x1, y1, 0.5+(0.4+1.2*lvl)*near, col.scale(near*near*(0.1+2.2*lvl*lvl)))
	}
	if hit {
		cv.splat(cx, cy, float32(h)*0.2, hexRGB(pal.Accent).scale(0.6))
	}
}

// --- Wormhole ---------------------------------------------------------

// Wormhole geometry, in tunnel radii: rings are wormSlice deep, the far end
// is wormFar away, and wormRings of spectrum history cover that distance.
const (
	wormBands = 16
	wormRings = 48
	wormSlice = 0.5
	wormFar   = 20.0
)

// renderWormhole is the demoscene texture tunnel with the music as its
// texture: around the wall runs the spectrum (mirrored, so the two halves
// meet), and along it runs time, each ring of tiles holding the spectrum
// from when it appeared at the far end, so sound flows down the tunnel
// towards you and each beat travels along it as a bright ring, while every
// tile also pulses with its band's live level. Loudness
// sets the flight speed, the mids the spin. A per-pixel depth/angle table
// makes each frame a lookup per pixel.
func (p *pixState) renderWormhole(a *App, dt float64) {
	cv := &p.cv
	w, h := cv.w, cv.h
	if len(p.tunZ) != w*h {
		p.tunZ, p.tunA = make([]float32, w*h), make([]float32, w*h)
		r0 := float64(min(w, h)) / 2
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dx, dy := float64(x)+0.5-float64(w)/2, float64(y)+0.5-float64(h)/2
				p.tunZ[y*w+x] = float32(r0 / math.Max(0.5, math.Hypot(dx, dy)))
				p.tunA[y*w+x] = float32(math.Atan2(dy, dx)/(2*math.Pi) + 0.5)
			}
		}
	}
	p.smoothLevels(a.visualizer.Levels(wormBands), dt)
	p.normalise(dt)
	energy := bandMean(p.norm, 0, 1)
	mid := bandMean(p.norm, 0.2, 0.6)
	if p.trackBeat(kickLevel(a.visualizer), dt) {
		p.ringBeat = true
	}
	p.travel += dt * (1.5 + 4*energy + 6*p.beat)
	p.spin += dt * (0.05 + 0.4*mid)
	twist := float32(0.03 * math.Sin(p.spin*0.7))

	// Rings that have come into view at the far end take the current levels.
	// The extra last slot marks the first ring after a beat, so each beat
	// flies down the tunnel as one lit ring.
	if len(p.hist) != wormRings {
		p.hist = make([][]float64, wormRings)
		for i := range p.hist {
			p.hist[i] = make([]float64, wormBands+1)
		}
		p.histN = -1
	}
	far := int((wormFar + p.travel) / wormSlice)
	for k := max(p.histN+1, far-wormRings+1); k <= far; k++ {
		ring := p.hist[k%wormRings]
		copy(ring, p.norm)
		ring[wormBands] = 0
		if p.ringBeat {
			ring[wormBands], p.ringBeat = 1, false
		}
	}
	p.histN = far

	pal := activePalette
	stops := []rgb{hexRGB(pal.Info), hexRGB(pal.Secondary), hexRGB(pal.Accent), hexRGB(pal.Warning)}
	// Rings near the camera were written seconds ago, so each band's live
	// level is mixed in everywhere to keep the whole wall in time.
	var bandCol [wormBands]rgb
	var live [wormBands]float32
	for j := range bandCol {
		bandCol[j] = gradient(stops, float32(j)/(wormBands-1)).scale(1.6)
		live[j] = float32(0.7 * p.norm[j])
	}
	grid := hexRGB(pal.Border).scale(0.5)
	flash := hexRGB(pal.Foreground)
	travel, spin := float32(p.travel), float32(p.spin)
	for i, z := range p.tunZ {
		o := i * 3
		if z > wormFar {
			cv.pix[o], cv.pix[o+1], cv.pix[o+2] = 0, 0, 0
			continue
		}
		ring := (z + travel) / wormSlice
		k := int(ring)
		fr := ring - float32(k)
		ang := p.tunA[i] + spin + twist*z
		ang -= float32(math.Floor(float64(ang)))
		b := float32(math.Abs(float64(ang*2-1))) * wormBands
		j := min(wormBands-1, int(b))
		fb := b - float32(j)
		fog := 1 / (1 + z*0.2)
		hist := p.hist[k%wormRings]
		lvl := clampf(0.5*float32(hist[j])+live[j], 0, 1)
		// Bevel each tile: bright in the middle, dark at its edges.
		bevel := 16 * fr * (1 - fr) * fb * (1 - fb)
		col := bandCol[j].scale((0.03 + lvl*lvl) * fog * (0.25 + 0.75*bevel))
		if fr < 0.07 || fb < 0.07 {
			g := grid
			if hist[wormBands] > 0 {
				g = lerpRGB(grid, flash, 0.7).scale(2)
			}
			col = rgb{col.r + g.r*fog, col.g + g.g*fog, col.b + g.b*fog}
		}
		cv.pix[o], cv.pix[o+1], cv.pix[o+2] = col.r, col.g, col.b
	}
}

// --- Copper -----------------------------------------------------------

// renderCopper draws Amiga copper bars: shaded horizontal bars bobbing on a
// sine, sorted so the near ones cover the far ones. Each bar is a frequency
// band whose level sets its thickness and brightness, loudness sets the bob
// speed, beats fan the bars apart, and the spectrum lights each bar along
// its length.
func (p *pixState) renderCopper(a *App, dt float64) {
	cv := &p.cv
	w, h := cv.w, cv.h
	const nb = 9
	p.smoothLevels(a.visualizer.Levels(nb), dt)
	p.normalise(dt)
	energy := bandMean(p.norm, 0, 1)
	p.trackBeat(kickLevel(a.visualizer), dt)
	p.spin += dt * (0.5 + 2*energy + 2.5*p.beat)

	// Brightness along each bar follows the spectrum across the width,
	// scaled by the same running peak as the bars.
	xs := a.spectrumLevels(max(8, min(96, w/8)))
	gain := 1 / math.Max(p.top, 0.02)
	xmod := make([]float32, w)
	for x := range xmod {
		f := (float64(x) + 0.5) / float64(w) * float64(len(xs)-1)
		i := int(f)
		j := min(len(xs)-1, i+1)
		v := clamp01((xs[i] + (xs[j]-xs[i])*(f-float64(i))) * gain)
		xmod[x] = float32(0.15 + 1.2*v*v)
	}

	type bar struct {
		y, depth float64
		i        int
	}
	bars := make([]bar, nb)
	for i := range bars {
		ph := p.spin + float64(i)*(0.32+0.45*p.beat)
		bars[i] = bar{y: float64(h) * (0.5 + (0.2+0.2*energy+0.06*p.beat)*math.Sin(ph)), depth: math.Cos(ph), i: i}
	}
	slices.SortFunc(bars, func(x, y bar) int {
		if x.depth < y.depth {
			return -1
		}
		if x.depth > y.depth {
			return 1
		}
		return 0
	})

	pal := activePalette
	stops := []rgb{hexRGB(pal.Info), hexRGB(pal.Success), hexRGB(pal.Warning), hexRGB(pal.Accent), hexRGB(pal.Secondary)}
	shine := hexRGB(pal.Foreground)
	cv.clear()
	for _, b := range bars {
		lvl := 0.2*p.norm[b.i] + 0.8*p.pulse[b.i]
		near := (b.depth + 1) / 2
		half := float64(h) * (0.008 + 0.065*lvl*lvl + 0.012*p.beat) * (0.75 + 0.25*near)
		gain := float32((0.35 + 0.65*near) * (0.12 + 1.3*lvl) * (1 + 0.6*p.beat))
		base := gradient(stops, float32(b.i)/(nb-1))
		for y := int(b.y - half); y <= int(b.y+half); y++ {
			if y < 0 || y >= h {
				continue
			}
			t := float32((float64(y) + 0.5 - b.y) / half)
			if t < -1 || t > 1 {
				continue
			}
			// A rounded tube: shaded body plus a specular line above centre.
			body := 1 - t*t
			spec := (t + 0.35) / 0.15
			col := lerpRGB(base.scale(body*body), shine, float32(math.Exp(-float64(spec*spec)))*0.7).scale(gain)
			row := cv.pix[y*w*3 : y*w*3+w*3]
			for x, k := range xmod {
				row[x*3], row[x*3+1], row[x*3+2] = col.r*k, col.g*k, col.b*k
			}
		}
	}
}

// --- Ridgeline --------------------------------------------------------

const ridgeLines = 40

// renderRidgeline is the Unknown Pleasures plot: a new spectrum ridge enters
// at the front a dozen times a second and the older ones recede up the
// screen. Ridges are drawn back to front, each blanking what it hides, so
// the terminal background shows through as the "black" between them.
func (p *pixState) renderRidgeline(a *App, dt float64) {
	cv := &p.cv
	w, h := cv.w, cv.h
	m := max(16, min(96, w/6))
	p.smoothLevels(a.spectrumLevels(m), dt)
	p.normalise(dt)
	p.trackBeat(kickLevel(a.visualizer), dt)
	if len(p.hist) != ridgeLines || len(p.hist[0]) != m {
		p.hist = make([][]float64, ridgeLines)
		for i := range p.hist {
			p.hist[i] = make([]float64, m)
		}
		p.histN = 0
	}
	p.tick += dt * 12
	for ; p.tick >= 1; p.tick-- {
		p.histN++
		ridge := p.hist[p.histN%ridgeLines]
		// Taper to flat ends, with a little noise so even quiet ridges
		// have texture, as in the original plot.
		for i := range ridge {
			t := (float64(i) + 0.5) / float64(m)
			win := math.Pow(math.Sin(math.Pi*t), 2)
			ridge[i] = win * (p.norm[i]*(0.2+0.8*p.pulse[i])*(1+0.3*p.beat) + 0.04*rand.Float64())
		}
	}
	pal := activePalette
	stops := []rgb{hexRGB(pal.Foreground), hexRGB(pal.Info), hexRGB(pal.Secondary)}
	x0, x1 := w/10, w-w/10
	if x1-x0 < 2 {
		return
	}
	spacing := float64(h) * 0.8 / ridgeLines
	amp := spacing * 11
	bottom := float64(h) - 2
	cv.clear()
	for r := ridgeLines - 1; r >= 0; r-- {
		ridge := p.hist[((p.histN-r)%ridgeLines+ridgeLines)%ridgeLines]
		pos := float64(r) + p.tick
		base := bottom - pos*spacing
		fade := math.Min(1, (ridgeLines-pos)/4) * math.Min(1, p.tick*3+float64(min(r, 1)))
		col := gradient(stops, float32(pos/ridgeLines)).scale(float32(fade * (1 - 0.5*pos/ridgeLines)))
		bi := int(base)
		prev := -1
		for x := x0; x < x1; x++ {
			f := float64(x-x0) / float64(x1-x0-1) * float64(m-1)
			i := int(f)
			j := min(m-1, i+1)
			y := int(base - amp*(ridge[i]+(ridge[j]-ridge[i])*(f-float64(i))))
			for yy := max(0, y+1); yy <= min(h-1, bi); yy++ {
				cv.set(x, yy, rgb{})
			}
			if prev < 0 {
				prev = y
			}
			for yy := min(prev, y); yy <= max(prev, y); yy++ {
				cv.add(x, yy, col)
			}
			prev = y
		}
	}
}

// --- Horizon ----------------------------------------------------------

// renderHorizon is a synthwave sunset: a slatted sun sinking behind
// mountains raised by the spectrum (bass at the edges, treble in the valley
// around the sun) over a perspective grid that scrolls with the bass and
// flashes on beats. Treble twinkles the stars.
func (p *pixState) renderHorizon(a *App, dt float64) {
	cv := &p.cv
	w, h := cv.w, cv.h
	const m = 32
	raw := a.visualizer.Levels(m)
	p.smoothLevels(raw, dt)
	p.normalise(dt)
	bass := bandMean(p.pulse, 0, 0.15)
	p.trackBeat(kickLevel(a.visualizer), dt)
	p.travel += dt * (0.6 + 2.5*bass + 1.5*p.beat)
	p.spin += dt
	if len(p.stars) == 0 {
		p.stars = make([]star, 80)
		for i := range p.stars {
			p.stars[i] = star{x: rand.Float32(), y: rand.Float32(), band: m/3 + rand.IntN(m-m/3)}
		}
	}

	pal := activePalette
	accent, warn, info := hexRGB(pal.Accent), hexRGB(pal.Warning), hexRGB(pal.Info)
	fw, fh := float64(w), float64(h)
	hz := h * 11 / 20
	cx := fw / 2
	cv.clear()

	for _, s := range p.stars {
		tw := float32(0.1 + 0.9*p.norm[s.band])
		cv.splat(s.x*float32(w), s.y*float32(hz)*0.85, 1.1, hexRGB(pal.Foreground).scale(tw))
	}

	// The sun, its lower half cut by slats that thicken towards the
	// horizon and drift downwards.
	r := math.Min(fw*0.16, fh*0.3) * (1 + 0.08*bass + 0.08*p.beat)
	sy := float64(hz) - r*0.45
	cv.splat(float32(cx), float32(sy), float32(r*1.9), accent.scale(0.12))
	for y := int(sy - r); y < hz; y++ {
		dy := float64(y) + 0.5 - sy
		if dy*dy > r*r {
			continue
		}
		t := (dy + r) / (2 * r)
		if t > 0.5 {
			gap := (t - 0.5) * 2 * 0.6
			if ph := (dy + p.spin*r*0.1) / (r * 0.16); ph-math.Floor(ph) < gap {
				continue
			}
		}
		half := math.Sqrt(r*r - dy*dy)
		cv.hspan(int(cx-half), int(cx+half), y, lerpRGB(warn, accent, float32(t)).scale(1.1))
	}

	// Mountains: blank fill to hide the sun, a neon ridge on top.
	fill := hexRGB(pal.Secondary).scale(0.08)
	prev := -1
	for x := 0; x < w; x++ {
		d := math.Abs(float64(x)+0.5-cx) / cx
		lvl := p.normAt(1-d) * (0.35 + 0.65*p.pulseAt(1-d))
		jag := 0.75 + 0.25*math.Sin(float64(x)*0.21)*math.Sin(float64(x)*0.047+1)
		height := fh * (0.02 + 0.42*lvl*jag) * (0.45 + 0.55*d)
		y := hz - int(height)
		for yy := max(0, y+1); yy < hz; yy++ {
			cv.set(x, yy, fill)
		}
		if prev < 0 {
			prev = y
		}
		for yy := min(prev, y); yy <= max(prev, y); yy++ {
			cv.add(x, yy, info.scale(0.9))
			cv.add(x, yy-1, info.scale(0.2))
		}
		prev = y
	}

	// The floor grid: rows at evenly spaced depths, columns converging on
	// the vanishing point.
	floor := fh - float64(hz)
	glow := float32(1 + 1.2*p.beat)
	const gap = 0.5
	off := p.travel - math.Floor(p.travel/gap)*gap
	for k := 0; k < 40; k++ {
		z := 1 + float64(k)*gap - off
		if z < 1 {
			continue
		}
		y := hz + int(floor/z)
		b := float32(math.Pow(1/z, 0.5)) * glow
		cv.hspan(0, w, y, accent.scale(0.7*b))
		cv.hspan(0, w, y-1, accent.scale(0.15*b))
		cv.hspan(0, w, y+1, accent.scale(0.15*b))
	}
	spread := fw * 0.09
	for i := -30; i <= 30; i++ {
		top := 0.06
		x0 := cx + float64(i)*spread*top
		cv.line(float32(x0), float32(float64(hz)+floor*top), float32(cx+float64(i)*spread), float32(fh), 0.8, accent.scale(0.35*glow))
	}
	cv.hspan(0, w, hz, accent.scale(1.2*glow))
	cv.hspan(0, w, hz-1, accent.scale(0.3*glow))
}

// --- Lava -------------------------------------------------------------

// renderLava is a lava lamp of metaballs: one blob per frequency band,
// swelling and brightening with its normalised level and wandering on
// Lissajous paths that speed up with the loudness. Each beat pumps every
// blob and throws the pack outwards. Where blobs meet they merge, blending
// their colours.
func (p *pixState) renderLava(a *App, dt float64) {
	cv := &p.cv
	w, h := cv.w, cv.h
	const nb = 7
	p.smoothLevels(a.visualizer.Levels(nb), dt)
	p.normalise(dt)
	energy := bandMean(p.norm, 0, 1)
	p.trackBeat(kickLevel(a.visualizer), dt)
	p.spin += dt * (0.12 + 1.3*energy*energy + 1.5*p.beat)

	pal := activePalette
	stops := []rgb{hexRGB(pal.Info), hexRGB(pal.Secondary), hexRGB(pal.Accent), hexRGB(pal.Warning)}
	var bx, by, br2 [nb]float32
	var bc [nb]rgb
	span := float64(min(w, h))
	spread := 1 + 0.25*p.beat
	for i := 0; i < nb; i++ {
		ph := float64(i) * 1.7
		lvl := p.norm[i]
		bx[i] = float32(float64(w) * (0.5 + 0.36*spread*math.Sin(p.spin*(0.6+0.13*float64(i))+ph)))
		by[i] = float32(float64(h) * (0.5 + 0.32*spread*math.Cos(p.spin*(0.5+0.11*float64(i))+ph*1.3)))
		r := span * (0.03 + 0.12*lvl*lvl + 0.05*p.beat)
		br2[i] = float32(r * r)
		bc[i] = gradient(stops, float32(i)/(nb-1)).scale(float32(0.45 + 0.9*lvl + 0.4*p.beat))
	}
	var dy2 [nb]float32
	for y := 0; y < h; y++ {
		fy := float32(y) + 0.5
		for i := range dy2 {
			d := fy - by[i]
			dy2[i] = d*d + 1
		}
		row := cv.pix[y*w*3 : y*w*3+w*3]
		for x := 0; x < w; x++ {
			fx := float32(x) + 0.5
			var f, cr, cg, cb float32
			for i := 0; i < nb; i++ {
				d := fx - bx[i]
				k := br2[i] / (d*d + dy2[i])
				f += k
				cr += bc[i].r * k
				cg += bc[i].g * k
				cb += bc[i].b * k
			}
			o := x * 3
			if f < 0.2 {
				row[o], row[o+1], row[o+2] = 0, 0, 0
				continue
			}
			// Solid body past f=1 with a bright rim, a faint halo outside.
			var b float32
			if f >= 1 {
				b = 0.75 + 0.15*clampf(f-1, 0, 1)
			} else {
				b = 0.3 * f * f * f * f
			}
			if e := (f - 1) * 14; e > -1 && e < 1 {
				b += 0.55 * (1 - e*e)
			}
			b /= f
			row[o], row[o+1], row[o+2] = cr*b, cg*b, cb*b
		}
	}
}

// --- LED --------------------------------------------------------------

// renderLED is a hardware-style analyser, as in Winamp's classic skin:
// bars built of discrete LED segments coloured by height (green, amber,
// red in the theme's terms), unlit segments faintly visible, and peak
// segments that hold then drop.
func (p *pixState) renderLED(a *App, dt float64) {
	cv := &p.cv
	w, h := cv.w, cv.h
	pitch := max(3, h/28)
	segs := max(2, (h-2)/pitch)
	n := max(8, min(64, w/(pitch*3)))
	p.smoothLevels(a.spectrumLevels(n), dt)
	p.updatePeaks(dt)

	pal := activePalette
	stops := []rgb{hexRGB(pal.Success), hexRGB(pal.Warning), hexRGB(pal.Error)}
	peakCol := hexRGB(pal.Foreground)
	colW := float64(w) / float64(n)
	gap := max(1, int(colW*0.2))
	cv.clear()
	for i, lvl := range p.levels {
		x0 := int(float64(i)*colW) + gap/2
		x1 := int(float64(i+1)*colW) - (gap+1)/2
		lit := int(clamp01(lvl)*float64(segs) + 0.5)
		peak := min(segs-1, int(p.peaks[i]*float64(segs)))
		for s := 0; s < segs; s++ {
			y1 := h - 1 - s*pitch
			y0 := y1 - pitch + 2
			col := gradient(stops, float32(s)/float32(segs-1))
			switch {
			case s < lit:
				cv.rect(x0-1, y0-1, x1+1, y1+2, col.scale(0.12))
				cv.rect(x0, y0, x1, y1+1, col)
			case s == peak && peak > 0:
				cv.rect(x0, y0, x1, y1+1, peakCol.scale(0.9))
			default:
				cv.rect(x0, y0, x1, y1+1, col.scale(0.06))
			}
		}
	}
}
