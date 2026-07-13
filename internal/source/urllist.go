package source

import (
	"bufio"
	"context"
	"net/http"
	"net/url"
	"strings"

	"preditrix/s1temap/internal/httpx"
)

// ListURLs reads a plain-text URL list (one URL per line) from an http(s) URL
// or a local file. Blank lines are skipped; host-relative lines (starting with
// "/") are kept as-is (a prefix URL is expected to complete them later); other
// lines are kept when they parse as URLs. The underlying resource is closed.
func ListURLs(ctx context.Context, client *http.Client, path string, opts ...httpx.RequestOption) ([]string, error) {
	body, err := openResource(ctx, client, path, opts...)
	if err != nil {
		return nil, err
	}
	defer drainClose(body)

	var urls []string
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "/") {
			urls = append(urls, line)
			continue
		}
		if u, err := url.Parse(line); err == nil {
			urls = append(urls, u.String())
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return urls, nil
}
