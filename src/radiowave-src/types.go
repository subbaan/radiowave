package main

import "time"

const Version = "0.6.3"

type Station struct {
	ChangeUUID  string `json:"changeuuid,omitempty"`
	StationUUID string `json:"stationuuid"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	URLResolved string `json:"url_resolved"`
	Homepage    string `json:"homepage"`
	Favicon     string `json:"favicon"`
	Tags        string `json:"tags"`
	Country     string `json:"country"`
	CountryCode string `json:"countrycode"`
	Language    string `json:"language"`
	Votes       int    `json:"votes"`
	Codec       string `json:"codec"`
	Bitrate     int    `json:"bitrate"`
	ClickCount  int    `json:"clickcount"`
	ClickTrend  int    `json:"clicktrend"`
	LastCheckOK int    `json:"lastcheckok"`
}

type HistoryEntry struct {
	Station  Station   `json:"station"`
	PlayedAt time.Time `json:"played_at"`
}

type Library struct {
	Favorites map[string]Station `json:"favorites"`
	Hidden    map[string]Station `json:"hidden"`
	History   []HistoryEntry     `json:"history"`
	State     PlaybackState      `json:"state"`
	Theme     ThemeSettings      `json:"theme,omitempty"`
}

// ThemeSettings describes where Radiowave gets its palette. Paths are kept
// separate so switching sources in Settings does not make the user re-enter
// them.
type ThemeSettings struct {
	Mode       string `json:"mode,omitempty"`
	KittyPath  string `json:"kitty_path,omitempty"`
	CustomPath string `json:"custom_path,omitempty"`
	Directory  string `json:"directory,omitempty"`
}

// PlaybackState is the persisted snapshot of global playback so Radiowave can
// resume the last station and preferences after a restart or unclean exit.
type PlaybackState struct {
	Station    *Station `json:"station,omitempty"`
	WasPlaying bool     `json:"was_playing"`
	Volume     int      `json:"volume"`
	Muted      bool     `json:"muted"`
	Visualizer string   `json:"visualizer,omitempty"`
	Layout     string   `json:"layout,omitempty"`
	VizStyle   string   `json:"viz_style,omitempty"`
	VizOrder   string   `json:"viz_order,omitempty"`
}

// pageKind selects which top-level page owns the screen. Playback itself is
// global and keeps running regardless of which page is showing.
type pageKind int

const (
	pagePlayer pageKind = iota
	pageManager
)

// playerLayoutMode selects how the Player page presents itself.
type playerLayoutMode int

const (
	layoutNormal playerLayoutMode = iota
	layoutVisual
	layoutCompact
	layoutMini
	layoutCount
)

func (m playerLayoutMode) String() string {
	switch m {
	case layoutVisual:
		return "visual"
	case layoutCompact:
		return "compact"
	case layoutMini:
		return "mini"
	default:
		return "normal"
	}
}

func parsePlayerLayout(s string) playerLayoutMode {
	switch s {
	case "visual":
		return layoutVisual
	case "compact":
		return layoutCompact
	case "mini":
		return layoutMini
	default:
		return layoutNormal
	}
}

type browseMode int

const (
	modeTrending browseMode = iota
	modePopular
	modeRandom
	modeSearch
	modeGenre
	modeFavorites
	modeHistory
	modeHidden
)

func (m browseMode) String() string {
	switch m {
	case modeTrending:
		return "Trending"
	case modePopular:
		return "Popular"
	case modeRandom:
		return "Random"
	case modeSearch:
		return "Search"
	case modeGenre:
		return "Genre"
	case modeFavorites:
		return "Favorites"
	case modeHistory:
		return "History"
	case modeHidden:
		return "Hidden"
	default:
		return "Browse"
	}
}

func (m browseMode) remote() bool {
	switch m {
	case modeTrending, modePopular, modeRandom, modeSearch, modeGenre:
		return true
	default:
		return false
	}
}
