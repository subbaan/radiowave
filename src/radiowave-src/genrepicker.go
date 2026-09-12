package main

import "strings"

type tagsResult struct {
	tags []Tag
	err  error
}

// openGenrePicker opens the live-filtering genre modal, fetching the tag
// catalogue on first use and caching it for the rest of the session (Radio
// Browser caps this at 1000 entries — small enough to filter locally
// rather than re-querying on every keystroke).
func (a *App) openGenrePicker() {
	a.genrePicker = true
	a.genreFilter = ""
	a.genreCursor = 0
	a.genreOffset = 0
	if a.genreTags == nil && !a.genreLoading {
		a.genreLoading = true
		a.genreErr = ""
		go func() {
			tags, err := a.rb.Tags()
			a.tagsCh <- tagsResult{tags: tags, err: err}
		}()
	}
}

func (a *App) handleGenrePickerKey(k Key) bool {
	switch {
	case k.Name == "ctrl+c":
		return true
	case k.Name == "esc":
		a.genrePicker = false
	case k.Name == "up":
		a.moveGenreCursor(-1)
	case k.Name == "down":
		a.moveGenreCursor(1)
	case k.Name == "pgup":
		a.moveGenreCursor(-a.genreListRows())
	case k.Name == "pgdn":
		a.moveGenreCursor(a.genreListRows())
	case k.Name == "home":
		a.genreCursor, a.genreOffset = 0, 0
	case k.Name == "end":
		if tags := a.filteredGenreTags(); len(tags) > 0 {
			a.genreCursor = len(tags) - 1
			a.ensureGenreCursorVisible()
		}
	case k.Name == "enter":
		a.selectGenre()
	case k.Name == "backspace":
		r := []rune(a.genreFilter)
		if len(r) > 0 {
			a.genreFilter = string(r[:len(r)-1])
			a.genreCursor, a.genreOffset = 0, 0
		}
	default:
		if k.Rune >= 32 {
			a.genreFilter += string(k.Rune)
			a.genreCursor, a.genreOffset = 0, 0
		}
	}
	return false
}

func (a *App) filteredGenreTags() []Tag {
	if a.genreFilter == "" {
		return a.genreTags
	}
	q := strings.ToLower(a.genreFilter)
	out := make([]Tag, 0, len(a.genreTags))
	for _, t := range a.genreTags {
		if strings.Contains(strings.ToLower(t.Name), q) {
			out = append(out, t)
		}
	}
	return out
}

func (a *App) moveGenreCursor(delta int) {
	tags := a.filteredGenreTags()
	if len(tags) == 0 {
		a.genreCursor, a.genreOffset = 0, 0
		return
	}
	a.genreCursor += delta
	if a.genreCursor < 0 {
		a.genreCursor = 0
	}
	if a.genreCursor >= len(tags) {
		a.genreCursor = len(tags) - 1
	}
	a.ensureGenreCursorVisible()
}

func (a *App) ensureGenreCursorVisible() {
	rows := a.genreListRows()
	total := len(a.filteredGenreTags())
	if a.genreCursor < a.genreOffset {
		a.genreOffset = a.genreCursor
	}
	if a.genreCursor >= a.genreOffset+rows {
		a.genreOffset = a.genreCursor - rows + 1
	}
	maxOffset := max(0, total-rows)
	if a.genreOffset > maxOffset {
		a.genreOffset = maxOffset
	}
	if a.genreOffset < 0 {
		a.genreOffset = 0
	}
}

// genreListRows is the shared budget used by both rendering and cursor
// clamping, kept modest since the picker is a centred modal, not a
// full-screen page.
func (a *App) genreListRows() int {
	rows := a.height - 12
	if rows < 5 {
		rows = 5
	}
	if rows > 20 {
		rows = 20
	}
	return rows
}

func (a *App) selectGenre() {
	tags := a.filteredGenreTags()
	if a.genreCursor < 0 || a.genreCursor >= len(tags) {
		return
	}
	tag := tags[a.genreCursor]
	a.genrePicker = false
	a.fetch(modeGenre, tag.Name)
}
