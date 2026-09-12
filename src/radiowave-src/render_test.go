package main

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDedupeStations(t *testing.T) {
	in := []Station{
		{StationUUID: "a", Name: "One", URLResolved: "https://one"},
		{StationUUID: "a", Name: "One duplicate", URLResolved: "https://other"},
		{StationUUID: "b", Name: "URL duplicate", URLResolved: "https://one"},
		{StationUUID: "c", Name: "Two", URLResolved: "https://two"},
	}
	got := dedupeStations(in)
	if len(got) != 2 {
		t.Fatalf("expected 2 unique stations, got %d", len(got))
	}
}

func TestStationNameScroll(t *testing.T) {
	a := &App{}
	s := Station{StationUUID: "long", Name: "Café Radio"}
	start := time.Unix(100, 0)
	for _, tc := range []struct {
		elapsed time.Duration
		want    string
	}{
		{0, "Café "},
		{time.Second, "Café "},
		{1200 * time.Millisecond, "afé R"},
		{2160 * time.Millisecond, "Radio"},
		{3300 * time.Millisecond, "Radio"},
		{3600 * time.Millisecond, "Café "},
	} {
		if got := a.stationListName(s, 5, 0, true, start.Add(tc.elapsed)); got != tc.want {
			t.Errorf("at %s: got %q, want %q", tc.elapsed, got, tc.want)
		}
	}
	later := start.Add(5 * time.Second)
	if got := a.stationListName(s, 5, 0, false, later); got != "Café…" {
		t.Fatalf("unselected name = %q", got)
	}
	if got := a.stationListName(s, 5, 1, true, later); got != "Café " {
		t.Fatalf("new selection did not restart: %q", got)
	}
	if got := a.stationListName(s, 6, 1, true, later.Add(time.Second)); got != "Café R" {
		t.Fatalf("resize did not restart: %q", got)
	}
	if got := a.stationListName(s, 20, 1, true, later); got != s.Name {
		t.Fatalf("fitting name changed: %q", got)
	}
}

func TestRenderModes(t *testing.T) {
	fav := Station{StationUUID: "a", Name: "Test Station", URLResolved: "https://stream", Codec: "MP3", Bitrate: 128, Votes: 10}
	app := &App{
		width: 120, height: 34, mode: modeTrending,
		stations:    []Station{fav},
		lib:         Library{Favorites: map[string]Station{"a": fav}, Hidden: map[string]Station{}},
		player:      NewPlayer(),
		visualizer:  NewDemoVisualizer(),
		visualizers: []Visualizer{NewDemoVisualizer()},
	}

	// Player is the default page (page zero value) with the Normal layout.
	player := stripANSI(renderApp(app))
	if !strings.Contains(player, "PLAYER") || !strings.Contains(player, "Test Station") {
		t.Fatalf("player normal layout missing expected content:\n%s", player)
	}

	app.playerLayout = layoutVisual
	visual := stripANSI(renderApp(app))
	if strings.Contains(visual, "VISUALIZER") || !strings.Contains(visual, "PLAYER") {
		t.Fatalf("player visual layout has unexpected permanent visualizer label:\n%s", visual)
	}
	app.playerLayout = layoutNormal

	app.page = pageManager
	manager := stripANSI(renderApp(app))
	if !strings.Contains(manager, "MANAGER") || !strings.Contains(manager, "TRENDING") || !strings.Contains(manager, "Test Station") {
		t.Fatalf("manager renderer missing expected content:\n%s", manager)
	}
}

func TestIdleStartupOpensFavorites(t *testing.T) {
	fav := Station{StationUUID: "a", Name: "Saved Station", URLResolved: "https://stream"}
	app := &App{
		page: pagePlayer,
		mode: modeTrending,
		lib:  Library{Favorites: map[string]Station{"a": fav}, Hidden: map[string]Station{}},
	}

	app.openStartupFavorites()

	if app.page != pageManager || app.mode != modeFavorites {
		t.Fatalf("idle startup opened page=%v mode=%v, want Manager Favorites", app.page, app.mode)
	}
	if len(app.stations) != 1 || app.stations[0].StationUUID != fav.StationUUID {
		t.Fatalf("idle startup did not load favorites: %#v", app.stations)
	}
}

func TestPlaybackResumeRendersBeforeGoingLive(t *testing.T) {
	station := Station{StationUUID: "a", Name: "Saved Station", URLResolved: "https://stream"}
	app := &App{
		width: 80, height: 20, playerLayout: layoutVisual,
		player: NewPlayer(), visualizer: NewDemoVisualizer(),
	}

	app.preparePlaybackResume(&station)

	if app.now != &station || app.playing || !app.reconnecting {
		t.Fatalf("resume state = now:%p playing:%v reconnecting:%v", app.now, app.playing, app.reconnecting)
	}
	got := stripANSI(renderApp(app))
	if !strings.Contains(got, "Saved Station") || !strings.Contains(got, "connecting") {
		t.Fatalf("resuming player should immediately identify the station and connection state:\n%s", got)
	}
}

func TestCompactPlayerFits64x12AndFavoursVisualizer(t *testing.T) {
	station := Station{StationUUID: "a", Name: "Test Station", Codec: "MP3", Bitrate: 128, Tags: "rock"}
	app := &App{
		width: 64, height: 12, playerLayout: layoutNormal,
		now:        &station,
		trackTitle: "Test Track",
		player:     NewPlayer(),
		visualizer: NewDemoVisualizer(),
		lib:        Library{Favorites: map[string]Station{"a": station}},
	}

	got := stripANSI(renderApp(app))
	if strings.Contains(got, "needs at least") {
		t.Fatalf("64x12 should render the compact player:\n%s", got)
	}
	if strings.Contains(got, "FAVOURITES") {
		t.Fatalf("small normal layout should give favourites rows to the visualizer:\n%s", got)
	}
	if !strings.Contains(got, "Test Station") {
		t.Fatalf("compact player should preserve the station name:\n%s", got)
	}
	if lines := strings.Split(got, "\n"); len(lines) != 12 {
		t.Fatalf("64x12 player rendered %d lines, want 12", len(lines))
	}
	for i, line := range strings.Split(renderPlayerPage(app), "\n") {
		if width := visibleLen(line); width > 64 {
			t.Fatalf("64x12 player line %d is %d columns wide: %q", i+1, width, stripANSI(line))
		}
	}

	panel := renderNowPlayingPanel(app, 64, 10, true)
	// top + station + track + tags + five graph rows + bottom
	if got, want := len(panel)-5, 5; got != want {
		t.Fatalf("compact player rendered %d visualizer rows, want %d", got, want)
	}
}

func TestManagerFits64x12WithoutWrapping(t *testing.T) {
	app := &App{
		width: 64, height: 12, page: pageManager, mode: modeTrending,
		player: NewPlayer(),
		lib:    Library{Favorites: map[string]Station{}, Hidden: map[string]Station{}},
	}
	for i, line := range strings.Split(renderManager(app), "\n") {
		if width := visibleLen(line); width > 64 {
			t.Fatalf("64x12 manager line %d is %d columns wide: %q", i+1, width, stripANSI(line))
		}
	}
}

func TestMiniPlayerFits24x4(t *testing.T) {
	station := Station{StationUUID: "a", Name: "Tiny FM", Tags: "ambient"}
	app := &App{
		width: 24, height: 4, playerLayout: layoutNormal,
		now: &station, playing: true, trackTitle: "Small Hours",
		player: NewPlayer(), visualizer: NewDemoVisualizer(),
	}

	got := stripANSI(renderApp(app))
	for _, want := range []string{"MINI", "Tiny FM", "Small Hours", "80%"} {
		if !strings.Contains(got, want) {
			t.Fatalf("24x4 mini player missing %q:\n%s", want, got)
		}
	}
	if lines := strings.Split(got, "\n"); len(lines) != 4 {
		t.Fatalf("mini player rendered %d rows, want 4", len(lines))
	}
	for i, line := range strings.Split(renderPlayerMini(app), "\n") {
		if width := visibleLen(line); width > 24 {
			t.Fatalf("24x4 mini player line %d is %d columns wide: %q", i+1, width, stripANSI(line))
		}
	}
}

func TestMiniPlayerStaysWithinSmallTerminal(t *testing.T) {
	station := Station{StationUUID: "a", Name: "A station name long enough to need truncation", Tags: "ambient"}
	for _, size := range [][2]int{{24, 4}, {30, 5}, {40, 8}, {63, 11}} {
		app := &App{
			width: size[0], height: size[1], playerLayout: layoutVisual,
			now: &station, playing: true, trackTitle: "A similarly long piece of track metadata",
			player: NewPlayer(), visualizer: NewDemoVisualizer(),
		}
		rendered := renderApp(app)
		lines := strings.Split(rendered, "\n")
		if len(lines) != app.height {
			t.Fatalf("mini player at %dx%d rendered %d rows", app.width, app.height, len(lines))
		}
		for i, line := range lines {
			if width := visibleLen(line); width > app.width {
				t.Fatalf("mini player at %dx%d line %d is %d columns wide", app.width, app.height, i+1, width)
			}
		}
		if app.playerLayout != layoutVisual {
			t.Fatalf("automatic mini mode should not overwrite the selected layout")
		}
	}
}

func TestMinimumTerminalSizeIs24x4(t *testing.T) {
	app := &App{width: 23, height: 4, player: NewPlayer()}
	if got := stripANSI(renderApp(app)); !strings.Contains(got, "Minimum size: 24x4") {
		t.Fatalf("expected width below 24 to show size warning, got %q", got)
	}
	app.width, app.height = 24, 3
	if got := stripANSI(renderApp(app)); !strings.Contains(got, "Minimum size: 24x4") {
		t.Fatalf("expected height below 4 to show size warning, got %q", got)
	}
}

func TestMiniPlayerLayoutRoundTrips(t *testing.T) {
	if got := parsePlayerLayout(layoutMini.String()); got != layoutMini {
		t.Fatalf("mini layout parsed as %v", got)
	}
}

func TestManagerKeeps64x12Minimum(t *testing.T) {
	app := &App{width: 40, height: 8, page: pageManager, player: NewPlayer()}
	got := stripANSI(renderApp(app))
	if !strings.Contains(got, "Manager needs 64x12") || !strings.Contains(got, "Tab returns") {
		t.Fatalf("small Manager should direct the user back to mini player, got %q", got)
	}
}

func TestHeaderPlacesVolumeBeforeConnectionStatus(t *testing.T) {
	app := &App{width: 64, player: NewPlayer()}
	header := stripANSI(renderHeader(app, "PLAYER"))
	vol := strings.Index(header, "Vol 80%")
	status := strings.Index(header, "ONLINE")
	if vol < 0 || status < 0 || vol >= status {
		t.Fatalf("expected volume immediately before connection status, got %q", header)
	}
}

func TestVisualizerLabelIsTransientAndInsideGraph(t *testing.T) {
	app := &App{visualizer: NewDemoVisualizer(), vizLabelUntil: time.Now().Add(time.Second)}
	graph := stripANSI(strings.Join(renderVisualizerPanel(app, 5, 60), "\n"))
	if !strings.Contains(graph, visualizerLabel(app)) || strings.Contains(graph, "VISUALIZER") {
		t.Fatalf("expected only the transient visualizer name inside graph, got %q", graph)
	}

	app.vizLabelUntil = time.Now().Add(-time.Second)
	graph = stripANSI(strings.Join(renderVisualizerPanel(app, 5, 60), "\n"))
	if strings.Contains(graph, visualizerLabel(app)) {
		t.Fatalf("expected expired visualizer name to disappear, got %q", graph)
	}
}

func TestViewportFollowsCursor(t *testing.T) {
	stations := make([]Station, 50)
	for i := range stations {
		stations[i] = Station{StationUUID: fmt.Sprintf("s%02d", i), Name: fmt.Sprintf("Station %02d", i+1), URLResolved: fmt.Sprintf("https://stream/%02d", i)}
	}
	app := &App{height: 20, stations: stations, cursor: 20}
	app.ensureCursorVisible()
	if app.offset != 7 { // 14 visible rows, cursor 20 must be the last visible row.
		t.Fatalf("expected viewport offset 7, got %d", app.offset)
	}
	app.move(1)
	if app.offset != 8 {
		t.Fatalf("expected viewport to scroll to offset 8, got %d", app.offset)
	}
}

func TestPageStepTracksTerminalHeight(t *testing.T) {
	app := &App{height: 34, page: pageManager}
	if got, want := app.pageStep(), 27; got != want {
		t.Fatalf("expected page step %d, got %d", want, got)
	}
}

// TestPanelMathFillsTerminalHeightExactly checks the *underlying* panel
// arithmetic (bypassing renderApp's fitToHeight safety net, which would
// trivially mask a miscount) against the bug this guards: rendered output
// falling a fixed number of rows short of the actual terminal height,
// leaving blank rows at the bottom that don't shrink as the window does.
func TestPanelMathFillsTerminalHeightExactly(t *testing.T) {
	fav := Station{StationUUID: "a", Name: "Test Station", URLResolved: "https://stream"}
	for _, height := range []int{20, 24, 34, 50} {
		app := &App{
			width: 120, height: height, mode: modeTrending,
			stations:    []Station{fav},
			lib:         Library{Favorites: map[string]Station{"a": fav}, Hidden: map[string]Station{}},
			player:      NewPlayer(),
			visualizer:  NewDemoVisualizer(),
			visualizers: []Visualizer{NewDemoVisualizer()},
		}
		for _, layout := range []playerLayoutMode{layoutNormal, layoutVisual, layoutCompact, layoutMini} {
			app.playerLayout = layout
			if got := len(strings.Split(renderPlayerPage(app), "\n")); got != height {
				t.Fatalf("player layout %v at height %d: rendered %d lines, want %d", layout, height, got, height)
			}
		}
		if got := len(strings.Split(renderManager(app), "\n")); got != height {
			t.Fatalf("manager at height %d: rendered %d lines, want %d", height, got, height)
		}
	}
}

func TestPlayerPageStepTracksLayout(t *testing.T) {
	app := &App{height: 34, page: pagePlayer, playerLayout: layoutNormal}
	if got := app.pageStep(); got < 1 {
		t.Fatalf("expected positive player page step, got %d", got)
	}
	app.playerLayout = layoutVisual
	if got := app.pageStep(); got < 1 {
		t.Fatalf("expected player page step to stay positive in visual layout, got %d", got)
	}
}

func TestPlayerLayoutMetricsGrowsVisualizerNotList(t *testing.T) {
	short := &App{height: 34, page: pagePlayer, playerLayout: layoutNormal}
	tall := &App{height: 70, page: pagePlayer, playerLayout: layoutNormal}

	shortNowH, shortListRows := short.playerLayoutMetrics()
	tallNowH, tallListRows := tall.playerLayoutMetrics()

	if tallNowH <= shortNowH {
		t.Fatalf("expected visualizer panel to grow with terminal height: short=%d tall=%d", shortNowH, tallNowH)
	}
	listGrowth := tallListRows - shortListRows
	nowGrowth := tallNowH - shortNowH
	if listGrowth >= nowGrowth {
		t.Fatalf("expected extra height to favour the visualizer panel over the favourites list: listGrowth=%d nowGrowth=%d", listGrowth, nowGrowth)
	}
	if tallListRows > 12 {
		t.Fatalf("expected favourites list rows to stay capped, got %d", tallListRows)
	}
}

func TestVisualizerLabelMarksDemoOnly(t *testing.T) {
	demo := &App{visualizer: NewDemoVisualizer()}
	if got := visualizerLabel(demo); !strings.Contains(got, "demo") {
		t.Fatalf("expected demo visualizer label to be tagged demo, got %q", got)
	}
	real := &App{visualizer: &fakeRealVisualizer{}}
	if got := visualizerLabel(real); strings.Contains(got, "demo") {
		t.Fatalf("expected real visualizer label not to be tagged demo, got %q", got)
	}
}

type fakeRealVisualizer struct{}

func (fakeRealVisualizer) Start() error           { return nil }
func (fakeRealVisualizer) Stop()                  {}
func (fakeRealVisualizer) Levels(n int) []float64 { return make([]float64, n) }
func (fakeRealVisualizer) Name() string           { return "fake" }
func (fakeRealVisualizer) Demo() bool             { return false }

func TestAppendUniqueStationsAcrossPages(t *testing.T) {
	dst := []Station{{StationUUID: "a", Name: "One", URLResolved: "https://one"}}
	src := []Station{
		{StationUUID: "a", Name: "UUID duplicate", URLResolved: "https://other"},
		{StationUUID: "b", Name: "URL duplicate", URLResolved: "https://one"},
		{StationUUID: "c", Name: "Two", URLResolved: "https://two"},
	}
	got := appendUniqueStations(dst, src)
	if len(got) != 2 || got[1].StationUUID != "c" {
		t.Fatalf("unexpected merged station set: %#v", got)
	}
}
