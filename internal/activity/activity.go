// Package activity turns message times into active time per day.
package activity

import (
	"slices"
	"time"
)

// DailyActive returns the active time per calendar day in loc, keyed by date
// in time.DateOnly format. Days without active time are left out.
//
// times may be unsorted and may come from several sessions; they are treated
// as one timeline, so parallel sessions are not counted twice, and times is
// not modified. The gap between two consecutive times counts as active when it
// is at most maxBreak; a longer gap is a break and counts nothing. A gap that
// crosses midnight in loc is split between the two days.
func DailyActive(times []time.Time, maxBreak time.Duration, loc *time.Location) map[string]time.Duration {
	sorted := slices.SortedFunc(slices.Values(times), time.Time.Compare)
	daily := make(map[string]time.Duration)
	for i := range len(sorted) - 1 {
		start, end := sorted[i], sorted[i+1]
		if end.Sub(start) <= maxBreak {
			addSpan(daily, start.In(loc), end.In(loc))
		}
	}
	return daily
}

// addSpan adds the time from start to end to daily, split at each midnight in
// start's location.
func addSpan(daily map[string]time.Duration, start, end time.Time) {
	for start.Before(end) {
		y, m, d := start.Date()
		next := time.Date(y, m, d+1, 0, 0, 0, 0, start.Location())
		if next.After(end) {
			next = end
		}
		daily[start.Format(time.DateOnly)] += next.Sub(start)
		start = next
	}
}
