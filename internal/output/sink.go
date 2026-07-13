// Package output provides concurrency-safe engine.Sink implementations for the
// CLI (NDJSON stdout, TSV file, JSON-array file) plus Multi and Filtered
// combinators.
package output

import (
	"encoding/json"
	"os"
	"path/filepath"

	"preditrix/s1temap/internal/engine"
	"preditrix/s1temap/internal/statusfilter"
)

// Multi fans a Result out to several sinks.
type Multi struct {
	sinks []engine.Sink
}

// NewMulti returns a sink forwarding to all of sinks.
func NewMulti(sinks ...engine.Sink) *Multi { return &Multi{sinks: sinks} }

func (m *Multi) Emit(r engine.Result) {
	for _, s := range m.sinks {
		s.Emit(r)
	}
}

func (m *Multi) Close() error {
	var firstErr error
	for _, s := range m.sinks {
		if err := s.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Filtered forwards only the results whose status passes the filter. It fixes
// the "output everything" vs "count everything" split: the engine still counts
// every result, while this only writes the ones matching the query.
type Filtered struct {
	inner  engine.Sink
	filter *statusfilter.Filter
}

// NewFiltered wraps inner so only results matching filter are emitted.
func NewFiltered(inner engine.Sink, filter *statusfilter.Filter) *Filtered {
	return &Filtered{inner: inner, filter: filter}
}

func (f *Filtered) Emit(r engine.Result) {
	if f.filter.Match(r.Status) {
		f.inner.Emit(r)
	}
}

func (f *Filtered) Close() error { return f.inner.Close() }

// marshalResult renders a Result as the wire JSON object: an error object when
// the request failed, otherwise a status object. Shapes preserve the original
// CLI format.
func marshalResult(r engine.Result) ([]byte, error) {
	if r.Err != "" {
		return json.Marshal(struct {
			Err      string `json:"err"`
			URL      string `json:"url"`
			Time     int64  `json:"time"`
			Duration int64  `json:"duration"`
		}{r.Err, r.URL, r.Timestamp.Unix(), r.Duration.Milliseconds()})
	}
	return json.Marshal(struct {
		Status   int    `json:"status"`
		URL      string `json:"url"`
		Time     int64  `json:"time"`
		Duration int64  `json:"duration"`
	}{r.Status, r.URL, r.Timestamp.Unix(), r.Duration.Milliseconds()})
}

// ensureDir makes the parent directory of path if needed.
func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
