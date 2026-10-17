package main

import (
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type fetchResult struct {
	mode     browseMode
	query    string
	offset   int
	stations []Station
	err      error
}

type App struct {
	width, height int
	page          pageKind

	mode            browseMode
	query           string
	stations        []Station
	cursor          int
	offset          int
	loading         bool
	errText         string
	inputMode       string
	inputBuffer     string
	help            bool
	nameScrollKey   string
	nameScrollSince time.Time

	configScreen        bool
	themeCursor         int
	themeEditing        bool
	themeEditRow        int
	themeEditBuf        string
	themeMessage        string
	themeError          string
	themeName           string
	themeHash           string
	themePicker         bool
	themeFiles          []themeFile
	themeFilter         string
	themePickCur        int
	themePickOff        int
	themePreviewErr     string
	themeOriginal       ThemePalette
	themeOriginalName   string
	themeOriginalError  string
	themeOriginalNotice string

	genrePicker  bool
	genreTags    []Tag
	genreLoading bool
	genreErr     string
	genreFilter  string
	genreCursor  int
	genreOffset  int

	playerCursor int
	playerOffset int
	playerLayout playerLayoutMode
	preBrowse    *Station

	now          *Station
	playing      bool
	reconnecting bool
	trackTitle   string

	rb            *RadioBrowser
	player        *Player
	store         *Store
	lib           Library
	visualizer    Visualizer
	visualizers   []Visualizer
	visIdx        int
	vizOrder      frequencyOrder
	effectLast    time.Time
	effectPhase   float64
	vizStyle      vizStyle
	vizPeaks      []float64
	vizHistory    [][]float64
	vizLabelUntil time.Time

	gfx          *kittyGfx // nil when pixel graphics are unavailable or off
	gfxProbe     gfxProbe
	pixScale     int
	cellW, cellH int
	pixRect      vizRect
	pix          pixState

	fetchCh        chan fetchResult
	tagsCh         chan tagsResult
	hasMore        bool
	loadingMore    bool
	nextPageOffset int
}

func main() {
	gfxWant, pixScale := "auto", 2
	for _, arg := range os.Args[1:] {
		switch {
		case arg == "--version" || arg == "-v" || arg == "-V":
			fmt.Println("radiowave", Version)
			return
		case arg == "--help" || arg == "-h":
			printUsage()
			return
		case strings.HasPrefix(arg, "--gfx="):
			gfxWant = strings.TrimPrefix(arg, "--gfx=")
			if gfxWant != "auto" && gfxWant != "shm" && gfxWant != "direct" && gfxWant != "off" {
				fmt.Fprintf(os.Stderr, "radiowave: --gfx must be auto, shm, direct or off, not %q\n", gfxWant)
				os.Exit(2)
			}
		case strings.HasPrefix(arg, "--pixel-scale="):
			n, err := strconv.Atoi(strings.TrimPrefix(arg, "--pixel-scale="))
			if err != nil || n < 1 || n > 4 {
				fmt.Fprintln(os.Stderr, "radiowave: --pixel-scale must be 1 to 4")
				os.Exit(2)
			}
			pixScale = n
		default:
			fmt.Fprintf(os.Stderr, "radiowave: unknown argument %q (try --help)\n", arg)
			os.Exit(2)
		}
	}

	store, err := NewStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, "radiowave:", err)
		os.Exit(1)
	}
	lib, err := store.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "radiowave: cannot load library:", err)
		os.Exit(1)
	}

	player := NewPlayer()
	player.volume = lib.State.Volume
	playerErr := player.Start()
	defer player.Close()
	if playerErr == nil && lib.State.Muted {
		_ = player.ToggleMute()
	}

	// CAVA (real audio analysis) leads the list when available; the demo
	// waveforms always follow as a fallback so a visualizer is never
	// unavailable just because cava isn't installed or can't reach an
	// audio source.
	var visualizers []Visualizer
	if cavaAvailable() {
		visualizers = append(visualizers, NewCavaVisualizer())
	}
	visualizers = append(visualizers, NewDemoVisualizer(), NewPulseDemoVisualizer())
	wantVisIdx := 0
	for i, v := range visualizers {
		if v.Name() == lib.State.Visualizer {
			wantVisIdx = i
		}
	}

	w, h := terminalSize()
	app := &App{
		width:        w,
		height:       h,
		page:         pagePlayer,
		mode:         modeTrending,
		playerLayout: parsePlayerLayout(lib.State.Layout),
		vizStyle:     parseVizStyle(lib.State.VizStyle),
		vizOrder:     parseFrequencyOrder(lib.State.VizOrder),
		rb:           NewRadioBrowser(),
		player:       player,
		store:        store,
		lib:          lib,
		visualizers:  visualizers,
		pixScale:     pixScale,
		fetchCh:      make(chan fetchResult, 4),
		tagsCh:       make(chan tagsResult, 1),
	}
	app.initTheme()
	app.selectVisualizer(wantVisIdx)
	defer func() {
		for _, v := range app.visualizers {
			v.Stop()
		}
	}()
	if playerErr != nil {
		app.errText = playerErr.Error()
	}

	term := &Terminal{}
	if err := term.Enter(); err != nil {
		fmt.Fprintln(os.Stderr, "radiowave:", err)
		os.Exit(1)
	}
	defer term.Leave()
	app.initGfx(gfxWant)
	defer app.closeGfx()

	done := make(chan struct{})
	defer close(done)
	keyCh := make(chan Key, 32)
	go readKeys(keyCh, done)

	sigCh := make(chan os.Signal, 4)
	signal.Notify(sigCh, syscall.SIGWINCH, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	resuming := lib.State.WasPlaying && lib.State.Station != nil
	if resuming {
		app.fetch(modeTrending, "")
		// Land straight in the big Visual layout on a resumed launch — the
		// same place `z` takes you — rather than making the user press it
		// after already knowing they're about to listen.
		app.playerLayout = layoutVisual
		app.preparePlaybackResume(lib.State.Station)
		app.syncPlayerCursorToNow()
		// Paint the restored station before asking mpv to reconnect. A stream
		// may spend several seconds buffering, but the terminal should still
		// show useful now-playing information during that wait.
		app.draw()
		app.playStation(lib.State.Station)
	} else {
		app.openStartupFavorites()
	}
	app.syncPlayerCursorToNow()

	redraw := time.NewTicker(80 * time.Millisecond)
	metadata := time.NewTicker(1 * time.Second)
	defer redraw.Stop()
	defer metadata.Stop()
	// Pixel styles animate on their own clock; the cell UI keeps its
	// 12.5 Hz redraw. A nil channel simply never fires.
	var pixelTick <-chan time.Time
	if app.gfx != nil {
		interval := time.Second / 60
		if app.gfx.tx == gfxDirect {
			interval = time.Second / 30
		}
		t := time.NewTicker(interval)
		defer t.Stop()
		pixelTick = t.C
	}

	app.draw()
	for {
		select {
		case key := <-keyCh:
			if app.handleKey(key) {
				return
			}
			app.draw()
		case res := <-app.fetchCh:
			if res.offset == 0 {
				app.loading = false
			} else {
				app.loadingMore = false
			}
			if res.err != nil {
				app.errText = res.err.Error()
			} else if app.mode == res.mode && app.query == res.query {
				app.errText = ""
				page := app.filterHidden(res.stations)
				app.hasMore = len(res.stations) >= 100
				app.nextPageOffset = res.offset + len(res.stations)
				if res.offset == 0 {
					app.stations = appendUniqueStations(nil, page)
					app.cursor, app.offset = 0, 0
				} else {
					app.stations = appendUniqueStations(app.stations, page)
				}
			}
			app.draw()
		case res := <-app.tagsCh:
			app.genreLoading = false
			if res.err != nil {
				app.genreErr = res.err.Error()
			} else {
				app.genreTags = res.tags
			}
			app.draw()
		case sig := <-sigCh:
			if sig == syscall.SIGWINCH {
				app.width, app.height = terminalSize()
				app.updateCellSize()
				app.ensureCursorVisible()
				app.ensurePlayerCursorVisible()
				app.ensureGenreCursorVisible()
				app.ensureThemePickerVisible()
				app.draw()
				continue
			}
			return
		case <-metadata.C:
			themeChanged := app.reloadTheme(false)
			if app.now != nil {
				title := app.player.Property("metadata/by-key/icy-title")
				if title == "" {
					title = app.player.Property("media-title")
				}
				if title != "" && title != app.now.Name {
					app.trackTitle = title
				}
			}
			if themeChanged {
				app.draw()
			}
		case <-pixelTick:
			app.drawPixels()
		case <-redraw.C:
			if app.page == pagePlayer {
				app.draw()
			}
		}
	}
}

func printUsage() {
	fmt.Println("radiowave", Version, "- terminal internet radio player")
	fmt.Println()
	fmt.Println("Usage: radiowave [options]")
	fmt.Println()
	fmt.Println("  -h, --help             show this help and exit")
	fmt.Println("  -V, -v, --version      show the version and exit")
	fmt.Println("      --gfx=MODE         pixel visualizers: auto (default), shm, direct, off")
	fmt.Println("      --pixel-scale=N    screen pixels per canvas pixel, 1-4 (default 2)")
	fmt.Println()
	fmt.Println("Run with no arguments to start the TUI. It reconnects to the last")
	fmt.Println("playing station automatically when playback was active on exit.")
	fmt.Println("Pixel styles (Neon, Nebula) need the kitty graphics protocol; other")
	fmt.Println("terminals keep the character-cell styles.")
}

func (a *App) fetch(mode browseMode, query string) {
	a.mode = mode
	a.query = query
	a.loading = true
	a.loadingMore = false
	a.hasMore = false
	a.nextPageOffset = 0
	a.errText = ""
	if mode == modeFavorites {
		a.stations = a.favoriteStations()
		a.cursor, a.offset = 0, 0
		a.loading = false
		return
	}
	if mode == modeHistory {
		a.stations = a.historyStations()
		a.cursor, a.offset = 0, 0
		a.loading = false
		return
	}
	if mode == modeHidden {
		a.stations = a.hiddenStations()
		a.cursor, a.offset = 0, 0
		a.loading = false
		return
	}
	go func() {
		stations, err := a.rb.StationsPage(mode, query, 0, 100)
		a.fetchCh <- fetchResult{mode: mode, query: query, offset: 0, stations: stations, err: err}
	}()
}

// openStartupFavorites gives an idle launch an immediately useful place to
// choose something to play. Resumed launches bypass this and stay in the
// Player's Visual layout.
func (a *App) openStartupFavorites() {
	a.page = pageManager
	a.fetch(modeFavorites, "")
}

// preparePlaybackResume hydrates the visible playback state before the
// backend is asked to reconnect. It deliberately does not mark playback live:
// that only happens once playStation has successfully handed the URL to mpv.
func (a *App) preparePlaybackResume(s *Station) {
	if s == nil {
		return
	}
	a.now = s
	a.trackTitle = ""
	a.playing = false
	a.reconnecting = true
}

func (a *App) fetchMore() {
	if a.loading || a.loadingMore || !a.hasMore || !a.mode.remote() {
		return
	}
	offset := a.nextPageOffset
	if offset < 1 {
		offset = len(a.stations)
	}
	a.loadingMore = true
	mode, query := a.mode, a.query
	go func() {
		stations, err := a.rb.StationsPage(mode, query, offset, 100)
		a.fetchCh <- fetchResult{mode: mode, query: query, offset: offset, stations: stations, err: err}
	}()
}

func (a *App) handleKey(k Key) bool {
	if a.themePicker {
		return a.handleThemePickerKey(k)
	}
	if a.configScreen {
		return a.handleThemeConfigKey(k)
	}
	if a.genrePicker {
		return a.handleGenrePickerKey(k)
	}
	if a.inputMode != "" {
		return a.handleInputKey(k)
	}
	if a.help {
		if k.Rune == '?' || k.Name == "esc" || k.Rune == 'q' {
			a.help = false
		}
		return false
	}
	if k.Name == "ctrl+c" || k.Rune == 'q' {
		return true
	}
	if k.Rune == '?' {
		a.help = true
		return false
	}
	if k.Rune == 'c' {
		a.openThemeConfig()
		return false
	}
	if k.Name == "tab" {
		a.togglePage()
		return false
	}
	if a.page == pagePlayer {
		return a.handlePlayerKey(k)
	}
	return a.handleManagerKey(k)
}

func (a *App) handlePlayerKey(k Key) bool {
	switch {
	case k.Name == "up" || k.Rune == 'k':
		a.movePlayer(-1)
	case k.Name == "down" || k.Rune == 'j':
		a.movePlayer(1)
	case k.Name == "pgup":
		a.movePlayer(-a.pageStep())
	case k.Name == "pgdn":
		a.movePlayer(a.pageStep())
	case k.Name == "home":
		a.playerCursor, a.playerOffset = 0, 0
	case k.Name == "end":
		favs := a.favoriteStations()
		if len(favs) > 0 {
			a.playerCursor = len(favs) - 1
			a.ensurePlayerCursorVisible()
		}
	case k.Name == "enter":
		a.playStation(a.playerSelected())
	case k.Name == "delete":
		a.togglePlayerFavorite()
	case k.Rune == 'v':
		a.cycleVizStyle(1)
	case k.Rune == 'V':
		a.cycleVizStyle(-1)
	case k.Rune == 'z':
		a.cyclePlayerLayout()
	case k.Rune == 's':
		a.stopPlayback()
	case k.Rune == 'm':
		a.toggleMute()
	case k.Rune == '+' || k.Rune == '=':
		a.adjustVolume(5)
	case k.Rune == '-':
		a.adjustVolume(-5)
	}
	return false
}

func (a *App) handleManagerKey(k Key) bool {
	switch {
	case k.Name == "up" || k.Rune == 'k':
		a.move(-1)
	case k.Name == "down" || k.Rune == 'j':
		a.move(1)
	case k.Name == "pgup":
		a.move(-a.pageStep())
	case k.Name == "pgdn":
		a.move(a.pageStep())
	case k.Name == "home":
		a.cursor = 0
		a.offset = 0
	case k.Name == "end":
		if len(a.stations) > 0 {
			a.cursor = len(a.stations) - 1
			a.ensureCursorVisible()
			a.fetchMore()
		}
	case k.Name == "enter":
		a.playSelected()
	case k.Rune == ' ':
		a.toggleFavorite()
	case k.Rune == 'd':
		a.removeFavorite()
	case k.Rune == 'x':
		a.hideSelected()
	case k.Rune == 't':
		a.fetch(modeTrending, "")
	case k.Rune == 'p':
		a.fetch(modePopular, "")
	case k.Rune == 'r':
		a.fetch(modeRandom, "")
	case k.Rune == 'f':
		a.fetch(modeFavorites, "")
	case k.Rune == 'h':
		a.fetch(modeHistory, "")
	case k.Rune == 'z':
		a.fetch(modeHidden, "")
	case k.Rune == '/':
		a.inputMode = "Search"
		a.inputBuffer = ""
	case k.Rune == 'g':
		a.openGenrePicker()
	case k.Name == "backspace":
		a.returnToPreBrowse()
	case k.Rune == 's':
		a.stopPlayback()
	case k.Rune == 'm':
		a.toggleMute()
	case k.Rune == '+' || k.Rune == '=':
		a.adjustVolume(5)
	case k.Rune == '-':
		a.adjustVolume(-5)
	}
	return false
}

func (a *App) handleInputKey(k Key) bool {
	switch k.Name {
	case "ctrl+c":
		return true
	case "esc":
		a.inputMode = ""
		a.inputBuffer = ""
	case "backspace":
		r := []rune(a.inputBuffer)
		if len(r) > 0 {
			a.inputBuffer = string(r[:len(r)-1])
		}
	case "enter":
		q := strings.TrimSpace(a.inputBuffer)
		a.inputMode = ""
		a.inputBuffer = ""
		if q != "" {
			a.fetch(modeSearch, q)
		}
	default:
		if k.Rune >= 32 {
			a.inputBuffer += string(k.Rune)
		}
	}
	return false
}

// togglePage switches between the Player and Manager pages without touching
// playback. Entering Manager remembers the station that was playing so
// Backspace can return to it after browsing elsewhere.
func (a *App) togglePage() {
	if a.page == pagePlayer {
		a.page = pageManager
		a.preBrowse = a.now
		return
	}
	a.page = pagePlayer
	a.syncPlayerCursorToNow()
}

func (a *App) syncPlayerCursorToNow() {
	if a.now == nil {
		return
	}
	favs := a.favoriteStations()
	for i, s := range favs {
		if s.StationUUID == a.now.StationUUID {
			a.playerCursor = i
			break
		}
	}
	a.ensurePlayerCursorVisible()
}

func (a *App) returnToPreBrowse() {
	if a.preBrowse == nil {
		return
	}
	a.playStation(a.preBrowse)
}

func (a *App) move(delta int) {
	if len(a.stations) == 0 {
		a.cursor = 0
		a.offset = 0
		return
	}
	a.cursor += delta
	if a.cursor < 0 {
		a.cursor = 0
	}
	if a.cursor >= len(a.stations) {
		a.cursor = len(a.stations) - 1
	}
	a.ensureCursorVisible()
	if len(a.stations)-a.cursor <= 10 {
		a.fetchMore()
	}
}

func (a *App) movePlayer(delta int) {
	favs := a.favoriteStations()
	if len(favs) == 0 {
		a.playerCursor = 0
		a.playerOffset = 0
		return
	}
	a.playerCursor += delta
	if a.playerCursor < 0 {
		a.playerCursor = 0
	}
	if a.playerCursor >= len(favs) {
		a.playerCursor = len(favs) - 1
	}
	a.ensurePlayerCursorVisible()
}

// visibleStationRows must track renderStations' own row budget exactly
// (usableH-4, where usableH is renderManager's a.height-2) — this is what
// PgUp/PgDn and cursor-visibility clamping use, so a mismatch here means
// the cursor can scroll to a row that isn't actually drawn.
func (a *App) visibleStationRows() int {
	rows := a.height - 6
	if rows < 1 {
		rows = 1
	}
	return rows
}

// playerListRows mirrors the favourites list height the renderer actually
// draws for the current Player layout, so keyboard paging and cursor
// clamping stay in sync with what is on screen.
func (a *App) playerListRows() int {
	_, rows := a.playerLayoutMetrics()
	return rows
}

// playerLayoutMetrics returns the now-playing panel's complete height and
// the favourites panel's content-row budget. In Normal their sum is
// height-4: header(1), footer(1), and favourites borders(2) are the fixed
// rows outside that sum. Visual, Compact, and Mini do not render favourites.
func (a *App) playerLayoutMetrics() (nowH, listRows int) {
	usable := a.height - 4
	if usable < 1 {
		usable = 1
	}
	switch a.playerLayout {
	case layoutVisual, layoutCompact, layoutMini:
		nowH = usable
		listRows = 0
		return
	default:
		// Extra terminal height grows the visualizer panel, not the
		// favourites list. Cap the list's share once it is useful, and
		// never reserve blank rows for favourites that do not exist.
		listCap := usable / 3
		if listCap > 12 {
			listCap = 12
		}
		if listCap < 5 {
			listCap = 5
		}
		listRows = len(a.favoriteStations())
		if listRows < 1 {
			listRows = 1 // the empty-state message
		}
		if listRows > listCap {
			listRows = listCap
		}
		nowH = usable - listRows
		if nowH < 10 {
			nowH = min(usable, 10)
			listRows = usable - nowH
		}
	}
	return
}

func (a *App) pageStep() int {
	rows := a.visibleStationRows()
	if a.page == pagePlayer {
		rows = a.playerListRows()
	}
	step := rows - 1
	if step < 1 {
		step = 1
	}
	return step
}

func (a *App) ensureCursorVisible() {
	rows := a.visibleStationRows()
	if a.cursor < a.offset {
		a.offset = a.cursor
	}
	if a.cursor >= a.offset+rows {
		a.offset = a.cursor - rows + 1
	}
	maxOffset := max(0, len(a.stations)-rows)
	if a.offset > maxOffset {
		a.offset = maxOffset
	}
	if a.offset < 0 {
		a.offset = 0
	}
}

func (a *App) ensurePlayerCursorVisible() {
	rows := a.playerListRows()
	if rows < 1 {
		rows = 1
	}
	total := len(a.favoriteStations())
	if a.playerCursor < a.playerOffset {
		a.playerOffset = a.playerCursor
	}
	if a.playerCursor >= a.playerOffset+rows {
		a.playerOffset = a.playerCursor - rows + 1
	}
	maxOffset := max(0, total-rows)
	if a.playerOffset > maxOffset {
		a.playerOffset = maxOffset
	}
	if a.playerOffset < 0 {
		a.playerOffset = 0
	}
}

func appendUniqueStations(dst, src []Station) []Station {
	seenUUID := make(map[string]bool, len(dst)+len(src))
	seenURL := make(map[string]bool, len(dst)+len(src))
	for _, s := range dst {
		seenUUID[s.StationUUID] = true
		seenURL[s.URLResolved] = true
	}
	for _, s := range src {
		if s.StationUUID == "" || s.URLResolved == "" || seenUUID[s.StationUUID] || seenURL[s.URLResolved] {
			continue
		}
		seenUUID[s.StationUUID] = true
		seenURL[s.URLResolved] = true
		dst = append(dst, s)
	}
	return dst
}

func (a *App) selected() *Station {
	if a.cursor < 0 || a.cursor >= len(a.stations) {
		return nil
	}
	s := a.stations[a.cursor]
	return &s
}

func (a *App) playerSelected() *Station {
	favs := a.favoriteStations()
	if a.playerCursor < 0 || a.playerCursor >= len(favs) {
		return nil
	}
	s := favs[a.playerCursor]
	return &s
}

// playStation is the single entry point for starting playback, used by both
// pages and by startup resume, so Player and Manager always share one
// playback state.
func (a *App) playStation(s *Station) {
	if s == nil {
		return
	}
	if err := a.player.Play(s.URLResolved); err != nil {
		a.reconnecting = false
		a.errText = err.Error()
		return
	}
	a.now = s
	a.trackTitle = ""
	a.errText = ""
	a.playing = true
	a.reconnecting = false
	a.lib.History = append([]HistoryEntry{{Station: *s, PlayedAt: time.Now()}}, a.lib.History...)
	if len(a.lib.History) > 500 {
		a.lib.History = a.lib.History[:500]
	}
	a.persistPlayback()
	go a.rb.CountClick(s.StationUUID)
}

func (a *App) playSelected() {
	a.playStation(a.selected())
}

func (a *App) stopPlayback() {
	if err := a.player.Stop(); err != nil {
		a.errText = err.Error()
		return
	}
	a.trackTitle = ""
	a.playing = false
	a.reconnecting = false
	a.persistPlayback()
}

func (a *App) toggleMute() {
	if err := a.player.ToggleMute(); err != nil {
		a.errText = err.Error()
		return
	}
	a.persistPlayback()
}

func (a *App) adjustVolume(delta int) {
	if err := a.player.ChangeVolume(delta); err != nil {
		a.errText = err.Error()
		return
	}
	a.persistPlayback()
}

// cycleVizStyle changes how the live levels are drawn (classic bars,
// mirrored, VU meters, ...), not which data source produces them — the
// data source (cava vs. demo fallback) is chosen automatically at startup.
func (a *App) cycleVizStyle(dir int) {
	n := int(vizStyleCount)
	s := int(a.vizStyle)
	for i := 0; i < n; i++ {
		s = ((s+dir)%n + n) % n
		if a.vizStyleAvailable(vizStyle(s)) {
			break
		}
	}
	a.vizStyle = vizStyle(s)
	a.vizHistory = nil
	a.vizLabelUntil = time.Now().Add(2 * time.Second)
	a.persistPlayback()
}

// selectVisualizer starts the visualizer at idx and, if it fails (e.g. cava
// can't reach an audio source), tries the rest of the list in order so a
// visualizer is always left running rather than silently dead.
func (a *App) selectVisualizer(idx int) {
	n := len(a.visualizers)
	if n == 0 {
		return
	}
	for i := 0; i < n; i++ {
		try := ((idx+i)%n + n) % n
		if err := a.visualizers[try].Start(); err == nil {
			a.visIdx = try
			a.visualizer = a.visualizers[try]
			a.vizLabelUntil = time.Now().Add(2 * time.Second)
			return
		}
	}
}

func (a *App) cyclePlayerLayout() {
	a.playerLayout = (a.playerLayout + 1) % layoutCount
	a.ensurePlayerCursorVisible()
	a.persistPlayback()
}

// persistPlayback snapshots playback + preference state and saves it
// immediately, so a crash or unclean exit still resumes correctly next time.
func (a *App) persistPlayback() {
	state := PlaybackState{
		WasPlaying: a.playing,
		Volume:     a.player.volume,
		Muted:      a.player.muted,
		Visualizer: a.visualizer.Name(),
		Layout:     a.playerLayout.String(),
		VizStyle:   a.vizStyle.String(),
		VizOrder:   a.vizOrder.String(),
	}
	if a.now != nil {
		snapshot := *a.now
		state.Station = &snapshot
	}
	a.lib.State = state
	if err := a.store.Save(a.lib); err != nil {
		a.errText = err.Error()
	}
}

func (a *App) toggleFavorite() {
	s := a.selected()
	if s == nil {
		return
	}
	if _, ok := a.lib.Favorites[s.StationUUID]; ok {
		return
	}
	a.lib.Favorites[s.StationUUID] = *s
	if err := a.store.Save(a.lib); err != nil {
		a.errText = err.Error()
	}
}

// removeFavorite un-favorites the selected station. It is a separate key
// from toggleFavorite so that Space can never remove a favourite by
// accident while browsing a list that is entirely favourites already.
func (a *App) removeFavorite() {
	s := a.selected()
	if s == nil {
		return
	}
	if _, ok := a.lib.Favorites[s.StationUUID]; !ok {
		return
	}
	delete(a.lib.Favorites, s.StationUUID)
	if err := a.store.Save(a.lib); err != nil {
		a.errText = err.Error()
	}
	if a.mode == modeFavorites {
		a.stations = a.favoriteStations()
		if a.cursor >= len(a.stations) {
			a.cursor = max(0, len(a.stations)-1)
		}
	}
}

func (a *App) togglePlayerFavorite() {
	s := a.playerSelected()
	if s == nil {
		return
	}
	delete(a.lib.Favorites, s.StationUUID)
	if err := a.store.Save(a.lib); err != nil {
		a.errText = err.Error()
		return
	}
	if a.playerCursor >= len(a.favoriteStations()) {
		a.playerCursor = max(0, len(a.favoriteStations())-1)
	}
	a.ensurePlayerCursorVisible()
}

func (a *App) hideSelected() {
	s := a.selected()
	if s == nil {
		return
	}
	if a.mode == modeHidden {
		delete(a.lib.Hidden, s.StationUUID)
	} else {
		a.lib.Hidden[s.StationUUID] = *s
		delete(a.lib.Favorites, s.StationUUID)
	}
	if err := a.store.Save(a.lib); err != nil {
		a.errText = err.Error()
		return
	}
	a.stations = append(a.stations[:a.cursor], a.stations[a.cursor+1:]...)
	if a.cursor >= len(a.stations) {
		a.cursor = max(0, len(a.stations)-1)
	}
}

func (a *App) filterHidden(in []Station) []Station {
	out := make([]Station, 0, len(in))
	for _, s := range in {
		if _, hidden := a.lib.Hidden[s.StationUUID]; !hidden {
			out = append(out, s)
		}
	}
	return out
}

func (a *App) favoriteStations() []Station {
	out := make([]Station, 0, len(a.lib.Favorites))
	for _, s := range a.lib.Favorites {
		if _, hidden := a.lib.Hidden[s.StationUUID]; !hidden {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

func (a *App) historyStations() []Station {
	seen := map[string]bool{}
	out := make([]Station, 0, len(a.lib.History))
	for _, h := range a.lib.History {
		s := h.Station
		if _, hidden := a.lib.Hidden[s.StationUUID]; seen[s.StationUUID] || hidden {
			continue
		}
		seen[s.StationUUID] = true
		out = append(out, s)
	}
	return out
}

func (a *App) hiddenStations() []Station {
	out := make([]Station, 0, len(a.lib.Hidden))
	for _, s := range a.lib.Hidden {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}
