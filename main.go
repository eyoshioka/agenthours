// Command agenthours prints the minutes per day you spent working with Claude
// Code, from the session logs in ~/.claude/projects. It keeps the totals in a
// store file, so days whose logs Claude Code has deleted are still printed.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"time"
	_ "time/tzdata" // -tz works on systems without a time zone database.

	"github.com/eyoshioka/agenthours/internal/activity"
	"github.com/eyoshioka/agenthours/internal/store"
	"github.com/eyoshioka/agenthours/internal/transcript"
)

const (
	defaultBreak = 15 * time.Minute
	defaultTZ    = "Asia/Tokyo"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "agenthours:", err)
		os.Exit(1)
	}
}

type config struct {
	dir      string
	store    string
	maxBreak time.Duration
	loc      *time.Location
}

func run(args []string, stdout io.Writer) error {
	cfg, err := parseFlags(args)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	times, err := readTimes(cfg.dir)
	if err != nil {
		return err
	}
	daily, err := mergeStore(cfg, minutes(activity.DailyActive(times, cfg.maxBreak, cfg.loc)))
	if err != nil {
		return err
	}
	return printDaily(stdout, daily)
}

func parseFlags(args []string) (config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return config{}, fmt.Errorf("find home directory: %w", err)
	}
	fs := flag.NewFlagSet("agenthours", flag.ContinueOnError)
	dir := fs.String("dir", filepath.Join(home, ".claude", "projects"), "Claude Code projects directory")
	storeFile := fs.String("store", filepath.Join(dataHome(home), "agenthours", "days.json"), "file that keeps the daily totals")
	maxBreak := fs.Duration("break", defaultBreak, "longest gap between messages that still counts as active")
	tz := fs.String("tz", defaultTZ, "time zone for day boundaries")
	err = fs.Parse(args)
	if err != nil {
		return config{}, fmt.Errorf("parse flags: %w", err)
	}
	loc, err := time.LoadLocation(*tz)
	if err != nil {
		return config{}, fmt.Errorf("load time zone: %w", err)
	}
	return config{dir: *dir, store: *storeFile, maxBreak: *maxBreak, loc: loc}, nil
}

// dataHome returns $XDG_DATA_HOME, or ~/.local/share when it is unset or
// relative, as the XDG Base Directory Specification says.
func dataHome(home string) string {
	if dir := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(home, ".local", "share")
}

// readTimes returns the time of every message in the transcripts under dir.
// os.Root keeps symlinks from leading outside dir.
func readTimes(dir string) (times []time.Time, err error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open projects directory: %w", err)
	}
	defer func() {
		if cerr := root.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("close projects directory: %w", cerr))
		}
	}()
	events, err := transcript.ReadFS(root.FS())
	if err != nil {
		return nil, fmt.Errorf("read transcripts: %w", err)
	}
	times = make([]time.Time, 0, len(events))
	for _, e := range events {
		times = append(times, e.Time)
	}
	return times, nil
}

// minutes converts active time per day to whole minutes, rounded down.
func minutes(daily map[string]time.Duration) map[string]int {
	m := make(map[string]int, len(daily))
	for day, d := range daily {
		m[day] = int(d.Minutes())
	}
	return m
}

// mergeStore merges the recomputed minutes into the store file and returns
// every stored day for the same settings.
func mergeStore(cfg config, recomputed map[string]int) (map[string]int, error) {
	s, err := store.Load(cfg.store)
	if err != nil {
		return nil, fmt.Errorf("load store: %w", err)
	}
	// No path filters yet; -path (#15) will add its prefixes to the key.
	key := store.Key{Break: cfg.maxBreak.String(), TZ: cfg.loc.String()}
	daily := s.Merge(key, recomputed)
	if err := s.Save(cfg.store); err != nil {
		return nil, fmt.Errorf("save store: %w", err)
	}
	return daily, nil
}

// printDaily writes one "YYYY-MM-DD<TAB>minutes" line per day, oldest first.
func printDaily(w io.Writer, daily map[string]int) error {
	for _, day := range slices.Sorted(maps.Keys(daily)) {
		if _, err := fmt.Fprintf(w, "%s\t%d\n", day, daily[day]); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
	}
	return nil
}
