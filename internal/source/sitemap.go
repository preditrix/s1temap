// Package source turns an input location (sitemap or URL list) into the list of
// URLs to crawl. All network/file resources are closed by the callers here.
package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"preditrix/s1temap/internal/httpx"

	"github.com/beevik/etree"
	"golang.org/x/sync/errgroup"
)

// Limits bounds sitemap traversal. A zero value means unlimited.
type Limits struct {
	MaxDepth    int // max sitemap-index nesting; root is depth 0
	MaxSitemaps int // max sitemap documents fetched
	MaxURLs     int // max URLs collected
}

// SitemapURLs fetches root (an XML sitemap or sitemap index, from an http(s)
// URL or a local file), walks nested indexes, and returns the discovered page
// URLs. Nested sitemap documents are fetched concurrently, bounded by workers
// (min 1). Traversal is cycle-safe (a visited set prevents infinite recursion)
// and bounded by lim. Result order is not guaranteed (documents are fetched
// concurrently); when MaxURLs caps the result, an arbitrary subset is kept. A
// failure fetching the root is returned as an error; failures on nested
// sitemaps are logged and skipped.
func SitemapURLs(ctx context.Context, client *http.Client, root string, lim Limits, workers int, opts ...httpx.RequestOption) ([]string, error) {
	if workers < 1 {
		workers = 1
	}
	w := &walker{client: client, lim: lim, opts: opts, workers: workers, seen: make(map[string]bool)}

	if err := w.walk(ctx, root, 0, true); err != nil {
		return nil, err
	}
	urls := w.urls
	if lim.MaxURLs > 0 && len(urls) > lim.MaxURLs {
		urls = urls[:lim.MaxURLs]
	}
	return urls, nil
}

type walker struct {
	client  *http.Client
	lim     Limits
	opts    []httpx.RequestOption
	workers int

	mu       sync.Mutex // guards seen, sitemaps, and urls
	seen     map[string]bool
	sitemaps int
	urls     []string
}

// visit atomically records path in the visited set and enforces MaxSitemaps.
// It returns ok=false when path was already visited (cycle) or the sitemap
// budget is exhausted, so the caller should stop.
func (w *walker) visit(path string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.seen[path] {
		return false
	}
	if w.lim.MaxSitemaps > 0 && w.sitemaps >= w.lim.MaxSitemaps {
		slog.Warn("skipping sitemap: MaxSitemaps reached", "url", path)
		return false
	}
	w.seen[path] = true
	w.sitemaps++
	return true
}

// walk fetches one sitemap document and collects its page URLs into w.urls.
// Nested sitemaps in an index are fetched concurrently (bounded by w.workers);
// URLs are appended as documents complete, so result order is not guaranteed.
func (w *walker) walk(ctx context.Context, path string, depth int, isRoot bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !w.visit(path) {
		return nil
	}

	doc, err := w.fetchDoc(ctx, path)
	if err != nil {
		if isRoot {
			return fmt.Errorf("could not read sitemap from %s: %w", path, err)
		}
		slog.Warn("skipping nested sitemap", "url", path, "err", err)
		return nil
	}

	if index := doc.FindElement("sitemapindex"); index != nil {
		children := index.ChildElements()
		slog.Debug("read sitemap index xml", "url", path, "depth", depth, "nested_sitemaps", countLocs(children))

		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(w.workers)
		var inlineErr error
		for _, child := range children {
			loc := child.FindElement("loc")
			if loc == nil {
				continue
			}
			nested := strings.TrimSpace(loc.Text())
			if w.lim.MaxDepth > 0 && depth+1 > w.lim.MaxDepth {
				slog.Warn("skipping sitemap: MaxDepth reached", "url", nested)
				continue
			}
			task := func() error { return w.walk(gctx, nested, depth+1, false) }
			// TryGo runs task in a new goroutine when under the concurrency
			// limit; at the limit it returns false and we run task inline on this
			// goroutine, so we never block on a full pool (no deadlock) and live
			// goroutines stay bounded by w.workers.
			if !g.TryGo(task) {
				if err := task(); err != nil {
					inlineErr = err
					break
				}
			}
		}
		if err := g.Wait(); err != nil {
			return err
		}
		return inlineErr
	}

	if urlset := doc.FindElement("urlset"); urlset != nil {
		entries := urlset.ChildElements()
		slog.Debug("read sitemap xml", "url", path, "depth", depth, "page_urls", countLocs(entries))
		local := make([]string, 0, len(entries))
		for _, child := range entries {
			loc := child.FindElement("loc")
			if loc == nil {
				continue
			}
			raw := strings.TrimSpace(loc.Text())
			if _, err := url.Parse(raw); err != nil {
				slog.Warn("skipping malformed sitemap URL", "url", raw, "err", err)
				continue
			}
			local = append(local, raw)
		}
		w.mu.Lock()
		w.urls = append(w.urls, local...)
		w.mu.Unlock()
		return nil
	}

	return nil
}

// countLocs reports how many of elems carry a <loc> child — i.e. the number of
// nested-sitemap entries (in an index) or page URLs (in a urlset) the document
// actually declares.
func countLocs(elems []*etree.Element) int {
	n := 0
	for _, e := range elems {
		if e.FindElement("loc") != nil {
			n++
		}
	}
	return n
}

func (w *walker) fetchDoc(ctx context.Context, path string) (*etree.Document, error) {
	body, err := openResource(ctx, w.client, path, w.opts...)
	if err != nil {
		return nil, err
	}
	defer drainClose(body)

	doc := etree.NewDocument()
	if _, err := doc.ReadFrom(body); err != nil {
		return nil, err
	}
	return doc, nil
}

// isHTTPURL reports whether path is an http(s) URL (scheme is compared
// case-insensitively via url.Parse), as opposed to a local file path. A Windows
// path like "C:\dir\sitemap.xml" parses with scheme "c" and is treated as a
// file; a plain name like "sitemap.xml" has no scheme and is also a file.
func isHTTPURL(path string) bool {
	u, err := url.Parse(path)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// openResource returns a ReadCloser for an http(s) URL or a local file path.
// The caller must close it.
func openResource(ctx context.Context, client *http.Client, path string, opts ...httpx.RequestOption) (io.ReadCloser, error) {
	if isHTTPURL(path) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}
		httpx.Apply(req, opts...)

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			drainClose(resp.Body)
			return nil, errors.New(resp.Status)
		}
		return resp.Body, nil
	}

	return os.Open(path)
}

// drainClose drains and closes a body so keep-alive connections can be reused.
func drainClose(rc io.ReadCloser) {
	if rc == nil {
		return
	}
	_, _ = io.Copy(io.Discard, rc)
	_ = rc.Close()
}
