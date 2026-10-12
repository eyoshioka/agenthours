package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const (
	dirFlag   = "-dir"
	storeFlag = "-store"
)

// tempStore returns a store path in a new temporary directory, so tests never
// touch the real store.
func tempStore(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "days.json")
}

// writeProjects creates dir/<project>/<session>.jsonl with the given lines and
// returns dir.
func writeProjects(t *testing.T, lines string) string {
	t.Helper()
	dir := t.TempDir()
	project := filepath.Join(dir, "project-a")
	if err := os.Mkdir(project, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "s1.jsonl"), []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRun(t *testing.T) {
	// 23:50 and 00:05 in Tokyo: 10 minutes on Oct 8 and 5 on Oct 9.
	dir := writeProjects(t, `{"type":"user","timestamp":"2026-10-08T14:50:00Z","sessionId":"s1"}
{"type":"assistant","timestamp":"2026-10-08T15:05:00Z","sessionId":"s1"}
`)
	var out bytes.Buffer

	if err := run([]string{dirFlag, dir, storeFlag, tempStore(t)}, &out); err != nil {
		t.Fatal(err)
	}
	want := "2026-10-08\t10\n2026-10-09\t5\n"
	if got := out.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestRunFlags(t *testing.T) {
	// A 5m40s gap from 00:00 Tokyo time (still Oct 8 in UTC), printed as 5
	// because minutes are rounded down; then a 12-minute gap, which -break 10m
	// excludes and the 15m default would count.
	dir := writeProjects(t, `{"type":"user","timestamp":"2026-10-08T15:00:00Z","sessionId":"s1"}
{"type":"assistant","timestamp":"2026-10-08T15:05:40Z","sessionId":"s1"}
{"type":"user","timestamp":"2026-10-08T15:17:40Z","sessionId":"s1"}
`)
	var out bytes.Buffer

	if err := run([]string{dirFlag, dir, storeFlag, tempStore(t), "-break", "10m", "-tz", "UTC"}, &out); err != nil {
		t.Fatal(err)
	}
	want := "2026-10-08\t5\n"
	if got := out.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestRunHelp(t *testing.T) {
	if err := run([]string{"-h"}, &bytes.Buffer{}); err != nil {
		t.Errorf("run(-h) error = %v, want nil", err)
	}
}

func TestRunErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "unknown flag", args: []string{"-nope"}},
		{name: "unknown time zone", args: []string{"-tz", "Nowhere/City"}},
		{name: "missing directory", args: []string{dirFlag, filepath.Join(t.TempDir(), "missing")}},
		{name: "broken store", args: []string{dirFlag, t.TempDir(), storeFlag, brokenStore(t)}},
		{name: "unwritable store", args: []string{dirFlag, t.TempDir(), storeFlag, readOnlyStore(t)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := run(tt.args, &bytes.Buffer{}); err == nil {
				t.Error("run() error = nil, want an error")
			}
		})
	}
}

var errWrite = errors.New("write failed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

func TestRunWriteError(t *testing.T) {
	dir := writeProjects(t, `{"type":"user","timestamp":"2026-10-08T10:00:00Z","sessionId":"s1"}
{"type":"assistant","timestamp":"2026-10-08T10:05:00Z","sessionId":"s1"}
`)

	err := run([]string{dirFlag, dir, storeFlag, tempStore(t)}, failingWriter{})
	if !errors.Is(err, errWrite) {
		t.Errorf("run() error = %v, want it to wrap %v", err, errWrite)
	}
}

// brokenStore returns the path of a store file that is not valid JSON.
func brokenStore(t *testing.T) string {
	t.Helper()
	name := tempStore(t)
	if err := os.WriteFile(name, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	return name
}

// readOnlyStore returns a store path, not yet created, in a directory that
// can't be written to.
func readOnlyStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil { //nolint:gosec // G302: a directory needs x to be searched.
		t.Fatal(err)
	}
	return filepath.Join(dir, "days.json")
}

func TestRunKeepsDeletedDays(t *testing.T) {
	storeFile := tempStore(t)
	oct8 := writeProjects(t, `{"type":"user","timestamp":"2026-10-08T01:00:00Z","sessionId":"s1"}
{"type":"assistant","timestamp":"2026-10-08T01:10:00Z","sessionId":"s1"}
`)
	if err := run([]string{dirFlag, oct8, storeFlag, storeFile}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	// Oct 8's log is gone; Oct 9 is new.
	oct9 := writeProjects(t, `{"type":"user","timestamp":"2026-10-09T01:00:00Z","sessionId":"s2"}
{"type":"assistant","timestamp":"2026-10-09T01:03:00Z","sessionId":"s2"}
`)
	var out bytes.Buffer

	if err := run([]string{dirFlag, oct9, storeFlag, storeFile}, &out); err != nil {
		t.Fatal(err)
	}
	want := "2026-10-08\t10\n2026-10-09\t3\n"
	if got := out.String(); got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestDataHome(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "me")
	xdg := filepath.Join(string(filepath.Separator), "xdg")
	fallback := filepath.Join(home, ".local", "share")
	tests := []struct {
		name, env, want string
	}{
		{name: "unset", env: "", want: fallback},
		{name: "absolute", env: xdg, want: xdg},
		{name: "relative is ignored", env: "data", want: fallback},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_DATA_HOME", tt.env)
			if got := dataHome(home); got != tt.want {
				t.Errorf("dataHome(%q) = %q, want %q", home, got, tt.want)
			}
		})
	}
}
