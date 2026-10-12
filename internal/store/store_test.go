package store

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"testing"
)

// Fixture dates.
const (
	oct8 = "2026-10-08"
	oct9 = "2026-10-09"
)

// fixtureKey returns the fixture settings: no path filters, a 15-minute break
// and Tokyo time.
func fixtureKey() Key {
	return Key{Paths: []string{}, Break: "15m0s", TZ: "Asia/Tokyo"}
}

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
			s := &Store{Version: Version, Totals: []Totals{{Key: fixtureKey(), Days: map[string]int{oct8: stored}}}}
			if got := s.Merge(fixtureKey(), tt.recomputed); !maps.Equal(got, tt.want) {
				t.Errorf("Merge() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeKeepsKeysApart(t *testing.T) {
	const stored, other = 50, 10
	base := fixtureKey()
	tests := []struct {
		name string
		key  Key
	}{
		{name: "paths", key: Key{Paths: []string{"b", "a"}, Break: base.Break, TZ: base.TZ}},
		{name: "break", key: Key{Paths: base.Paths, Break: "1h0m0s", TZ: base.TZ}},
		{name: "time zone", key: Key{Paths: base.Paths, Break: base.Break, TZ: "UTC"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Store{Version: Version}
			s.Merge(base, map[string]int{oct8: stored})

			got := s.Merge(tt.key, map[string]int{oct8: other})
			if want := map[string]int{oct8: other}; !maps.Equal(got, want) {
				t.Errorf("Merge(%+v) = %v, want %v", tt.key, got, want)
			}
			if got := s.Merge(base, nil); !maps.Equal(got, map[string]int{oct8: stored}) {
				t.Errorf("Merge(%+v) afterwards = %v, want %d minutes", base, got, stored)
			}
		})
	}
}

func TestMergePathOrder(t *testing.T) {
	const minutes = 10
	s := &Store{Version: Version}
	s.Merge(Key{Paths: []string{"b", "a"}}, map[string]int{oct8: minutes})

	got := s.Merge(Key{Paths: []string{"a", "b"}}, nil)
	if want := map[string]int{oct8: minutes}; !maps.Equal(got, want) {
		t.Errorf("Merge(a, b) = %v, want %v", got, want)
	}
	if len(s.Totals) != 1 {
		t.Errorf("len(Totals) = %d, want 1", len(s.Totals))
	}
}

func TestMergeNullDays(t *testing.T) {
	const minutes = 5
	s, err := Load(writeFile(t, `{"version":1,"totals":[{"paths":[],"break":"15m0s","tz":"Asia/Tokyo","days":null}]}`))
	if err != nil {
		t.Fatal(err)
	}
	got := s.Merge(fixtureKey(), map[string]int{oct8: minutes})
	if want := map[string]int{oct8: minutes}; !maps.Equal(got, want) {
		t.Errorf("Merge() = %v, want %v", got, want)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	const minutes = 95
	dir := filepath.Join(t.TempDir(), "new")
	name := filepath.Join(dir, "days.json")
	s := &Store{Version: Version}
	s.Merge(fixtureKey(), map[string]int{oct8: minutes})

	if err := s.Save(name); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(name)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := loaded.Merge(fixtureKey(), nil), (map[string]int{oct8: minutes}); !maps.Equal(got, want) {
		t.Errorf("days after reload = %v, want %v", got, want)
	}
	assertOnlyFile(t, dir, "days.json")
}

func TestSaveFormat(t *testing.T) {
	const minutes = 95
	name := filepath.Join(t.TempDir(), "days.json")
	s := &Store{Version: Version}
	s.Merge(fixtureKey(), map[string]int{oct8: minutes})

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
      "break": "15m0s",
      "tz": "Asia/Tokyo",
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

func TestSaveKeepsSymlink(t *testing.T) {
	const minutes = 95
	dir := t.TempDir()
	target := filepath.Join(dir, "synced.json")
	link := filepath.Join(dir, "days.json")
	if err := os.WriteFile(target, []byte(`{"version":1,"totals":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	s := &Store{Version: Version}
	s.Merge(fixtureKey(), map[string]int{oct8: minutes})

	if err := s.Save(link); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("Lstat(%s) = %v, %v, want a symlink", link, fi, err)
	}
	loaded, err := Load(target)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := loaded.Merge(fixtureKey(), nil), (map[string]int{oct8: minutes}); !maps.Equal(got, want) {
		t.Errorf("days in the link's target = %v, want %v", got, want)
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
