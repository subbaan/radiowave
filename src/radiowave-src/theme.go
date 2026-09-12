package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	themeModeDefault = "default"
	themeModeKitty   = "kitty"
	themeModeFile    = "file"
)

// ThemePalette is deliberately semantic. Importers translate terminal and
// Base16 slots once; render.go only needs to know what a colour means.
type ThemePalette struct {
	Name, Background, Foreground, Muted, Accent string
	Success, Warning, Error, Secondary, Info    string
	SelectionBackground, SelectionForeground    string
	Border                                      string
}

var defaultPalette = ThemePalette{
	Name:                "Radiowave default",
	Background:          "#17131c",
	Foreground:          "#eeeeee",
	Muted:               "#9999a6",
	Accent:              "#ff5cb0",
	Success:             "#a6e22e",
	Warning:             "#fd971f",
	Error:               "#f92672",
	Secondary:           "#ae81ff",
	Info:                "#66d9cb",
	SelectionBackground: "#4a1c43",
	SelectionForeground: "#eeeeee",
	Border:              "#56425c",
}

// These names are retained for the renderer and visualizer code, but are
// variables now: applying a palette changes every existing view at once.
var (
	reset, fgText, fgMuted, magenta, green, orange string
	red, purple, cyanish, bgSel, bgBase, border    string
	fgSelected                                     string
	activePalette                                  ThemePalette
)

func init() { applyTheme(defaultPalette) }

func applyTheme(p ThemePalette) {
	activePalette = p
	fgText = ansiForeground(p.Foreground)
	fgMuted = ansiForeground(p.Muted)
	magenta = ansiForeground(p.Accent)
	green = ansiForeground(p.Success)
	orange = ansiForeground(p.Warning)
	red = ansiForeground(p.Error)
	purple = ansiForeground(p.Secondary)
	cyanish = ansiForeground(p.Info)
	fgSelected = ansiForeground(p.SelectionForeground)
	bgSel = ansiBackground(p.SelectionBackground)
	bgBase = ansiBackground(p.Background)
	border = ansiForeground(p.Border)
	// A plain SGR reset would leak the terminal's own foreground/background
	// into padding. Re-apply the active base colours after every reset.
	reset = "\x1b[0m" + fgText + bgBase
}

func ansiForeground(hex string) string { return ansiColour(38, hex) }
func ansiBackground(hex string) string { return ansiColour(48, hex) }

func ansiColour(kind int, hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return ""
	}
	r, _ := strconv.ParseUint(hex[0:2], 16, 8)
	g, _ := strconv.ParseUint(hex[2:4], 16, 8)
	b, _ := strconv.ParseUint(hex[4:6], 16, 8)
	return fmt.Sprintf("\x1b[%d;2;%d;%d;%dm", kind, r, g, b)
}

func defaultThemeSettings() ThemeSettings {
	home, _ := os.UserHomeDir()
	return ThemeSettings{
		Mode:      themeModeDefault,
		KittyPath: filepath.Join(home, ".config", "kitty", "current-theme.conf"),
	}
}

func expandThemePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if path == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return filepath.Clean(os.ExpandEnv(path))
}

func parseThemeFile(path string) (ThemePalette, []byte, error) {
	path = expandThemePath(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return ThemePalette{}, nil, err
	}
	if len(data) > 2<<20 {
		return ThemePalette{}, nil, fmt.Errorf("theme file is too large")
	}
	p, err := parseTheme(data, filepath.Base(path))
	return p, data, err
}

// parseTheme accepts raw Base16/Base24 YAML, generated i3 variables, Kitty
// colour files, and a small semantic key/value format for custom palettes.
func parseTheme(data []byte, fallbackName string) (ThemePalette, error) {
	values := map[string]string{}
	name := strings.TrimSuffix(fallbackName, filepath.Ext(fallbackName))
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "scheme:") || strings.HasPrefix(lower, "name:") {
			if v := unquoteValue(line[strings.IndexByte(line, ':')+1:]); v != "" {
				name = v
			}
		}
		fields := strings.Fields(line)
		if len(fields) >= 3 && strings.EqualFold(fields[0], "set") && strings.HasPrefix(fields[1], "$base") {
			if c, ok := normaliseHex(fields[2]); ok {
				values[strings.ToLower(strings.TrimPrefix(fields[1], "$"))] = c
			}
			continue
		}
		key, value, ok := splitThemeAssignment(line)
		if !ok {
			continue
		}
		if c, ok := firstHex(value); ok {
			values[normaliseThemeKey(key)] = c
		}
	}

	p, err := paletteFromValues(values)
	if err != nil {
		return ThemePalette{}, err
	}
	p.Name = name
	return p, nil
}

func splitThemeAssignment(line string) (string, string, bool) {
	if i := strings.IndexAny(line, ":="); i > 0 {
		return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:]), true
	}
	fields := strings.Fields(line)
	if len(fields) >= 2 {
		return fields[0], strings.Join(fields[1:], " "), true
	}
	return "", "", false
}

func unquoteValue(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, " #"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return strings.Trim(strings.TrimSpace(s), "\"'")
}

func firstHex(s string) (string, bool) {
	s = strings.TrimSpace(s)
	for _, token := range strings.Fields(s) {
		token = strings.Trim(token, "\"',;")
		if c, ok := normaliseHex(token); ok {
			return c, true
		}
	}
	return "", false
}

func normaliseHex(s string) (string, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return "", false
	}
	if _, err := strconv.ParseUint(s, 16, 24); err != nil {
		return "", false
	}
	return "#" + strings.ToLower(s), true
}

func normaliseThemeKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(s, "$")))
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

func paletteFromValues(v map[string]string) (ThemePalette, error) {
	if v["base00"] != "" && v["base05"] != "" {
		return ThemePalette{
			Background: v["base00"], Foreground: v["base05"], Muted: pick(v, "base03", "base04"),
			Accent: pick(v, "base0e", "base0d"), Success: pick(v, "base0b", "base0c"),
			Warning: pick(v, "base0a", "base09"), Error: pick(v, "base08", "base09"),
			Secondary: pick(v, "base0d", "base0e"), Info: pick(v, "base0c", "base0d"),
			SelectionBackground: pick(v, "base02", "base01"), SelectionForeground: v["base05"],
			Border: pick(v, "base02", "base03"),
		}, nil
	}

	if v["background"] == "" || v["foreground"] == "" {
		return ThemePalette{}, fmt.Errorf("not a supported theme (need foreground/background or base00/base05)")
	}
	p := defaultPalette
	p.Background, p.Foreground = v["background"], v["foreground"]
	p.Muted = pick(v, "muted", "color8", "color7", "foreground")
	p.Accent = pick(v, "accent", "color13", "color5", "cursor", "foreground")
	p.Success = pick(v, "success", "color10", "color2", "foreground")
	p.Warning = pick(v, "warning", "color11", "color3", "foreground")
	p.Error = pick(v, "error", "color9", "color1", "foreground")
	p.Secondary = pick(v, "secondary", "color12", "color4", "accent", "foreground")
	p.Info = pick(v, "info", "color14", "color6", "foreground")
	p.SelectionBackground = pick(v, "selection_background", "active_tab_background", "color8", "background")
	p.SelectionForeground = pick(v, "selection_foreground", "active_tab_foreground", "foreground")
	p.Border = pick(v, "border", "color8", "inactive_border_color", "active_border_color", "muted", "foreground")
	return p, nil
}

func pick(v map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := v[key]; value != "" {
			return value
		}
	}
	return ""
}

func themeFingerprint(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

type themeFile struct {
	Path, Label string
}

func scanThemeDirectory(root string) ([]themeFile, error) {
	root = expandThemePath(root)
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", root)
	}
	var files []themeFile
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if len(files) >= 5000 || !themeFileExtension(path) {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		files = append(files, themeFile{Path: path, Label: rel})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return strings.ToLower(files[i].Label) < strings.ToLower(files[j].Label) })
	return files, nil
}

func themeFileExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml", ".conf", ".config", ".theme", ".txt":
		return true
	default:
		return false
	}
}
