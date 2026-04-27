package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// LogEntry is one captured request/response pair.
type LogEntry struct {
	Time         string `json:"time"`
	Method       string `json:"method"`
	Path         string `json:"path"`
	Status       int    `json:"status"`
	DurationMs   int64  `json:"duration_ms"`
	RequestBody  string `json:"request_body"`
	ResponseBody string `json:"response_body"`
}

// LogStore is a small ring buffer of LogEntry.
type LogStore struct {
	mu      sync.Mutex
	entries []LogEntry
	max     int
}

// NewLogStore creates a store with capacity max.
func NewLogStore(max int) *LogStore {
	if max <= 0 {
		max = 256
	}
	return &LogStore{max: max}
}

// Add records one entry.
func (s *LogStore) Add(e LogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, e)
	if len(s.entries) > s.max {
		s.entries = s.entries[len(s.entries)-s.max:]
	}
}

// List returns the most recent up to limit entries.
func (s *LogStore) List(limit int) []LogEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Clamp to safe range.
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	if limit > len(s.entries) {
		limit = len(s.entries)
	}
	out := make([]LogEntry, limit)
	copy(out, s.entries[len(s.entries)-limit:])
	return out
}

// Stats returns aggregate counts.
func (s *LogStore) Stats() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	byStatus := map[int]int{}
	for _, e := range s.entries {
		byStatus[e.Status]++
	}
	keys := map[string]int{}
	for k, v := range byStatus {
		keys[strconv.Itoa(k)] = v
	}
	return map[string]any{
		"total":       len(s.entries),
		"by_status":   keys,
		"buffer_size": s.max,
	}
}

// withLogging records every request when cfg.Debug is true.
type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *loggingResponseWriter) WriteHeader(s int) {
	w.status = s
	w.ResponseWriter.WriteHeader(s)
}

func (w *loggingResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *loggingResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func withLogging(store *LogStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body bytes.Buffer
		if r.Body != nil {
			_, _ = io.Copy(&body, r.Body)
			r.Body = io.NopCloser(bytes.NewReader(body.Bytes()))
		}
		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(lw, r)
		store.Add(LogEntry{
			Time:         start.UTC().Format(time.RFC3339Nano),
			Method:       r.Method,
			Path:         r.URL.Path,
			Status:       lw.status,
			DurationMs:   time.Since(start).Milliseconds(),
			RequestBody:  body.String(),
			ResponseBody: lw.body.String(),
		})
	})
}

func handleLogsList(store *LogStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		entries := store.List(limit)
		writeJSON(w, http.StatusOK, map[string]any{"data": entries})
	}
}

func handleLogsStats(store *LogStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, store.Stats())
	}
}

// envelopeJSON is a tiny helper so debug branches can return tiny JSON.
func envelopeJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
