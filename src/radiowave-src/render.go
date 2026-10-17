package main

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	bold          = "\x1b[1m"
	dim           = "\x1b[2m"
	miniMinWidth  = 24
	miniMinHeight = 4
	fullMinWidth  = 64
	fullMinHeight = 12
)

func clearAndHome() string { return bgBase + fgText + "\x1b[H\x1b[2J" }

func pad(s string, width int) string {
	n := visibleLen(s)
	if n >= width {
		return truncate(s, width)
	}
	return s + strings.Repeat(" ", width-n)
}

func visibleLen(s string) int {
	inEsc := false
	n := 0
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			inEsc = true
		}
		if inEsc {
			if s[i] == 'm' {
				inEsc = false
			}
			i++
			continue
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		n++
		i += size
	}
	return n
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if visibleLen(s) <= width {
		return s
	}
	plain := stripANSI(s)
	r := []rune(plain)
	if len(r) <= width {
		return plain
	}
	if width <= 1 {
		return string(r[:width])
	}
	return string(r[:width-1]) + "…"
}

// stationListName pauses at both ends so a selected long name can be read
// without changing the width of the surrounding columns.
func (a *App) stationListName(s Station, width, index int, selected bool, now time.Time) string {
	if !selected {
		return truncate(s.Name, width)
	}
	key := fmt.Sprintf("%d/%d/%s/%d/%s/%s/%d", a.page, a.mode, a.query, index, s.StationUUID, s.Name, width)
	if key != a.nameScrollKey {
		a.nameScrollKey = key
		a.nameScrollSince = now
	}
	runes := []rune(stripANSI(s.Name))
	if width <= 0 || len(runes) <= width {
		return truncate(s.Name, width)
	}
	const pause = 1200 * time.Millisecond
	const step = 240 * time.Millisecond
	distance := len(runes) - width
	cycle := 2*pause + time.Duration(distance)*step
	elapsed := now.Sub(a.nameScrollSince)
	if elapsed < 0 {
		elapsed = 0
	}
	phase := elapsed % cycle
	offset := 0
	if phase >= pause {
		offset = min(distance, 1+int((phase-pause)/step))
	}
	return string(runes[offset : offset+width])
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if c == 'm' {
				inEsc = false
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func boxTop(w int, title string) string {
	if w < 4 {
		return ""
	}
	t := ""
	if title != "" && w >= 5 {
		t = " " + truncate(title, w-5) + " "
	}
	dashes := max(0, w-2-visibleLen(t))
	left := min(1, dashes)
	return border + "┌" + strings.Repeat("─", left) + magenta + t + border + strings.Repeat("─", dashes-left) + "┐" + reset
}

func boxBottom(w int) string {
	if w < 2 {
		return ""
	}
	return border + "└" + strings.Repeat("─", w-2) + "┘" + reset
}

func boxLine(w int, content string) string {
	inner := w - 2
	if inner < 0 {
		inner = 0
	}
	return border + "│" + reset + pad(content, inner) + border + "│" + reset
}

func renderApp(a *App) string {
	if a.width < miniMinWidth || a.height < miniMinHeight {
		message := fmt.Sprintf("Minimum size: %dx%d", miniMinWidth, miniMinHeight)
		hint := fmt.Sprintf("Current: %dx%d", a.width, a.height)
		return renderSizeNotice(a, message, hint)
	}
	if a.width < fullMinWidth || a.height < fullMinHeight {
		// A small terminal is still a useful radio remote. Only the Player's
		// mini view is safe here; multi-column pages and modal editors retain
		// their larger floor rather than pretending to fit.
		if a.page == pagePlayer && !a.themePicker && !a.configScreen && !a.help && !a.genrePicker {
			return clearAndHome() + fitToHeight(renderPlayerMini(a), a.height)
		}
		message := fmt.Sprintf("This screen needs %dx%d. Current: %dx%d", fullMinWidth, fullMinHeight, a.width, a.height)
		hint := "Esc closes this screen"
		if a.page == pageManager && a.inputMode == "" && !a.themePicker && !a.configScreen && !a.help && !a.genrePicker {
			message = fmt.Sprintf("Manager needs %dx%d. Current: %dx%d", fullMinWidth, fullMinHeight, a.width, a.height)
			hint = "Tab returns to the mini player"
		}
		return renderSizeNotice(a, message, hint)
	}
	a.pixRect = vizRect{}
	var body string
	switch {
	case a.themePicker:
		body = renderThemePicker(a)
	case a.configScreen:
		body = renderThemeConfig(a)
	case a.help:
		body = renderHelp(a)
	case a.genrePicker:
		body = renderGenrePicker(a)
	case a.page == pagePlayer:
		body = renderPlayerPage(a)
	default:
		body = renderManager(a)
	}
	return clearAndHome() + fitToHeight(body, a.height)
}

func renderSizeNotice(a *App, message, hint string) string {
	lines := []string{fgText + truncate(message, a.width) + reset}
	if hint != "" && a.height > 1 {
		lines = append(lines, fgMuted+truncate(hint, a.width)+reset)
	}
	return clearAndHome() + fitToHeight(strings.Join(lines, "\n"), a.height)
}

// fitToHeight pads or truncates rendered output to exactly `height`
// terminal rows. Panel layout math only needs to be approximately right —
// this is what guarantees the result always fills the window exactly,
// instead of a small, fixed miscalculation leaving a gap of blank rows
// that doesn't shrink as the window does.
func fitToHeight(body string, height int) string {
	lines := strings.Split(body, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func renderHeader(a *App, label string) string {
	status := green + "● ONLINE" + reset
	if a.loading {
		status = orange + "● LOADING" + reset
	}
	if a.errText != "" {
		status = red + "● ERROR" + reset
	}
	title := bold + magenta + "Radiowave" + reset + fgMuted + " · " + label + " · v" + Version + reset
	volume := "Vol --%"
	if a.player != nil {
		volume = fmt.Sprintf("Vol %d%%", a.player.volume)
		if a.player.muted {
			volume += " MUTED"
		}
	}
	right := fgText + volume + reset + "  " + status
	maxTitle := a.width - visibleLen(right) - 3
	if visibleLen(title) > maxTitle {
		title = truncate(title, maxTitle)
	}
	gap := a.width - visibleLen(title) - visibleLen(right) - 2
	if gap < 1 {
		gap = 1
	}
	return " " + title + strings.Repeat(" ", gap) + right + "\n"
}

// --- Manager page -----------------------------------------------------

func renderManager(a *App) string {
	var out strings.Builder
	out.WriteString(renderHeader(a, "MANAGER"))
	// header(1) + footer(1) = 2 rows of fixed overhead; each column already
	// includes its own top/bottom border within usableH.
	usableH := a.height - 2
	leftW := 22
	rightW := 31
	centerW := a.width - leftW - rightW - 2
	if centerW < 45 {
		leftW = 18
		rightW = 26
		centerW = a.width - leftW - rightW - 2
	}

	left := renderNav(a, leftW, usableH)
	center := renderStations(a, centerW, usableH)
	right := renderNowPlaying(a, rightW, usableH)
	out.WriteString(joinColumns(left, center, right))
	out.WriteString("\n")
	out.WriteString(renderManagerFooter(a))
	return out.String()
}

func renderNav(a *App, w, h int) []string {
	lines := []string{boxTop(w, "BROWSE")}
	items := []struct {
		key   string
		label string
		mode  browseMode
	}{
		{"t", "Trending", modeTrending},
		{"p", "Popular", modePopular},
		{"r", "Random", modeRandom},
		{"/", "Search", modeSearch},
		{"g", "Genre", modeGenre},
		{"f", "Favorites", modeFavorites},
		{"h", "History", modeHistory},
		{"z", "Hidden", modeHidden},
	}
	for _, it := range items {
		style := fgText
		if a.mode == it.mode {
			style = magenta + bold
		}
		count := ""
		if it.mode == modeFavorites {
			count = fmt.Sprintf(" %d", len(a.lib.Favorites))
		}
		keyed := fmt.Sprintf("[%s] %s", it.key, it.label)
		content := fmt.Sprintf(" %s%-13s%s%s", style, keyed, reset, fgMuted+count+reset)
		lines = append(lines, boxLine(w, content))
	}
	lines = append(lines, boxLine(w, ""))
	lines = append(lines, boxLine(w, " "+purple+"PLAYBACK"+reset))
	lines = append(lines, boxLine(w, " "+fgMuted+"Enter  play"+reset))
	lines = append(lines, boxLine(w, " "+fgMuted+"Space  favorite"+reset))
	lines = append(lines, boxLine(w, " "+fgMuted+"d      unfavorite"+reset))
	lines = append(lines, boxLine(w, " "+fgMuted+"x      hide"+reset))
	lines = append(lines, boxLine(w, " "+fgMuted+"⌫      pre-browse"+reset))
	lines = append(lines, boxLine(w, " "+fgMuted+"Tab    player"+reset))
	for len(lines) < h-1 {
		lines = append(lines, boxLine(w, ""))
	}
	lines = append(lines, boxBottom(w))
	return lines[:h]
}

func renderStations(a *App, w, h int) []string {
	title := strings.ToUpper(a.mode.String())
	if a.query != "" && (a.mode == modeSearch || a.mode == modeGenre) {
		title += " · " + a.query
	}
	lines := []string{boxTop(w, title)}
	nameW := max(8, w-29)
	header := fmt.Sprintf("  %2s %-*s %-5s %4s %6s", "#", nameW, "NAME", "CODEC", "RATE", "VOTES")
	lines = append(lines, boxLine(w, green+header+reset))

	rows := h - 4 // top + header + status + bottom
	if rows < 1 {
		rows = 1
	}
	a.ensureCursorVisible()

	end := min(len(a.stations), a.offset+rows)
	for i := a.offset; i < end; i++ {
		s := a.stations[i]
		marker := " "
		if _, ok := a.lib.Favorites[s.StationUUID]; ok {
			marker = "★"
		}
		if a.playing && a.now != nil && a.now.StationUUID == s.StationUUID {
			marker = "▶"
		}
		name := a.stationListName(s, nameW, i, i == a.cursor, time.Now())
		content := fmt.Sprintf(" %s%2d %-*s %-5s %4d %6d", marker, i+1, nameW, name, truncate(s.Codec, 5), s.Bitrate, s.Votes)
		if i == a.cursor {
			content = bgSel + fgSelected + content + reset
		} else if marker == "▶" {
			content = green + content + reset
		} else if marker == "★" {
			content = magenta + content + reset
		}
		lines = append(lines, boxLine(w, content))
	}
	for len(lines) < h-2 {
		lines = append(lines, boxLine(w, ""))
	}

	status := ""
	switch {
	case a.errText != "":
		status = " " + red + truncate(a.errText, w-5) + reset
	case a.loading && len(a.stations) == 0:
		status = " " + orange + "Fetching stations…" + reset
	case len(a.stations) == 0:
		status = " " + fgMuted + "No stations in this view." + reset
	default:
		first := a.offset + 1
		last := min(len(a.stations), a.offset+rows)
		more := ""
		if a.loadingMore {
			more = " · " + orange + "loading more…" + reset + fgMuted
		} else if a.hasMore {
			more = " · more ↓"
		}
		status = fmt.Sprintf(" %s%d/%d selected · showing %d-%d · %d loaded%s%s", fgMuted, a.cursor+1, len(a.stations), first, last, len(a.stations), more, reset)
	}
	lines = append(lines, boxLine(w, status))
	lines = append(lines, boxBottom(w))
	return lines[:h]
}

func renderNowPlaying(a *App, w, h int) []string {
	lines := []string{boxTop(w, "NOW PLAYING")}
	if a.now == nil {
		lines = append(lines,
			boxLine(w, ""),
			boxLine(w, " "+fgMuted+"Nothing playing"+reset),
			boxLine(w, ""),
		)
	} else {
		s := a.now
		lines = append(lines,
			boxLine(w, " "+bold+fgText+truncate(s.Name, w-5)+reset),
			boxLine(w, " "+magenta+truncate(s.Tags, w-5)+reset),
			boxLine(w, " "+fgMuted+truncate(s.Country, w-5)+reset),
			boxLine(w, ""),
			boxLine(w, fmt.Sprintf(" %sCodec%s    %s", purple, reset, s.Codec)),
			boxLine(w, fmt.Sprintf(" %sBitrate%s  %d kbps", purple, reset, s.Bitrate)),
			boxLine(w, fmt.Sprintf(" %sVotes%s    %d", purple, reset, s.Votes)),
			boxLine(w, ""),
			boxLine(w, " "+green+"CURRENT TRACK"+reset),
			boxLine(w, " "+fgText+truncate(firstNonEmpty(a.trackTitle, "Waiting for stream metadata…"), w-5)+reset),
		)
	}
	lines = append(lines, boxLine(w, ""))
	for len(lines) < h-1 {
		lines = append(lines, boxLine(w, ""))
	}
	lines = append(lines, boxBottom(w))
	return lines[:h]
}

func renderManagerFooter(a *App) string {
	if a.inputMode != "" {
		content := a.inputMode + "> " + a.inputBuffer + "_   Enter apply   Esc cancel"
		return " " + magenta + truncate(content, a.width-1) + reset
	}
	hint := "↑↓ move  Enter play  Space ★  d unfav  Tab player  q quit"
	if a.width >= 80 {
		hint = "↑↓ move  PgUp/Dn page  Enter play  Space ★  d unfav  / search  g genre  Tab player  ? help  q quit"
	}
	if a.width >= 105 {
		hint = "↑↓ move  PgUp/PgDn page  Home/End  Enter play  Space ★  d unfav  x hide  ⌫ pre-browse  / search  g genre  Tab player  ? help  q quit"
	}
	return " " + fgMuted + hint + reset
}

// --- Player page --------------------------------------------------------

func renderPlayerPage(a *App) string {
	if a.playerLayout == layoutMini {
		return renderPlayerMini(a)
	}
	// When vertical space is tight, keep the identity and visualizer rather
	// than squeezing in a favourites panel with no useful room of its own.
	if a.height < 20 && a.playerLayout == layoutNormal {
		return renderPlayerCompact(a)
	}
	switch a.playerLayout {
	case layoutVisual:
		return renderPlayerVisual(a)
	case layoutCompact:
		return renderPlayerCompact(a)
	default:
		return renderPlayerNormal(a)
	}
}

// renderPlayerMini is deliberately borderless: at the true 24x4 floor,
// decoration would consume half the useful rows and columns. It keeps the
// controls needed to use Radiowave as a small always-visible radio remote;
// when more than four rows are available, the surplus becomes a visualizer.
func renderPlayerMini(a *App) string {
	lines := []string{renderMiniHeader(a)}

	state := orange + "■ STOPPED" + reset
	if a.reconnecting {
		state = orange + "● CONNECTING" + reset
	} else if a.playing {
		state = green + "● LIVE" + reset
	}
	station := fgMuted + "Nothing playing" + reset
	if a.now != nil {
		station = bold + fgText + a.now.Name + reset
	}
	lines = append(lines, " "+truncate(state+"  "+station, a.width-1))

	track := "No track metadata"
	if a.errText != "" {
		track = "Error: " + a.errText
	} else if a.trackTitle != "" {
		track = a.trackTitle
	} else if a.now != nil && strings.TrimSpace(a.now.Tags) != "" {
		track = a.now.Tags
	}
	lines = append(lines, " "+fgMuted+truncate(track, a.width-1)+reset)

	vizRows := max(0, a.height-len(lines)-1)
	if vizRows > 0 && a.visualizer != nil {
		for _, row := range renderVisualizerPanel(a, vizRows, max(1, a.width-2), len(lines)+1, 2) {
			lines = append(lines, " "+row)
		}
	}
	for len(lines) < a.height-1 {
		lines = append(lines, "")
	}
	lines = append(lines, renderMiniFooter(a))
	return strings.Join(lines, "\n")
}

func renderMiniHeader(a *App) string {
	left := bold + magenta + "Radiowave" + reset + fgMuted + " · MINI" + reset
	volume := 80
	muted := false
	if a.player != nil {
		volume = a.player.volume
		muted = a.player.muted
	}
	right := fmt.Sprintf("%d%%", volume)
	if muted {
		right = "MUTE"
	}
	gap := max(1, a.width-visibleLen(left)-visibleLen(right)-1)
	return " " + truncate(left+strings.Repeat(" ", gap)+fgText+right+reset, a.width-1)
}

func renderMiniFooter(a *App) string {
	hint := "s stop  m mute  -/+  q"
	if a.width >= fullMinWidth && a.height >= fullMinHeight {
		hint = "s stop  m mute  -/+ vol  z layout  Tab browse  q"
	}
	return " " + fgMuted + truncate(hint, a.width-1) + reset
}

func renderPlayerNormal(a *App) string {
	var out strings.Builder
	out.WriteString(renderHeader(a, "PLAYER"))
	nowH, listRows := a.playerLayoutMetrics()
	nowPanel := renderNowPlayingPanel(a, a.width, nowH, true, 2)
	for _, l := range nowPanel {
		out.WriteString(l + "\n")
	}
	favPanel := renderPlayerFavourites(a, a.width, listRows)
	for _, l := range favPanel {
		out.WriteString(l + "\n")
	}
	out.WriteString(renderPlayerFooter(a, false))
	return out.String()
}

func renderPlayerCompact(a *App) string {
	var out strings.Builder
	out.WriteString(renderHeader(a, "PLAYER"))
	nowPanel := renderNowPlayingPanel(a, a.width, max(1, a.height-2), true, 2)
	for _, l := range nowPanel {
		out.WriteString(l + "\n")
	}
	out.WriteString(renderPlayerFooter(a, true))
	return out.String()
}

func renderPlayerVisual(a *App) string {
	var out strings.Builder
	out.WriteString(renderHeader(a, "PLAYER"))
	// Header, box borders, now-playing status, and footer are the only fixed
	// rows. The old permanent VISUAL label row now belongs to the graph.
	h := a.height - 5
	if h < 1 {
		h = 1
	}
	out.WriteString(border + "┌" + strings.Repeat("─", a.width-2) + "┐" + reset + "\n")
	graph := renderVisualizerPanel(a, h, a.width-4, 3, 3)
	for _, line := range graph {
		out.WriteString(border + "│" + reset + " " + pad(line, a.width-4) + " " + border + "│" + reset + "\n")
	}
	out.WriteString(border + "└" + strings.Repeat("─", a.width-2) + "┘" + reset + "\n")
	now := "Nothing playing"
	if a.now != nil {
		state := orange + "stopped" + reset
		if a.reconnecting {
			state = orange + "● connecting" + reset
		} else if a.playing {
			state = green + "● live" + reset
		}
		now = bold + a.now.Name + reset + "  " + state
		if a.trackTitle != "" {
			now += fgMuted + "  │  " + reset + a.trackTitle
		}
	}
	out.WriteString(" " + truncate(now, a.width-2) + "\n")
	out.WriteString(renderPlayerFooter(a, true))
	return out.String()
}

// visualizerLabel names the active style and data source, tagging the
// source "demo" only when it actually is one — real analysis must never
// look like a demo, and vice versa (project rule: demo data must never be
// presented as real analysis).
func visualizerLabel(a *App) string {
	label := a.activeVizStyle().Label() + " · " + a.visualizer.Name()
	if a.visualizer.Demo() {
		label += " · demo"
	}
	return label
}

// renderVisualizerPanel briefly puts the selected style/source name inside
// the graph itself. It costs no permanent UI row and disappears after the
// user has had enough time to identify a newly selected style.
//
// top/left are the panel's 1-based screen position. Pixel styles leave the
// cells blank and record the rectangle so the frame image can be placed
// under them; the label still draws on top of the image.
func renderVisualizerPanel(a *App, rows, cols, top, left int) []string {
	var graph []string
	if a.pixelStyleActive() {
		rows = max(0, min(rows, a.height-top+1))
		a.pixRect = vizRect{top: top, left: left, rows: rows, cols: cols}
		graph = make([]string, rows)
	} else {
		graph = a.renderVisualizer(rows, cols)
	}
	if len(graph) == 0 || !time.Now().Before(a.vizLabelUntil) {
		return graph
	}
	label := truncate(visualizerLabel(a), cols)
	indent := max(0, (cols-visibleLen(label))/2)
	graph[0] = strings.Repeat(" ", indent) + fgMuted + label + reset
	return graph
}

// renderNowPlayingPanel draws the panel with its top border on screen row top.
func renderNowPlayingPanel(a *App, w, h int, withVisualizer bool, top int) []string {
	lines := []string{boxTop(w, "NOW PLAYING")}
	if a.now == nil {
		lines = append(lines, boxLine(w, " "+fgMuted+"Nothing playing — press Tab to browse stations."+reset))
	} else {
		s := a.now
		state := orange + "● stopped" + reset
		if a.reconnecting {
			state = orange + "● CONNECTING" + reset
		} else if a.playing {
			state = green + "● LIVE" + reset
		}
		codec := fmt.Sprintf("%s%s %dk%s", fgMuted, s.Codec, s.Bitrate, reset)
		nameLine := fmt.Sprintf(" %s%s%s", bold+fgText, truncate(s.Name, w-30), reset)
		gap := w - visibleLen(nameLine) - visibleLen(state) - visibleLen(codec) - 4
		if gap < 1 {
			gap = 1
		}
		lines = append(lines, boxLine(w, nameLine+strings.Repeat(" ", gap)+state+"  "+codec))
		if a.trackTitle != "" {
			lines = append(lines, boxLine(w, " "+fgText+truncate(a.trackTitle, w-5)+reset))
		}
		if strings.TrimSpace(s.Tags) != "" {
			lines = append(lines, boxLine(w, " "+magenta+truncate(s.Tags, w-5)+reset))
		}
	}
	if withVisualizer {
		vizRows := h - len(lines) - 1
		if vizRows < 1 {
			vizRows = 1
		}
		for _, row := range renderVisualizerPanel(a, vizRows, w-4, top+len(lines), 3) {
			lines = append(lines, boxLine(w, " "+row))
		}
	}
	for len(lines) < h-1 {
		lines = append(lines, boxLine(w, ""))
	}
	lines = append(lines, boxBottom(w))
	if len(lines) > h {
		lines = lines[:h]
	}
	return lines
}

func renderPlayerFavourites(a *App, w, rows int) []string {
	favs := a.favoriteStations()
	lines := []string{boxTop(w, fmt.Sprintf("FAVOURITES · %d stations", len(favs)))}
	if len(favs) == 0 {
		lines = append(lines, boxLine(w, " "+fgMuted+"No favourites yet. Press Tab to browse the Manager and Space to favourite a station."+reset))
	}
	end := min(len(favs), a.playerOffset+rows)
	for i := a.playerOffset; i < end; i++ {
		s := favs[i]
		marker := "  "
		if i == a.playerCursor {
			marker = "> "
		}
		playing := ""
		if a.now != nil && a.now.StationUUID == s.StationUUID {
			if a.reconnecting {
				playing = orange + "● connecting" + reset
			} else if a.playing {
				playing = green + "● playing" + reset
			} else {
				playing = orange + "○ stopped" + reset
			}
		}
		nameW := max(10, w-40)
		tagsW := 20
		name := a.stationListName(s, nameW, i, i == a.playerCursor, time.Now())
		content := fmt.Sprintf(" %s%-*s %-*s %s", marker, nameW, name, tagsW, truncate(s.Tags, tagsW), playing)
		if i == a.playerCursor {
			content = bgSel + fgSelected + content + reset
		}
		lines = append(lines, boxLine(w, content))
	}
	for len(lines) < rows+1 {
		lines = append(lines, boxLine(w, ""))
	}
	lines = append(lines, boxBottom(w))
	if len(lines) > rows+2 {
		lines = lines[:rows+2]
	}
	return lines
}

func renderPlayerFooter(a *App, visual bool) string {
	if a.width < 80 {
		if !visual {
			return " " + fgMuted + "↑↓ move  Enter play  z layout  v/V viz  c settings  Tab manager  q quit" + reset
		}
		return " " + fgMuted + "z layout  v/V viz  m mute  c settings  Tab manager  q quit" + reset
	}
	if visual {
		return " " + fgMuted + "z layout  v/V viz style  s stop  m mute  +/- volume  c settings  Tab manager  ? help  q quit" + reset
	}
	hint := "↑↓ move  Enter play  Del ★  z layout  v/V viz  Tab manager  ? help  q quit"
	if a.width >= 110 {
		hint = "↑↓ move  PgUp/PgDn page  Enter play  Del ★  z layout  v/V viz style  s stop  m mute  +/- volume  Tab manager  ? help  q quit"
	}
	return " " + fgMuted + hint + reset
}

func helpVisualizerNote(a *App) string {
	for _, v := range a.visualizers {
		if !v.Demo() {
			return green + "cava is providing real audio analysis (v/V to cycle visualizers)."
		}
	}
	return orange + "cava not found — using demo waveforms, not real audio analysis."
}

func renderGenrePicker(a *App) string {
	w := min(a.width-4, 70)
	rows := a.genreListRows()
	tags := a.filteredGenreTags()

	filterText := a.genreFilter
	if filterText == "" {
		filterText = fgMuted + "type to filter…" + reset + magenta
	}
	lines := []string{
		boxTop(w, fmt.Sprintf("GENRES · %d", len(a.genreTags))),
		boxLine(w, " "+magenta+"Search: "+reset+filterText+"_"+reset),
		boxLine(w, ""),
	}

	switch {
	case a.genreLoading:
		lines = append(lines, boxLine(w, " "+orange+"Loading genres…"+reset))
	case a.genreErr != "":
		lines = append(lines, boxLine(w, " "+red+truncate(a.genreErr, w-4)+reset))
	case len(tags) == 0:
		lines = append(lines, boxLine(w, " "+fgMuted+"No matching genres."+reset))
	default:
		end := min(len(tags), a.genreOffset+rows)
		for i := a.genreOffset; i < end; i++ {
			t := tags[i]
			marker := "  "
			if i == a.genreCursor {
				marker = "> "
			}
			nameW := max(10, w-16)
			content := fmt.Sprintf(" %s%-*s %6d", marker, nameW, truncate(t.Name, nameW), t.StationCount)
			if i == a.genreCursor {
				content = bgSel + fgSelected + content + reset
			} else {
				content = fgText + content + reset
			}
			lines = append(lines, boxLine(w, content))
		}
	}
	for len(lines) < rows+3 {
		lines = append(lines, boxLine(w, ""))
	}
	lines = append(lines, boxLine(w, ""))
	lines = append(lines, boxLine(w, " "+fgMuted+"↑↓ move  PgUp/PgDn page  Enter select  Esc cancel  type to filter"+reset))
	lines = append(lines, boxBottom(w))

	left := max(0, (a.width-w)/2)
	top := max(0, (a.height-len(lines))/2)
	var out strings.Builder
	out.WriteString(strings.Repeat("\n", top))
	for _, line := range lines {
		out.WriteString(strings.Repeat(" ", left) + line + "\n")
	}
	return out.String()
}

func renderHelp(a *App) string {
	w := min(a.width-4, 90)
	lines := []string{
		boxTop(w, "HELP"),
		boxLine(w, " "+bold+magenta+"Radiowave v"+Version+reset+"  terminal internet radio"),
		boxLine(w, ""),
		boxLine(w, " "+green+"PAGES"+reset),
		boxLine(w, "  Tab          switch between Player and Manager"),
		boxLine(w, "  c            colour theme settings (available from either page)"),
		boxLine(w, "  Playback is global: switching pages never interrupts a stream."),
		boxLine(w, ""),
		boxLine(w, " "+green+"PLAYER"+reset),
		boxLine(w, "  ↑/↓ or j/k   move favourite selection"),
		boxLine(w, "  Enter        play highlighted favourite"),
		boxLine(w, "  Del          remove highlighted favourite"),
		boxLine(w, "  z            cycle layout: normal / visual / compact / mini"),
		boxLine(w, "  v / V        next / previous visualizer style"),
		boxLine(w, "                classic, mirrored, blocks, peaks, VU, oscilloscope, waterfall"),
		boxLine(w, ""),
		boxLine(w, " "+green+"MANAGER"+reset),
		boxLine(w, "  ↑/↓ or j/k   move station selection; list scrolls with it"),
		boxLine(w, "  PgUp/PgDn    move by one visible page"),
		boxLine(w, "  Home/End     first / last loaded station"),
		boxLine(w, "  Enter        play selected station (normal playback, not a preview)"),
		boxLine(w, "  Space        toggle favourite"),
		boxLine(w, "  x            hide station locally (z view restores it)"),
		boxLine(w, "  Backspace    return to the station playing before you browsed"),
		boxLine(w, "  t/p/r        trending / popular / random"),
		boxLine(w, "  /            search station names"),
		boxLine(w, "  g            genre picker — live filter, ↑↓ + Enter to browse"),
		boxLine(w, "  f/h/z        favorites / history / hidden stations"),
		boxLine(w, "  Long remote lists load another 100 rows near the bottom."),
		boxLine(w, ""),
		boxLine(w, " "+green+"PLAYBACK (either page)"+reset),
		boxLine(w, "  s            stop"),
		boxLine(w, "  m            mute"),
		boxLine(w, "  +/-          volume"),
		boxLine(w, "  ?            close this help"),
		boxLine(w, "  q            quit"),
		boxLine(w, ""),
		boxLine(w, " "+helpVisualizerNote(a)+reset),
		boxBottom(w),
	}
	left := max(0, (a.width-w)/2)
	top := max(0, (a.height-len(lines))/2)
	var out strings.Builder
	out.WriteString(strings.Repeat("\n", top))
	for _, line := range lines {
		out.WriteString(strings.Repeat(" ", left) + line + "\n")
	}
	return out.String()
}

func joinColumns(cols ...[]string) string {
	h := 0
	for _, c := range cols {
		if len(c) > h {
			h = len(c)
		}
	}
	var out strings.Builder
	for y := 0; y < h; y++ {
		for i, c := range cols {
			if y < len(c) {
				out.WriteString(c[y])
			}
			if i < len(cols)-1 {
				out.WriteString(" ")
			}
		}
		if y < h-1 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

func spectrum(levels []float64, rows int) string {
	glyphs := []rune("▁▂▃▄▅▆▇█")
	var b strings.Builder
	colors := []string{magenta, green, orange, red}
	for i, v := range levels {
		idx := int(v * float64(len(glyphs)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(glyphs) {
			idx = len(glyphs) - 1
		}
		b.WriteString(colors[(i/5)%len(colors)])
		b.WriteRune(glyphs[idx])
	}
	b.WriteString(reset)
	return b.String()
}

func verticalSpectrum(levels []float64, height, width int) []string {
	if height < 1 {
		return nil
	}
	if len(levels) < 1 {
		return make([]string, height)
	}
	gap := 1
	maxBars := width / 2
	if maxBars < 1 {
		maxBars = 1
	}
	if len(levels) > maxBars {
		levels = levels[:maxBars]
	}
	rows := make([]string, height)
	colors := []string{magenta, green, orange, red, purple}
	for row := 0; row < height; row++ {
		threshold := float64(height-row) / float64(height)
		var b strings.Builder
		for i, v := range levels {
			if v >= threshold {
				b.WriteString(colors[(i/7)%len(colors)])
				b.WriteRune('█')
			} else {
				b.WriteRune(' ')
			}
			if gap > 0 {
				b.WriteRune(' ')
			}
		}
		b.WriteString(reset)
		rows[row] = b.String()
	}
	return rows
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
