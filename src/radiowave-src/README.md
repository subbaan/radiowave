# Radiowave v0.7.1

A terminal-native internet radio player and discovery tool for Linux.

Radiowave has two pages that share one global playback state:

- **Player** — the listening screen. Shows what's playing, a full-width
  visualizer, and your favourites as a playlist. Resumed playback opens here.
- **Manager** — station discovery: trending, popular, random, search, exact
  genre tags, favourites, history and hidden stations. An idle launch opens
  here with Favorites selected, ready to play.

Switching pages (`Tab`) never interrupts playback. You can browse the Manager
for several minutes while auditioning a station before deciding whether to
favourite it — that's normal listening, not a preview mode.

If [`cava`](https://github.com/karlstav/cava) is installed, the visualizer is
real audio analysis of whatever is playing through your default audio
device — Radiowave drives it in raw-output mode and renders its spectrum
data directly. Without `cava` on `PATH`, it falls back to demo waveforms. The
active style, source and its "demo" marker appear briefly inside the
visualizer on startup and after `v`/`V`, without permanently taking a row
from the graph. Growing your terminal window gives the extra height to the
visualizer panel, not the favourites list.

The Player scales all the way down to a practical 24×4 mini player with
station, track, play state, volume and core playback shortcuts. Shrinking
below the full UI's 64×12 floor switches to Mini automatically; `z` also
selects Mini explicitly at any size. Extra mini-player rows become a small
visualizer. Compact keeps the visual-first panel arrangement, while Visual
gives nearly the entire window to the graph.

Ten visualizer styles (`v`/`V` to cycle) apply to whichever data source is
active: Classic, Mirrored, Blocks, Peaks, VU, Oscilloscope, Waterfall,
Plasma, Tunnel, and Ribbons. The three new effects use bass, midrange and
treble energy to drive motion, shape and detail, with a bounded 160×60
effect grid at the existing 12.5 redraws per second.

In Kitty (or any terminal with the Kitty graphics protocol), three pixel
styles also join the cycle: **Neon** (bars with beat sparks and a rippling
reflection), **Nebula** (feedback swirl) and **Spectrogram** (scrolling
heat map). They draw real RGBA
graphics at 60 fps with alpha blending over the theme background, on a
canvas of 2×2-screen-pixel blocks sent through shared memory. Other
terminals and tmux keep the character styles; `--gfx=auto|shm|direct|off`
and `--pixel-scale=N` (1–4) override the defaults.

Press `c`, select **Frequency order**, and press `Enter` to switch between
**Bass centre → highs outward** (default) and **Bass left → highs right**.
The choice is saved and applies across spectrum styles; it does not change
playback stereo. Visual analysis averages the two audio channels. Mirrored
reflects bars vertically, independently of frequency order. Settings also
has a **Visualizer style** row; press `Enter` to cycle it.

Plasma draws flowing colour fields, Tunnel draws twisting concentric rings,
and Ribbons draws three undulating bands. These are audio-reactive when
CAVA is active; the fallback remains explicitly labelled demo.

## Requirements

- Linux terminal with true-colour support. Kitty is ideal.
- A terminal at least 24 columns by 4 rows for the mini player. Manager,
  Help and Settings require 64×12.
- Go 1.23+ to build from source.
- `mpv` for playback.
- `cava` (optional) for a real audio spectrum instead of demo waveforms.
- Internet access for Radio Browser.

On Manjaro/Arch:

```bash
sudo pacman -S go mpv cava
```

## Build and install

```bash
./install.sh
```

This installs `radiowave` to `~/.local/bin/radiowave`. Make sure that
directory is on your `PATH` (most distros add it by default for login
shells).

Or build locally from the project root (also recreates the root executable symlink):

```bash
make build
./radiowave
```

```bash
radiowave --version   # or -V / -v
radiowave --help      # or -h
```

## Controls

### Both pages

| Key | Action |
| --- | --- |
| `Tab` | Switch between Player and Manager |
| `s` | Stop playback |
| `m` | Mute/unmute |
| `+` / `-` | Volume |
| `?` | Help overlay |
| `c` | Colour theme settings |
| `q` | Quit |

### Player

| Key | Action |
| --- | --- |
| `↑` / `↓`, `j` / `k`, `PgUp`/`PgDn`, `Home`/`End` | Move favourite selection |
| `Enter` | Play the highlighted favourite |
| `Del` | Remove the highlighted favourite |
| `z` | Cycle layout: Normal → Visual → Compact → Mini |
| `v` / `V` | Next / previous visualizer style |

### Manager

| Key | Action |
| --- | --- |
| `↑` / `↓`, `j` / `k` | Move selection |
| `Home` / `End` | First / last loaded station |
| `PageUp` / `PageDown` | Move by one visible page |
| `Enter` | Play the selected station (normal playback, not a timed preview) |
| `Space` | Toggle favourite |
| `x` | Hide station. In the Hidden view, restores it |
| `Backspace` | Return to the station that was playing before you started browsing |
| `t` / `p` / `r` | Trending / Popular / Random |
| `/` | Search station names |
| `g` | Genre picker — live-filter Radio Browser's tag catalogue |
| `f` / `h` / `z` | Favourites / History / Hidden stations |

Long names in the highlighted station row scroll horizontally after a brief
pause, in both Manager and the Player favourites list. Scrolling pauses at
the end before repeating, and restarts when the selection or column width changes.

Long station lists use a resize-aware viewport. The selected row stays
visible, the status line shows the selected position and visible range, and
remote searches load another 100 results automatically as you approach the
end of the loaded list.

## Persistence and resume

Playback state is saved immediately on every play/stop/volume/mute/layout/
visualizer change, not only on a clean shutdown. If Radiowave was playing a
station when it last exited (including an unclean exit — terminal killed,
crash, `SIGTERM`), the next launch reconnects to that station automatically
and opens directly into the big Visual layout. If you explicitly stopped
playback before quitting, the next launch stays stopped and opens the
Manager's Favorites view.

Persisted alongside favourites/history/hidden stations:

- Last playing station (full snapshot, not just an ID)
- Whether playback was active
- Volume and mute state
- Selected visualizer and visualizer style
- Player layout (normal / visual / compact / mini)
- Colour-theme source and paths

## Colour themes

Press `c` from either main page to open Settings. A theme applies
immediately without interrupting playback. Radiowave supports:

- **Radiowave default** — the built-in palette.
- **Follow Kitty** — watches `~/.config/kitty/current-theme.conf` by default
  and reloads whenever its contents change. The path can be edited in the
  Settings screen.
- **Custom theme file** — reads a Kitty-style colour file, a Radiowave
  semantic key/value palette, raw Base16/Base24 scheme YAML, or a generated
  i3 Base16 theme containing `set $base00` through `set $base0F`.
- **Theme folder browser** — recursively indexes `.yaml`, `.yml`, `.conf`,
  `.config`, `.theme`, and `.txt` files. Type in the browser to live-filter
  a large Tinted Theming collection. Moving through the results previews each
  palette immediately; `Enter` saves it and `Esc` restores the previous theme.

For [Tinted Theming](https://github.com/tinted-theming/schemes), the raw
files under `schemes/base16` are the most direct choice. Generated i3 files
also work because Radiowave imports their Base16
variables and ignores the i3-specific rules. Base16 roles are mapped
semantically: `base00`/`base05` provide the background/text,
`base08`–`base0E` provide status/accent colours, and `base02` provides the
selection and border tone.

A custom semantic palette needs at least `background` and `foreground`, and
can additionally define `muted`, `accent`, `success`, `warning`, `error`,
`secondary`, `info`, `selection_background`, `selection_foreground`, and
`border`, with `#RRGGBB` values.

Both Follow Kitty and custom-file modes compare the small source file's
contents once per second. This also notices atomic replacements and symlink
target changes commonly made by desktop theme switchers.

## Local data

Radiowave stores its library at:

```text
$XDG_CONFIG_HOME/radiowave/library.json
```

or, when `XDG_CONFIG_HOME` is unset:

```text
~/.config/radiowave/library.json
```

Favourites are stored as local station snapshots, so a favourite is not lost
merely because its Radio Browser database entry later changes or disappears.

## Architecture

See [`architecture.md`](architecture.md) for the technical layout. In short:

- `api.go` — Radio Browser client and mirror discovery.
- `player.go` — persistent mpv process controlled over JSON IPC.
- `store.go` — atomic local library + playback-state persistence.
- `terminal.go` — raw-terminal input and sizing.
- `render.go` — true-colour TUI renderer for the Player and Manager pages.
- `theme.go` / `themescreen.go` — theme import, live reload, settings, and browser.
- `visualizer.go` — visualizer interface and demo waveforms.
- `cava.go` — real audio spectrum data via cava's raw-output mode.
- `vizstyle.go` — visualizer render styles (classic/mirrored/blocks/peaks/VU/oscilloscope/waterfall).
- `genrepicker.go` — live-filtering genre picker over the Radio Browser tag catalogue.
- `main.go` — application state, event loop, and page/playback logic.

This keeps Kitty graphics, SQLite storage, and related-station discovery
from leaking into playback/library logic.

## Roadmap

See [`project.md`](project.md) for the full product direction. Near-term:

1. Add countries and languages browsers (same live-filter pattern as the
   genre picker).
2. Add related-station discovery and a richer station detail view.
3. Improve stream metadata/track history.
4. Add collections and user-defined tags.
