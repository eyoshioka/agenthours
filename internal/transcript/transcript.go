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
	"strings"
	"time"
)

// Event is one user or assistant message in a transcript.
type Event struct {
	Time      time.Time
	SessionID string
	// Paths are the absolute or ~/ paths the message's tool uses touch, as
	// written in the transcript (~ is not expanded).
	Paths []string
}

// line holds the fields of a transcript line that agenthours uses.
type line struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"sessionId"`
	Message   struct {
		// Content is a string for plain user messages and an array of
		// items otherwise; only arrays can hold tool uses.
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// contentItem holds the tool-use inputs that can name a path. Only tool uses
// have an input; other items (text, tool results) decode to empty fields.
type contentItem struct {
	Input struct {
		FilePath     string `json:"file_path"`     //nolint:tagliatelle // Claude Code's tool inputs are snake_case.
		NotebookPath string `json:"notebook_path"` //nolint:tagliatelle // Claude Code's tool inputs are snake_case.
		Path         string `json:"path"`
		Command      string `json:"command"`
	} `json:"input"`
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
	return Event{Time: l.Timestamp, SessionID: l.SessionID, Paths: toolPaths(l.Message.Content)}, true
}

// toolPaths returns the paths named by the tool uses in a message's content.
func toolPaths(content json.RawMessage) []string {
	var items []contentItem
	if err := json.Unmarshal(content, &items); err != nil {
		return nil // A plain string, or no content.
	}
	var paths []string
	for _, it := range items {
		for _, p := range []string{it.Input.FilePath, it.Input.NotebookPath, it.Input.Path} {
			if isPath(p) {
				paths = append(paths, p)
			}
		}
		paths = append(paths, commandPaths(it.Input.Command)...)
	}
	return paths
}

// commandPaths returns the words in a shell command that look like absolute
// or ~/ paths. Words are split at whitespace, quotes and shell operators, so
// a quoted path with spaces is cut at the first space.
func commandPaths(command string) []string {
	var paths []string
	for _, w := range strings.FieldsFunc(command, isWordBreak) {
		if isPath(w) {
			paths = append(paths, w)
		}
	}
	return paths
}

func isWordBreak(r rune) bool {
	return strings.ContainsRune(" \t\n'\"`;&|()<>=", r)
}

func isPath(s string) bool {
	return strings.HasPrefix(s, "/") || strings.HasPrefix(s, "~/") || s == "~"
}
