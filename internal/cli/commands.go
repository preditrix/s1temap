package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"preditrix/s1temap/internal/httpx"
	"preditrix/s1temap/internal/source"
)

// defaultToolsClient is a plain 30s HTTP client for the tools command (no
// throttling, no TLS skip).
func defaultToolsClient() *http.Client {
	return httpx.NewClient(30*time.Second, false, 0)
}

// signalContext returns a context canceled on the first SIGINT/SIGTERM so a
// crawl stops promptly and its sink still gets Closed (flushing --output-json).
// The caller must invoke the returned stop function.
func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// StartCmd crawls all URLs found in a sitemap.
type StartCmd struct {
	crawlFlags
	Path string `arg:"" name:"sitemap-path" help:"Sitemap URL or file path"`
}

func (c *StartCmd) Run() error {
	if err := c.validate(); err != nil {
		return fmt.Errorf("flag options are invalid: %w", err)
	}

	ctx, stop := signalContext()
	defer stop()
	urls, err := source.SitemapURLs(ctx, c.client(), c.Path, c.limits(), c.NumWorkers, c.options()...)
	if err != nil {
		return err
	}
	return runCrawl(ctx, c.crawlFlags, urls, os.Stdout)
}

// limits maps the crawl flags to sitemap traversal limits. The CLI does not
// currently expose depth/count flags, so these stay unlimited.
func (c *StartCmd) limits() source.Limits { return source.Limits{} }

// ListCmd crawls all URLs read from a URL list.
type ListCmd struct {
	crawlFlags
	Path string `arg:"" name:"url-list" help:"URL list URL or file path"`
}

func (c *ListCmd) Run() error {
	if err := c.validate(); err != nil {
		return fmt.Errorf("flag options are invalid: %w", err)
	}

	ctx, stop := signalContext()
	defer stop()
	urls, err := source.ListURLs(ctx, c.client(), c.Path, c.options()...)
	if err != nil {
		return fmt.Errorf("could not read url list from %s: %w", c.Path, err)
	}
	return runCrawl(ctx, c.crawlFlags, urls, os.Stdout)
}

// ToolsCmd groups utility subcommands under "tools".
type ToolsCmd struct {
	ConvertSitemapToURLList ConvertSitemapToURLListCmd `cmd:"" name:"convert-sitemap-to-urllist" help:"Convert a sitemap to a URL list and print it to stdout"`
}

// ConvertSitemapToURLListCmd prints a sitemap's URLs to stdout, one per line.
type ConvertSitemapToURLListCmd struct {
	NumWorkers    int    `name:"num-workers" default:"2" help:"concurrent workers for fetching nested sitemaps"`
	RemoveBaseURL bool   `name:"remove-base-url" help:"remove base url from urls"`
	Path          string `arg:"" name:"sitemap-path" help:"Sitemap URL or file path"`
}

func (c *ConvertSitemapToURLListCmd) Run() error {
	ctx, stop := signalContext()
	defer stop()
	urls, err := source.SitemapURLs(ctx, defaultToolsClient(), c.Path, source.Limits{}, c.NumWorkers)
	if err != nil {
		return err
	}
	if c.RemoveBaseURL {
		urls = source.StripBaseURLs(urls)
	}
	for _, u := range urls {
		fmt.Println(u)
	}
	return nil
}
