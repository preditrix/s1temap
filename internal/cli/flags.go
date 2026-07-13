package cli

import (
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"preditrix/s1temap/internal/httpx"
	"preditrix/s1temap/internal/meta"
)

// crawlFlags are the flags shared by the crawl commands (start, list). They are
// embedded into each command struct so Kong renders them per command.
type crawlFlags struct {
	NumWorkers     int           `name:"num-workers" default:"2" help:"set number of workers for crawling"`
	Method         string        `name:"method" default:"HEAD" enum:"HEAD,GET" help:"request method: HEAD with a GET fallback on 400/403/405/501, or GET"`
	HeartbeatEvery int           `name:"heartbeat-every" default:"100" help:"print a progress summary to stderr every N processed URLs (0 disables)"`
	PrefixURL      string        `name:"prefix-url" help:"prefix/replace all request urls with this one"`
	HTTPTimeout    time.Duration `name:"http-timeout" default:"30s" help:"set http timeout for requests"`
	IdleTimeout    time.Duration `name:"idle-timeout" help:"delay between requests to avoid hammering the target; when set, --num-workers is forced to 1"`
	AuthUser       string        `name:"auth-user" help:"set HTTP basic authentication username"`
	AuthPass       string        `name:"auth-pass" help:"set HTTP basic authentication password"`
	Cookies        []string      `name:"cookie" help:"add cookies (as key=value pairs) to each request"`
	Headers        []string      `name:"header" help:"add headers (as key=value pairs) to each request"`
	UserAgent      string        `name:"user-agent" help:"set the User-Agent header (defaults to the built-in s1temap agent)"`
	FilterStatus   string        `name:"filter-status" help:"filter logs by status"`
	OutputFile     string        `name:"output-file" help:"set output file for results in text format (example: \"./path/to/output.txt\")"`
	OutputJSON     string        `name:"output-json" help:"set output file for results in json format (example: \"./path/to/output.json\")"`
	Insecure       bool          `name:"insecure" help:"skip TLS certificate verification"`
}

// LogValue redacts the auth password when the flags are logged.
func (f crawlFlags) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int("num_workers", f.NumWorkers),
		slog.String("method", f.Method),
		slog.Int("heartbeat_every", f.HeartbeatEvery),
		slog.Duration("http_timeout", f.HTTPTimeout),
		slog.Duration("idle_timeout", f.IdleTimeout),
		slog.String("prefix_url", f.PrefixURL),
		slog.String("auth_user", f.AuthUser),
		slog.Bool("auth_pass_set", f.AuthPass != ""),
		slog.Int("cookies", len(f.Cookies)),
		slog.Int("headers", len(f.Headers)),
		slog.String("user_agent", f.UserAgent),
		slog.String("filter_status", f.FilterStatus),
		slog.String("output_file", f.OutputFile),
		slog.String("output_json", f.OutputJSON),
		slog.Bool("insecure", f.Insecure),
	)
}

func (f crawlFlags) validate() error {
	if f.NumWorkers <= 0 {
		return fmt.Errorf("number of workers should be at least 1")
	}
	if f.IdleTimeout < 0 {
		return fmt.Errorf("idle timeout cannot be negative")
	}
	if f.PrefixURL != "" && !strings.HasPrefix(f.PrefixURL, "http") {
		return fmt.Errorf("prefix url is not a proper url: %s", f.PrefixURL)
	}
	if err := validateKeyValues("cookie", f.Cookies); err != nil {
		return err
	}
	return validateKeyValues("header", f.Headers)
}

func (f crawlFlags) client() *http.Client {
	return httpx.NewClient(f.HTTPTimeout, f.Insecure, f.IdleTimeout)
}

// options builds the per-request options from the flags. User-Agent is always
// set (--user-agent when given, else the built-in agent); auth/cookies/headers/
// prefix are added when configured.
func (f crawlFlags) options() []httpx.RequestOption {
	userAgent := f.UserAgent
	if userAgent == "" {
		userAgent = meta.UserAgent()
	}
	opts := []httpx.RequestOption{httpx.WithUserAgent(userAgent)}
	if f.PrefixURL != "" {
		opts = append(opts, httpx.WithPrefixURL(f.PrefixURL))
	}
	if f.AuthUser != "" || f.AuthPass != "" {
		opts = append(opts, httpx.WithBasicAuth(f.AuthUser, f.AuthPass))
	}
	if len(f.Cookies) > 0 {
		opts = append(opts, httpx.WithCookies(keyValueMap(f.Cookies)))
	}
	if len(f.Headers) > 0 {
		opts = append(opts, httpx.WithHeaders(keyValueMap(f.Headers)))
	}
	return opts
}

var keyValueRe = regexp.MustCompile(`.+=.+`)

func validateKeyValues(name string, pairs []string) error {
	for _, kv := range pairs {
		if !keyValueRe.MatchString(kv) {
			return fmt.Errorf("%s does not match pattern %s_name=%s_value for: %s", name, name, name, kv)
		}
	}
	return nil
}

func keyValueMap(pairs []string) map[string]string {
	m := make(map[string]string, len(pairs))
	for _, kv := range pairs {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}
