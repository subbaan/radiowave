# Radiowave Project Specification

**Document version:** 0.1.0  
**Project:** Radiowave  
**Current prototype baseline:** v0.5.0  
**Status:** Hobby project, active design and implementation

## 1. Project Summary

Radiowave is a terminal radio player and discovery application focused entirely on internet radio.

The project should feel closer to a polished dedicated radio appliance than a general music player. It should combine two complementary workflows:

1. **Player** - the default everyday listening screen.
2. **Manager** - a station discovery, browsing, previewing, favouriting and organisation screen.

The application should remain fast, keyboard-friendly, visually attractive, and comfortable to use for long sessions in Kitty and other capable terminals.

Radiowave is a hobby project. Keep the architecture clean and maintainable without introducing enterprise-style process, excessive abstraction, or large dependency stacks without a clear benefit.

---

## 2. Core Product Goals

Radiowave should make these activities effortless:

- Start the application and continue listening to the last active station.
- Browse favourite stations like a playlist.
- See clear now-playing information.
- Enjoy attractive full-width audio visualizers.
- Discover new stations by genre, popularity, trend, random selection, country, language and search.
- Preview stations directly from the Manager before deciding whether to favourite them.
- Quickly reject stations with speech, adverts, DJs, jingles or other unwanted interruptions.
- Favourite good discoveries immediately.
- Hide stations the user does not want to encounter again.
- Recover accidentally hidden stations.
- Move freely between Player and Manager without interrupting playback.

The application should favour immediacy over configuration-heavy workflows.

---

## 3. Product Model

Radiowave has two primary pages.

### 3.1 Player

The Player is the default page and the main identity of Radiowave.

Its purpose is listening rather than discovery.

The Player should contain:

- Current station name.
- Current stream metadata when available.
- Codec and bitrate.
- Connection/buffer status.
- Volume and mute state.
- Favourite state.
- Listening duration where useful.
- A large visualizer area.
- A favourites list acting as the practical equivalent of a playlist.

The favourites list should be fast to navigate with the keyboard. Selecting a favourite and pressing Enter should immediately start it.

The application data model should continue to call these entries favourites or stations rather than traditional playlist tracks. Internet radio has different semantics from file-based playback.

### 3.2 Manager

The current v0.1.x browsing interface should evolve into the Manager page.

Its purpose is finding, evaluating and organising stations.

The Manager should provide access to:

- Discover.
- Search.
- Genres.
- Trending.
- Popular.
- Random.
- Countries.
- Languages.
- Favourites.
- History.
- Hidden stations.
- Related stations.
- Collections later if useful.

The Manager must remain fully capable of playback.

Pressing Enter on a highlighted station should play it through the same global player used by the Player page. This is an important workflow. The user may listen for several minutes to determine whether a station contains DJs, talking, adverts or other interruptions before deciding whether to favourite it.

Manager playback is therefore normal playback, not a short timed preview mode.

---

## 4. Global Playback Behaviour

Playback state belongs to the application, not to a page.

Player and Manager must both control the same playback backend and observe the same state.

Switching pages must never restart or interrupt a stream unless the user explicitly chooses another station or stops playback.

Persist at least:

- Last playing station.
- Whether playback was active.
- Volume.
- Mute state.
- Selected visualizer.
- Player layout mode.

If Radiowave exits while a station is playing, the next launch should reconnect to that station automatically.

If the user explicitly stops playback before quitting, the next launch should remain stopped.

Persist the last-played state when playback changes rather than relying only on a clean program shutdown. This allows sensible recovery after terminal crashes, session termination or system shutdown.

### Previous-station return

When entering Manager while listening to a station, remember that station as the pre-browse station.

While exploring other stations, provide a convenient command to return to that previous station. A likely default binding is Backspace.

This allows experimentation without losing the station the user was listening to before browsing.

---

## 5. Player Page Design

The Player should be visually richer and more focused than the Manager.

A broad target layout is:

```text
┌─ Radiowave · PLAYER ─────────────────────────────────────────────────────────────┐
│                                                                                │
│  NIGHTWAVE PLAZA                                       ● LIVE   AAC 128k       │
│  The Midnight - Sunset                                 ★ Favourite             │
│  synthwave · retrowave · electronic                                           │
│                                                                                │
│   ▁▂▃▅▇██▇▅▃▂▂▄▆██▇▅▂▁▂▄▆▇██▆▄▂▁▂▃▅▇██▇▅▃▂▂▄▆██▇▅▂▁▂▄▆▇██▆▄▂              │
│   ▂▄▆████▆▄▃▄▆█████▇▅▃▂▃▅▇████▆▄▂▃▄▆█████▇▅▃▂▃▅▇████▆▄                    │
│   ▃▆████████▇▆▅▆█████████▆▄▃▄▆████████▇▆▅▆█████████▆▄                      │
│                                                                                │
├─ FAVOURITES ──────────────────────────────────────────── 27 stations ────────────┤
│ > Nightwave Plaza              synthwave                        ● playing       │
│   SomaFM Groove Salad          ambient                                          │
│   Deep Space One               space ambient                                    │
│   Nightride FM                 synthwave                                         │
│   Smooth Jazz Florida          jazz                                              │
│                                                                                │
├────────────────────────────────────────────────────────────────────────────────┤
│ ↑↓ station  Enter play  Space ★  v visualizer  m mute  +/- volume  M manager   │
└────────────────────────────────────────────────────────────────────────────────┘
```

This is a direction, not a rigid pixel specification. Adapt it to actual terminal dimensions.

### 5.1 Player layout modes

Support three conceptual layouts:

#### Normal

- Large visualizer.
- Now-playing information.
- Favourites list.

#### Visual

- Visualizer occupies most of the terminal.
- Keep a compact now-playing/status strip visible.
- Hide the favourites list while this mode is active.

#### Compact

- Prioritise station/favourites navigation.
- Use a small visualizer or meter.
- Useful for short terminals or users who prefer information density.

The renderer should react sensibly to terminal size rather than assuming one fixed resolution.

---

## 6. Visualizers

Visualizers are a planned core feature and should influence architecture now, even if the first production implementation supports only a subset.

Do not tightly couple visualization code to the playback backend or page renderer.

Keep a visualizer abstraction such as:

```go
type Visualizer interface {
    Start() error
    Stop()
    Levels(n int) []float64
    Name() string
}
```

Adjust the exact interface as the implementation matures.

### 6.1 Desired visual modes

Aim eventually for several visually distinct modes, for example:

- Classic spectrum.
- Mirrored spectrum.
- Centre-out spectrum.
- Fine/thin spectrum.
- Chunky block spectrum.
- Spectrum with peak markers.
- Stereo VU meters.
- Oscilloscope-style view.
- Waterfall/history view.
- Minimal meter mode.

A family/variant model is preferable to dozens of unrelated implementations where practical.

Potential controls:

- `v` - next visualizer.
- `V` - previous visualizer.
- A separate key or menu for direct visualizer selection.
- A dedicated key for large Visual layout mode.

### 6.2 Audio data

CAVA raw output is a good candidate for spectrum data on Linux.

Keep this behind a backend/interface so Radiowave can later support visualizers needing a different audio data source.

The v0.1.x fake/demo spectrum must never be presented as real audio analysis once genuine visualizers begin landing.

---

## 7. Manager Page Behaviour

The Manager is the discovery and organisation workspace.

The existing station list and lazy-loaded viewport are a useful foundation.

### Required behaviour

- Long station lists must scroll properly.
- PgUp/PgDn should move by the actual visible page size.
- Home/End should behave predictably.
- Terminal resize must preserve a sensible selection and viewport.
- Lazy-loading should fetch additional Radio Browser results as the user approaches the bottom.
- Duplicate stations should be filtered where practical.
- The UI should clearly show selected position and loaded result count.
- Esc should move back through browsing levels where applicable instead of unexpectedly quitting the whole application.

### Playback while browsing

`Enter` plays the highlighted station immediately.

The currently playing station should be visibly marked in the results list, for example with `▶`.

`Space` should toggle favourite status.

A station can continue playing while the user searches or changes category.

### Hidden stations

Hidden stations should be persistent and excluded from normal discovery views.

The Hidden view must allow stations to be restored easily.

Hiding is therefore reversible, not destructive.

---

## 8. Genre Browser

Genre browsing should become a proper Manager sub-page rather than requiring exact free-text tag entry.

Target interaction:

```text
GENRES

Search: synth_

★ synthwave                  428
  synthpop                   193
  synth                     1120
  darksynth                   84
  synth rock                  61
  retrowave                  207
```

Requirements:

- Fetch/use the live Radio Browser tag catalogue.
- Show station count where available.
- Type-to-filter or search.
- Keyboard selection.
- Enter opens stations for that genre.
- Esc returns from stations to the genre list.
- Favourites or recently used genres may eventually be surfaced near the top.

Apply the same pattern later to Countries and Languages where appropriate.

---

## 9. Discovery

Radiowave should make browsing enjoyable rather than act as a thin API viewer.

Useful discovery modes include:

- Trending.
- Popular.
- Random.
- Recently added where useful.
- High-bitrate results.
- Related stations.

### Related stations

Related Stations should become a significant discovery feature.

When listening to or highlighting a station, build recommendations using overlapping tags. Rarer/more specific tags should be weighted more strongly than broad generic tags where practical.

This should make it possible to move naturally from one station to similar ones without returning to the top-level browser repeatedly.

---

## 10. Favourites and Collections

Favourites are the everyday station list shown on the Player page.

Keep the default favourites workflow flat and immediate.

Potential later additions:

- Collections/groups.
- User-defined tags such as `coding`, `late night`, `background`, `excellent`.
- Reordering favourites.

Do not make organisation mandatory before a station can be favourited.

Store enough station information locally that a favourite remains usable if its Radio Browser directory entry later changes or disappears.

At minimum snapshot useful fields such as:

- Radio Browser UUID.
- Name.
- Stream URL.
- Resolved stream URL.
- Homepage.
- Tags.
- Country.
- Language.
- Codec.
- Bitrate.

---

## 11. History and Audition Behaviour

The user may rapidly try several stations while searching.

Do not let very short auditions make normal listening history useless.

A sensible starting rule is to promote a station into normal History after approximately 60 seconds of playback, or immediately when it is favourited.

Internal short-play information may still be retained later if useful for reliability or statistics.

History should eventually support:

- Recently played stations.
- Most played stations.
- Last played time.
- Listening duration.

Track metadata history is also desirable when streams provide ICY/current-track information.

---

## 12. Playback Backend

Continue using mpv as the initial playback engine.

Use one persistent mpv process controlled through its JSON IPC socket where practical.

Keep mpv details behind a player abstraction. UI code should work with application-level player state instead of constructing mpv commands directly.

Conceptually:

```go
type Player interface {
    Play(station Station) error
    Stop() error
    SetVolume(int) error
    SetMuted(bool) error
    State() PlayerState
}
```

The exact API can evolve.

The player layer should expose or derive:

- Current station.
- Playing/stopped state.
- Current metadata title.
- Codec/bitrate where available.
- Buffer/connection state where practical.
- Volume.
- Mute state.

---

## 13. Radio Browser Integration

Radio Browser is the primary directory provider.

Keep it behind a provider/client boundary rather than scattering HTTP/API assumptions throughout the UI.

The client should support:

- Mirror discovery/fallback.
- Descriptive User-Agent.
- Search.
- Exact tag/genre lookup.
- Tag catalogue.
- Trending/popular/random ordering.
- Country/language queries.
- Pagination/lazy-loading.
- Broken-station filtering.
- Duplicate suppression.

A future provider abstraction may allow custom sources or imports without rewriting the UI.

---

## 14. Local Data

The existing JSON storage is acceptable while the project remains small.

Use atomic writes and keep the format inspectable.

Move to SQLite when changing data becomes sufficiently relational, especially once these become real features:

- Collections.
- User tags.
- Track history.
- Rich listening statistics.
- Reliability records.

Do not introduce SQLite merely for architectural fashion. Introduce it when it simplifies the program.

Likely locations:

```text
~/.config/radiowave/config.toml
~/.local/share/radiowave/library.json
```

or later:

```text
~/.local/share/radiowave/radiowave.db
```

---

## 15. Terminal and Rendering Requirements

Radiowave is a true TUI first.

Core functionality must not require terminal graphics protocols.

Use:

- ANSI colours.
- Unicode/block characters.
- Nerd Font icons where they provide useful polish, with sensible fallbacks if needed.

Optional Kitty image support may be added later for station favicons/artwork, but it must remain an enhancement. The application must still look complete without images.

### Terminal correctness

The v0.1.x prototype exposed important terminal handling lessons. Preserve them:

- Raw input mode must not break normal line rendering.
- Ensure output newline behaviour remains correct.
- Disable/restore auto-wrap appropriately if full-width drawing requires it.
- Always restore terminal modes on normal exit and error paths.
- Prefer the alternate screen for the full-screen UI.
- Respond correctly to terminal resize.

Terminal state corruption on exit is a serious bug.

---

## 16. Appearance

Radiowave should feel modern and distinctive without trying to imitate a GUI inside the terminal.

Suggested visual direction:

- Near-black/dark background.
- Magenta/pink primary accent.
- Green for healthy/live/connected states.
- Amber/orange for secondary status where useful.
- Thin, understated panel borders.
- High information density without cramped text.
- Colour and motion should come particularly from the visualizer.

Avoid a blue-led theme.

Theme architecture can be added later. A known palette such as Dracula is a suitable starting reference.

---

## 17. Keyboard Model

Bindings can evolve, but they should remain visible and consistent.

### Common

```text
↑/↓ or j/k     Move
PgUp/PgDn      Page
Home/End       Extremes
Enter          Play/activate
Space          Favourite
?              Help
q              Quit
M              Toggle Player/Manager
```

### Player candidates

```text
v              Next visualizer
V              Previous visualizer
z              Large visual layout
s              Stop
m              Mute
+/-            Volume
```

### Manager candidates

```text
Enter          Play highlighted station
Space          Favourite
Backspace      Return to pre-browse station
x              Hide/unhide where appropriate
/              Search
g              Genres
R              Related stations
Esc            Back one browsing level
```

Avoid stealing ordinary typing keys while a search/text field has focus.

A persistent compact hint line plus `?` full help should remain part of the UI.

---

## 18. CLI Behaviour

The TUI is the main application, but a few shell-friendly entry points are desirable later:

```bash
radiowave
radiowave --last
radiowave --random
radiowave --favorite "Nightwave Plaza"
radiowave --version
```

Do not let CLI feature work distract from the main Player/Manager experience.

---

## 19. Reliability and Error Handling

Internet radio is inherently unreliable. Treat failures as routine application states rather than exceptional disasters.

Handle gracefully:

- Dead streams.
- Slow streams.
- DNS failures.
- Radio Browser mirror failure.
- API timeout.
- Invalid metadata.
- Station disappearing from the directory.
- mpv exiting unexpectedly.

A favourite should remain locally known even if Radio Browser no longer returns it.

Potential later feature: local station reliability scoring based on successful starts and repeated failures.

---

## 20. Architecture Direction

Keep clear boundaries between concerns without over-engineering.

A useful conceptual structure is:

```text
Radiowave App
│
├── UI
│   ├── Player Page
│   ├── Manager Page
│   ├── Help/overlays
│   └── Visualizer renderer
│
├── Application State
│   ├── Playback state
│   ├── Navigation state
│   ├── Selection/viewports
│   └── persisted preferences
│
├── Services
│   ├── Discovery
│   ├── Favourites/library
│   ├── History
│   └── Related stations
│
└── Backends
    ├── Radio Browser client
    ├── mpv IPC player
    ├── local storage
    └── CAVA/audio visualizer later
```

Prefer small interfaces at real subsystem boundaries.

Avoid abstraction layers that only wrap one trivial function.

The current dependency-light Go approach is a good fit. Add external dependencies when they clearly reduce complexity or materially improve reliability.

---

## 21. Documentation

Keep documentation useful and lightweight.

The repository should maintain these documents:

### `README.md`

User-facing overview containing:

- What Radiowave is.
- Current screenshots when available.
- Installation/build instructions.
- Runtime dependencies.
- Main controls.
- Current major features.

### `project.md`

This file.

Use it for product direction, intended behaviour and roadmap. Update it when major product decisions change. Do not turn it into a changelog.

### `architecture.md`

Keep a concise technical architecture document covering:

- Package/module layout.
- Major interfaces.
- Playback architecture.
- Radio Browser integration.
- Storage model.
- TUI rendering/input model.
- Visualizer pipeline once implemented.
- Important architectural decisions and constraints.

Update it whenever implementation changes make existing architecture descriptions materially incorrect.

It should explain how the program fits together, not narrate every code change.

### `CHANGELOG.md`

A small changelog is useful now that versions are already being produced.

Keep entries concise and grouped by version. Record user-visible changes and important fixes.

No additional documentation files are required unless a real need develops.

---

## 22. Versioning and Development Practice

Every released or shared build should have an explicit version number.

Increment the version on each user-facing revision.

The version should be available through:

```bash
radiowave --version
```

Keep version references in documentation current when appropriate.

Before handing off a build, run at minimum:

```bash
go test ./...
go vet ./...
```

Also manually verify the TUI in a real pseudo-terminal or terminal session when changing:

- Raw mode.
- Rendering.
- Resize handling.
- Key input.
- Alternate-screen behaviour.
- Exit/terminal restoration.

Tests should focus on behaviour that is easy to regress, such as:

- Viewport calculations.
- Pagination.
- Duplicate filtering.
- Persistence.
- Navigation state.
- History promotion rules.
- Related-station ranking when introduced.

Keep the test suite proportionate to a hobby project.

---

## 23. Near-Term Roadmap

### Phase 1 - Establish the Player

Priority now.

- Make Player the default page.
- Build Normal Player layout.
- Build large Visual layout.
- Show favourites as the station playlist.
- Share playback state between Player and Manager.
- Resume a playing station on next launch.
- Persist volume, mute, visualizer and layout choice.
- Keep Manager playback fully functional.
- Add previous/pre-browse station return.

### Phase 2 - Real Visualizers

- Integrate real spectrum data, likely via CAVA raw output.
- Add multiple spectrum modes.
- Add visualizer selection controls.
- Preserve responsive layouts.
- Explore VU, peak and oscilloscope modes where technically appropriate.

### Phase 3 - Improve Manager Navigation

- Proper genre browser.
- Countries browser.
- Languages browser.
- Improve Manager width allocation and information density.
- Related stations.
- Better station information view.
- Mouse wheel support if it improves use without complicating input handling.

### Phase 4 - Library Depth

- Collections if still wanted.
- User tags.
- Better history.
- Track metadata history.
- Optional reliability scoring.
- Consider SQLite at this stage if it meaningfully simplifies storage.

### Phase 5 - Polish

- Themes.
- Optional Kitty station artwork/favicons.
- Additional CLI conveniences.
- Import/export where useful.

---

## 24. Current Design Principles

When making implementation decisions, favour these principles:

1. **Radio first.** Avoid inheriting music-player concepts that do not help internet radio.
2. **Player and Manager have distinct jobs.** Player is where the user listens. Manager is where the user discovers and organises.
3. **Playback is global.** Browsing never requires a separate temporary player.
4. **Auditioning is normal listening.** Users may need several minutes to evaluate a station.
5. **Favouriting is immediate.** Organisation can come later.
6. **Discovery should be enjoyable.** Related stations, random browsing and good genre navigation matter.
7. **Visualizers are part of the product identity.** Design for them early.
8. **Terminal-native first.** Optional graphics should enhance rather than define the application.
9. **Keep dependencies and abstractions justified.** This is a hobby project and should stay pleasant to maintain.
10. **Protect user state.** Favourites, hidden stations, history and resume state should survive API changes and ordinary failures.

---

## 25. Immediate Coding-Agent Brief

The next development milestone should focus on converting the application from a browser-first prototype into the two-page Player/Manager design.

Do this without regressing the current Manager functionality.

Specifically:

- Retain existing station browsing, scrolling, lazy-loading, searching, favouriting, hiding and playback.
- Rename/reframe the existing primary browsing screen internally as Manager where appropriate.
- Introduce a new Player page and make it the default startup view.
- Show favourites as the main station list on Player.
- Keep one shared player instance and one shared playback state across both pages.
- Allow Manager stations to be played normally for extended evaluation before favouriting.
- Implement automatic resume of the last station only when playback was active when the application last stopped running.
- Persist playback state immediately enough to survive unclean exits reasonably well.
- Add Normal and large Visual Player layouts.
- Keep the visualizer interface modular. A demo visualizer is acceptable while the Player layout is being built, but clearly identify it as demo data.
- Update `README.md`, `project.md`, `architecture.md` and `CHANGELOG.md` as appropriate when completing the milestone.
- Keep documentation concise and accurate.
- Increment the application version for every handed-off revision.
- Run tests and vet before handing off each build.

The goal of this milestone is to make Radiowave already feel like a dedicated radio player even before the real audio visualizers and deeper Manager improvements arrive.
