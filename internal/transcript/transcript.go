// Package transcript reads Claude Code session transcripts
// (~/.claude/projects/<project>/<session>.jsonl).
package transcript

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"time"
)

// Event is one user or assistant message in a transcript.
type Event struct {
	Time      time.Time
	SessionID string
}

// line holds the fields of a transcript line that agenthours uses.
type line struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"sessionId"`
}

// Read returns the user and assistant events in r, in file order.
//
// Lines that don't decode are skipped: the last line may be cut off while a
// session is still writing, and the format is not a public API.
func Read(r io.Reader) ([]Event, error) {
	// bufio.Scanner is not used because a line can exceed its 64 KiB default
	// limit (for example, a tool result holding a whole file).
	br := bufio.NewReader(r)
	var events []Event
	for {
		b, err := br.ReadBytes('\n')
		if e, ok := parseLine(b); ok {
			events = append(events, e)
		}
		if errors.Is(err, io.EOF) {
			return events, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read transcript: %w", err)
		}
	}
}

// ReadFS returns the events of every transcript in fsys, laid out as
// <project>/<session>.jsonl like ~/.claude/projects. Files are read in
// lexical order; deeper files, such as subagent transcripts, are not read.
func ReadFS(fsys fs.FS) ([]Event, error) {
	names, err := fs.Glob(fsys, "*/*.jsonl")
	if err != nil {
		return nil, fmt.Errorf("list transcripts: %w", err)
	}
	var events []Event
	for _, name := range names {
		es, err := readFile(fsys, name)
		if err != nil {
			return nil, err
		}
		events = append(events, es...)
	}
	return events, nil
}

func readFile(fsys fs.FS, name string) (events []Event, err error) {
	f, err := fsys.Open(name)
	if err != nil {
		return nil, fmt.Errorf("open transcript: %w", err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			err = errors.Join(err, fmt.Errorf("close %s: %w", name, cerr))
		}
	}()
	events, err = Read(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return events, nil
}

// parseLine returns the event in one transcript line. ok is false when the
// line is not a user or assistant message, or doesn't decode.
func parseLine(b []byte) (e Event, ok bool) {
	var l line
	if err := json.Unmarshal(b, &l); err != nil {
		return Event{}, false
	}
	if l.Type != "user" && l.Type != "assistant" {
		return Event{}, false
	}
	if l.Timestamp.IsZero() {
		return Event{}, false
	}
	return Event{Time: l.Timestamp, SessionID: l.SessionID}, true
}
