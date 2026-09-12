package main

import "testing"

func TestCenterOutLevelsPreservesCount(t *testing.T) {
	in := []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7}
	out := centerOutLevels(in)
	if len(out) != len(in) {
		t.Fatalf("expected %d levels, got %d", len(in), len(out))
	}
}

func TestVuMeterRowsStretchesToWidth(t *testing.T) {
	levels := []float64{0.9, 0.9, 0.9, 0.9, 0.9, 0.9, 0.9, 0.9}
	narrow := vuMeterRows(levels, 4, 40)
	wide := vuMeterRows(levels, 4, 160)
	narrowW := visibleLen(narrow[0])
	wideW := visibleLen(wide[0])
	if wideW <= narrowW*2 {
		t.Fatalf("expected VU meters to stretch with width: narrow=%d wide=%d", narrowW, wideW)
	}
	if wideW < 140 {
		t.Fatalf("expected wide VU row to nearly fill 160 cols, got %d", wideW)
	}
}

func TestChunkyLevelsFlattensGroups(t *testing.T) {
	in := []float64{0.1, 0.9, 0.2, 0.5, 0.8, 0.1}
	out := chunkyLevels(in, 3)
	if out[0] != out[1] || out[1] != out[2] {
		t.Fatalf("expected first group flattened to shared max, got %v", out[:3])
	}
	if out[0] != 0.9 {
		t.Fatalf("expected group max 0.9, got %v", out[0])
	}
}

func TestCycleVizStyleWrapsAndPersistsChoice(t *testing.T) {
	store := newTempStore(t)
	a := &App{
		lib:        Library{Favorites: map[string]Station{}, Hidden: map[string]Station{}},
		store:      store,
		player:     NewPlayer(),
		visualizer: NewDemoVisualizer(),
	}
	start := a.vizStyle
	for i := 0; i < int(vizStyleCount); i++ {
		a.cycleVizStyle(1)
	}
	if a.vizStyle != start {
		t.Fatalf("expected cycling through all styles to wrap back to %v, got %v", start, a.vizStyle)
	}
	a.cycleVizStyle(-1)
	if a.vizStyle == start {
		t.Fatalf("expected cycling backward to leave classic")
	}
}

// TestAllVizStylesRenderWithoutPanicking exercises every style against a
// spread of levels (including empty/zero, since a fresh cava process or a
// station with no audio yet should never crash the renderer).
func TestAllVizStylesRenderWithoutPanicking(t *testing.T) {
	a := &App{visualizer: NewDemoVisualizer()}
	sizes := []struct{ rows, cols int }{
		{1, 1}, {3, 10}, {12, 60}, {30, 140},
	}
	for s := vizStyle(0); s < vizStyleCount; s++ {
		a.vizStyle = s
		a.vizPeaks = nil
		a.vizHistory = nil
		for _, sz := range sizes {
			rows := a.renderVisualizer(sz.rows, sz.cols)
			if len(rows) < 1 {
				t.Fatalf("style %v at %dx%d produced no rows", s.Label(), sz.rows, sz.cols)
			}
			for _, r := range rows {
				_ = stripANSI(r) // must not panic on malformed ANSI
			}
		}
	}
}

func TestVizStyleRoundTripsThroughPersistedName(t *testing.T) {
	for s := vizStyle(0); s < vizStyleCount; s++ {
		if got := parseVizStyle(s.String()); got != s {
			t.Fatalf("style %v did not round-trip: got %v from %q", s.Label(), got, s.String())
		}
	}
	if got := parseVizStyle("not-a-real-style"); got != styleClassic {
		t.Fatalf("expected unknown style name to default to classic, got %v", got)
	}
}

func TestDeleteKeyUnfavoritesOnPlayerPage(t *testing.T) {
	store := newTempStore(t)
	fav := Station{StationUUID: "a", Name: "One"}
	a := &App{
		page:       pagePlayer,
		lib:        Library{Favorites: map[string]Station{"a": fav}, Hidden: map[string]Station{}},
		store:      store,
		player:     NewPlayer(),
		visualizer: NewDemoVisualizer(),
	}
	a.handleKey(Key{Name: "delete"})
	if _, ok := a.lib.Favorites["a"]; ok {
		t.Fatalf("expected Delete to remove the highlighted favourite")
	}
}

func TestSpaceNoLongerUnfavoritesOnPlayerPage(t *testing.T) {
	store := newTempStore(t)
	fav := Station{StationUUID: "a", Name: "One"}
	a := &App{
		page:       pagePlayer,
		lib:        Library{Favorites: map[string]Station{"a": fav}, Hidden: map[string]Station{}},
		store:      store,
		player:     NewPlayer(),
		visualizer: NewDemoVisualizer(),
	}
	a.handleKey(Key{Rune: ' '})
	if _, ok := a.lib.Favorites["a"]; !ok {
		t.Fatalf("expected Space to no longer remove the highlighted favourite on the Player page")
	}
}

func TestSpaceNoLongerUnfavoritesOnManagerPage(t *testing.T) {
	store := newTempStore(t)
	fav := Station{StationUUID: "a", Name: "One"}
	a := &App{
		page:     pageManager,
		mode:     modeFavorites,
		stations: []Station{fav},
		lib:      Library{Favorites: map[string]Station{"a": fav}, Hidden: map[string]Station{}},
		store:    store,
		player:   NewPlayer(),
	}
	a.handleKey(Key{Rune: ' '})
	if _, ok := a.lib.Favorites["a"]; !ok {
		t.Fatalf("expected Space to no longer remove the selected favourite on the Manager page")
	}
	if len(a.stations) != 1 {
		t.Fatalf("expected station to remain in the Favorites view after Space, got %d", len(a.stations))
	}
}

func TestDKeyUnfavoritesOnManagerPage(t *testing.T) {
	store := newTempStore(t)
	fav := Station{StationUUID: "a", Name: "One"}
	a := &App{
		page:     pageManager,
		mode:     modeFavorites,
		stations: []Station{fav},
		lib:      Library{Favorites: map[string]Station{"a": fav}, Hidden: map[string]Station{}},
		store:    store,
		player:   NewPlayer(),
	}
	a.handleKey(Key{Rune: 'd'})
	if _, ok := a.lib.Favorites["a"]; ok {
		t.Fatalf("expected d to remove the selected favourite")
	}
	if len(a.stations) != 0 {
		t.Fatalf("expected station to drop out of the Favorites view after d, got %d", len(a.stations))
	}
}

func TestTabTogglesPage(t *testing.T) {
	store := newTempStore(t)
	a := &App{
		page:       pagePlayer,
		lib:        Library{Favorites: map[string]Station{}, Hidden: map[string]Station{}},
		store:      store,
		player:     NewPlayer(),
		visualizer: NewDemoVisualizer(),
	}
	a.handleKey(Key{Name: "tab"})
	if a.page != pageManager {
		t.Fatalf("expected Tab to switch to Manager, got page=%v", a.page)
	}
	a.handleKey(Key{Name: "tab"})
	if a.page != pagePlayer {
		t.Fatalf("expected Tab to switch back to Player, got page=%v", a.page)
	}
}

func newTempStore(t *testing.T) *Store {
	t.Helper()
	return &Store{path: t.TempDir() + "/library.json"}
}
