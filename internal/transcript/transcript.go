// Package transcript reads Claude Code session transcripts
// (~/.claude/projects/<project>/<session>.jsonl).
package transcript

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

// Event is one user or assistant message in a transcript.
type Event struct {
	Time      time.Time
	SessionID string
}

// line holds the fields of a transcript line that ccgrass uses.
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
