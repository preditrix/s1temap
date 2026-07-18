package engine

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"preditrix/s1temap/internal/httpx"
)

// Config configures a crawl run.
type Config struct {
	// Workers is the number of concurrent request goroutines (min 1).
	Workers int
	// Method selects the request method. "HEAD" (the default when empty) issues a
	// HEAD with a GET fallback on 400/403/405/501; "GET" issues GET only.
	Method string
	// Client is the HTTP client used for every request (must be non-nil). Its
	// timeout and any global throttling live on the client.
	Client *http.Client
	// Options customize every outgoing request (user-agent, auth, cookies,
	// headers, prefix URL).
	Options []httpx.RequestOption
	// HeartbeatEvery, when > 0, calls OnHeartbeat with a snapshot of the running
	// Summary after every HeartbeatEvery processed URLs.
	HeartbeatEvery int
	// OnHeartbeat receives the periodic Summary snapshot (see HeartbeatEvery).
	// Run serializes these calls, but the callback should not block for long.
	OnHeartbeat func(Summary)
}

// Run checks every URL in urls using a bounded worker pool, emitting each
// Result to sink and returning an aggregate Summary. The producer closes the
// job channel and workers drain it; cancelling ctx stops the run promptly. Run
// does not close sink — the caller owns its lifecycle.
func Run(ctx context.Context, urls []string, cfg Config, sink Sink) (Summary, error) {
	workers := max(cfg.Workers, 1)
	// Default to HEAD (with GET fallback) when unset; honor an explicit GET.
	method := cfg.Method
	if method == "" {
		method = http.MethodHead
	}
	if method != http.MethodHead {
		method = http.MethodGet
	}

	summary := Summary{ByStatus: make(map[int]int), StartedAt: time.Now()}
	var mu sync.Mutex
	var hbMu sync.Mutex // serializes OnHeartbeat calls

	jobs := make(chan string)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for rawURL := range jobs {
				res := check(ctx, cfg.Client, method, rawURL, cfg.Options)

				mu.Lock()
				summary.Total++
				summary.SumDuration += res.Duration
				if res.Err != "" {
					summary.Errors++
				} else {
					summary.ByStatus[res.Status]++
				}
				var hb *Summary
				if cfg.HeartbeatEvery > 0 && cfg.OnHeartbeat != nil && summary.Total%cfg.HeartbeatEvery == 0 {
					snap := cloneSummary(summary)
					now := time.Now()
					snap.EndedAt = now
					snap.Duration = now.Sub(snap.StartedAt)
					hb = &snap
				}
				mu.Unlock()

				sink.Emit(res)

				if hb != nil {
					hbMu.Lock()
					cfg.OnHeartbeat(*hb)
					hbMu.Unlock()
				}
			}
		}()
	}

producer:
	for _, rawURL := range urls {
		select {
		case <-ctx.Done():
			break producer
		case jobs <- rawURL:
		}
	}
	close(jobs)
	wg.Wait()

	summary.EndedAt = time.Now()
	summary.Duration = summary.EndedAt.Sub(summary.StartedAt)
	return summary, ctx.Err()
}

// cloneSummary returns a deep copy of s (including its ByStatus map) so a
// heartbeat snapshot is safe to read after the lock is released.
func cloneSummary(s Summary) Summary {
	cp := s
	if s.ByStatus != nil {
		cp.ByStatus = make(map[int]int, len(s.ByStatus))
		for k, v := range s.ByStatus {
			cp.ByStatus[k] = v
		}
	}
	return cp
}

func check(ctx context.Context, client *http.Client, method, rawURL string, opts []httpx.RequestOption) Result {
	start := time.Now()
	res := Result{URL: rawURL, Timestamp: start}

	status, headStatus, fallback, finalMethod, err := doRequest(ctx, client, method, rawURL, opts)
	res.Duration = time.Since(start)
	res.Method = finalMethod
	res.Fallback = fallback
	res.HeadStatus = headStatus
	if err != nil {
		res.Err = err.Error()
		return res
	}
	res.Status = status
	return res
}

// doRequest performs the check. For HEAD it retries with GET when the server
// rejects HEAD (400/403/405/501).
func doRequest(ctx context.Context, client *http.Client, method, rawURL string, opts []httpx.RequestOption) (status, headStatus int, fallback bool, finalMethod string, err error) {
	if method == http.MethodHead {
		s, e := singleRequest(ctx, client, http.MethodHead, rawURL, opts)
		if e != nil {
			return 0, 0, false, http.MethodHead, e
		}
		if headRejected(s) {
			gs, ge := singleRequest(ctx, client, http.MethodGet, rawURL, opts)
			if ge != nil {
				return 0, s, true, http.MethodGet, ge
			}
			return gs, s, true, http.MethodGet, nil
		}
		return s, 0, false, http.MethodHead, nil
	}

	s, e := singleRequest(ctx, client, http.MethodGet, rawURL, opts)
	return s, 0, false, http.MethodGet, e
}

func headRejected(status int) bool {
	switch status {
	case http.StatusBadRequest, http.StatusForbidden, http.StatusMethodNotAllowed, http.StatusNotImplemented:
		return true
	default:
		return false
	}
}

func singleRequest(ctx context.Context, client *http.Client, method, rawURL string, opts []httpx.RequestOption) (int, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return 0, err
	}
	httpx.Apply(req, opts...)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	// Drain and close so the connection can be reused (fixes the old leak).
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	return resp.StatusCode, nil
}
