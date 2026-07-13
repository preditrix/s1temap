package source

import (
	"log/slog"
	"net/url"
	"strings"
)

// FilterCrawlableURLs returns the URLs that can actually be requested. When
// prefixURL is empty, host-relative paths (starting with "/") have no scheme or
// host, so each one is logged as an error to stderr and dropped from the list.
// When prefixURL is set every URL is crawlable and the input is returned as-is.
// Shared by the CLI and the HTTP API.
func FilterCrawlableURLs(urls []string, prefixURL string) []string {
	if prefixURL != "" {
		return urls
	}
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		if strings.HasPrefix(u, "/") {
			slog.Error("dropping partial URL: needs a prefix URL to supply scheme and host", "url", u)
			continue
		}
		out = append(out, u)
	}
	return out
}

// StripBaseURLs removes the scheme and host from each URL, keeping only the
// path, query and fragment (e.g. "https://x.com/a?b#c" -> "/a?b#c"). Unparseable
// entries are dropped. This is the single shared implementation used by both the
// CLI tools command and the HTTP API.
func StripBaseURLs(urls []string) []string {
	out := make([]string, 0, len(urls))
	for _, raw := range urls {
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}
		result := u.Path
		if u.RawQuery != "" {
			result += "?" + u.RawQuery
		}
		if u.Fragment != "" {
			result += "#" + u.Fragment
		}
		out = append(out, result)
	}
	return out
}
