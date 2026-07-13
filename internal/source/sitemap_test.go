package source

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSitemapURLs_Nested(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/index.xml", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap><loc>` + baseURL(r) + `/pages.xml</loc></sitemap>
</sitemapindex>`))
	})
	mux.HandleFunc("/pages.xml", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/a</loc></url>
  <url><loc>https://example.com/b</loc></url>
</urlset>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	urls, err := SitemapURLs(context.Background(), srv.Client(), srv.URL+"/index.xml", Limits{}, 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(urls) != 2 || urls[0] != "https://example.com/a" || urls[1] != "https://example.com/b" {
		t.Fatalf("got %v, want [a b]", urls)
	}
}

// A multi-child index fetched with several workers must collect every URL from
// every child sitemap (order is not guaranteed under concurrency).
func TestSitemapURLs_ParallelComplete(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/index.xml", func(w http.ResponseWriter, r *http.Request) {
		b := baseURL(r)
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap><loc>` + b + `/a.xml</loc></sitemap>
  <sitemap><loc>` + b + `/b.xml</loc></sitemap>
  <sitemap><loc>` + b + `/c.xml</loc></sitemap>
</sitemapindex>`))
	})
	page := func(locs ...string) http.HandlerFunc {
		body := `<?xml version="1.0"?>` + "\n" + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`
		for _, l := range locs {
			body += `<url><loc>` + l + `</loc></url>`
		}
		body += `</urlset>`
		return func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }
	}
	mux.HandleFunc("/a.xml", page("https://example.com/a1", "https://example.com/a2"))
	mux.HandleFunc("/b.xml", page("https://example.com/b1"))
	mux.HandleFunc("/c.xml", page("https://example.com/c1", "https://example.com/c2"))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	want := map[string]bool{
		"https://example.com/a1": true, "https://example.com/a2": true,
		"https://example.com/b1": true,
		"https://example.com/c1": true, "https://example.com/c2": true,
	}
	// Run repeatedly: parallel fetch order varies; the collected set must not.
	for i := 0; i < 20; i++ {
		urls, err := SitemapURLs(context.Background(), srv.Client(), srv.URL+"/index.xml", Limits{}, 4)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := make(map[string]bool, len(urls))
		for _, u := range urls {
			got[u] = true
		}
		if len(got) != len(want) {
			t.Fatalf("run %d: got %d unique urls, want %d: %v", i, len(got), len(want), urls)
		}
		for u := range want {
			if !got[u] {
				t.Fatalf("run %d: missing %s in %v", i, u, urls)
			}
		}
	}
}

// A sitemap index that references itself must not recurse forever.
func TestSitemapURLs_CycleSafe(t *testing.T) {
	var self string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap><loc>` + self + `</loc></sitemap>
</sitemapindex>`))
	}))
	defer srv.Close()
	self = srv.URL + "/self.xml"

	done := make(chan struct{})
	go func() {
		_, _ = SitemapURLs(context.Background(), srv.Client(), self, Limits{}, 4)
		close(done)
	}()
	select {
	case <-done: // terminated — good
	case <-time.After(3 * time.Second):
		t.Fatal("SitemapURLs did not terminate on a self-referential sitemap index")
	}
}

// MaxURLs caps the number of collected URLs.
func TestSitemapURLs_MaxURLs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/a</loc></url>
  <url><loc>https://example.com/b</loc></url>
  <url><loc>https://example.com/c</loc></url>
</urlset>`))
	}))
	defer srv.Close()

	urls, err := SitemapURLs(context.Background(), srv.Client(), srv.URL, Limits{MaxURLs: 2}, 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(urls) != 2 {
		t.Fatalf("got %d urls, want 2 (MaxURLs)", len(urls))
	}
	if urls[0] != "https://example.com/a" || urls[1] != "https://example.com/b" {
		t.Fatalf("MaxURLs truncation lost order: got %v, want [a b]", urls)
	}
}

func baseURL(r *http.Request) string {
	return "http://" + r.Host
}
