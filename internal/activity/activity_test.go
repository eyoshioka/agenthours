package activity

import (
	"maps"
	"slices"
	"testing"
	"time"
)

const maxBreak = 15 * time.Minute

// Day keys in time.DateOnly format.
const (
	oct8 = "2026-10-08"
	oct9 = "2026-10-09"
)

func tokyo(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// times parses each "2006-01-02 15:04" string as Tokyo time.
func times(t *testing.T, ss ...string) []time.Time {
	t.Helper()
	loc := tokyo(t)
	ts := make([]time.Time, 0, len(ss))
	for _, s := range ss {
		tm, err := time.ParseInLocation("2006-01-02 15:04", s, loc)
		if err != nil {
			t.Fatal(err)
		}
		ts = append(ts, tm)
	}
	return ts
}

func TestDailyActive(t *testing.T) {
	tests := []struct {
		name  string
		times []time.Time
		want  map[string]time.Duration
	}{
		{
			name:  "no events",
			times: nil,
			want:  map[string]time.Duration{},
		},
		{
			name:  "a single event has no active time",
			times: times(t, "2026-10-08 10:00"),
			want:  map[string]time.Duration{},
		},
		{
			name:  "a gap within the break counts",
			times: times(t, "2026-10-08 10:00", "2026-10-08 10:10"),
			want:  map[string]time.Duration{oct8: 10 * time.Minute},
		},
		{
			name:  "a gap equal to the break counts",
			times: times(t, "2026-10-08 10:00", "2026-10-08 10:15"),
			want:  map[string]time.Duration{oct8: 15 * time.Minute},
		},
		{
			name:  "a gap longer than the break counts nothing",
			times: times(t, "2026-10-08 10:00", "2026-10-08 10:16"),
			want:  map[string]time.Duration{},
		},
		{
			name: "two runs separated by a break",
			times: times(t,
				"2026-10-08 10:00", "2026-10-08 10:10", // 10 min
				"2026-10-08 11:00", "2026-10-08 11:05", // 5 min
			),
			want: map[string]time.Duration{oct8: 15 * time.Minute},
		},
		{
			name:  "identical times count nothing",
			times: times(t, "2026-10-08 10:00", "2026-10-08 10:00"),
			want:  map[string]time.Duration{},
		},
		{
			name: "unsorted input",
			times: times(t,
				"2026-10-08 11:05", "2026-10-08 10:00",
				"2026-10-08 11:00", "2026-10-08 10:10",
			),
			want: map[string]time.Duration{oct8: 15 * time.Minute},
		},
		{
			name: "parallel sessions are not counted twice",
			times: times(t,
				"2026-10-08 10:00", "2026-10-08 10:10", // session A
				"2026-10-08 10:05", // session B
			),
			want: map[string]time.Duration{oct8: 10 * time.Minute},
		},
		{
			name:  "a gap across midnight is split",
			times: times(t, "2026-10-08 23:55", "2026-10-09 00:05"),
			want: map[string]time.Duration{
				oct8: 5 * time.Minute,
				oct9: 5 * time.Minute,
			},
		},
		{
			name: "days follow loc, not UTC",
			// 2026-10-09 00:01 and 00:05 in Tokyo are still 2026-10-08 in UTC.
			times: []time.Time{
				time.Date(2026, 10, 8, 15, 1, 0, 0, time.UTC),
				time.Date(2026, 10, 8, 15, 5, 0, 0, time.UTC),
			},
			want: map[string]time.Duration{oct9: 4 * time.Minute},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DailyActive(tt.times, maxBreak, tokyo(t))
			if !maps.Equal(got, tt.want) {
				t.Errorf("DailyActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDailyActiveKeepsInputOrder(t *testing.T) {
	in := times(t, "2026-10-08 10:10", "2026-10-08 10:00")
	want := slices.Clone(in)

	DailyActive(in, maxBreak, tokyo(t))

	if !slices.Equal(in, want) {
		t.Errorf("DailyActive reordered its input: got %v, want %v", in, want)
	}
}
