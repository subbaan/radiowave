# Changelog

All notable user-visible changes to Radiowave are recorded here, grouped by
version. This is a hobby project changelog — concise entries, not a diff
narration.

## v0.7.1

### Added

- Spectrogram pixel style: a scrolling frequency-over-time heat map in the
  theme's colours.
- Beat sparks: Neon throws particles from its tallest bars, Nebula flings
  them outward from the ring as comet streaks.
- Neon's floor reflection ripples with the bass.
- README hero clip of the RGBA visualizer running in a translucent kitty.

## v0.7.0

### Added

- Pixel visualizers via the Kitty graphics protocol: real RGBA frames at
  60 fps with alpha blending over the theme background, sent through
  shared memory (direct transmission as a fallback). Character-cell styles
  remain the fallback in other terminals and tmux.
- Neon: glowing gradient bars with a hot tip, falling peak caps, phosphor
  trails and a floor reflection.
- Nebula: a Milkdrop-style zoom/rotate feedback swirl around a
  beat-reactive spectrum ring.
- `--gfx=auto|shm|direct|off` and `--pixel-scale=N` options; `-V` now
  prints the version too.

### Changed

- CAVA analysis runs at 60 fps (was 30) to feed the pixel styles.

## v0.6.3

### Changed

- Double the Ribbons line thickness, including its audio-reactive variation.

## v0.6.2

### Changed

- Give Ribbons stronger bass-driven movement, larger midrange bends and
  treble ripples, with each ribbon displaced by its own frequency band.

## v0.6.1

### Fixed

- Ribbons now spans the full visualizer height even at low bass levels,
  while retaining audio-reactive speed, shape, thickness and brightness.

## v0.6.0

### Added

- Persisted Frequency order choice in Settings: centred bass or bass on
  the left, highs on the right, independent of playback stereo.
- Plasma, Tunnel and Ribbons: lightweight reactive terminal effects.
- Visualizer style selection in Settings, with scrolling on short screens.

### Changed

- Normalize CAVA analysis to mono averaged low-to-high data before arranging
  spectrum columns. Mirrored now reflects bars vertically.
- Preserve the full frequency range in narrow spectrum and scope panels.

## v0.5.5

### Added

- Scroll overflowing names in the highlighted station row in Manager and
  Player favourites, with pauses at both ends and a restart on selection or resize.

## v0.5.4

### Fixed

- Align the Manager's CODEC, RATE and VOTES headers with their data columns.
- Add a project-local `make build` that recreates the root executable symlink.

## v0.5.3

### Changed

- The BROWSE nav no longer shows a `›` marker for the active view; the row's
  colour highlight is the only indicator now.
- `Space` in Manager now only ever adds a station to Favorites — it no longer
  unfavorites one you've already starred, so browsing the Favorites view with
  Space can't accidentally remove a station anymore. A new `d` key
  unfavorites explicitly.

## v0.5.2

### Changed

- The BROWSE nav in Manager now shows each item's keybind, e.g. `[f] Favorites`,
  instead of leaving the shortcut to be memorized.

## v0.5.1

### Fixed

- Resumed launches now render the saved station and a connecting state before
  asking mpv to restart the stream, so the Player is immediately visible while
  slow stations buffer instead of appearing only once playback begins.

## v0.5.0

Milestone: runtime colour themes and a dedicated Settings screen.

### Added

- **Mini Player** brings the usable window floor down from 64×12 to 24×4,
  retaining station/track identity, play state, volume and core playback
  controls. It activates automatically below 64×12, is also an explicit
  persisted `z` layout, and uses any spare rows for a compact visualizer.
  Manager, Help and Settings retain their honest 64×12 requirement.
- **Colour theme Settings** (`c` from Player or Manager) with immediate
  built-in/default, Follow Kitty, and custom-file selection.
- Follow Kitty watches `~/.config/kitty/current-theme.conf` (or an edited
  path) and content-reloads it once a second, including atomic replacements
  and symlink target changes made by desktop theme switchers.
- Direct import of Kitty colour configs, raw Base16/Base24 scheme YAML,
  generated i3 Base16 variables, and Radiowave semantic key/value palettes.
- Recursive theme-folder browser with live type-to-filter selection for large
  Tinted Theming collections. Moving the selection live-previews each palette;
  `Enter` commits it and `Esc` restores the previously active theme.
- Theme source and paths persist alongside the existing local library state.

### Changed

- Launching without playback to resume now opens Manager directly on
  **Favorites**, ready to choose a saved station. Launching with resumable
  playback still reconnects and opens the Player's Visual layout.

## v0.4.0

Milestone: proper genre browser (Phase 3's first item), plus a visual fix.

### Added

- **Genre picker** (`g` in Manager) replaces exact-tag text entry with a
  live-filtering modal over Radio Browser's real tag catalogue (fetched
  once per session, ~1000 tags, most-popular-first). Type to filter,
  `↑`/`↓`/`PgUp`/`PgDn`/`Home`/`End` to move, `Enter` to browse that genre's
  stations, `Esc` to cancel. No more needing to know an exact tag spelling.

### Fixed

- The VU visualizer style only used the leftmost ~30-50% of the screen
  regardless of terminal width — meter width was hardcoded (3 columns)
  instead of stretching to fill the available space. Meters now scale with
  window width, keeping the same few-big-meters look at any size.

### Changed

- The Player now scales down to 64×12 (from 80×20). At short heights its
  Normal layout automatically becomes a visual-first compact view that keeps
  station/track identity and at least five graph rows; Compact now uses that
  arrangement at every size.
- Volume moved into the header immediately before the connection indicator.
  Empty metadata/visualizer-label rows and unused favourite rows no longer
  displace the graph.
- The permanent `VISUALIZER (...)` row is gone. The active style, data source,
  and demo marker now appear inside the graph for two seconds on startup or
  after cycling styles.

## v0.3.1

### Fixed

- Fixed a fixed-size (not proportional) row-count miscalculation in the
  Player Normal layout and the Manager page that left the rendered UI
  consistently short of the actual terminal height — around 4 blank rows
  at the bottom for Player Normal, 2 for Manager. Because the gap was a
  constant offset rather than something that scaled with window size,
  shrinking the terminal to get rid of it didn't work — the gap stayed the
  same size right up until the real "terminal too small" floor. Root cause
  was `playerLayoutMetrics()` (main.go) and `renderManager()`'s `usableH`
  (render.go) double-subtracting fixed overhead that individual panels had
  already accounted for internally. Also fixed the Player Visual layout's
  smaller 1-row version of the same bug, and `visibleStationRows()` (used
  for Manager's PgUp/PgDn and cursor clamping), which needed to move in
  lockstep with the `renderManager` fix.
- `renderApp` now also pads/truncates its final output to exactly the
  terminal height as a safety net, so any future layout-math drift shows
  up as a test failure (`TestPanelMathFillsTerminalHeightExactly`) rather
  than a silently reappearing gap.

### Changed

- Resuming a station on launch (playback was active when Radiowave last
  exited) now lands directly in the Player's big Visual layout — the same
  place `z` takes you — instead of Normal layout with the user needing to
  press `z` themselves right after start.

## v0.3.0

Milestone: keybinding changes plus seven visualizer render styles — the
rest of Phase 2's "multiple spectrum modes" and "VU, peak and oscilloscope
modes" roadmap items.

### Changed

- **Unfavorite is now `Del`, not `Space`**, on the Player page. Space no
  longer removes a favourite there. (Manager's `Space` is unchanged — it
  still toggles favourite add/remove while browsing.)
- **`Tab` switches between Player and Manager**, replacing `M`.
- `v`/`V` now cycle *visualizer style* (how the live levels are drawn),
  not which data source produces them. The data source (cava, or the demo
  fallback) is still chosen automatically at startup — there's no longer a
  manual way to force the demo waveforms when cava is available, since in
  practice that wasn't useful.

### Added

- Seven visualizer styles, all working over either data source (cava or
  demo), cycled with `v`/`V` and persisted across restarts:
  - **Classic** — the original vertical bar spectrum.
  - **Mirrored** — low frequencies centred, spreading outward (the same
    arrangement cava's own "stereo" output mode uses, replicated as a
    display transform so it works for any data source).
  - **Blocks** — chunky, wide bars instead of thin ones.
  - **Peaks** — classic bars plus a slowly-decaying peak-hold marker per
    column, like a hardware spectrum analyser.
  - **VU** — a handful of large meters with fixed green/amber/red zones by
    row, like a real VU meter's markings.
  - **Oscilloscope** — a connected line trace instead of bottom-anchored
    bars (a creative liberty, since the data is frequency magnitude, not a
    raw waveform).
  - **Waterfall** — scrolling spectrogram history, newest frame on top.
- The Delete key (`⌫`, `ESC[3~`) is now decoded at all — previously it was
  silently misread as `Esc`.

### Fixed

- `radiowave` didn't stop its `cava` child process on normal exit — quitting
  left a live `cava` process running in the background indefinitely
  (`main()` had no `defer` for it). Also fixed `CavaVisualizer.Stop()` not
  reaping the process after killing it, which left a zombie behind it.
- The new Oscilloscope style briefly rendered at roughly double its
  intended width (two visible characters per data point instead of one),
  truncating every row.

## v0.2.1

Milestone: real audio visualization (Phase 2's first item), plus a sizing
fix requested after v0.2.0 landed.

### Added

- **Real visualizer backend via cava.** When `cava` is on `PATH`, Radiowave
  runs it in raw-output mode and renders its actual spectrum data — no
  longer a synthetic waveform. It captures whatever is playing through the
  system's default audio device, so no special mpv wiring was needed.
  Falls back to the demo waveforms (still clearly labelled "demo") when
  `cava` isn't installed or can't reach an audio source, trying the rest of
  the visualizer list automatically rather than leaving the display dead.
- Opt-in integration test (`cava_test.go`, `RADIOWAVE_TEST_CAVA=1`) that
  exercises the real `cava` binary end-to-end.

### Fixed

- Growing the terminal window now grows the Player's Normal-layout
  visualizer panel, not the favourites list. The list caps out once it's
  tall enough to be useful (≤12 rows); everything past that goes to the
  visualizer.
- The Normal layout's visualizer now renders a real multi-row vertical
  spectrum that fills the available panel height, instead of repeating the
  same single horizontal bar row.

## v0.2.0

Milestone: convert the single-page browsing prototype into the two-page
Player/Manager design described in `project.md`.

### Added

- **Player page**, now the default screen on launch. Shows now-playing
  info, a full-width demo visualizer, and favourites as a playable list.
- **Manager page** — the previous browsing screen, reframed as the
  discovery/organisation workspace. All existing browsing, search, genre,
  favourites, history, and hidden-station behaviour is unchanged.
- `M` switches between Player and Manager without interrupting playback —
  both pages share one global player instance and playback state.
- Three Player layouts, cycled with `z`: **Normal** (visualizer + now
  playing + favourites), **Visual** (large full-width visualizer), and
  **Compact** (favourites-focused, minimal visualizer).
- Pre-browse return: entering Manager remembers the station that was
  playing; `Backspace` returns to it.
- Automatic resume: if playback was active when Radiowave last exited
  (including an unclean exit), the next launch reconnects to that station.
  If playback was explicitly stopped, the next launch stays stopped.
- Playback preferences — volume, mute, selected visualizer, and Player
  layout — now persist across restarts, saved immediately on every change.
- A second demo visualizer waveform ("pulse bands") so `v`/`V` cycling has
  something to cycle through ahead of a real CAVA backend.
- `▶` now marks the currently playing station in the Manager station list
  (previously only favourites were marked, with `★`).
- `--help`/`-h` prints usage.
- `architecture.md` added.

### Changed

- The old Manager-only "big visualizer" full-screen toggle (`v` in the
  single-page prototype) is gone — the visualizer now lives on the Player
  page's Visual layout, reachable from anywhere via `M` then `z`.
- Header now shows which page is active (`Radiowave · PLAYER` /
  `Radiowave · MANAGER`).
- `library.json` gained a `state` section (last station, was-playing,
  volume, mute, visualizer, layout). Existing favourites/hidden/history
  data is read unchanged; missing `state` fields default sanely on load.

### Internal

- `playStation()` is now the single entry point for starting playback,
  used by both pages and by startup resume, so history/persistence only
  happens in one place.
- `Visualizer` gained `Start()`/`Stop()` lifecycle methods ahead of a real
  audio-analysis backend.

## v0.1.6 and earlier

Early prototype phase (single-page browser: trending/popular/random/
search/genre/favourites/history/hidden, mpv IPC playback, Radio Browser
mirror discovery, resize-aware viewport, demo visualizer layout preview).
No changelog was kept during this phase; see `project.md` for the product
direction that shaped it.
