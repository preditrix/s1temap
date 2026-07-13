// Package httpx builds HTTP clients and per-request options used by the crawler.
package httpx

import (
	"net/http"
	"net/url"
)

// RequestOption mutates an outgoing request. Options are the single canonical
// way to customize requests (user-agent, auth, cookies, headers, prefix URL).
type RequestOption func(*http.Request)

// Apply runs every option against req, skipping nil options.
func Apply(req *http.Request, opts ...RequestOption) {
	for _, opt := range opts {
		if opt != nil {
			opt(req)
		}
	}
}

// WithUserAgent sets the User-Agent header when ua is non-empty.
func WithUserAgent(ua string) RequestOption {
	return func(req *http.Request) {
		if ua != "" {
			req.Header.Set("User-Agent", ua)
		}
	}
}

// WithBasicAuth sets HTTP basic auth when either credential is non-empty.
func WithBasicAuth(username, password string) RequestOption {
	return func(req *http.Request) {
		if username != "" || password != "" {
			req.SetBasicAuth(username, password)
		}
	}
}

// WithCookies adds each key=value pair as a request cookie.
func WithCookies(cookies map[string]string) RequestOption {
	return func(req *http.Request) {
		for name, value := range cookies {
			req.AddCookie(&http.Cookie{Name: name, Value: value})
		}
	}
}

// WithHeaders sets each key=value pair as a request header.
func WithHeaders(headers map[string]string) RequestOption {
	return func(req *http.Request) {
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	}
}

// WithPrefixURL replaces the request's scheme and host with those of prefix,
// e.g. to redirect a production sitemap's URLs at a staging host. A blank or
// unparseable prefix is ignored.
func WithPrefixURL(prefix string) RequestOption {
	return func(req *http.Request) {
		if prefix == "" {
			return
		}
		parsed, err := url.Parse(prefix)
		if err != nil || parsed.String() == "" {
			return
		}
		req.URL.Scheme = parsed.Scheme
		req.URL.Host = parsed.Host
	}
}
