package output

import (
	"io"
	"sync"

	"preditrix/s1temap/internal/engine"
)

// NDJSON writes one JSON object per line to an io.Writer (e.g. stdout). It does
// not own the writer, so Close is a no-op.
type NDJSON struct {
	mu sync.Mutex
	w  io.Writer
}

// NewNDJSON returns an NDJSON sink writing to w.
func NewNDJSON(w io.Writer) *NDJSON { return &NDJSON{w: w} }

func (n *NDJSON) Emit(r engine.Result) {
	line, err := marshalResult(r)
	if err != nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	_, _ = n.w.Write(line)
	_, _ = n.w.Write([]byte("\n"))
}

func (n *NDJSON) Close() error { return nil }
