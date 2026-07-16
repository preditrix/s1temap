---
name: s1temap
description: >-
  Crawl URLs from an XML sitemap or a URL list to warm caches, check HTTP status
  codes across a large URL set, find broken or slow pages, verify a staging or
  preview environment against a production sitemap, or extract all URLs from a
  sitemap. Use whenever the user mentions sitemap crawling, cache warming,
  dead-link/broken-page checking, bulk URL status checks, or converting a sitemap
  to a URL list.
---

# Skill: s1temap

s1temap crawls a set of URLs — discovered from a web-site **sitemap** (root URL) or read from a provided
**URL list** — and reports each URL's HTTP status, timing, and errors, plus a
summary.

Use this skill when the user wants to:
- Check full sitemap of a web-site by its root sitemap URL
- Warm a site cache with list of provided URLs
- Check HTTP response codes across a large URL set
- Find broken or slow pages
- Verify a staging or preview environment against a production sitemap
- Extract all URLs from an XML sitemap

This skill covers the **CLI**. An optional HTTP API server also exists
(`./cmd/api`) but is not required for the tasks above.

---

## Where the binary lives (read this first)

s1temap is a compiled Go binary. It is **always installed to one fixed,
session-independent location**, so every future agent session can find it by the
same path without guessing:

| Platform    | Fixed path                                              |
|-------------|---------------------------------------------------------|
| Linux/macOS | `$S1TEMAP_HOME/s1temap`  (default `~/.s1temap/bin/s1temap`) |
| Windows     | `%S1TEMAP_HOME%\s1temap.exe` (default `%USERPROFILE%\.s1temap\bin\s1temap.exe`) |

The environment variable `S1TEMAP_HOME` is the single source of truth. If it is
set, the binary lives in that directory; otherwise the default above is used.

**Do not** build into `/tmp`, into the current working directory, or into the
repo root — those locations do not survive across sessions and are the usual
cause of "binary not found next time".

---

## Setup (per session)

At the start of each session, check the fixed path **first**. Only build if it is
missing.

```bash
# Linux / macOS
BIN="${S1TEMAP_HOME:-$HOME/.s1temap/bin}/s1temap"
[ -x "$BIN" ] && echo "found: $BIN" || echo "not found — build it"
```

```powershell
# Windows (PowerShell)
$Bin = if ($env:S1TEMAP_HOME) { "$env:S1TEMAP_HOME\s1temap.exe" } else { "$env:USERPROFILE\.s1temap\bin\s1temap.exe" }
if (Test-Path $Bin) { "found: $Bin" } else { "not found — build it" }
```

- **Found** → run it directly via `$BIN` / `$Bin`.
- **Not found** → build it (below).

### Build

Building needs the Go module source, so this step requires the repository.

1. Clone the repo and enter it:
   ```bash
   git clone https://github.com/preditrix/s1temap
   cd s1temap
   ```
2. Run the bundled setup script. It verifies the Go version, finds the module
   root by walking up to `go.mod`, builds **straight into the fixed install
   location**, skips the build if the binary already exists, and prints the
   absolute binary path as its **last stdout line**:

   ```bash
   # Linux / macOS
   bash skill/setup.sh

   # Windows (PowerShell)
   .\skill\setup.ps1
   ```

3. **Remember the printed path** (the script's last line) in Agent/Skill memory,
   so all future sessions invoke the binary directly without rebuilding.

Or build manually into the fixed location from the module root (the folder with
`go.mod`):

```bash
# Linux / macOS  -> ~/.s1temap/bin/s1temap
mkdir -p "${S1TEMAP_HOME:-$HOME/.s1temap/bin}"
go build -trimpath -ldflags="-s -w" -o "${S1TEMAP_HOME:-$HOME/.s1temap/bin}/s1temap" ./cmd/cli
```

```powershell
# Windows -> %USERPROFILE%\.s1temap\bin\s1temap.exe
$dir = if ($env:S1TEMAP_HOME) { $env:S1TEMAP_HOME } else { "$env:USERPROFILE\.s1temap\bin" }
New-Item -ItemType Directory -Force -Path $dir | Out-Null
go build -trimpath -ldflags="-s -w" -o "$dir\s1temap.exe" ./cmd/cli
```

p.s. Optional HTTP API server (if needed): `go build -o s1temap-api ./cmd/api`.

### Optional: put it on PATH

If you'd rather call `s1temap` without the full path, symlink the fixed binary
into a directory already on `PATH` (e.g. `~/.local/bin`):

```bash
ln -sfn "${S1TEMAP_HOME:-$HOME/.s1temap/bin}/s1temap" "$HOME/.local/bin/s1temap"
```

The fixed path remains the primary, reliable mechanism; PATH is just a
convenience.

---

## Prerequisites

s1temap is written in Go. **Required: Go 1.26+.**

```bash
go version   # expect: go version go1.26.x ...
```

| Platform | Install |
|----------|---------|
| Windows  | `winget install GoLang.Go` or https://go.dev/dl/ |
| macOS    | `brew install go` or https://go.dev/dl/ |
| Linux    | `sudo apt install golang-go` (Debian/Ubuntu) or https://go.dev/dl/ |

---

## Commands

### `start <sitemap>`

Fetch an XML sitemap (URL or local file), walk nested sitemap indexes, and crawl
every discovered page URL.

```bash
s1temap start <sitemap-url-or-path>
```

### `list <file-or-url>`

Crawl every URL from a plain-text list (one URL per line; local path or remote
URL).

```bash
s1temap list <file-or-url>
```

### `tools convert-sitemap-to-urllist <sitemap>`

Extract all URLs from a sitemap and print them to stdout, one per line (no
crawling).

```bash
s1temap tools convert-sitemap-to-urllist <sitemap-url-or-path> [--remove-base-url] [--num-workers N]
```

`--remove-base-url` strips scheme+host, emitting paths only (e.g. `/about`).
`--num-workers` (default 2) sets how many nested sitemaps are fetched
concurrently. Nested sitemaps in an index are walked in parallel across all
commands; the output URL order is not guaranteed.

---

## Common flags (for `start` and `list`)

| Flag | Default | Description                                                                     |
|------|---------|---------------------------------------------------------------------------------|
| `--num-workers` | `2` | concurrent HTTP workers                                                         |
| `--method` | `HEAD` | `HEAD` with a GET fallback on 400/403/405/501, or `GET`                         |
| `--heartbeat-every` | `100` | progress summary to stderr every N processed URLs (0 disables)                   |
| `--http-timeout` | `30s` | per-request timeout (e.g. `10s`, `1m`)                                          |
| `--idle-timeout` | — | global min delay between requests (rate limiting); forces `--num-workers=1`     |
| `--prefix-url` | — | replace scheme+host of every request URL (e.g. point a prod sitemap at staging) |
| `--auth-user` | — | HTTP basic auth username                                                        |
| `--auth-pass` | — | HTTP basic auth password                                                        |
| `--cookie` | — | add a cookie as `key=value` (repeatable)                                        |
| `--header` | — | add a header as `key=value` (repeatable)                                        |
| `--user-agent` | built-in | override the `User-Agent` header                                                |
| `--filter-status` | — | filter output by HTTP status (see below)                                        |
| `--output-file` | — | write results to a TSV file (suppresses stdout)                                 |
| `--output-json` | — | write results to a JSON array file (suppresses stdout)                          |
| `--insecure` | `false` | skip TLS certificate verification                                               |

### `--filter-status` syntax

| Expression | Meaning |
|------------|---------|
| `200` | only 200 |
| `!200` | everything except 200 |
| `500-599` | inclusive range |
| `200,404` | 200 or 404 |
| `>500` | greater than 500 |
| `<300` | less than 300 |

---

## Examples

```bash
# Warm a production cache with 8 workers
s1temap start https://example.com/sitemap.xml --num-workers=8

# Find broken pages (non-200) and save them
s1temap start https://example.com/sitemap.xml --filter-status=!200 --output-json broken.json

# Test staging against the production sitemap
s1temap start https://example.com/sitemap.xml --prefix-url=https://staging.example.com --num-workers=4

# Crawl with auth + a bypass cookie
s1temap start https://example.com/sitemap.xml --auth-user=admin --auth-pass=secret --cookie=bypass_cache=1

# Rate-limited slow crawl
s1temap start https://example.com/sitemap.xml --idle-timeout=500ms

# Extract URLs, then crawl the list
s1temap tools convert-sitemap-to-urllist https://example.com/sitemap.xml > urls.txt
s1temap list ./urls.txt --num-workers=4 --output-json results.json
```

---

## Output format

By default each crawled URL prints one JSON object per line to **stdout**, so
stdout stays valid NDJSON:

```
{"status":200,"url":"https://example.com/","time":1699999999,"duration":123}
{"err":"...","url":"https://example.com/x","time":1699999999,"duration":45}
```

The summary block goes to **stderr** — both the final one and a periodic
progress heartbeat every `--heartbeat-every` URLs (default 100; 0 disables):

```
--- Summary ---
Total:      2
Duration:   1.2s
Avg time:   84ms
Status 200: 1
Errors:     1
```

- `--output-file` writes TSV lines: `<status|err> \t url \t unixSeconds \t <ms>ms`.
- `--output-json` writes a single JSON array of result objects after crawling.

The summary counts **every** URL regardless of `--filter-status`; the filter only
affects which per-URL lines/rows are written.

## Logging

Log verbosity is controlled by the `SMAP_LOG_LEVEL` environment variable
(`debug` | `info` | `warn` | `error`; default `debug`). Debug/info go to stdout,
warn/error to stderr. Set `SMAP_LOG_LEVEL=error` for quiet runs.

---

## Agent checklist

Run this at the start of every session before issuing crawl commands:

1. **Binary at the fixed path?** Check
   `${S1TEMAP_HOME:-~/.s1temap/bin}/s1temap` (Linux/macOS) or
   `%S1TEMAP_HOME%\s1temap.exe` / `%USERPROFILE%\.s1temap\bin\s1temap.exe`
   (Windows). Found & executable → go to step 4.
2. **Build it.** Clone the repo if needed, then run `bash skill/setup.sh`
   (or `.\skill\setup.ps1`). The script builds into the fixed path and prints
   the absolute binary path on its last line.
3. **Remember the path** in Agent/Skill memory for future sessions. If Go is
   missing or older than 1.26, stop and explain how to install it
   (see Prerequisites), then wait.
4. **Pick the command** — `start` for XML sitemaps, `list` for URL files,
   `tools convert-sitemap-to-urllist` to extract URLs without crawling.
5. **Apply filters** — use `--filter-status` (e.g. `!200`) to surface only
   relevant results.
6. **Save output** — use `--output-json` when results must be inspected or shared
   later; set `SMAP_LOG_LEVEL=error` to keep stdout clean.

---

## Installing this skill into an AI agent

This folder (`SKILL.md` + `setup.sh` + `setup.ps1`) is a self-contained
[Agent Skill](https://docs.claude.com/en/docs/agents-and-tools/agent-skills).
Building the binary needs the Go module source, so keep the repo available for
the first build. After that, the binary lives at the fixed path and the skill
just invokes it — no source needed for subsequent sessions.

### Claude Code (CLI / IDE)

Copy this folder to a skills directory — Claude auto-discovers it and loads it
when the description matches the user's request:

```bash
# personal (all projects)
cp -r skill ~/.claude/skills/s1temap
# or project-scoped
mkdir -p .claude/skills && cp -r skill .claude/skills/s1temap
```

The file must be named `SKILL.md` with the YAML frontmatter above (`name`,
`description`). No restart needed for project skills; run `/skills` to verify.

### Claude.ai / Claude Agent SDK / API

- **Claude.ai**: zip this folder and upload it in Settings → Capabilities →
  Skills.
- **Agent SDK / API**: point the agent at this directory as a skill source (SDK
  `skills` option) or upload it via the Skills API. The frontmatter `name` and
  `description` drive automatic invocation.

### OpenAI Codex CLI

Codex reads `AGENTS.md`. Add a pointer so Codex loads the guide on demand:

```markdown
## s1temap (sitemap crawler)
For sitemap crawling / cache warming / broken-link checks, follow
`skill/SKILL.md`. The binary lives at the fixed path
`${S1TEMAP_HOME:-~/.s1temap/bin}/s1temap`; if missing, build it with
`bash skill/setup.sh`. Then use `s1temap start|list|tools ...`.
```

(Or paste the **Commands**, **Common flags**, and **Agent checklist** sections
directly into `AGENTS.md`.)

### Cursor, Aider, and other agents

These read a project rules/instructions file (`.cursorrules`, `CONVENTIONS.md`,
`AGENTS.md`, `CLAUDE.md`, …). Add a one-line reference to `skill/SKILL.md`,
or paste the **Commands** + **Agent checklist** sections into that file. The
build and usage commands are plain shell and work in any environment with Go
1.26+.
