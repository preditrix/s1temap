package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Manager keeps the in-memory set of jobs, guarded by an RWMutex.
type Manager struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

func NewManager() *Manager {
	return &Manager{jobs: make(map[string]*Job)}
}

func (m *Manager) Create(operation Operation, cancel context.CancelFunc, eventsURL string) *Job {
	j := newJob(operation, cancel, eventsURL)

	m.mu.Lock()
	m.jobs[j.state.ID] = j
	m.mu.Unlock()

	return j
}

func (m *Manager) Get(id string) (*Job, bool) {
	m.mu.RLock()
	j, ok := m.jobs[id]
	m.mu.RUnlock()
	return j, ok
}

func (m *Manager) Snapshot(id string) (JobState, bool) {
	j, ok := m.Get(id)
	if !ok {
		return JobState{}, false
	}
	return j.snapshot(), true
}

func (m *Manager) List() []JobState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	states := make([]JobState, 0, len(m.jobs))
	for _, j := range m.jobs {
		states = append(states, j.snapshot())
	}
	return states
}

func (m *Manager) Cancel(id string) bool {
	j, ok := m.Get(id)
	if !ok {
		return false
	}
	return j.cancel()
}

func newJob(operation Operation, cancel context.CancelFunc, eventsURL string) *Job {
	now := time.Now()
	return &Job{
		state: JobState{
			ID:        newJobID(),
			Operation: operation,
			Status:    JobStatusQueued,
			CreatedAt: now,
			Progress:  Progress{},
			EventsURL: eventsURL,
		},
		cancelFn:    cancel,
		subscribers: make(map[chan Event]struct{}),
		byStatus:    make(map[int]int),
	}
}

func newJobID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

// Job holds one job's state and subscribers, guarded by mu.
type Job struct {
	mu          sync.Mutex
	state       JobState
	history     []Event
	subscribers map[chan Event]struct{}
	cancelFn    context.CancelFunc
	byStatus    map[int]int
	errors      int
}

func (j *Job) snapshot() JobState {
	j.mu.Lock()
	defer j.mu.Unlock()

	snap := j.state
	if j.state.StartedAt != nil {
		startedAt := *j.state.StartedAt
		snap.StartedAt = &startedAt
	}
	if j.state.EndedAt != nil {
		endedAt := *j.state.EndedAt
		snap.EndedAt = &endedAt
	}
	if j.state.Summary != nil {
		summary := *j.state.Summary
		if j.state.Summary.ByStatus != nil {
			summary.ByStatus = make(map[int]int, len(j.state.Summary.ByStatus))
			for k, v := range j.state.Summary.ByStatus {
				summary.ByStatus[k] = v
			}
		}
		snap.Summary = &summary
	}
	if j.state.Output != nil {
		snap.Output = j.state.Output
	}
	return snap
}

func (j *Job) subscribe() ([]Event, <-chan Event, func(), bool) {
	j.mu.Lock()
	defer j.mu.Unlock()

	history := append([]Event(nil), j.history...)
	ch := make(chan Event, 1024)
	if j.isTerminalLocked() {
		close(ch)
		return history, ch, func() {}, true
	}

	j.subscribers[ch] = struct{}{}
	unsubscribe := func() {
		j.mu.Lock()
		if _, ok := j.subscribers[ch]; ok {
			delete(j.subscribers, ch)
			close(ch)
		}
		j.mu.Unlock()
	}
	return history, ch, unsubscribe, true
}

func (j *Job) cancel() bool {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.isTerminalLocked() {
		return false
	}
	if j.cancelFn != nil {
		j.cancelFn()
		return true
	}
	return false
}

func (j *Job) markRunning() {
	j.mu.Lock()
	defer j.mu.Unlock()

	now := time.Now()
	j.state.Status = JobStatusRunning
	j.state.StartedAt = &now
	j.appendEventLocked(Event{
		Type:      "job.started",
		JobID:     j.state.ID,
		Status:    j.state.Status,
		Progress:  copyProgress(&j.state.Progress),
		Timestamp: now,
	})
}

func (j *Job) recordResult(res Result, visible bool) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.state.Progress.Checked++
	switch {
	case res.Error != "" || res.Status == 0:
		j.state.Progress.Failed++
		j.errors++
	case res.Status < 400:
		j.state.Progress.OK++
		j.byStatus[res.Status]++
	default:
		j.state.Progress.Failed++
		j.byStatus[res.Status]++
	}
	if !visible {
		j.state.Progress.Skipped++
	}

	result := res
	progress := j.state.Progress
	j.appendEventLocked(Event{
		Type:      "job.result",
		JobID:     j.state.ID,
		Status:    j.state.Status,
		Progress:  copyProgress(&progress),
		Result:    &result,
		Timestamp: time.Now(),
	})
	j.appendEventLocked(Event{
		Type:      "job.progress",
		JobID:     j.state.ID,
		Status:    j.state.Status,
		Progress:  copyProgress(&progress),
		Timestamp: time.Now(),
	})
}

func (j *Job) setProgress(progress Progress) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.state.Progress = progress
	j.appendEventLocked(Event{
		Type:      "job.progress",
		JobID:     j.state.ID,
		Status:    j.state.Status,
		Progress:  copyProgress(&progress),
		Timestamp: time.Now(),
	})
}

func (j *Job) complete(output any) {
	j.finish(JobStatusCompleted, "job.completed", output, nil)
}

func (j *Job) fail(err error) {
	j.finish(JobStatusFailed, "job.failed", nil, err)
}

func (j *Job) canceled() {
	j.finish(JobStatusCanceled, "job.canceled", nil, nil)
}

// finish applies a terminal state, builds the summary, emits the terminal event
// and closes subscribers.
func (j *Job) finish(status JobStatus, eventType string, output any, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	now := time.Now()
	j.state.Status = status
	j.state.EndedAt = &now
	if output != nil {
		j.state.Output = output
	}
	if err != nil {
		j.state.Error = err.Error()
	}

	summary := Summary{
		Operation:  j.state.Operation,
		Progress:   j.state.Progress,
		ByStatus:   copyStatusCounts(j.byStatus),
		Errors:     j.errors,
		StartedAt:  valueOrNow(j.state.StartedAt, now),
		EndedAt:    now,
		DurationMS: now.Sub(valueOrNow(j.state.StartedAt, now)).Milliseconds(),
	}
	j.state.Summary = &summary

	ev := Event{
		Type:      eventType,
		JobID:     j.state.ID,
		Status:    j.state.Status,
		Progress:  copyProgress(&j.state.Progress),
		Summary:   &summary,
		Output:    output,
		Timestamp: now,
	}
	if err != nil {
		ev.Error = err.Error()
	}
	j.appendEventLocked(ev)
	j.closeSubscribersLocked()
}

// appendEventLocked stores the event and fans it out. A subscriber whose buffer
// is full drops the event (non-blocking) to avoid stalling the crawl.
func (j *Job) appendEventLocked(ev Event) {
	j.history = append(j.history, ev)
	for ch := range j.subscribers {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (j *Job) closeSubscribersLocked() {
	for ch := range j.subscribers {
		close(ch)
		delete(j.subscribers, ch)
	}
}

func (j *Job) isTerminalLocked() bool {
	switch j.state.Status {
	case JobStatusCompleted, JobStatusFailed, JobStatusCanceled:
		return true
	default:
		return false
	}
}

func copyStatusCounts(src map[int]int) map[int]int {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[int]int, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func valueOrNow(t *time.Time, now time.Time) time.Time {
	if t == nil {
		return now
	}
	return *t
}

func copyProgress(p *Progress) *Progress {
	if p == nil {
		return nil
	}
	cp := *p
	return &cp
}
