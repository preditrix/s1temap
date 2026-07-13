package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"preditrix/s1temap/internal/engine"
	"preditrix/s1temap/internal/output"
	"preditrix/s1temap/internal/source"
	"preditrix/s1temap/internal/statusfilter"
)

// runCrawl is the shared crawl orchestration for the start and list commands.
// out receives NDJSON lines (unless a file sink is configured) and the final
// summary.
func runCrawl(ctx context.Context, f crawlFlags, urls []string, out io.Writer) error {
	if f.IdleTimeout > 0 && f.NumWorkers != 1 {
		slog.Warn("--idle-timeout set; forcing --num-workers to 1",
			"requested_workers", f.NumWorkers, "idle_timeout", f.IdleTimeout)
		f.NumWorkers = 1
	}

	urls = source.FilterCrawlableURLs(urls, f.PrefixURL)

	filter, err := statusfilter.Parse(f.FilterStatus)
	if err != nil {
		return fmt.Errorf("invalid --filter-status: %w", err)
	}

	sink, err := buildSink(f, out)
	if err != nil {
		return err
	}

	slog.Debug("crawl options", "opts", f, "url_count", len(urls))

	summary, runErr := engine.Run(ctx, urls, engine.Config{
		Workers:        f.NumWorkers,
		Method:         f.Method,
		Client:         f.client(),
		Options:        f.options(),
		HeartbeatEvery: f.HeartbeatEvery,
		OnHeartbeat:    func(s engine.Summary) { s.WriteSummary(os.Stderr) },
	}, output.NewFiltered(sink, filter))

	if closeErr := sink.Close(); closeErr != nil && runErr == nil {
		runErr = closeErr
	}
	summary.WriteSummary(os.Stderr)
	return runErr
}

// buildSink chooses output sinks: file sinks (JSON and/or TSV) when configured
// — which suppress stdout — otherwise NDJSON to out.
func buildSink(f crawlFlags, out io.Writer) (engine.Sink, error) {
	var sinks []engine.Sink

	if f.OutputJSON != "" {
		s, err := output.NewJSONArrayFile(f.OutputJSON)
		if err != nil {
			return nil, fmt.Errorf("could not open json output file: %w", err)
		}
		sinks = append(sinks, s)
	}
	if f.OutputFile != "" {
		s, err := output.NewTSVFile(f.OutputFile)
		if err != nil {
			return nil, fmt.Errorf("could not open text output file: %w", err)
		}
		sinks = append(sinks, s)
	}

	if len(sinks) == 0 {
		return output.NewNDJSON(out), nil
	}
	if len(sinks) == 1 {
		return sinks[0], nil
	}
	return output.NewMulti(sinks...), nil
}
