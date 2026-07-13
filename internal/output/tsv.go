package output

import (
	"fmt"
	"os"
	"sync"

	"preditrix/s1temap/internal/engine"
)

// TSVFile appends tab-separated lines to a file:
//
//	<status|err>\t<url>\t<unixSeconds>\t<ms>ms
type TSVFile struct {
	mu sync.Mutex
	f  *os.File
}

// NewTSVFile creates (with parent dirs) and opens path for appending.
func NewTSVFile(path string) (*TSVFile, error) {
	if err := ensureDir(path); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &TSVFile{f: f}, nil
}

func (t *TSVFile) Emit(r engine.Result) {
	var line string
	if r.Err != "" {
		line = fmt.Sprintf("%s\t%s\t%d\t%dms\n", r.Err, r.URL, r.Timestamp.Unix(), r.Duration.Milliseconds())
	} else {
		line = fmt.Sprintf("%d\t%s\t%d\t%dms\n", r.Status, r.URL, r.Timestamp.Unix(), r.Duration.Milliseconds())
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	_, _ = t.f.WriteString(line)
}

func (t *TSVFile) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.f.Close()
}
