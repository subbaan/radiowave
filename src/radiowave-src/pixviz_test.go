package main

import (
	"strings"
	"testing"
	"time"
)

func TestEncodeRGBAUpscalesAndUsesBrightnessAsAlpha(t *testing.T) {
	var cv pixCanvas
	cv.resize(2, 1)
	cv.add(0, 0, rgb{4, 0, 0}) // saturates
	dst := make([]byte, 3*2*4) // 3x2 output crops the 2x upscale of 2x1
	cv.encodeRGBA(dst, 2, 3, 2)
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			px := dst[(y*3+x)*4:]
			if px[0] != 255 || px[3] != 255 || px[1] != 0 {
				t.Fatalf("pixel %d,%d = %v, want opaque red", x, y, px[:4])
			}
		}
		if px := dst[(y*3+2)*4:]; px[3] != 0 {
			t.Fatalf("black pixel should be transparent, got %v", px[:4])
		}
	}
}

func TestPixelStylesFallBackWithoutGraphics(t *testing.T) {
	a := &App{vizStyle: styleNebula, visualizer: NewDemoVisualizer()}
	if a.activeVizStyle() != styleClassic || a.pixelStyleActive() {
		t.Fatalf("pixel style without graphics should draw as Classic")
	}
	a.gfx = &kittyGfx{tx: gfxSHM}
	if a.activeVizStyle() != styleNebula || !a.pixelStyleActive() {
		t.Fatalf("pixel style with graphics should stay active")
	}
}

func TestPixelStyleReservesVisualRect(t *testing.T) {
	app := &App{
		width: 100, height: 30, playerLayout: layoutVisual,
		lib:    Library{Favorites: map[string]Station{}, Hidden: map[string]Station{}},
		player: NewPlayer(), visualizer: NewDemoVisualizer(),
	}
	app.gfx = &kittyGfx{tx: gfxSHM}
	app.vizStyle = styleNeon
	renderApp(app)
	want := vizRect{top: 3, left: 3, rows: 25, cols: 96}
	if app.pixRect != want {
		t.Fatalf("pixRect = %+v, want %+v", app.pixRect, want)
	}
	app.help = true
	renderApp(app)
	if app.pixRect.valid() {
		t.Fatalf("overlay screens should not reserve a pixel rect")
	}
	if esc := (&kittyGfx{tx: gfxSHM, placed: true}).hide(); !strings.Contains(esc, "a=d") {
		t.Fatalf("hide should delete the placement, got %q", esc)
	}
}

func TestPixelStylesRender(t *testing.T) {
	a := &App{visualizer: NewDemoVisualizer()}
	a.gfx = &kittyGfx{tx: gfxSHM}
	now := time.Now()
	for _, s := range []vizStyle{styleNeon, styleNebula, styleSpectrogram} {
		a.vizStyle = s
		for i := 0; i < 10; i++ {
			now = now.Add(16 * time.Millisecond)
			a.pix.render(a, 120, 50, now)
		}
		var lit bool
		for _, v := range a.pix.cv.pix {
			if v > 0 {
				lit = true
				break
			}
		}
		if !lit {
			t.Fatalf("%v rendered an empty canvas", s)
		}
	}
}

// BenchmarkPixelFrame measures one full frame for a 1800x900 px visualizer
// at the default 2x scale: render + tone-map + upscale into the output.
func BenchmarkPixelFrame(b *testing.B) {
	for _, s := range []vizStyle{styleNeon, styleNebula, styleSpectrogram} {
		b.Run(s.Label(), func(b *testing.B) {
			a := &App{visualizer: NewDemoVisualizer(), vizStyle: s}
			a.gfx = &kittyGfx{tx: gfxSHM}
			out := make([]byte, 1800*900*4)
			now := time.Now()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				a.pix.render(a, 900, 450, now.Add(time.Duration(i)*16*time.Millisecond))
				a.pix.cv.encodeRGBA(out, 2, 1800, 900)
			}
		})
	}
}
