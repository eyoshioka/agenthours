package store

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"testing"
)

// Fixture dates and minutes.
const (
	oct8 = "2026-10-08"
	oct9 = "2026-10-09"
)

func writeFile(t *testing.T, content string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "days.json")
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestLoadMissing(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Version != Version || len(s.Totals) != 0 {
		t.Errorf("Load(missing) = %+v, want an empty version %d store", s, Version)
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr error
	}{
		{name: "not JSON", content: "{"},
		{name: "newer version", content: `{"version":2,"totals":[]}`, wantErr: ErrVersion},
		{name: "no version", content: `{"totals":[]}`, wantErr: ErrVersion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(writeFile(t, tt.content))
			if err == nil || (tt.wantErr != nil && !errors.Is(err, tt.wantErr)) {
				t.Errorf("Load() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadUnreadable(t *testing.T) {
	// A directory can't be read as a file.
	if _, err := Load(t.TempDir()); err == nil {
		t.Error("Load(directory) error = nil, want an error")
	}
}

func TestMerge(t *testing.T) {
	const stored, lower, higher = 30, 20, 40
	tests := []struct {
		name       string
		recomputed map[string]int
		want       map[string]int
	}{
		{name: "keeps days not recomputed", recomputed: map[string]int{oct9: lower}, want: map[string]int{oct8: stored, oct9: lower}},
		{name: "keeps a higher stored value", recomputed: map[string]int{oct8: lower}, want: map[string]int{oct8: stored}},
		{name: "takes a higher recomputed value", recomputed: map[string]int{oct8: higher}, want: map[string]int{oct8: higher}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Store{Version: Version, Totals: []Totals{{Paths: []string{}, Days: map[string]int{oct8: stored}}}}
			if got := s.Merge(nil, tt.recomputed); !maps.Equal(got, tt.want) {
				t.Errorf("Merge() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeKeepsPathSetsApart(t *testing.T) {
	const all, filtered = 50, 10
	s := &Store{Version: Version}
	s.Merge(nil, map[string]int{oct8: all})
	s.Merge([]string{"b", "a"}, map[string]int{oct8: filtered})

	// Same set in another order.
	got := s.Merge([]string{"a", "b"}, nil)
	if want := map[string]int{oct8: filtered}; !maps.Equal(got, want) {
		t.Errorf("Merge(a, b) = %v, want %v", got, want)
	}
	if len(s.Totals) != 2 {
		t.Errorf("len(Totals) = %d, want 2", len(s.Totals))
	}
}

func TestMergeNullDays(t *testing.T) {
	const minutes = 5
	s, err := Load(writeFile(t, `{"version":1,"totals":[{"paths":[],"days":null}]}`))
	if err != nil {
		t.Fatal(err)
	}
	got := s.Merge(nil, map[string]int{oct8: minutes})
	if want := map[string]int{oct8: minutes}; !maps.Equal(got, want) {
		t.Errorf("Merge() = %v, want %v", got, want)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	const minutes = 95
	dir := filepath.Join(t.TempDir(), "new")
	name := filepath.Join(dir, "days.json")
	s := &Store{Version: Version}
	s.Merge(nil, map[string]int{oct8: minutes})

	if err := s.Save(name); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(name)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := loaded.Merge(nil, nil), (map[string]int{oct8: minutes}); !maps.Equal(got, want) {
		t.Errorf("days after reload = %v, want %v", got, want)
	}
	assertOnlyFile(t, dir, "days.json")
}

func TestSaveFormat(t *testing.T) {
	const minutes = 95
	name := filepath.Join(t.TempDir(), "days.json")
	s := &Store{Version: Version}
	s.Merge(nil, map[string]int{oct8: minutes})

	if err := s.Save(name); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(name) //nolint:gosec // G304: the test's own temporary file.
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "version": 1,
  "totals": [
    {
      "paths": [],
      "days": {
        "2026-10-08": 95
      }
    }
  ]
}
`
	if got := string(data); got != want {
		t.Errorf("file =\n%s\nwant\n%s", got, want)
	}
}

func TestSaveErrors(t *testing.T) {
	file := writeFile(t, "")
	readOnly := t.TempDir()
	if err := os.Chmod(readOnly, 0o500); err != nil { //nolint:gosec // G302: a directory needs x to be searched.
		t.Fatal(err)
	}
	tests := []struct {
		name, path string
	}{
		{name: "parent is a file", path: filepath.Join(file, "days.json")},
		{name: "read-only directory", path: filepath.Join(readOnly, "days.json")},
		{name: "target is a directory", path: nonEmptyDir(t, t.TempDir())},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := (&Store{Version: Version}).Save(tt.path); err == nil {
				t.Errorf("Save(%s) error = nil, want an error", tt.path)
			}
		})
	}
}

// nonEmptyDir creates parent/days.json as a directory with an entry in it,
// which a rename can't replace, and returns its path.
func nonEmptyDir(t *testing.T, parent string) string {
	t.Helper()
	dir := filepath.Join(parent, "days.json")
	if err := os.MkdirAll(filepath.Join(dir, "child"), 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSaveRemovesTempFileOnError(t *testing.T) {
	parent := t.TempDir()
	if err := (&Store{Version: Version}).Save(nonEmptyDir(t, parent)); err == nil {
		t.Fatal("Save() error = nil, want an error")
	}
	assertOnlyFile(t, parent, "days.json")
}

// assertOnlyFile fails unless dir holds exactly one entry, named name.
func assertOnlyFile(t *testing.T, dir, name string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != name {
		t.Errorf("entries in %s = %v, want only %s", dir, entries, name)
	}
}
