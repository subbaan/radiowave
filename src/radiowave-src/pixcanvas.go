package main

import (
	"math"
	"strconv"
	"strings"
	"unsafe"
)

// pixCanvas is a small software framebuffer of additive light: three
// float32 channels per pixel on black, where 1.0 is "full colour" and
// anything brighter is headroom for glow. It is tone-mapped to 8-bit only
// on the way out, and its brightness doubles as alpha so black is fully
// transparent and the terminal's background shows through.
type pixCanvas struct {
	w, h int
	pix  []float32
}

type rgb struct{ r, g, b float32 }

func (c rgb) scale(k float32) rgb { return rgb{c.r * k, c.g * k, c.b * k} }

func lerpRGB(a, b rgb, t float32) rgb {
	return rgb{a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t}
}

// gradient samples evenly spaced stops at t in [0,1].
func gradient(stops []rgb, t float32) rgb {
	if len(stops) == 1 || t <= 0 {
		return stops[0]
	}
	if t >= 1 {
		return stops[len(stops)-1]
	}
	f := t * float32(len(stops)-1)
	i := int(f)
	return lerpRGB(stops[i], stops[i+1], f-float32(i))
}

func hexRGB(hex string) rgb {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return rgb{1, 1, 1}
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return rgb{1, 1, 1}
	}
	return rgb{float32(v>>16&255) / 255, float32(v>>8&255) / 255, float32(v&255) / 255}
}

func (c *pixCanvas) resize(w, h int) bool {
	if c.w == w && c.h == h {
		return false
	}
	c.w, c.h = w, h
	c.pix = make([]float32, w*h*3)
	return true
}

func (c *pixCanvas) clear() { clear(c.pix) }

func (c *pixCanvas) fade(k float32) {
	for i := range c.pix {
		c.pix[i] *= k
	}
}

func (c *pixCanvas) add(x, y int, col rgb) {
	if x < 0 || y < 0 || x >= c.w || y >= c.h {
		return
	}
	i := (y*c.w + x) * 3
	c.pix[i] += col.r
	c.pix[i+1] += col.g
	c.pix[i+2] += col.b
}

// hspan adds col to pixels [x0,x1) of row y.
func (c *pixCanvas) hspan(x0, x1, y int, col rgb) {
	if y < 0 || y >= c.h {
		return
	}
	x0, x1 = max(0, x0), min(c.w, x1)
	row := c.pix[y*c.w*3:]
	for x := x0; x < x1; x++ {
		row[x*3] += col.r
		row[x*3+1] += col.g
		row[x*3+2] += col.b
	}
}

// splat adds a soft round dot: (1-d²/r²)² falloff, which looks close to a
// gaussian without paying for exp() per pixel.
func (c *pixCanvas) splat(fx, fy, radius float32, col rgb) {
	r2 := radius * radius
	x0, x1 := int(fx-radius), int(fx+radius)+1
	y0, y1 := int(fy-radius), int(fy+radius)+1
	x0, y0 = max(0, x0), max(0, y0)
	x1, y1 = min(c.w-1, x1), min(c.h-1, y1)
	for y := y0; y <= y1; y++ {
		dy := float32(y) + 0.5 - fy
		for x := x0; x <= x1; x++ {
			dx := float32(x) + 0.5 - fx
			d := (dx*dx + dy*dy) / r2
			if d >= 1 {
				continue
			}
			k := (1 - d) * (1 - d)
			i := (y*c.w + x) * 3
			c.pix[i] += col.r * k
			c.pix[i+1] += col.g * k
			c.pix[i+2] += col.b * k
		}
	}
}

// line draws soft dots along a segment, about one per pixel of length.
func (c *pixCanvas) line(x0, y0, x1, y1, radius float32, col rgb) {
	steps := int(math.Hypot(float64(x1-x0), float64(y1-y0))) + 1
	for s := 0; s <= steps; s++ {
		t := float32(s) / float32(steps)
		c.splat(x0+(x1-x0)*t, y0+(y1-y0)*t, radius, col)
	}
}

// warpFrom fills c from src sampled through a zoom/rotation about the
// centre (bilinear), scaled by decay. zoom < 1 samples nearer the centre, so
// the picture flows outwards — the classic Milkdrop feedback loop.
func (c *pixCanvas) warpFrom(src *pixCanvas, zoom, rot, decay float32) {
	w, h := c.w, c.h
	cx, cy := float32(w)/2, float32(h)/2
	sin, cos := float32(math.Sin(float64(rot))), float32(math.Cos(float64(rot)))
	// Canvas pixels are square, so rotation needs no aspect correction.
	// The source point moves by (a, by) per destination pixel along a row.
	a, bx := zoom*cos, -zoom*sin
	by, d := zoom*sin, zoom*cos
	p := src.pix
	stride := w * 3
	maxX, maxY := float32(w-1), float32(h-1)
	for y := 0; y < h; y++ {
		dy := float32(y) + 0.5 - cy
		dx := 0.5 - cx
		sx := cx + a*dx + bx*dy - 0.5
		sy := cy + by*dx + d*dy - 0.5
		out := c.pix[y*stride : y*stride+stride]
		for o := 0; o+2 < len(out); o, sx, sy = o+3, sx+a, sy+by {
			if sx < 0 || sy < 0 || sx >= maxX || sy >= maxY {
				out[o], out[o+1], out[o+2] = 0, 0, 0
				continue
			}
			ix, iy := int(sx), int(sy)
			fx, fy := sx-float32(ix), sy-float32(iy)
			i := iy*stride + ix*3
			t := p[i : i+6 : i+6]
			b := p[i+stride : i+stride+6 : i+stride+6]
			gy := fy * decay
			ty := decay - gy
			w00, w10 := (1-fx)*ty, fx*ty
			w01, w11 := (1-fx)*gy, fx*gy
			px := out[o : o+3 : o+3]
			px[0] = t[0]*w00 + t[3]*w10 + b[0]*w01 + b[3]*w11
			px[1] = t[1]*w00 + t[4]*w10 + b[1]*w01 + b[4]*w11
			px[2] = t[2]*w00 + t[5]*w10 + b[2]*w01 + b[5]*w11
		}
	}
}

// Tone-map lookup: additive light in [0, toneRange) to 8 bits with a soft
// shoulder (1-e^-2x), so stacked glow saturates gracefully instead of
// clipping to flat white.
const (
	toneSize  = 2048
	toneRange = 4.0
)

var toneLUT, invAlpha = func() ([toneSize]uint8, [256]uint32) {
	var t [toneSize]uint8
	for i := range t {
		x := float64(i) / toneSize * toneRange
		t[i] = uint8(math.Round(255 * (1 - math.Exp(-2*x))))
	}
	var inv [256]uint32
	for a := 1; a < 256; a++ {
		inv[a] = (255 << 16) / uint32(a)
	}
	return t, inv
}()

func toneByte(v float32) uint8 {
	i := int(v * (toneSize / toneRange))
	if i <= 0 {
		return 0
	}
	if i >= toneSize {
		return 255
	}
	return toneLUT[i]
}

// encodeRGBA writes the canvas as straight-alpha RGBA into dst (outW*outH*4
// bytes), nearest-neighbour upscaled by scale and cropped to outW x outH.
// Alpha is the brightest channel, so compositing over the terminal
// background behaves like adding the light to it.
func (c *pixCanvas) encodeRGBA(dst []byte, scale, outW, outH int) {
	if len(dst) < outW*outH*4 || outW <= 0 || outH <= 0 {
		return
	}
	// One little-endian uint32 per pixel (amd64/arm64): bytes R,G,B,A.
	out := unsafe.Slice((*uint32)(unsafe.Pointer(&dst[0])), outW*outH)
	for oy := 0; oy < outH; oy += scale {
		sy := min(c.h-1, oy/scale)
		row := out[oy*outW : oy*outW+outW]
		src := c.pix[sy*c.w*3 : sy*c.w*3+c.w*3]
		ox := 0
		for si := 0; si+2 < len(src) && ox < len(row); si += 3 {
			px := encodePixel(src[si], src[si+1], src[si+2])
			n := min(scale, len(row)-ox)
			for k := 0; k < n; k++ {
				row[ox+k] = px
			}
			ox += n
		}
		for k := 1; k < scale && oy+k < outH; k++ {
			copy(out[(oy+k)*outW:(oy+k)*outW+outW], row)
		}
	}
}

func encodePixel(rf, gf, bf float32) uint32 {
	const k = toneSize / toneRange
	if rf+gf+bf < 1/k {
		return 0
	}
	r, g, b := toneByte(rf), toneByte(gf), toneByte(bf)
	a := r // render.go's max is int-only
	if g > a {
		a = g
	}
	if b > a {
		a = b
	}
	if a == 0 {
		return 0
	}
	inv := invAlpha[a]
	return uint32(r)*inv>>16 | (uint32(g)*inv>>16)<<8 | (uint32(b)*inv>>16)<<16 | uint32(a)<<24
}

// clampf exists because render.go's min and max shadow the builtins with
// int-only versions.
func clampf(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// set overwrites one pixel, for styles that occlude rather than add light.
func (c *pixCanvas) set(x, y int, col rgb) {
	if x < 0 || y < 0 || x >= c.w || y >= c.h {
		return
	}
	i := (y*c.w + x) * 3
	c.pix[i], c.pix[i+1], c.pix[i+2] = col.r, col.g, col.b
}

// rect adds col to pixels [x0,x1) x [y0,y1).
func (c *pixCanvas) rect(x0, y0, x1, y1 int, col rgb) {
	for y := max(0, y0); y < min(c.h, y1); y++ {
		c.hspan(x0, x1, y, col)
	}
}

// drawField overwrites the canvas with a scalar field in [0,1] (fw x fh,
// usually coarser than the canvas), bilinearly stretched to fit and
// coloured through lut.
func (c *pixCanvas) drawField(f []float32, fw, fh int, lut *[256]rgb) {
	if fw < 1 || fh < 1 || len(f) < fw*fh {
		return
	}
	sx, sy := float32(fw)/float32(c.w), float32(fh)/float32(c.h)
	maxX, maxY := float32(fw-1), float32(fh-1)
	for y := 0; y < c.h; y++ {
		fy := clampf((float32(y)+0.5)*sy-0.5, 0, maxY)
		iy := int(fy)
		ty := fy - float32(iy)
		r0, r1 := f[iy*fw:iy*fw+fw], f[min(fh-1, iy+1)*fw:][:fw]
		out := c.pix[y*c.w*3 : y*c.w*3+c.w*3]
		for x := 0; x < c.w; x++ {
			fx := clampf((float32(x)+0.5)*sx-0.5, 0, maxX)
			ix := int(fx)
			tx := fx - float32(ix)
			jx := min(fw-1, ix+1)
			top := r0[ix] + (r0[jx]-r0[ix])*tx
			bot := r1[ix] + (r1[jx]-r1[ix])*tx
			v := top + (bot-top)*ty
			k := int(v * 255)
			k = max(0, min(255, k))
			col := lut[k]
			out[x*3], out[x*3+1], out[x*3+2] = col.r, col.g, col.b
		}
	}
}
