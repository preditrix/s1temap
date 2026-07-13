package httpx

import (
	"context"
	"crypto/tls"
	"net/http"
	"sync"
	"time"
)

// NewClient builds an HTTP client with the given per-request timeout.
//
// When insecure is true, TLS certificate verification is skipped. When idle is
// greater than zero, the transport is wrapped so that a global minimum interval
// of idle elapses between ANY two requests issued through this client — the
// throttle is shared across all goroutines, not per-worker.
func NewClient(timeout time.Duration, insecure bool, idle time.Duration) *http.Client {
	var transport http.RoundTripper = http.DefaultTransport

	if insecure {
		if base, ok := http.DefaultTransport.(*http.Transport); ok {
			cloned := base.Clone()
			cloned.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
			transport = cloned
		} else {
			transport = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			}
		}
	}

	if idle > 0 {
		transport = &throttledTransport{base: transport, throttle: &throttle{interval: idle}}
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

// throttle hands out request time slots spaced interval apart. It locks only to
// reserve the next slot, then callers sleep outside the lock, so it does not
// serialize goroutines on the mutex.
type throttle struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func (t *throttle) wait(ctx context.Context) error {
	t.mu.Lock()
	now := time.Now()
	if t.next.Before(now) {
		t.next = now
	}
	wait := t.next.Sub(now)
	t.next = t.next.Add(t.interval)
	t.mu.Unlock()

	if wait <= 0 {
		return nil
	}

	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type throttledTransport struct {
	base     http.RoundTripper
	throttle *throttle
}

func (tt *throttledTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := tt.throttle.wait(req.Context()); err != nil {
		return nil, err
	}
	return tt.base.RoundTrip(req)
}
