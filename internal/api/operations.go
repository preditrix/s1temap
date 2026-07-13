package api

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"preditrix/s1temap/internal/engine"
	"preditrix/s1temap/internal/httpx"
	"preditrix/s1temap/internal/meta"
	"preditrix/s1temap/internal/source"
	"preditrix/s1temap/internal/statusfilter"
)

func validateJobRequest(req JobRequest) error {
	switch req.Operation {
	case OperationCrawlURLs:
		if len(req.URLs) == 0 {
			return fmt.Errorf("urls are required for crawl_urls")
		}
	case OperationCrawlSitemap, OperationConvertSitemapToURLList:
		if req.SitemapURL == "" {
			return fmt.Errorf("sitemap_url is required for %s", req.Operation)
		}
	default:
		return fmt.Errorf("unsupported operation %q", req.Operation)
	}

	if req.Options.PrefixURL != "" {
		parsed, err := url.Parse(req.Options.PrefixURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("prefix_url is not a proper url: %s", req.Options.PrefixURL)
		}
	}
	return nil
}

func (s *Server) runJob(ctx context.Context, j *Job, req JobRequest) {
	if ctx.Err() != nil {
		j.canceled()
		return
	}

	j.markRunning()

	var (
		output any
		err    error
	)

	switch req.Operation {
	case OperationCrawlURLs:
		err = s.runCrawlURLs(ctx, j, req)
	case OperationCrawlSitemap:
		err = s.runCrawlSitemap(ctx, j, req)
	case OperationConvertSitemapToURLList:
		output, err = s.runConvertSitemapToURLList(ctx, j, req)
	default:
		err = fmt.Errorf("unsupported operation %q", req.Operation)
	}

	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			j.canceled()
			return
		}
		j.fail(err)
		return
	}

	if ctx.Err() != nil {
		j.canceled()
		return
	}
	j.complete(output)
}

func (s *Server) runCrawlURLs(ctx context.Context, j *Job, req JobRequest) error {
	if len(req.URLs) == 0 {
		return fmt.Errorf("no crawlable urls")
	}
	urls := source.FilterCrawlableURLs(req.URLs, req.Options.PrefixURL)
	return s.crawl(ctx, j, urls, req.Options)
}

func (s *Server) runCrawlSitemap(ctx context.Context, j *Job, req JobRequest) error {
	opts := req.Options
	client := httpx.NewClient(timeoutFromOptions(opts), opts.Insecure, idleTimeoutFromOptions(opts))

	urls, err := source.SitemapURLs(ctx, client, req.SitemapURL, limitsFromOptions(opts), workersFromOptions(opts), fetchOptions(opts)...)
	if err != nil {
		return err
	}
	if len(urls) == 0 {
		return fmt.Errorf("no crawlable urls")
	}
	urls = source.FilterCrawlableURLs(urls, opts.PrefixURL)
	return s.crawl(ctx, j, urls, opts)
}

// crawl runs the engine against urls, wiring each result into the job.
func (s *Server) crawl(ctx context.Context, j *Job, urls []string, opts CrawlOptions) error {
	j.mu.Lock()
	j.state.Progress.Total = len(urls)
	j.mu.Unlock()

	filter, err := statusfilter.Parse(opts.StatusFilter)
	if err != nil {
		return fmt.Errorf("invalid status_filter: %w", err)
	}

	cfg := engine.Config{
		Workers: workersFromOptions(opts),
		Method:  opts.Method,
		Client:  httpx.NewClient(timeoutFromOptions(opts), opts.Insecure, idleTimeoutFromOptions(opts)),
		Options: pageOptions(opts),
	}

	_, err = engine.Run(ctx, urls, cfg, &jobSink{job: j, filter: filter})
	return err
}

func (s *Server) runConvertSitemapToURLList(ctx context.Context, j *Job, req JobRequest) ([]string, error) {
	opts := req.Options
	client := httpx.NewClient(timeoutFromOptions(opts), opts.Insecure, 0)

	urls, err := source.SitemapURLs(ctx, client, req.SitemapURL, limitsFromOptions(opts), workersFromOptions(opts), fetchOptions(opts)...)
	if err != nil {
		return nil, err
	}
	if opts.StripBaseURL {
		urls = source.StripBaseURLs(urls)
	}

	j.setProgress(Progress{Total: len(urls), Checked: len(urls), OK: len(urls)})
	return urls, nil
}

// jobSink forwards each engine result into the job, marking it skipped when it
// does not pass the status filter.
type jobSink struct {
	job    *Job
	filter *statusfilter.Filter
}

func (s *jobSink) Emit(res engine.Result) {
	s.job.recordResult(Result{
		URL:        res.URL,
		Status:     res.Status,
		Error:      res.Err,
		Method:     res.Method,
		Fallback:   res.Fallback,
		HeadStatus: res.HeadStatus,
		DurationMS: res.Duration.Milliseconds(),
		Timestamp:  res.Timestamp,
	}, s.filter.Match(res.Status))
}

func (s *jobSink) Close() error { return nil }

// fetchOptions builds request options for fetching a sitemap (no prefix URL).
func fetchOptions(opts CrawlOptions) []httpx.RequestOption {
	options := []httpx.RequestOption{httpx.WithUserAgent(userAgentFromOptions(opts))}
	if opts.AuthUsername != "" || opts.AuthPassword != "" {
		options = append(options, httpx.WithBasicAuth(opts.AuthUsername, opts.AuthPassword))
	}
	if len(opts.Cookies) > 0 {
		options = append(options, httpx.WithCookies(opts.Cookies))
	}
	if len(opts.Headers) > 0 {
		options = append(options, httpx.WithHeaders(opts.Headers))
	}
	return options
}

// pageOptions extends fetchOptions with the prefix-URL rewrite for page checks.
func pageOptions(opts CrawlOptions) []httpx.RequestOption {
	options := fetchOptions(opts)
	if opts.PrefixURL != "" {
		options = append(options, httpx.WithPrefixURL(opts.PrefixURL))
	}
	return options
}

func workersFromOptions(opts CrawlOptions) int {
	if opts.NumWorkers > 0 {
		return opts.NumWorkers
	}
	return 2
}

func idleTimeoutFromOptions(opts CrawlOptions) time.Duration {
	if opts.IdleTimeoutMS <= 0 {
		return 0
	}
	return time.Duration(opts.IdleTimeoutMS) * time.Millisecond
}

func timeoutFromOptions(opts CrawlOptions) time.Duration {
	if opts.HTTPTimeoutMS <= 0 {
		return 30 * time.Second
	}
	return time.Duration(opts.HTTPTimeoutMS) * time.Millisecond
}

func userAgentFromOptions(opts CrawlOptions) string {
	if opts.UserAgent != "" {
		return opts.UserAgent
	}
	return meta.UserAgent()
}

func limitsFromOptions(opts CrawlOptions) source.Limits {
	return source.Limits{
		MaxDepth:    opts.MaxDepth,
		MaxSitemaps: opts.MaxSitemaps,
		MaxURLs:     opts.MaxURLs,
	}
}
