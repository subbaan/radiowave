package main

import "testing"

func testGenreApp(t *testing.T) *App {
	t.Helper()
	return &App{
		height: 34,
		store:  newTempStore(t),
		lib:    Library{Favorites: map[string]Station{}, Hidden: map[string]Station{}},
		player: NewPlayer(),
		rb:     NewRadioBrowser(),
		tagsCh: make(chan tagsResult, 1),
		genreTags: []Tag{
			{Name: "synthwave", StationCount: 428},
			{Name: "synthpop", StationCount: 193},
			{Name: "rock", StationCount: 2941},
			{Name: "retrowave", StationCount: 207},
		},
	}
}

func TestFilteredGenreTagsLiveFilters(t *testing.T) {
	a := testGenreApp(t)
	a.genreFilter = "synth"
	got := a.filteredGenreTags()
	if len(got) != 2 {
		t.Fatalf("expected 2 matches for %q, got %d: %v", a.genreFilter, len(got), got)
	}
	for _, tag := range got {
		if tag.Name != "synthwave" && tag.Name != "synthpop" {
			t.Fatalf("unexpected match %q for filter %q", tag.Name, a.genreFilter)
		}
	}
}

func TestFilteredGenreTagsEmptyFilterReturnsAll(t *testing.T) {
	a := testGenreApp(t)
	if got := len(a.filteredGenreTags()); got != len(a.genreTags) {
		t.Fatalf("expected all %d tags with empty filter, got %d", len(a.genreTags), got)
	}
}

func TestGenrePickerTypingFiltersAndResetsCursor(t *testing.T) {
	a := testGenreApp(t)
	a.genrePicker = true
	a.genreCursor = 2
	a.handleGenrePickerKey(Key{Rune: 's'})
	a.handleGenrePickerKey(Key{Rune: 'y'})
	if a.genreFilter != "sy" {
		t.Fatalf("expected filter %q, got %q", "sy", a.genreFilter)
	}
	if a.genreCursor != 0 {
		t.Fatalf("expected cursor reset to 0 after typing, got %d", a.genreCursor)
	}
	if got := len(a.filteredGenreTags()); got != 2 {
		t.Fatalf("expected 2 matches for %q, got %d", a.genreFilter, got)
	}
}

func TestGenrePickerBackspaceNarrowsFilter(t *testing.T) {
	a := testGenreApp(t)
	a.genrePicker = true
	a.genreFilter = "synth"
	a.handleGenrePickerKey(Key{Name: "backspace"})
	if a.genreFilter != "synt" {
		t.Fatalf("expected filter %q after backspace, got %q", "synt", a.genreFilter)
	}
}

func TestGenrePickerEnterSelectsAndClosesPicker(t *testing.T) {
	a := testGenreApp(t)
	a.genrePicker = true
	a.genreFilter = "rock"
	a.handleGenrePickerKey(Key{Name: "enter"})
	if a.genrePicker {
		t.Fatalf("expected Enter to close the genre picker")
	}
	if a.mode != modeGenre || a.query != "rock" {
		t.Fatalf("expected fetch(modeGenre, %q), got mode=%v query=%q", "rock", a.mode, a.query)
	}
}

func TestGenrePickerEscCancelsWithoutSelecting(t *testing.T) {
	a := testGenreApp(t)
	a.genrePicker = true
	a.mode = modeTrending
	a.handleGenrePickerKey(Key{Name: "esc"})
	if a.genrePicker {
		t.Fatalf("expected Esc to close the genre picker")
	}
	if a.mode != modeTrending {
		t.Fatalf("expected Esc not to change browse mode, got %v", a.mode)
	}
}

func TestGKeyOpensGenrePickerAndTriggersFetch(t *testing.T) {
	a := testGenreApp(t)
	a.page = pageManager
	a.genreTags = nil // force a fetch
	a.handleKey(Key{Rune: 'g'})
	if !a.genrePicker {
		t.Fatalf("expected 'g' to open the genre picker")
	}
	if !a.genreLoading {
		t.Fatalf("expected genre picker to trigger a background tag fetch on first open")
	}
	<-a.tagsCh // drain so the goroutine doesn't leak past the test
}

func TestMoveGenreCursorClampsToFilteredList(t *testing.T) {
	a := testGenreApp(t)
	a.genreFilter = "synth"
	a.moveGenreCursor(10)
	if got := len(a.filteredGenreTags()); a.genreCursor != got-1 {
		t.Fatalf("expected cursor clamped to %d, got %d", got-1, a.genreCursor)
	}
	a.moveGenreCursor(-100)
	if a.genreCursor != 0 {
		t.Fatalf("expected cursor clamped to 0, got %d", a.genreCursor)
	}
}
