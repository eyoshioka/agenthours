// Command agenthours prints the minutes per day you spent working with Claude
// Code, from the session logs in ~/.claude/projects.
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
	return printDaily(stdout, activity.DailyActive(times, cfg.maxBreak, cfg.loc))
}

func parseFlags(args []string) (config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return config{}, fmt.Errorf("find home directory: %w", err)
	}
	fs := flag.NewFlagSet("agenthours", flag.ContinueOnError)
	dir := fs.String("dir", filepath.Join(home, ".claude", "projects"), "Claude Code projects directory")
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
	return config{dir: *dir, maxBreak: *maxBreak, loc: loc}, nil
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

// printDaily writes one "YYYY-MM-DD<TAB>minutes" line per day, oldest first.
// Minutes are rounded down.
func printDaily(w io.Writer, daily map[string]time.Duration) error {
	for _, day := range slices.Sorted(maps.Keys(daily)) {
		if _, err := fmt.Fprintf(w, "%s\t%d\n", day, int(daily[day].Minutes())); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
	}
	return nil
}
