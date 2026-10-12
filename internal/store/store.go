// Package store keeps active minutes per day in a JSON file, so the history
// survives after the agent deletes old transcripts.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

// Version is the format version written to and accepted from the file.
const Version = 1

// dirPerm is the mode of a store directory that Save creates. The file itself
// gets 0600 from os.CreateTemp.
const dirPerm = 0o700

// ErrVersion is returned by Load for a file written in another format version.
var ErrVersion = errors.New("unsupported store version")

// Store is the content of the file. It holds only dates and minutes, never
// message content.
type Store struct {
	Version int      `json:"version"`
	Totals  []Totals `json:"totals"`
}

// Key is the set of settings that a day's minutes depend on. Totals computed
// with different settings are kept apart, because the larger-value-wins merge
// could never take back minutes that one run with, say, a longer break added.
type Key struct {
	// Paths are the path filters; empty means every session. Their order
	// doesn't matter.
	Paths []string `json:"paths"`
	// Break is the longest gap counted as active, as time.Duration.String
	// formats it.
	Break string `json:"break"`
	// TZ is the name of the time zone for day boundaries.
	TZ string `json:"tz"`
}

// Totals are the minutes per day for one Key. Days are keyed by date in
// time.DateOnly format.
type Totals struct {
	Key

	Days map[string]int `json:"days"`
}

// Load reads the store at name. A missing file is an empty store.
func Load(name string) (*Store, error) {
	data, err := os.ReadFile(name) //nolint:gosec // G304: name is the user's own -store path.
	if errors.Is(err, fs.ErrNotExist) {
		return &Store{Version: Version}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read store: %w", err)
	}
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse store %s: %w", name, err)
	}
	if s.Version != Version {
		return nil, fmt.Errorf("%w %d in %s (want %d)", ErrVersion, s.Version, name, Version)
	}
	return &s, nil
}

// Merge folds recomputed minutes per day into the totals for key and returns
// the merged days. Days that are not recomputed keep their stored value. For a
// day in both, the larger value wins, because a lower recomputed value means
// part of that day's transcripts was already deleted. The returned map is the
// store's own, so changes to it are saved.
func (s *Store) Merge(key Key, recomputed map[string]int) map[string]int {
	t := s.totals(key)
	for day, minutes := range recomputed {
		t.Days[day] = max(t.Days[day], minutes)
	}
	return t.Days
}

// Save writes s to name, creating its directory if needed. When name is a
// symbolic link, the file it points to is replaced, so the link is kept.
func (s *Store) Save(name string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode store: %w", err)
	}
	if err = os.MkdirAll(filepath.Dir(name), dirPerm); err != nil {
		return fmt.Errorf("create store directory: %w", err)
	}
	if target, err := filepath.EvalSymlinks(name); err == nil {
		name = target
	}
	return replace(name, append(data, '\n'))
}

// totals returns the entry for key, adding an empty one if there is none.
func (s *Store) totals(key Key) *Totals {
	key.Paths = slices.Sorted(slices.Values(key.Paths))
	for i := range s.Totals {
		if t := &s.Totals[i]; t.equal(key) {
			if t.Days == nil { // "days": null or missing in the file.
				t.Days = make(map[string]int)
			}
			return t
		}
	}
	if key.Paths == nil {
		key.Paths = []string{} // Written as [] rather than null.
	}
	s.Totals = append(s.Totals, Totals{Key: key, Days: make(map[string]int)})
	return &s.Totals[len(s.Totals)-1]
}

// equal reports whether t is for key, whose paths must be sorted.
func (t *Totals) equal(key Key) bool {
	return t.Break == key.Break && t.TZ == key.TZ && slices.Equal(t.Paths, key.Paths)
}

// replace writes data to a temporary file in name's directory and renames it
// over name, so an interrupted run never leaves a broken store.
func replace(name string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(name), filepath.Base(name)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary store: %w", err)
	}
	err = writeAndClose(tmp, data)
	if err == nil {
		err = os.Rename(tmp.Name(), name)
	}
	if err != nil {
		return errors.Join(fmt.Errorf("replace store: %w", err), removeIfExists(tmp.Name()))
	}
	return nil
}

// writeAndClose writes data to f, flushes it to disk and closes f, so the
// rename in replace never exposes a partly written file.
func writeAndClose(f *os.File, data []byte) error {
	_, err := f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("write temporary store: %w", err)
	}
	return nil
}

func removeIfExists(name string) error {
	if err := os.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove temporary store: %w", err)
	}
	return nil
}
