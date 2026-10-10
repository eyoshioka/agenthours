package transcript

import (
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
