package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (a *App) initTheme() {
	applyTheme(defaultPalette)
	a.themeName = defaultPalette.Name
	if a.lib.Theme.Mode != themeModeDefault {
		a.reloadTheme(true)
	}
}

func (a *App) activeThemePath() string {
	switch a.lib.Theme.Mode {
	case themeModeKitty:
		return a.lib.Theme.KittyPath
	case themeModeFile:
		return a.lib.Theme.CustomPath
	default:
		return ""
	}
}

// reloadTheme polls by content hash rather than mtime. Desktop theme tools
// often replace a file or symlink atomically, and hashing this tiny config
// once a second catches both cases reliably.
func (a *App) reloadTheme(force bool) bool {
	if a.themePicker && !force {
		return false
	}
	path := a.activeThemePath()
	if path == "" {
		return false
	}
	p, data, err := parseThemeFile(path)
	if err != nil {
		message := fmt.Sprintf("%s: %v", filepath.Base(path), err)
		changed := message != a.themeError
		a.themeError = message
		return changed
	}
	hash := themeFingerprint(data)
	if !force && hash == a.themeHash {
		return false
	}
	applyTheme(p)
	a.themeHash = hash
	a.themeName = p.Name
	a.themeError = ""
	if !force {
		a.themeMessage = "Theme reloaded after its source changed."
	}
	return true
}

func (a *App) saveThemeSettings() bool {
	if a.store == nil {
		return true
	}
	if err := a.store.Save(a.lib); err != nil {
		a.themeError = err.Error()
		return false
	}
	return true
}

func (a *App) applyThemeSource(mode, path string) bool {
	if mode == themeModeDefault {
		applyTheme(defaultPalette)
		a.lib.Theme.Mode = themeModeDefault
		a.themeName = defaultPalette.Name
		a.themeHash = ""
		a.themeError = ""
		a.themeMessage = "Using the built-in Radiowave theme."
		return a.saveThemeSettings()
	}

	p, data, err := parseThemeFile(path)
	if err != nil {
		a.themeError = fmt.Sprintf("%s: %v", filepath.Base(path), err)
		a.themeMessage = "The previous theme is still active."
		return false
	}
	applyTheme(p)
	a.lib.Theme.Mode = mode
	if mode == themeModeKitty {
		a.lib.Theme.KittyPath = expandThemePath(path)
		a.themeMessage = "Following the Kitty theme file for changes."
	} else {
		a.lib.Theme.CustomPath = expandThemePath(path)
		a.themeMessage = "Custom theme applied; file changes will reload automatically."
	}
	a.themeName = p.Name
	a.themeHash = themeFingerprint(data)
	a.themeError = ""
	return a.saveThemeSettings()
}

func (a *App) openThemeConfig() {
	a.configScreen = true
	a.themePicker = false
	a.themeEditing = false
	a.themeCursor = 0
	a.themeMessage = "Enter applies · paths support ~ and environment variables."
}

func (a *App) handleThemeConfigKey(k Key) bool {
	if a.themeEditing {
		return a.handleThemeEditKey(k)
	}
	switch {
	case k.Name == "ctrl+c" || k.Rune == 'q':
		return true
	case k.Name == "esc" || k.Rune == 'c':
		a.configScreen = false
	case k.Name == "up" || k.Rune == 'k':
		a.themeCursor = max(0, a.themeCursor-1)
	case k.Name == "down" || k.Rune == 'j':
		a.themeCursor = min(8, a.themeCursor+1)
	case k.Name == "home":
		a.themeCursor = 0
	case k.Name == "end":
		a.themeCursor = 8
	case k.Name == "enter":
		a.activateThemeConfigRow()
	}
	return false
}

func (a *App) activateThemeConfigRow() {
	switch a.themeCursor {
	case 7:
		a.vizOrder = (a.vizOrder + 1) % 2
		a.vizPeaks, a.vizHistory = nil, nil
		a.persistPlayback()
	case 8:
		a.cycleVizStyle(1)
	case 0:
		a.applyThemeSource(themeModeDefault, "")
	case 1:
		a.applyThemeSource(themeModeKitty, a.lib.Theme.KittyPath)
	case 2:
		a.startThemeEdit(2, a.lib.Theme.KittyPath)
	case 3:
		a.startThemeEdit(3, a.lib.Theme.CustomPath)
	case 4:
		a.startThemeEdit(4, a.lib.Theme.Directory)
	case 5:
		a.openThemePicker()
	case 6:
		if a.lib.Theme.Mode == themeModeDefault {
			a.themeMessage = "The built-in theme is already current."
		} else if a.reloadTheme(true) {
			a.themeMessage = "Theme reloaded from disk."
		}
	}
}

func (a *App) startThemeEdit(row int, value string) {
	a.themeEditing = true
	a.themeEditRow = row
	a.themeEditBuf = value
}

func (a *App) handleThemeEditKey(k Key) bool {
	switch k.Name {
	case "ctrl+c":
		return true
	case "esc":
		a.themeEditing = false
	case "backspace":
		r := []rune(a.themeEditBuf)
		if len(r) > 0 {
			a.themeEditBuf = string(r[:len(r)-1])
		}
	case "enter":
		value := expandThemePath(a.themeEditBuf)
		a.themeEditing = false
		switch a.themeEditRow {
		case 2:
			a.lib.Theme.KittyPath = value
			a.themeMessage = "Kitty theme path saved. Choose Follow Kitty to apply it."
			a.themeError = ""
			a.saveThemeSettings()
		case 3:
			a.lib.Theme.CustomPath = value
			a.applyThemeSource(themeModeFile, value)
		case 4:
			a.lib.Theme.Directory = value
			a.themeMessage = "Theme folder saved. Choose Browse Base16 themes."
			a.themeError = ""
			a.saveThemeSettings()
		}
	default:
		if k.Rune >= 32 {
			a.themeEditBuf += string(k.Rune)
		}
	}
	return false
}

func (a *App) openThemePicker() {
	files, err := scanThemeDirectory(a.lib.Theme.Directory)
	if err != nil {
		a.themeError = err.Error()
		a.themeMessage = "Set Theme folder to your Tinted schemes or generated themes directory."
		return
	}
	a.themeFiles = files
	a.themeFilter = ""
	a.themePickCur, a.themePickOff = 0, 0
	a.themeOriginal = activePalette
	a.themeOriginalName = a.themeName
	a.themeOriginalError = a.themeError
	a.themeOriginalNotice = a.themeMessage
	a.themePreviewErr = ""
	a.themePicker = true
	a.themeError = ""
	if len(files) == 0 {
		a.themeMessage = "No supported theme files found in that folder."
	} else {
		a.previewSelectedTheme()
	}
}

func (a *App) filteredThemeFiles() []themeFile {
	if a.themeFilter == "" {
		return a.themeFiles
	}
	q := strings.ToLower(a.themeFilter)
	out := make([]themeFile, 0, len(a.themeFiles))
	for _, file := range a.themeFiles {
		if strings.Contains(strings.ToLower(file.Label), q) {
			out = append(out, file)
		}
	}
	return out
}

func (a *App) handleThemePickerKey(k Key) bool {
	switch {
	case k.Name == "ctrl+c":
		return true
	case k.Name == "esc":
		a.cancelThemePicker()
	case k.Name == "up":
		a.moveThemePicker(-1)
	case k.Name == "down":
		a.moveThemePicker(1)
	case k.Name == "pgup":
		a.moveThemePicker(-a.themePickerRows())
	case k.Name == "pgdn":
		a.moveThemePicker(a.themePickerRows())
	case k.Name == "home":
		a.themePickCur, a.themePickOff = 0, 0
		a.previewSelectedTheme()
	case k.Name == "end":
		if files := a.filteredThemeFiles(); len(files) > 0 {
			a.themePickCur = len(files) - 1
			a.ensureThemePickerVisible()
			a.previewSelectedTheme()
		}
	case k.Name == "enter":
		files := a.filteredThemeFiles()
		if a.themePickCur >= 0 && a.themePickCur < len(files) {
			if a.applyThemeSource(themeModeFile, files[a.themePickCur].Path) {
				a.themePicker = false
			}
		}
	case k.Name == "backspace":
		r := []rune(a.themeFilter)
		if len(r) > 0 {
			a.themeFilter = string(r[:len(r)-1])
			a.themePickCur, a.themePickOff = 0, 0
			a.previewSelectedTheme()
		}
	default:
		if k.Rune >= 32 {
			a.themeFilter += string(k.Rune)
			a.themePickCur, a.themePickOff = 0, 0
			a.previewSelectedTheme()
		}
	}
	return false
}

func (a *App) themePickerRows() int { return max(1, a.height-6) }

func (a *App) moveThemePicker(delta int) {
	files := a.filteredThemeFiles()
	if len(files) == 0 {
		a.themePickCur, a.themePickOff = 0, 0
		return
	}
	a.themePickCur = min(len(files)-1, max(0, a.themePickCur+delta))
	a.ensureThemePickerVisible()
	a.previewSelectedTheme()
}

func (a *App) previewSelectedTheme() {
	files := a.filteredThemeFiles()
	if a.themePickCur < 0 || a.themePickCur >= len(files) {
		applyTheme(a.themeOriginal)
		a.themeName = a.themeOriginalName
		a.themePreviewErr = ""
		return
	}
	p, _, err := parseThemeFile(files[a.themePickCur].Path)
	if err != nil {
		applyTheme(a.themeOriginal)
		a.themeName = a.themeOriginalName
		a.themePreviewErr = err.Error()
		return
	}
	applyTheme(p)
	a.themeName = p.Name
	a.themePreviewErr = ""
}

func (a *App) cancelThemePicker() {
	applyTheme(a.themeOriginal)
	a.themeName = a.themeOriginalName
	a.themeError = a.themeOriginalError
	a.themeMessage = a.themeOriginalNotice
	a.themePreviewErr = ""
	a.themePicker = false
}

func (a *App) ensureThemePickerVisible() {
	rows := a.themePickerRows()
	if a.themePickCur < a.themePickOff {
		a.themePickOff = a.themePickCur
	}
	if a.themePickCur >= a.themePickOff+rows {
		a.themePickOff = a.themePickCur - rows + 1
	}
	a.themePickOff = min(max(0, len(a.filteredThemeFiles())-rows), max(0, a.themePickOff))
}

func renderThemeConfig(a *App) string {
	var out strings.Builder
	out.WriteString(renderHeader(a, "SETTINGS"))
	out.WriteString(boxTop(a.width, "THEME & VISUALIZER") + "\n")
	status := " Active: " + bold + a.themeName + reset + fgMuted + " · " + themeModeLabel(a.lib.Theme.Mode)
	if a.themeError != "" {
		status = " " + red + truncate(a.themeError, a.width-5) + reset
	} else if a.themeMessage != "" {
		status += " · " + a.themeMessage + reset
	}
	out.WriteString(boxLine(a.width, status) + "\n")

	rows := []string{
		"Use Radiowave default",
		"Follow Kitty theme       " + displayThemePath(a.lib.Theme.KittyPath),
		"Kitty theme file         " + displayThemePath(a.lib.Theme.KittyPath),
		"Custom theme file        " + displayThemePath(a.lib.Theme.CustomPath),
		"Theme folder             " + displayThemePath(a.lib.Theme.Directory),
		"Browse Base16 themes     " + displayThemePath(filepath.Base(a.lib.Theme.CustomPath)),
		"Reload active theme",
		"Frequency order         " + a.vizOrder.Label(),
		"Visualizer style        " + a.vizStyle.Label(),
	}
	visibleRows := max(1, a.height-5)
	start := max(0, a.themeCursor-visibleRows+1)
	end := min(len(rows), start+visibleRows)
	for i := start; i < end; i++ {
		row := rows[i]
		marker := "  "
		if (i == 0 && a.lib.Theme.Mode == themeModeDefault) || (i == 1 && a.lib.Theme.Mode == themeModeKitty) || (i == 3 && a.lib.Theme.Mode == themeModeFile) {
			marker = "● "
		}
		content := " " + marker + truncate(row, a.width-7)
		if i == a.themeCursor {
			content = bgSel + fgSelected + content + reset
		}
		out.WriteString(boxLine(a.width, content) + "\n")
	}
	out.WriteString(boxBottom(a.width) + "\n")
	footer := "↑↓ move  Enter select/edit  c/Esc close  q quit"
	if a.themeEditing {
		footer = "Path> " + a.themeEditBuf + "_   Enter save   Esc cancel"
	}
	out.WriteString(" " + fgMuted + truncate(footer, a.width-1) + reset)
	return out.String()
}

func renderThemePicker(a *App) string {
	var out strings.Builder
	out.WriteString(renderHeader(a, "SETTINGS · THEMES"))
	out.WriteString(boxTop(a.width, fmt.Sprintf("BASE16 / CUSTOM · %d files", len(a.themeFiles))) + "\n")
	filter := a.themeFilter
	if filter == "" {
		filter = fgMuted + "type to filter…" + reset + magenta
	}
	out.WriteString(boxLine(a.width, " "+magenta+"Filter: "+reset+filter+"_"+reset) + "\n")
	files := a.filteredThemeFiles()
	rows := a.themePickerRows()
	end := min(len(files), a.themePickOff+rows)
	for i := a.themePickOff; i < end; i++ {
		content := fmt.Sprintf(" %4d  %s", i+1, truncate(files[i].Label, a.width-10))
		if i == a.themePickCur {
			content = bgSel + fgSelected + content + reset
		}
		out.WriteString(boxLine(a.width, content) + "\n")
	}
	for i := end - a.themePickOff; i < rows; i++ {
		message := ""
		if i == 0 && len(files) == 0 {
			message = " " + fgMuted + "No matching supported theme files." + reset
		}
		out.WriteString(boxLine(a.width, message) + "\n")
	}
	status := fmt.Sprintf(" %d matches", len(files))
	if len(files) > 0 {
		status = fmt.Sprintf(" %d/%d previewing · Enter applies · Esc restores", a.themePickCur+1, len(files))
	}
	if a.themePreviewErr != "" {
		status = " " + red + truncate(a.themePreviewErr, a.width-5) + reset
	}
	out.WriteString(boxLine(a.width, fgMuted+status+reset) + "\n")
	out.WriteString(boxBottom(a.width) + "\n")
	out.WriteString(" " + fgMuted + "↑↓ live preview  PgUp/PgDn page  Enter apply  Esc restore  type to filter" + reset)
	return out.String()
}

func displayThemePath(path string) string {
	if strings.TrimSpace(path) == "" || path == "." {
		return "(not set)"
	}
	return path
}

func themeModeLabel(mode string) string {
	switch mode {
	case themeModeKitty:
		return "following Kitty"
	case themeModeFile:
		return "custom file"
	default:
		return "built-in"
	}
}
