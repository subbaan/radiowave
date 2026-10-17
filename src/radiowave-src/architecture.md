# Radiowave Architecture

**Covers:** v0.7.1. Update this file whenever a change makes a description
below materially incorrect — it should explain how the program fits
together, not narrate every commit.

## Package layout

Radiowave is a single Go package (`main`) split by concern into separate
files rather than sub-packages, matching its hobby-project scope:

```text
main.go        Application state, event loop, key handling, page/playback logic
render.go      True-colour TUI renderer for the Player and Manager pages
types.go       Station/Library/PlaybackState data model, mode/layout enums
store.go       Atomic local JSON persistence
api.go         Radio Browser HTTP client and mirror discovery
player.go      mpv process lifecycle and JSON IPC control
visualizer.go  Visualizer interface and demo waveform implementations
terminal.go    Raw-mode terminal setup, sizing, and key decoding
theme.go       Palette model, Kitty/Base16/custom import, and live reload
themescreen.go Theme settings and live-filtering directory browser
kgfx.go        Kitty graphics protocol: capability probe, shm/direct transport
pixcanvas.go   Additive-light software framebuffer, warp, RGBA tone-mapping
pixviz.go      Pixel visualizer styles (Neon, Nebula, Spectrogram) and App glue
```

## Application model

`App` (main.go) is the single mutable state struct. There is one instance,
constructed in `main()` and mutated by the event loop — no framework, no
message-passing architecture. This is intentional: the state is small enough
that direct mutation plus a render-after-every-event loop stays easy to
reason about.

### Pages vs. playback

`App.page` (`pagePlayer` / `pageManager`) selects which screen is drawn and
which key map (`handlePlayerKey` / `handleManagerKey`) is active. It does
**not** gate playback. Playback lives in `App.now`, `App.playing`,
`App.trackTitle`, and the single `*Player` instance — all reachable and
mutated identically from either page. `togglePage` (bound to `Tab`) only
changes `App.page`; it never touches the player. This is what makes
"switching pages never interrupts a stream" true by construction rather than
by convention.

`App.playStation(*Station)` is the single entry point that starts playback.
Manager's `Enter`, Player's `Enter`, Backspace's pre-browse return, and
startup auto-resume all call it, so there is exactly one code path that can
put a station on the air and exactly one place playback gets recorded into
history and persisted.

Startup branches on the persisted `WasPlaying` flag. A resumable station
opens Player/Visual and reconnects through `playStation`; otherwise
`openStartupFavorites` opens Manager/Favorites without making a remote fetch.

### Manager's browsing state vs. Player's favourites state

Manager keeps `cursor`/`offset`/`stations`/`mode`/`query` — a single active
list that gets replaced wholesale by `fetch()` when the user switches
Browse category. Player keeps its own `playerCursor`/`playerOffset` over
`favoriteStations()` (recomputed on demand from `App.lib.Favorites`, not
cached). The two are deliberately independent: Player always shows the full
favourites list regardless of what Manager is currently browsing, and moving
around one page's list can't disturb the other's scroll position.

`pageStep()` and the cursor-visibility helpers are shared where the math is
identical (`ensureCursorVisible` for Manager, `ensurePlayerCursorVisible` for
Player), but `pageStep()` itself branches on `App.page` because the two
pages' list heights come from different layout math
(`visibleStationRows()` vs. `playerLayoutMetrics()`).

### Pre-browse return

`App.preBrowse` is set to `App.now` the moment `togglePage()` moves from
Player to Manager. It is intentionally session-only (not persisted) — it
answers "what was playing right before I started browsing this session",
not "what was playing last time". `Backspace` in Manager calls
`playStation(a.preBrowse)`.

## Playback architecture

`Player` (player.go) wraps a single long-lived `mpv --idle` process,
controlled entirely over its JSON IPC unix socket (`--input-ipc-server`).
Radiowave never re-execs mpv per station; `Play()` issues `loadfile ...
replace` against the running instance. This keeps station switches instant
and avoids reinitializing the audio device on every change.

`Player` exposes imperative methods (`Play`, `Stop`, `ToggleMute`,
`ChangeVolume`, `Property`) rather than a polled `State()` snapshot. `App`
tracks the semantic playback state itself (`now`, `playing`, `trackTitle`)
because mpv's own state (buffering, codec, ICY title) is fetched
opportunistically on a 1s ticker via `Property()`, not treated as the
source of truth for "is a station selected/favourited/queued".

`Player.command()` writes a JSON command to the socket and, when a response
matters (`Property`, not fire-and-forget commands like `Play`/`Stop`), reads
one line back under a 500ms deadline. This bounds every IPC call so a wedged
mpv can't stall the UI event loop.

## Radio Browser integration

`RadioBrowser` (api.go) resolves live mirrors via reverse DNS on
`all.api.radio-browser.info` at startup, falling back to two hardcoded hosts
if discovery yields nothing. Every request iterates the mirror list in order
and fails over on error/non-2xx, so one dead mirror degrades rather than
breaks discovery.

All discovery modes (`Trending`, `Popular`, `Random`, `Search`, `Genre`) go
through one endpoint (`StationsPage`) with `browseMode`-specific query
parameters — there's no per-mode HTTP method, keeping the addition of a new
browse mode a matter of extending the mode enum and its `switch` cases in
`api.go` and `main.go`'s Manager key map.

Pagination is offset-based; `App.fetchMore()` triggers the next page when
the cursor is within 10 rows of the loaded end. `appendUniqueStations` /
`dedupeStations` filter by both UUID and resolved URL so the same station
served under two different UUIDs (a known Radio Browser quirk) doesn't
appear twice.

### Genre picker

`RadioBrowser.Tags()` hits `/json/tags` once (Radio Browser caps this at
1000 entries server-side — small enough to fetch whole and keep in memory
for the session rather than re-querying per keystroke). `App.genreTags`
caches the result; `openGenrePicker()` (genrepicker.go) only fires the
fetch on first open, via the same async-goroutine-plus-channel pattern as
station fetches (`tagsCh` alongside `fetchCh`), so the UI stays responsive
while it loads.

Filtering is a plain client-side substring match
(`filteredGenreTags()`) — no server round-trip per keystroke, which is
both simpler and immediate at this catalogue size. Selecting a tag
(`selectGenre()`) just calls the existing `App.fetch(modeGenre, tag.Name)`
— the picker only replaces *how a genre name gets chosen*, not the
browsing path once one is.

The picker captures all key input while open (`App.handleKey` checks
`a.genrePicker` before the help/input-mode/page dispatch), the same
pattern as the help overlay and the search/genre text-input mode it
replaced for genre — see "TUI rendering and input model" below for why
that ordering matters.

## Storage model

`Store` (store.go) persists one JSON file
(`$XDG_CONFIG_HOME/radiowave/library.json`), written atomically
(temp file + rename) on every mutation — there is no debounce or batching.
For a hobby-scale library (hundreds of stations, low write frequency even
during a `+`/`-` volume-adjustment burst) this is simpler and safer than a
database, per the project's stated preference to avoid SQLite until
relational features (collections, user tags, richer history) actually need
it.

`Library.State` (`PlaybackState`, types.go) is the persisted playback
snapshot: last station (a full field snapshot, not just a UUID — so resume
doesn't depend on the station still existing in Favorites/History), whether
playback was active, volume, mute, visualizer name, and layout. `App`
rebuilds and saves this snapshot via `persistPlayback()` after every
playback-affecting mutation (play, stop, volume, mute, visualizer cycle,
layout cycle) — not just on shutdown — so an unclean exit still resumes
correctly. `Store.Load()`'s `withDefaults()` fills in sane defaults
(volume 80, layout "normal") for a first run or a file written by an older
schema version.

`Library.Theme` separately persists the colour source plus Kitty/custom/folder
paths, so `persistPlayback()` can continue rebuilding `Library.State` without
discarding appearance preferences. `theme.go` translates all imported formats
into one semantic `ThemePalette`; applying it updates the renderer's shared ANSI
role strings. Kitty and custom-file sources are content-hashed once per second,
which catches in-place edits, symlink changes, and atomic file replacement.

The Settings screen (`themescreen.go`, global `c`) owns path editing and source
selection. Its recursive folder browser filters locally like the genre picker.
Raw Base16 YAML and generated i3 `set $baseXX` files use the same semantic
mapping, while Kitty's foreground/background/ANSI slots map to equivalent UI
roles.

## TUI rendering and input model

Rendering is a pure function: `renderApp(*App) string`. Every key event (and
every visualizer-redraw tick) calls it and does one `fmt.Print` of the whole
frame, prefixed with `\x1b[H\x1b[2J` (home + clear). There's no diffing —
simplicity was chosen over flicker-minimization, and in practice terminal
double-buffering makes this unnoticeable at the ~80ms redraw rate used while
any Player layout is on screen. Manager redraws only in response to input or
data arriving, not on the visualizer ticker.

The full multi-panel UI has a 64×12 floor, but `renderApp` routes a smaller
Player page to the borderless Mini renderer down to 24×4. This responsive
fallback does not mutate `playerLayout`: growing the terminal restores the
user's selected layout. Mini is also a fourth explicit, persisted layout.
Manager and modal screens keep the full floor and render a concise size notice
when the terminal is smaller.

`terminal.go` puts the tty into raw mode via `stty raw -echo`, then
explicitly re-enables `opost`/`onlcr` — raw mode disables output
post-processing along with canonical input, and without restoring those two,
every `\n` the renderer emits fails to return to column 0. This was a real
regression found in early prototyping (see `terminal_test.go`) and is worth
preserving as a comment-level warning for anyone touching `rawModeArgs()`.
`Terminal.Enter`/`Leave` are strictly paired via `defer` in `main()`, and
`Leave` restores the saved `stty` state unconditionally — including on the
signal-triggered exit path (`SIGINT`/`SIGTERM`), not just the `q` path.

Key decoding (`readKeys`) is a hand-rolled byte-level reader recognizing a
handful of ANSI escape sequences (arrows, PgUp/PgDn, Home/End) plus raw
printable-ASCII passthrough. It intentionally does not attempt full
terminfo-based decoding — the escape sequences handled cover every binding
Radiowave currently uses.

## Visualizer pipeline

`Visualizer` (visualizer.go) is deliberately minimal —
`Start()`/`Stop()`/`Levels(n) []float64`/`Name() string`/`Demo() bool` — so
alternative backends can implement the same interface without any renderer
change. `App.visualizers` holds the data-source candidates (cava, then the
demo fallbacks); `Name()` is persisted so the chosen one survives a
restart. Note that `v`/`V` do **not** cycle this list — see "Visualizer
style" below for what they actually do.

`CavaVisualizer` (cava.go) is the real backend: it runs `cava -p <tmp
config>` in raw-output mode (`data_format = ascii`, one semicolon-delimited
frame per line on stdout) and a background goroutine parses each frame into
`[]float64`, guarded by a mutex against concurrent `Levels()` reads from the
render path. It requests a fixed `cavaBars` (128) resolution from cava and
resamples (nearest-neighbour) to whatever width `Levels(n)` is asked for —
cava itself has no notion of the caller's terminal width. It captures
whatever is playing through the system's default audio sink (cava's own
`pulse`/`auto` input), not anything specific to mpv, so no special playback
wiring was required. `cavaAvailable()` (an `exec.LookPath` check) gates
whether it's added to `App.visualizers` at all — on a machine without cava
installed, the list is just the demo waveforms.

`App.selectVisualizer(idx)` (main.go) is what actually activates a
visualizer: it calls `Start()` and, on error (cava installed but can't
reach an audio source, e.g. no default sink), tries the rest of the list in
index order rather than leaving the display dead. `NewDemoVisualizer` /
`NewPulseDemoVisualizer` are synthetic waveforms whose `Start()` always
succeeds, so they're the guaranteed fallback at the end of that search.

`Demo() bool` exists specifically so the renderer can label demo data
"demo" (Player panel header, Visual layout title, help text) and never
apply that label to `CavaVisualizer` — per the project's explicit rule,
demo data must never be presented as if it were real audio analysis, and
real analysis must never look like a demo either. `visualizerLabel()` in
render.go is the single place that decision is made.

### Visualizer style

`vizStyle` (vizstyle.go) is a second, independent axis from the data
source above: it's *how* `Levels()` output gets drawn — classic bars,
mirrored, blocks, peak-hold, VU meters, oscilloscope, waterfall,
plasma, tunnel, or ribbons — and it's what `v`/`V` (`App.cycleVizStyle`) actually cycle.
Splitting this from `Visualizer` means every style automatically works
with both cava and the demo fallback, and adding a style never touches
`cava.go` or `visualizer.go`.

CAVA explicitly supplies mono averaged, low-to-high analysis; playback
remains stereo. `frequencyOrder` is saved as `state.viz_order`, defaults
to centred for older libraries, and reorders spectrum data once before
rendering. Mirrored reflects bars vertically, Blocks groups adjacent bars
(`chunkyLevels`), and Peaks is a light variant
(`peakSpectrumRows`) layered on the same `verticalSpectrum()` used by
Classic — cheap, and they inherit its correctness for free. VU and
Oscilloscope are their own renderers (`vuMeterRows`, `oscilloscopeRows`)
since their layout is structurally different (fixed-zone meters; a
connected line trace rather than bottom-anchored bars). Peak-hold and
Waterfall need state that must outlive a single render call — decaying
peaks (`App.vizPeaks`) and scrolling frame history (`App.vizHistory`) live
on `App`, not as locals in the render functions, specifically so they stay
continuous across a Normal ⟷ Visual layout switch instead of resetting
every frame. `App.renderVisualizer(rows, cols)` is the single dispatch
point all Player layouts call into, which is what keeps that state
shared correctly between them.

`effects.go` renders Plasma, Tunnel and Ribbons on a grid capped at
160×60 cells, expanded to the panel size. Three frequency-band averages
control motion and shape. The existing 80 ms redraw drives animation;
there are no extra timers or rendering processes. Settings rows scroll
to keep the selected theme or visualizer preference visible at 64×12.

`oscilloscopeRows` is worth flagging as a precedent: it once drew two
visible characters per data point (glyph + trailing gap, matching the bar
styles' spacing convention) while still being asked for one data point per
column of width — silently doubling its rendered width and getting
truncated by `pad()`. Any new style that emits more than one visible
character per `Levels()` sample must size its `Levels(n)` request
accordingly (halve it, matching `verticalSpectrum`'s `maxBars := width/2`)
rather than assume `n == width`.

`vuMeterRows` had the opposite mistake: a hardcoded `barWidth = 3` meant
the meters used a small, constant slice of the box regardless of terminal
width — correct-looking at a typical size, silently wrong on a wide one.
Any style with a small, fixed number of "cells" (VU's meters; a future
style with big fixed-size blocks) needs to compute cell width from the
`width` it's given, not hardcode one — `verticalSpectrum` and the
bar-per-column styles don't have this failure mode since a *column's*
width is inherently 1 character, only a cell spanning multiple characters
can drift from the available space this way.

### Pixel styles

Neon, Nebula and Spectrogram (`pixviz.go`) are drawn as real RGBA images through the
kitty graphics protocol instead of characters. The cell renderer still
owns layout: when a pixel style is active, `renderVisualizerPanel` returns
blank rows and records the panel's screen rectangle in `App.pixRect`
(1-based cells; reset at the top of every `renderApp`, so overlay screens
and the Manager simply leave it empty and the image is deleted). The image
is placed there at `z=-1`, above cell backgrounds but below text, so the
transient style label still draws over it.

`initGfx` probes once at startup, in raw mode before the key reader
starts (graphics query for direct and shm transmission, CSI 16 t for cell
size, DA1 to bound the wait). No answer, `--gfx=off`, or `$TMUX` without
a forced mode leaves `App.gfx` nil; pixel styles are then skipped by
`cycleVizStyle` and a remembered one draws as Classic via
`activeVizStyle()`. Frames are sent with `q=2`, so the terminal never
writes replies that the key reader would misread as Esc.

Frames render at a fixed 60 Hz (`drawPixels`, 30 Hz on the direct
transport) independent of the 12.5 Hz cell redraw. Kitty deletes images
on ED 2, so in graphics mode `draw()` swaps the full-screen clear for a
whole-line erase before each line; the image then survives cell redraws
and is only re-sent when its rectangle or the cell size changes
(`kittyGfx.shown`). Cells and image go out inside one synchronized update
(`?2026`).

`pixCanvas` stores additive light as float32 RGB on black and tone-maps
through a LUT on output; brightness doubles as alpha, so black is
transparent and the theme background shows through. Canvas pixels are
`--pixel-scale` screen pixels square (default 2), upscaled nearest-neighbour
straight into the shm mapping. `pixCanvasBudget` caps canvas size, raising
the scale on very large panels instead of the CPU cost. Neon draws only
its bars into a scratch frame and max-blends it over the decaying previous
frame (phosphor trails without additive blow-out); Nebula is one bilinear
zoom/rotate/decay warp of the previous frame plus a ring of soft dots;
Spectrogram memmoves the canvas left and paints only the new columns.
Beat sparks (`stepSparks`, capped at `maxSparks`) are shared: Neon draws
them into its scratch frame so they inherit its phosphor trails, Nebula
into the feedback canvas so the warp turns them into streaks. Neon's
reflection ripple is a per-row sideways offset computed once per frame.
Colours come from the active palette every frame, so theme changes apply
live.

## Constraints and decisions worth preserving

- **No UI framework.** Direct string-building renderer + a single `App`
  struct was chosen over e.g. Bubble Tea to keep the dependency surface at
  zero beyond the standard library. Revisit only if render.go's manual
  column/box layout math becomes the bottleneck to further UI growth.
- **One player process for the app's lifetime.** Station switches are
  `loadfile replace` on the existing mpv instance, never a new mpv spawn.
  Any future multi-stream feature (e.g. crossfade) would need to reconsider
  this.
- **Playback state is independent of page state.** Do not gate any playback
  mutation on `App.page` — that coupling is exactly what this milestone
  removed (the old "big visualizer" mode lived inside the single-page
  browser and implicitly tied visualizer display to the browse view; it's
  now Player's Visual layout, reachable regardless of what Manager is
  doing).
- **Persist on mutation, not on timer or on exit.** Every playback-state
  change calls `persistPlayback()` synchronously. This trades a small amount
  of write volume for correctness under unclean exits, which the project
  treats as the common case for a terminal app, not the exception.
