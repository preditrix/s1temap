package output

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"preditrix/s1temap/internal/engine"
	"preditrix/s1temap/internal/statusfilter"
)

func TestJSONArrayFile_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out", "results.json")
	sink, err := NewJSONArrayFile(path)
	if err != nil {
		t.Fatalf("NewJSONArrayFile: %v", err)
	}

	sink.Emit(engine.Result{URL: "https://x/a", Status: 200, Timestamp: time.Unix(1000, 0), Duration: 5 * time.Millisecond})
	sink.Emit(engine.Result{URL: "https://x/b", Err: "boom", Timestamp: time.Unix(1001, 0)})
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var arr []map[string]any
	if err := json.Unmarshal(data, &arr); err != nil {
		t.Fatalf("output is not a valid JSON array: %v\n%s", err, data)
	}
	if len(arr) != 2 {
		t.Fatalf("array len = %d, want 2", len(arr))
	}
	if _, ok := arr[1]["err"]; !ok {
		t.Errorf("second element should carry an \"err\" field: %v", arr[1])
	}
}

// Filtered forwards only matching results; the count of emitted lines drops.
func TestFiltered(t *testing.T) {
	f, _ := statusfilter.Parse("200")
	capture := &countSink{}
	sink := NewFiltered(capture, f)

	sink.Emit(engine.Result{Status: 200})
	sink.Emit(engine.Result{Status: 404})
	sink.Emit(engine.Result{Status: 200})

	if capture.n != 2 {
		t.Errorf("forwarded %d results, want 2 (only 200s)", capture.n)
	}
}

type countSink struct{ n int }

func (c *countSink) Emit(engine.Result) { c.n++ }
func (c *countSink) Close() error       { return nil }
