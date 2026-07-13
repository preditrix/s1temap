// Package engine crawls a set of URLs concurrently and reports each outcome
// through a single Sink, returning an aggregate Summary.
package engine

import (
	"fmt"
	"io"
	"sort"
	"time"
)

// Result is the outcome of checking one URL.
type Result struct {
	URL        string
	Status     int    // 0 on network error
	Method     string // method that produced Status ("GET" or "HEAD")
	Fallback   bool   // HEAD failed and GET was retried
	HeadStatus int    // original HEAD status when Fallback is true
	Duration   time.Duration
	Err        string // non-empty on network error
	Timestamp  time.Time
}

// Summary aggregates a whole crawl. It counts every checked URL regardless of
// any output-side status filtering.
type Summary struct {
	Total       int
	Errors      int
	ByStatus    map[int]int
	SumDuration time.Duration // sum of per-request durations (for averaging)
	StartedAt   time.Time
	EndedAt     time.Time
	Duration    time.Duration // wall-clock duration of the whole run
}

//goland:noinspection GoUnhandledErrorResult
func (s Summary) WriteSummary(out io.Writer) {
	avg := time.Duration(0)
	if s.Total > 0 {
		avg = s.SumDuration / time.Duration(s.Total)
	}

	fmt.Fprintln(out, "--- Summary ---")
	fmt.Fprintf(out, "Total:      %d\n", s.Total)
	fmt.Fprintf(out, "Duration:   %s\n", s.Duration.Round(time.Millisecond))
	fmt.Fprintf(out, "Avg time:   %s\n", avg.Round(time.Millisecond))

	statuses := make([]int, 0, len(s.ByStatus))
	for code := range s.ByStatus {
		statuses = append(statuses, code)
	}
	sort.Ints(statuses)
	for _, code := range statuses {
		fmt.Fprintf(out, "Status %d: %d\n", code, s.ByStatus[code])
	}

	fmt.Fprintf(out, "Errors:     %d\n", s.Errors)
}

// Sink receives every Result exactly once. Emit is called concurrently from
// worker goroutines, so implementations must be safe for concurrent use. Close
// flushes and releases resources after the crawl finishes.
type Sink interface {
	Emit(Result)
	Close() error
}
