package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// createJob posts a job and returns its id.
func createJob(t *testing.T, base string, req JobRequest) string {
	t.Helper()
	body, _ := json.Marshal(req)
	resp, err := http.Post(base+jobsPrefix, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("create job status = %d, want 202", resp.StatusCode)
	}
	var st JobState
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		t.Fatalf("decode job: %v", err)
	}
	return st.ID
}

// waitJob polls until the job reaches a terminal status or times out.
func waitJob(t *testing.T, base, id string) JobState {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + jobsPrefix + "/" + id)
		if err != nil {
			t.Fatalf("get job: %v", err)
		}
		var st JobState
		_ = json.NewDecoder(resp.Body).Decode(&st)
		resp.Body.Close()
		switch st.Status {
		case JobStatusCompleted, JobStatusFailed, JobStatusCanceled:
			return st
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("job did not reach a terminal state in time")
	return JobState{}
}

func TestAPI_CrawlURLs(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/nf" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	api := httptest.NewServer(NewServer(nil).Handler())
	defer api.Close()

	id := createJob(t, api.URL, JobRequest{
		Operation: OperationCrawlURLs,
		URLs:      []string{target.URL + "/ok", target.URL + "/nf"},
	})
	st := waitJob(t, api.URL, id)

	if st.Status != JobStatusCompleted {
		t.Fatalf("status = %s, want completed (err=%q)", st.Status, st.Error)
	}
	if st.Progress.Checked != 2 {
		t.Errorf("checked = %d, want 2", st.Progress.Checked)
	}
	if st.Summary == nil || st.Summary.ByStatus[200] != 1 {
		t.Errorf("summary by-status = %+v, want 200:1", st.Summary)
	}
}

func TestAPI_ConvertSitemap(t *testing.T) {
	sitemap := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/a</loc></url>
  <url><loc>https://example.com/b</loc></url>
</urlset>`))
	}))
	defer sitemap.Close()

	api := httptest.NewServer(NewServer(nil).Handler())
	defer api.Close()

	id := createJob(t, api.URL, JobRequest{
		Operation:  OperationConvertSitemapToURLList,
		SitemapURL: sitemap.URL,
		Options:    CrawlOptions{StripBaseURL: true},
	})
	st := waitJob(t, api.URL, id)

	if st.Status != JobStatusCompleted {
		t.Fatalf("status = %s, want completed (err=%q)", st.Status, st.Error)
	}
	out, ok := st.Output.([]any)
	if !ok || len(out) != 2 {
		t.Fatalf("output = %#v, want 2 urls", st.Output)
	}
	if out[0] != "/a" {
		t.Errorf("first url = %v, want /a (base stripped)", out[0])
	}
}
