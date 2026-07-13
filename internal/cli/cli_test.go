package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kong"
)

func newParser(t *testing.T, cli *CLI) *kong.Kong {
	t.Helper()
	parser, err := kong.New(cli, kong.Name("s1temap"), kong.Vars{"version": "test"})
	if err != nil {
		t.Fatalf("build parser: %v", err)
	}
	return parser
}

// happy path: "start" selects the sitemap crawl and populates its flags/arg.
func TestParse_Start(t *testing.T) {
	var cli CLI
	ctx, err := newParser(t, &cli).Parse([]string{"start", "--num-workers", "5", "http://x/s.xml"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !strings.HasPrefix(ctx.Command(), "start") {
		t.Errorf("command = %q, want start...", ctx.Command())
	}
	if cli.Start.Path != "http://x/s.xml" || cli.Start.NumWorkers != 5 {
		t.Errorf("start = %+v, want path=http://x/s.xml workers=5", cli.Start)
	}
}

// happy path: tools convert with its bool flag and default worker value on list.
func TestParse_ToolsAndListDefaults(t *testing.T) {
	var cli CLI
	if _, err := newParser(t, &cli).Parse([]string{"tools", "convert-sitemap-to-urllist", "--remove-base-url", "http://x/s.xml"}); err != nil {
		t.Fatalf("parse tools: %v", err)
	}
	if !cli.Tools.ConvertSitemapToURLList.RemoveBaseURL {
		t.Error("remove-base-url not set")
	}

	var cli2 CLI
	if _, err := newParser(t, &cli2).Parse([]string{"list", "./urls.txt"}); err != nil {
		t.Fatalf("parse list: %v", err)
	}
	if cli2.List.NumWorkers != 2 || cli2.List.HTTPTimeout.String() != "30s" {
		t.Errorf("list defaults = workers %d timeout %s, want 2 / 30s", cli2.List.NumWorkers, cli2.List.HTTPTimeout)
	}
}

// corner case: missing required positional arg errors.
func TestParse_Start_MissingArg(t *testing.T) {
	var cli CLI
	if _, err := newParser(t, &cli).Parse([]string{"start"}); err == nil {
		t.Fatal("expected error for missing sitemap-path")
	}
}

// cancellation guarantee: even when the context is already canceled, runCrawl
// still Closes the sink so the --output-json file is flushed (this is what makes
// SIGINT in the CLI safe for the buffered JSON-array sink).
func TestRunCrawl_CanceledContextFlushesJSON(t *testing.T) {
	jsonPath := filepath.Join(t.TempDir(), "out.json")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // canceled before the crawl starts

	f := crawlFlags{NumWorkers: 2, HTTPTimeout: time.Second, Method: "GET", OutputJSON: jsonPath}
	var out bytes.Buffer
	err := runCrawl(ctx, f, []string{"http://example.invalid/a", "http://example.invalid/b"}, &out)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("runCrawl err = %v, want context.Canceled", err)
	}

	data, readErr := os.ReadFile(jsonPath)
	if readErr != nil {
		t.Fatalf("json output not written on cancellation: %v", readErr)
	}
	var records []map[string]any
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatalf("flushed json is not a valid array: %v (%q)", err, data)
	}
}

// flag validation rejects malformed cookies.
func TestValidate_BadCookie(t *testing.T) {
	f := crawlFlags{NumWorkers: 2, Cookies: []string{"noequalsign"}}
	if err := f.validate(); err == nil {
		t.Fatal("expected validation error for malformed cookie")
	}
}
