package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"preditrix/s1temap/internal/meta"
)

const jobsPrefix = "/api/v1/jobs"

type Server struct {
	manager *Manager
	logger  *slog.Logger
}

func NewServer(logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		manager: NewManager(),
		logger:  logger,
	}
}

func (s *Server) Handler() http.Handler {
	return s.corsMiddleware(s)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/healthz":
		s.handleHealthz(w, r)
	case r.URL.Path == "/version":
		s.handleVersion(w, r)
	case r.URL.Path == jobsPrefix:
		s.handleJobsRoot(w, r)
	case strings.HasPrefix(r.URL.Path, jobsPrefix+"/"):
		s.handleJobsSubpath(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, VersionInfo{
		Version:   meta.VersionString(),
		GitCommit: meta.GitCommit,
	})
}

func (s *Server) handleJobsRoot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"jobs": s.manager.List()})
	case http.MethodPost:
		s.handleCreateJob(w, r)
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleJobsSubpath(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, jobsPrefix+"/")
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	id := parts[0]
	if len(parts) == 1 {
		s.handleJobByID(w, r, id)
		return
	}

	switch parts[1] {
	case "events":
		s.handleJobEvents(w, r, id)
	case "cancel":
		s.handleJobCancel(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var req JobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid JSON body: %w", err))
		return
	}
	if err := validateJobRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	j := s.manager.Create(req.Operation, cancel, "")
	j.mu.Lock()
	j.state.EventsURL = fmt.Sprintf("%s/%s/events", jobsPrefix, j.state.ID)
	j.mu.Unlock()

	go s.runJob(cancelCtx, j, req)

	writeJSON(w, http.StatusAccepted, j.snapshot())
}

func (s *Server) handleJobByID(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		state, ok := s.manager.Snapshot(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, state)
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w, http.MethodGet)
	}
}

func (s *Server) handleJobCancel(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodPost:
		if !s.manager.Cancel(id) {
			state, ok := s.manager.Snapshot(id)
			if !ok {
				http.NotFound(w, r)
				return
			}
			writeJSON(w, http.StatusConflict, state)
			return
		}
		state, ok := s.manager.Snapshot(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusAccepted, state)
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w, http.MethodPost)
	}
}

func (s *Server) handleJobEvents(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		j, ok := s.manager.Get(id)
		if !ok {
			http.NotFound(w, r)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		history, events, unsubscribe, _ := j.subscribe()
		defer unsubscribe()

		for _, event := range history {
			if err := writeSSEEvent(w, event); err != nil {
				return
			}
			flusher.Flush()
		}

		for {
			select {
			case <-r.Context().Done():
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				if err := writeSSEEvent(w, event); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w, http.MethodGet)
	}
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func methodNotAllowed(w http.ResponseWriter, allowed ...string) {
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func writeSSEEvent(w http.ResponseWriter, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", event.Type); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}
	return nil
}
