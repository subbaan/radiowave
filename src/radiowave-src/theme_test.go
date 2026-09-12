package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseKittyTheme(t *testing.T) {
	data := []byte(`foreground         #faffff
background         #242e34
background_opacity 1.0
cursor             #faffff
active_tab_foreground     #242e34
active_tab_background     #faffff
active_border_color   #faffff
inactive_border_color #242e34
color0 #242e34
color8 #384851
color1 #c8cacc
color9 #e3feff
color2 #BDC2C6
color10 #dcfeff
color3 #B8BDC2
color11 #d6f5ff
color4 #949DA2
color12 #acd1e6
color5 #ABB1B6
color13 #c7e8ff
color6 #9BA3A9
color14 #b4d6f0
color7 #CED2D5
color15 #faffff`)

	p, err := parseTheme(data, "current-theme.conf")
	if err != nil {
		t.Fatal(err)
	}
	if p.Background != "#242e34" || p.Foreground != "#faffff" {
		t.Fatalf("wrong base colours: %#v", p)
	}
	if p.Accent != "#c7e8ff" || p.Success != "#dcfeff" || p.Border != "#384851" {
		t.Fatalf("wrong Kitty semantic mapping: %#v", p)
	}
	if p.SelectionBackground != "#faffff" || p.SelectionForeground != "#242e34" {
		t.Fatalf("wrong selected-row mapping: %#v", p)
	}
}

func TestParseBase16YAML(t *testing.T) {
	data := []byte(`system: "base16"
name: "Tomorrow Night"
author: "Chris Kempson"
variant: "dark"
palette:
  base00: "1d1f21"
  base01: "282a2e"
  base02: "373b41"
  base03: "969896"
  base04: "b4b7b4"
  base05: "c5c8c6"
  base06: "e0e0e0"
  base07: "ffffff"
  base08: "cc6666"
  base09: "de935f"
  base0A: "f0c674"
  base0B: "b5bd68"
  base0C: "8abeb7"
  base0D: "81a2be"
  base0E: "b294bb"
  base0F: "a3685a"`)

	p, err := parseTheme(data, "tomorrow-night.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Tomorrow Night" || p.Background != "#1d1f21" || p.Foreground != "#c5c8c6" {
		t.Fatalf("wrong Base16 identity/base mapping: %#v", p)
	}
	if p.Error != "#cc6666" || p.Success != "#b5bd68" || p.Accent != "#b294bb" {
		t.Fatalf("wrong Base16 semantic mapping: %#v", p)
	}
}

func TestParseGeneratedI3Base16Theme(t *testing.T) {
	data := []byte(`set $base00 #1d1f21
set $base01 #282a2e
set $base02 #373b41
set $base03 #969896
set $base04 #b4b7b4
set $base05 #c5c8c6
set $base06 #e0e0e0
set $base07 #ffffff
set $base08 #cc6666
set $base09 #de935f
set $base0A #f0c674
set $base0B #b5bd68
set $base0C #8abeb7
set $base0D #81a2be
set $base0E #b294bb
set $base0F #a3685a`)
	p, err := parseTheme(data, "base16-tomorrow-night.config")
	if err != nil {
		t.Fatal(err)
	}
	if p.Background != "#1d1f21" || p.Accent != "#b294bb" {
		t.Fatalf("wrong i3 Base16 mapping: %#v", p)
	}
}

func TestThemeSourceReloadsWhenFileContentsChange(t *testing.T) {
	defer applyTheme(defaultPalette)
	dir := t.TempDir()
	path := filepath.Join(dir, "current-theme.conf")
	write := func(bg string) {
		t.Helper()
		if err := os.WriteFile(path, []byte("foreground #eeeeee\nbackground "+bg+"\ncolor8 #555555\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("#111111")
	a := &App{store: newTempStore(t), lib: withDefaults(Library{})}
	if !a.applyThemeSource(themeModeKitty, path) {
		t.Fatalf("could not apply initial theme: %s", a.themeError)
	}
	oldHash := a.themeHash
	write("#222222")
	if !a.reloadTheme(false) {
		t.Fatal("expected changed theme contents to reload")
	}
	if a.themeHash == oldHash || a.themeName != "current-theme" {
		t.Fatalf("reload did not update theme identity/hash: %#v", a)
	}
}

func TestThemeDirectoryPickerFindsSupportedFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"one.yaml", "two.config", "ignored.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("foreground #eeeeee\nbackground #111111\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	files, err := scanThemeDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0].Label != "one.yaml" || files[1].Label != "two.config" {
		t.Fatalf("unexpected theme files: %#v", files)
	}
}

func TestThemeSettingsScreenFitsMinimumTerminal(t *testing.T) {
	a := &App{
		width: 64, height: 12, configScreen: true,
		player: NewPlayer(), lib: withDefaults(Library{}), themeName: defaultPalette.Name,
	}
	rendered := renderApp(a)
	if lines := strings.Split(rendered, "\n"); len(lines) != 12 {
		t.Fatalf("settings rendered %d rows, want 12", len(lines))
	}
	for i, line := range strings.Split(renderThemeConfig(a), "\n") {
		if width := visibleLen(line); width > 64 {
			t.Fatalf("settings line %d is %d columns wide: %q", i+1, width, stripANSI(line))
		}
	}
}

func TestThemePickerLivePreviewsAndEscRestores(t *testing.T) {
	defer applyTheme(defaultPalette)
	dir := t.TempDir()
	writeTheme := func(name, background string) {
		t.Helper()
		data := "foreground #eeeeee\nbackground " + background + "\ncolor8 #555555\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeTheme("a.conf", "#111111")
	writeTheme("b.conf", "#222222")

	applyTheme(defaultPalette)
	a := &App{
		width: 80, height: 20, store: newTempStore(t),
		lib: withDefaults(Library{}), themeName: defaultPalette.Name,
	}
	a.lib.Theme.Directory = dir
	a.openThemePicker()
	if !a.themePicker || activePalette.Background != "#111111" {
		t.Fatalf("opening picker should preview first theme, picker=%v palette=%#v", a.themePicker, activePalette)
	}
	a.handleThemePickerKey(Key{Name: "down"})
	if activePalette.Background != "#222222" {
		t.Fatalf("moving selection should preview second theme, got %#v", activePalette)
	}
	if a.lib.Theme.Mode != themeModeDefault {
		t.Fatalf("preview must not persist a source, got %q", a.lib.Theme.Mode)
	}
	a.handleThemePickerKey(Key{Name: "esc"})
	if a.themePicker || activePalette != defaultPalette || a.themeName != defaultPalette.Name {
		t.Fatalf("Esc should restore original palette, picker=%v palette=%#v name=%q", a.themePicker, activePalette, a.themeName)
	}
}

func TestThemePickerEnterCommitsPreview(t *testing.T) {
	defer applyTheme(defaultPalette)
	dir := t.TempDir()
	path := filepath.Join(dir, "chosen.yaml")
	if err := os.WriteFile(path, []byte("foreground: '#eeeeee'\nbackground: '#303030'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := &App{
		width: 80, height: 20, store: newTempStore(t),
		lib: withDefaults(Library{}), themeName: defaultPalette.Name,
	}
	a.lib.Theme.Directory = dir
	a.openThemePicker()
	a.handleThemePickerKey(Key{Name: "enter"})
	if a.themePicker || a.lib.Theme.Mode != themeModeFile || a.lib.Theme.CustomPath != path {
		t.Fatalf("Enter did not commit preview: picker=%v settings=%#v", a.themePicker, a.lib.Theme)
	}
	if activePalette.Background != "#303030" {
		t.Fatalf("committed palette was not left active: %#v", activePalette)
	}
}
