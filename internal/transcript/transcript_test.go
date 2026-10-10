package transcript

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseLine(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		want   Event
		wantOK bool
	}{
		{
			name:   "user message",
			line:   `{"type":"user","timestamp":"2026-10-08T13:58:09Z","sessionId":"s1"}`,
			want:   Event{Time: time.Date(2026, 10, 8, 13, 58, 9, 0, time.UTC), SessionID: "s1"},
			wantOK: true,
		},
		{
			name:   "assistant message",
			line:   `{"type":"assistant","timestamp":"2026-10-08T13:59:00Z","sessionId":"s1"}`,
			want:   Event{Time: time.Date(2026, 10, 8, 13, 59, 0, 0, time.UTC), SessionID: "s1"},
			wantOK: true,
		},
		{
			name: "other event type",
			line: `{"type":"attachment","timestamp":"2026-10-08T13:58:10Z","sessionId":"s1"}`,
		},
		{
			name: "no timestamp",
			line: `{"type":"user","sessionId":"s1"}`,
		},
		{
			name: "not JSON",
			line: `not json`,
		},
		{
			name: "cut off",
			line: `{"type":"user","timestamp":"2026-10-08T1`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseLine([]byte(tt.line))
			if ok != tt.wantOK {
				t.Fatalf("parseLine() ok = %v, want %v", ok, tt.wantOK)
			}
			if !got.Time.Equal(tt.want.Time) || got.SessionID != tt.want.SessionID {
				t.Errorf("parseLine() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// The fixture also holds lines that must be skipped: a non-message event, an
// attachment, a line that isn't JSON, a message without a timestamp, and a
// last line cut off without a newline.
func TestRead(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "session.jsonl"))
	if err != nil {
		t.Fatal(err)
	}

	got, err := Read(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	want := []Event{
		{Time: time.Date(2026, 10, 8, 13, 58, 9, 783_000_000, time.UTC), SessionID: "s1"},
		{Time: time.Date(2026, 10, 8, 13, 59, 0, 0, time.UTC), SessionID: "s1"},
		{Time: time.Date(2026, 10, 8, 14, 5, 0, 0, time.UTC), SessionID: "s1"},
	}
	if len(got) != len(want) {
		t.Fatalf("Read() returned %d events, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if !got[i].Time.Equal(want[i].Time) || got[i].SessionID != want[i].SessionID {
			t.Errorf("event %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// Real transcripts have lines over 1 MB, beyond bufio.Scanner's default limit.
func TestReadLongLine(t *testing.T) {
	const textSize = 2 << 20 // 2 MiB
	text := strings.Repeat("x", textSize)
	in := `{"type":"user","timestamp":"2026-10-08T13:58:09Z","sessionId":"s1","message":{"content":"` +
		text + `"}}` + "\n"

	got, err := Read(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("Read() returned %d events, want 1", len(got))
	}
}

var errDisk = errors.New("disk error")

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func TestReadError(t *testing.T) {
	_, err := Read(failingReader{err: errDisk})
	if !errors.Is(err, errDisk) {
		t.Errorf("Read() error = %v, want it to wrap %v", err, errDisk)
	}
}

// A complete last line without a trailing newline still counts; only lines
// that don't decode are skipped.
func TestReadLastLineWithoutNewline(t *testing.T) {
	in := `{"type":"user","timestamp":"2026-10-08T13:58:09Z","sessionId":"s1"}`

	got, err := Read(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("Read() returned %d events, want 1", len(got))
	}
}
