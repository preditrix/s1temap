# s1temap

s1temap is a fast SiteMap Scanner which in parallel crawls tons of URLs for a sitemap URL or list of URLs.
It check response times, and find dead pages. It ships a CLI and as a http server with Rest API.
And finally - it also can be used as AI Agent skill, support and instructions provided.

The Go module lives at the repository root. See
[`TECH_DESIGN.md`](./TECH_DESIGN.md) for the architecture.

## Build

```bash
# from the repository root
go build -o s1temap ./cmd/cli           # CLI
go build -o s1temap-api ./cmd/api       # HTTP API server
```

## Test

```bash
# from the repository root
go test ./...
go test -race ./...   # requires a C compiler (CGO)
```

## CLI

```bash
s1temap start <sitemap-url-or-path>     # crawl every URL in a sitemap
s1temap list  <url-list-url-or-path>    # crawl every URL in a text list
s1temap tools convert-sitemap-to-urllist <sitemap> [--remove-base-url] [--num-workers N]
s1temap --version
```

Nested sitemaps in a sitemap index are fetched concurrently (bounded by
`--num-workers`); the discovered URL order is not guaranteed.

Common flags (for `start` and `list`):

| Flag | Default | Description |
|------|---------|-------------|
| `--num-workers` | `2` | concurrent workers |
| `--method` | `HEAD` | `HEAD` with a GET fallback on 400/403/405/501, or `GET` |
| `--heartbeat-every` | `100` | print a progress summary to stderr every N processed URLs (0 disables) |
| `--http-timeout` | `30s` | per-request timeout |
| `--idle-timeout` | — | global min delay between requests (throttle); forces 1 worker |
| `--prefix-url` | — | replace scheme+host of every request URL |
| `--auth-user` / `--auth-pass` | — | HTTP basic auth |
| `--cookie` | — | `key=value` cookie (repeatable) |
| `--header` | — | `key=value` header (repeatable) |
| `--user-agent` | built-in | override the `User-Agent` header |
| `--filter-status` | — | filter output: `200`, `!200`, `500-599`, `200,404`, `>500`, `<300` |
| `--output-file` | — | write TSV results to a file (suppresses stdout) |
| `--output-json` | — | write a JSON array of results to a file (suppresses stdout) |
| `--insecure` | `false` | skip TLS verification |

Per-URL results are newline-delimited JSON on **stdout**; the summary — both the
final one and the periodic `--heartbeat-every` progress — is written to
**stderr**, so stdout stays valid NDJSON. Logging level is controlled by
`SMAP_LOG_LEVEL` (`debug`|`info`|`warn`|`error`).

## Use as AI Agent Skill

s1temap ships as a self-contained [Agent Skill](./skill/SKILL.md) so AI coding
agents can crawl sitemaps, warm caches, and check for broken pages on demand.
The skill folder (`skill/`) bundles the full command guide plus build scripts
(`setup.sh`, `setup.ps1`).

- **Claude Code**: copy the folder into a skills directory — it is
  auto-discovered.
  ```bash
  cp -r skill ~/.claude/skills/s1temap          # personal
  # or, project-scoped:
  mkdir -p .claude/skills && cp -r skill .claude/skills/s1temap
  ```
- **Claude.ai / Agent SDK / API**: upload the `skill/` folder as a skill (or
  point the SDK `skills` option at it).
- **OpenAI Codex**: reference `skill/SKILL.md` from your `AGENTS.md`.
- **Cursor / Aider / others**: reference `skill/SKILL.md` from the project
  rules/instructions file.

Full instructions and the command reference are in
[`skill/SKILL.md`](./skill/SKILL.md).


## HTTP API

```bash
LISTEN=:8080 ./s1temap-api      # defaults to :8080 when LISTEN is unset
```

| Method | Path | Description |
|--------|------|-------------|
| `GET`  | `/healthz` | health check |
| `GET`  | `/version` | version info |
| `POST` | `/api/v1/jobs` | create an async job |
| `GET`  | `/api/v1/jobs` | list jobs |
| `GET`  | `/api/v1/jobs/{id}` | job state |
| `POST` | `/api/v1/jobs/{id}/cancel` | cancel a job |
| `GET`  | `/api/v1/jobs/{id}/events` | live SSE event stream |

Jobs are async and stored in memory: `POST` returns `202` with the job's `id`
and `events_url`; you then poll `GET /api/v1/jobs/{id}` or stream
`GET /api/v1/jobs/{id}/events` (SSE).

### Create a job

`operation` is one of `crawl_urls` (needs `urls`), `crawl_sitemap` /
`convert_sitemap_to_urllist` (need `sitemap_url`). All `options` are optional:

```bash
curl -sX POST localhost:8080/api/v1/jobs \
  -H 'Content-Type: application/json' \
  -d '{
    "operation": "crawl_sitemap",
    "sitemap_url": "https://example.com/sitemap.xml",
    "options": {
      "num_workers": 8,
      "method": "HEAD",
      "http_timeout_ms": 10000,
      "idle_timeout_ms": 0,
      "status_filter": "!200",
      "prefix_url": "https://staging.example.com",
      "user_agent": "s1temap-bot",
      "auth_username": "admin",
      "auth_password": "secret",
      "cookies": {"bypass_cache": "1"},
      "headers": {"X-Debug": "1"},
      "strip_base_url": false,
      "max_depth": 5,
      "max_sitemaps": 100,
      "max_urls": 10000,
      "insecure": false
    }
  }'
```

Response (`202 Accepted`):

```json
{
  "id": "a1b2c3d4e5f6a7b8",
  "operation": "crawl_sitemap",
  "status": "queued",
  "created_at": "2026-07-06T12:00:00Z",
  "progress": {"checked": 0, "ok": 0, "failed": 0, "skipped": 0},
  "events_url": "/api/v1/jobs/a1b2c3d4e5f6a7b8/events"
}
```

`options` fields: `num_workers`, `method` (`HEAD` with GET fallback — the default
— or `GET`),
`http_timeout_ms`, `idle_timeout_ms` (global throttle; `0` = off), `status_filter`
(same grammar as the CLI), `prefix_url`, `user_agent`, `auth_username`,
`auth_password`, `cookies`, `headers` (both `key: value` objects), `strip_base_url`
(convert only), `max_depth` / `max_sitemaps` / `max_urls` (sitemap limits), and
`insecure`.

### Poll state / cancel

```bash
curl -s localhost:8080/api/v1/jobs/a1b2c3d4e5f6a7b8          # full JobState (+summary when done)
curl -sX POST localhost:8080/api/v1/jobs/a1b2c3d4e5f6a7b8/cancel   # 202, or 409 if already terminal
```

### Stream events (SSE)

```bash
curl -sN localhost:8080/api/v1/jobs/a1b2c3d4e5f6a7b8/events
```

Event types: `job.started`, `job.result`, `job.progress`, and one terminal
`job.completed` / `job.failed` / `job.canceled`. On subscribe the full event
history is replayed first, so a late subscriber misses nothing. Each frame is:

```
event: job.result
data: {"type":"job.result","job_id":"a1b2c3d4e5f6a7b8","status":"running","progress":{"checked":1,"ok":1,"failed":0,"skipped":0},"result":{"url":"https://example.com/","status":200,"method":"HEAD","duration_ms":42,"timestamp":"2026-07-06T12:00:01Z"},"timestamp":"2026-07-06T12:00:01Z"}
```
