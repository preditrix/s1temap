package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// captureSink is a concurrency-safe test sink.
type captureSink struct {
	mu      sync.Mutex
	results []Result
}

func (c *captureSink) Emit(r Result) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results = append(c.results, r)
}
func (c *captureSink) Close() error { return nil }

func TestRun_Basic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/nf" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	urls := []string{srv.URL + "/ok", srv.URL + "/ok2", srv.URL + "/nf"}
	sink := &captureSink{}
	sum, err := Run(context.Background(), urls, Config{Workers: 4, Client: srv.Client()}, sink)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sum.Total != 3 {
		t.Errorf("Total = %d, want 3", sum.Total)
	}
	if sum.ByStatus[200] != 2 || sum.ByStatus[404] != 1 {
		t.Errorf("ByStatus = %v, want 200:2 404:1", sum.ByStatus)
	}
	if len(sink.results) != 3 {
		t.Errorf("emitted %d results, want 3", len(sink.results))
	}
}

func TestRun_HeadFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sink := &captureSink{}
	_, err := Run(context.Background(), []string{srv.URL + "/x"}, Config{Workers: 1, Method: http.MethodHead, Client: srv.Client()}, sink)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := sink.results[0]
	if got.Status != 200 || !got.Fallback || got.Method != http.MethodGet || got.HeadStatus != 405 {
		t.Errorf("fallback result = %+v, want status 200 via GET fallback from HEAD 405", got)
	}
}

func TestRun_Cancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	sink := &captureSink{}
	_, err := Run(ctx, []string{"http://example.com/a", "http://example.com/b"}, Config{Workers: 2, Client: http.DefaultClient}, sink)
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
}

func TestRun_Heartbeat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	urls := make([]string, 250)
	for i := range urls {
		urls[i] = srv.URL
	}

	var mu sync.Mutex
	var beats []Summary
	_, err := Run(context.Background(), urls, Config{
		Workers:        4,
		Client:         srv.Client(),
		HeartbeatEvery: 100,
		OnHeartbeat: func(s Summary) {
			mu.Lock()
			beats = append(beats, s)
			mu.Unlock()
		},
	}, &captureSink{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 250 URLs at every-100 → heartbeats at Total 100 and 200.
	if len(beats) != 2 {
		t.Fatalf("got %d heartbeats, want 2", len(beats))
	}
	for _, b := range beats {
		if b.Total != 100 && b.Total != 200 {
			t.Errorf("heartbeat Total = %d, want 100 or 200", b.Total)
		}
		if b.ByStatus[200] != b.Total {
			t.Errorf("heartbeat ByStatus[200] = %d, want %d (snapshot must be consistent)", b.ByStatus[200], b.Total)
		}
		if b.Duration <= 0 {
			t.Errorf("heartbeat Duration = %v, want > 0", b.Duration)
		}
	}
}

func TestRun_HeartbeatDisabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	urls := make([]string, 20)
	for i := range urls {
		urls[i] = srv.URL
	}

	called := false
	_, _ = Run(context.Background(), urls, Config{
		Workers:     2,
		Client:      srv.Client(),
		OnHeartbeat: func(Summary) { called = true }, // HeartbeatEvery unset (0)
	}, &captureSink{})
	if called {
		t.Fatal("OnHeartbeat should not be called when HeartbeatEvery is 0")
	}
}
