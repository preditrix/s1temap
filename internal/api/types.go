// Package api implements the HTTP API server wrapping the crawl engine.
package api

import "time"

type Operation string

const (
	OperationCrawlURLs               Operation = "crawl_urls"
	OperationCrawlSitemap            Operation = "crawl_sitemap"
	OperationConvertSitemapToURLList Operation = "convert_sitemap_to_urllist"
)

type JobStatus string

const (
	JobStatusQueued    JobStatus = "queued"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCanceled  JobStatus = "canceled"
)

type JobRequest struct {
	Operation  Operation    `json:"operation"`
	URLs       []string     `json:"urls,omitempty"`
	SitemapURL string       `json:"sitemap_url,omitempty"`
	Options    CrawlOptions `json:"options,omitempty"`
}

type CrawlOptions struct {
	NumWorkers    int               `json:"num_workers,omitempty"`
	HTTPTimeoutMS int               `json:"http_timeout_ms,omitempty"`
	IdleTimeoutMS int               `json:"idle_timeout_ms,omitempty"`
	UserAgent     string            `json:"user_agent,omitempty"`
	StatusFilter  string            `json:"status_filter,omitempty"`
	PrefixURL     string            `json:"prefix_url,omitempty"`
	AuthUsername  string            `json:"auth_username,omitempty"`
	AuthPassword  string            `json:"auth_password,omitempty"`
	Method        string            `json:"method,omitempty"`
	StripBaseURL  bool              `json:"strip_base_url,omitempty"`
	MaxDepth      int               `json:"max_depth,omitempty"`
	MaxSitemaps   int               `json:"max_sitemaps,omitempty"`
	MaxURLs       int               `json:"max_urls,omitempty"`
	Insecure      bool              `json:"insecure,omitempty"`
	Cookies       map[string]string `json:"cookies,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
}

type Progress struct {
	Total   int `json:"total,omitempty"`
	Checked int `json:"checked"`
	OK      int `json:"ok"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

type Result struct {
	URL        string    `json:"url"`
	Status     int       `json:"status"`
	Error      string    `json:"error,omitempty"`
	Method     string    `json:"method,omitempty"`
	Fallback   bool      `json:"fallback,omitempty"`
	HeadStatus int       `json:"head_status,omitempty"`
	DurationMS int64     `json:"duration_ms"`
	Timestamp  time.Time `json:"timestamp"`
}

type Summary struct {
	Operation  Operation   `json:"operation"`
	Progress   Progress    `json:"progress"`
	ByStatus   map[int]int `json:"by_status,omitempty"`
	Errors     int         `json:"errors,omitempty"`
	StartedAt  time.Time   `json:"started_at"`
	EndedAt    time.Time   `json:"ended_at"`
	DurationMS int64       `json:"duration_ms"`
}

type JobState struct {
	ID        string     `json:"id"`
	Operation Operation  `json:"operation"`
	Status    JobStatus  `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	Progress  Progress   `json:"progress"`
	Summary   *Summary   `json:"summary,omitempty"`
	Output    any        `json:"output,omitempty"`
	Error     string     `json:"error,omitempty"`
	EventsURL string     `json:"events_url,omitempty"`
}

type Event struct {
	Type      string    `json:"type"`
	JobID     string    `json:"job_id"`
	Status    JobStatus `json:"status,omitempty"`
	Progress  *Progress `json:"progress,omitempty"`
	Result    *Result   `json:"result,omitempty"`
	Summary   *Summary  `json:"summary,omitempty"`
	Output    any       `json:"output,omitempty"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type VersionInfo struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit,omitempty"`
}
