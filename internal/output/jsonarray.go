package output

import (
	"encoding/json"
	"os"
	"sync"

	"preditrix/s1temap/internal/engine"
)

// JSONArrayFile buffers results and writes them as a single pretty-printed JSON
// array on Close — O(n) total, unlike the old per-result rewrite.
type JSONArrayFile struct {
	mu      sync.Mutex
	path    string
	records []json.RawMessage
}

// NewJSONArrayFile prepares path (creating parent dirs) for a JSON array.
func NewJSONArrayFile(path string) (*JSONArrayFile, error) {
	if err := ensureDir(path); err != nil {
		return nil, err
	}
	return &JSONArrayFile{path: path}, nil
}

func (j *JSONArrayFile) Emit(r engine.Result) {
	rec, err := marshalResult(r)
	if err != nil {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.records = append(j.records, json.RawMessage(rec))
}

func (j *JSONArrayFile) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.records == nil {
		j.records = []json.RawMessage{}
	}
	data, err := json.MarshalIndent(j.records, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(j.path, data, 0o644)
}
