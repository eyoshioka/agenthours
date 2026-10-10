// Package transcript reads Claude Code session transcripts
// (~/.claude/projects/<project>/<session>.jsonl).
package transcript

import (
	"encoding/json"
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
