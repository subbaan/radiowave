package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Store struct {
	path string
}

func NewStore() (*Store, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(base, "radiowave")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{path: filepath.Join(dir, "library.json")}, nil
}

func (s *Store) Load() (Library, error) {
	lib := Library{
		Favorites: make(map[string]Station),
		Hidden:    make(map[string]Station),
		History:   []HistoryEntry{},
	}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return withDefaults(lib), nil
	}
	if err != nil {
		return lib, err
	}
	if err := json.Unmarshal(data, &lib); err != nil {
		return lib, err
	}
	return withDefaults(lib), nil
}

func withDefaults(lib Library) Library {
	if lib.Favorites == nil {
		lib.Favorites = make(map[string]Station)
	}
	if lib.Hidden == nil {
		lib.Hidden = make(map[string]Station)
	}
	if lib.History == nil {
		lib.History = []HistoryEntry{}
	}
	if lib.State.Volume <= 0 {
		lib.State.Volume = 80
	}
	if lib.State.Layout == "" {
		lib.State.Layout = layoutNormal.String()
	}
	defaults := defaultThemeSettings()
	switch lib.Theme.Mode {
	case themeModeDefault, themeModeKitty, themeModeFile:
	default:
		lib.Theme.Mode = defaults.Mode
	}
	if lib.Theme.KittyPath == "" {
		lib.Theme.KittyPath = defaults.KittyPath
	}
	return lib
}

func (s *Store) Save(lib Library) error {
	data, err := json.MarshalIndent(lib, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
