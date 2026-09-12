package main

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFrequencyOrderAndSettingsPersistence(t *testing.T) {
	v := NewCavaVisualizer()
	v.levels = []float64{0, .2, .4, .6, .8, 1}
	a := &App{visualizer: v, player: NewPlayer(), store: newTempStore(t),
		lib: Library{Favorites: map[string]Station{}, Hidden: map[string]Station{}}}
	if got := a.spectrumLevels(6); !reflect.DeepEqual(got, []float64{1, .6, .2, 0, .4, .8}) {
		t.Fatalf("centred spectrum: %v", got)
	}
	a.themeCursor = 7
	a.handleThemeConfigKey(Key{Name: "enter"})
	if got := a.spectrumLevels(6); !reflect.DeepEqual(got, v.levels) {
		t.Fatalf("linear spectrum: %v", got)
	}
	lib, err := a.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if parseFrequencyOrder(lib.State.VizOrder) != frequencyLinear {
		t.Fatal("frequency order did not persist")
	}
	a.width, a.height = 64, 12
	screen := stripANSI(renderThemeConfig(a))
	if !strings.Contains(screen, "Bass left") || len(strings.Split(screen, "\n")) > a.height {
		t.Fatalf("setting not visible at minimum size:\n%s", screen)
	}
	if parseFrequencyOrder("") != frequencyCentered {
		t.Fatal("old preferences should default to centred")
	}
}

func TestReactiveEffectsRespondAndFit(t *testing.T) {
	start := time.Unix(100, 0)
	for _, style := range []vizStyle{stylePlasma, styleTunnel, styleRibbons} {
		for _, size := range [][2]int{{1, 1}, {3, 10}, {30, 140}, {65, 170}} {
			a := &App{vizStyle: style}
			quiet := a.reactiveEffectRows(nil, size[0], size[1], start)
			loud := a.reactiveEffectRows([]float64{1, 1, 1}, size[0], size[1], start)
			if size[1] > 1 && reflect.DeepEqual(quiet, loud) {
				t.Fatalf("%s does not respond to audio", style.Label())
			}
			for _, row := range loud {
				if visibleLen(row) != size[1] {
					t.Fatalf("%s wrong width", style.Label())
				}
			}
			moving := a.reactiveEffectRows([]float64{1, 1, 1}, size[0], size[1], start.Add(160*time.Millisecond))
			if size[1] > 10 && reflect.DeepEqual(loud, moving) {
				t.Fatalf("%s does not animate", style.Label())
			}
		}
	}
}

func BenchmarkReactiveEffects(b *testing.B) {
	for _, style := range []vizStyle{stylePlasma, styleTunnel, styleRibbons} {
		b.Run(style.Label(), func(b *testing.B) {
			a := &App{vizStyle: style}
			now := time.Unix(100, 0)
			levels := []float64{.8, .5, .3}
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				a.reactiveEffectRows(levels, 40, 160, now.Add(time.Duration(i)*80*time.Millisecond))
			}
		})
	}
}
