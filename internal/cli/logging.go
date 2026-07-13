package cli

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

const (
	logLevelEnv = "SMAP_LOG_LEVEL"
	// timeLayout produces e.g. "time=20260529T20:09:25.505+04".
	timeLayout = "20060102T15:04:05.000-07"
)

// splitHandler routes records by level: warn/error to stderr, debug/info to
// stdout.
type splitHandler struct {
	stdout slog.Handler
	stderr slog.Handler
}

func (h *splitHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if level >= slog.LevelWarn {
		return h.stderr.Enabled(ctx, level)
	}
	return h.stdout.Enabled(ctx, level)
}

func (h *splitHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelWarn {
		return h.stderr.Handle(ctx, r)
	}
	return h.stdout.Handle(ctx, r)
}

func (h *splitHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &splitHandler{stdout: h.stdout.WithAttrs(attrs), stderr: h.stderr.WithAttrs(attrs)}
}

func (h *splitHandler) WithGroup(name string) slog.Handler {
	return &splitHandler{stdout: h.stdout.WithGroup(name), stderr: h.stderr.WithGroup(name)}
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelDebug
	}
}

// setupLogging installs the default slog logger. Level comes from SMAP_LOG_LEVEL
// (debug|info|warn|error; default debug). Debug/info go to stdout, warn/error to
// stderr.
func setupLogging() {
	opts := &slog.HandlerOptions{
		Level: parseLogLevel(os.Getenv(logLevelEnv)),
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && a.Key == slog.TimeKey {
				return slog.String(slog.TimeKey, a.Value.Time().Format(timeLayout))
			}
			return a
		},
	}
	slog.SetDefault(slog.New(&splitHandler{
		stdout: slog.NewTextHandler(os.Stdout, opts),
		stderr: slog.NewTextHandler(os.Stderr, opts),
	}))
}
