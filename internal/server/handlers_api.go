package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/zvdy/parsero-go/internal/safety"
	"github.com/zvdy/parsero-go/internal/store"
)

// createScanRequest is the POST /api/scans body.
type createScanRequest struct {
	Target     string `json:"target"`
	Only200    bool   `json:"only200"`
	SearchBing bool   `json:"search_bing"`
}

// scanResponse is the JSON view of a scan.
type scanResponse struct {
	ID              string  `json:"id"`
	Target          string  `json:"target"`
	Status          string  `json:"status"`
	Cached          bool    `json:"cached,omitempty"`
	Only200         bool    `json:"only200"`
	SearchBing      bool    `json:"search_bing"`
	DurationSeconds float64 `json:"duration_seconds"`
	TotalPaths      int     `json:"total_paths"`
	Status200       int     `json:"status_200"`
	OtherStatus     int     `json:"other_status"`
	Errors          int     `json:"errors"`
	ErrorMessage    string  `json:"error_message,omitempty"`
	CreatedAt       string  `json:"created_at"`
}

func toScanResponse(sc store.Scan, cached bool) scanResponse {
	return scanResponse{
		ID:              sc.ID,
		Target:          sc.Target,
		Status:          sc.Status,
		Cached:          cached,
		Only200:         sc.Only200,
		SearchBing:      sc.SearchBing,
		DurationSeconds: sc.DurationSeconds,
		TotalPaths:      sc.TotalPaths,
		Status200:       sc.Status200,
		OtherStatus:     sc.OtherStatus,
		Errors:          sc.Errors,
		ErrorMessage:    sc.ErrorMessage,
		CreatedAt:       sc.CreatedAt.Format(time.RFC3339),
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// submitScan is the shared core behind the REST and UI submit paths. It
// normalizes + validates the target, applies cache lookup, enforces throttling,
// then persists and enqueues. It returns the scan, whether it was a cache hit,
// an HTTP status code, and an error message (empty on success).
func (s *Server) submitScan(ctx context.Context, userID string, req createScanRequest) (store.Scan, bool, int, string) {
	target, err := safety.NormalizeTarget(req.Target)
	if err != nil {
		return store.Scan{}, false, http.StatusBadRequest, err.Error()
	}
	if err := safety.ResolveAndCheck(ctx, target); err != nil {
		return store.Scan{}, false, http.StatusBadRequest, "target not allowed: " + err.Error()
	}

	hash := store.OptionsHash(target, req.Only200, req.SearchBing)

	// Cache: Redis first, Postgres fallback.
	if id, ok, _ := s.cache.GetScanID(ctx, hash); ok {
		if sc, err := s.store.GetScan(ctx, id); err == nil && sc.Status == "done" {
			return sc, true, http.StatusOK, ""
		}
	}
	if sc, err := s.store.FindCachedScan(ctx, hash, s.cfg.ScanCacheTTL); err == nil {
		_ = s.cache.PutScanID(ctx, hash, sc.ID, s.cfg.ScanCacheTTL)
		return sc, true, http.StatusOK, ""
	}

	// Backpressure: reject when the queue is saturated.
	if s.cfg.MaxQueueDepth > 0 {
		if depth, err := s.queue.Depth(); err == nil && depth >= s.cfg.MaxQueueDepth {
			return store.Scan{}, false, http.StatusTooManyRequests, "service busy, try again later"
		}
	}

	// Throttle: per-user + global in-flight caps (atomic reservation).
	ok, err := s.cache.TryAcquire(ctx, userID, s.cfg.MaxPerUser, s.cfg.MaxInflight)
	if err != nil {
		return store.Scan{}, false, http.StatusInternalServerError, "throttle check failed"
	}
	if !ok {
		return store.Scan{}, false, http.StatusTooManyRequests, "too many concurrent scans; try again later"
	}

	// Persist then enqueue. If enqueue fails, release the slot we reserved.
	id, err := s.store.CreateScan(ctx, store.Scan{
		UserID: userID, Target: target, OptionsHash: hash,
		Only200: req.Only200, SearchBing: req.SearchBing,
	})
	if err != nil {
		s.cache.Release(ctx, userID)
		return store.Scan{}, false, http.StatusInternalServerError, "could not create scan"
	}
	if err := s.queue.Enqueue(ctx, id); err != nil {
		s.cache.Release(ctx, userID)
		_ = s.store.FailScan(ctx, id, "enqueue failed")
		return store.Scan{}, false, http.StatusInternalServerError, "could not enqueue scan"
	}

	sc, err := s.store.GetScan(ctx, id)
	if err != nil {
		return store.Scan{ID: id, Target: target, Status: "queued"}, false, http.StatusAccepted, ""
	}
	return sc, false, http.StatusAccepted, ""
}

func (s *Server) handleCreateScan(w http.ResponseWriter, r *http.Request) {
	var req createScanRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	sc, cached, code, msg := s.submitScan(r.Context(), identity(r), req)
	if msg != "" {
		writeErr(w, code, msg)
		return
	}
	writeJSON(w, code, toScanResponse(sc, cached))
}

func (s *Server) handleListScans(w http.ResponseWriter, r *http.Request) {
	scans, err := s.store.ListScansByUser(r.Context(), identity(r), 50)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not list scans")
		return
	}
	out := make([]scanResponse, 0, len(scans))
	for _, sc := range scans {
		out = append(out, toScanResponse(sc, false))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetScan(w http.ResponseWriter, r *http.Request) {
	sc, err := s.loadOwnedScan(r)
	if err != nil {
		s.writeScanLoadErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toScanResponse(sc, false))
}

// resultResponse is the JSON view of a single probed path.
type resultResponse struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code,omitempty"`
	Status     string `json:"status,omitempty"`
	Error      string `json:"error,omitempty"`
	Source     string `json:"source"`
}

func (s *Server) handleGetResults(w http.ResponseWriter, r *http.Request) {
	sc, err := s.loadOwnedScan(r)
	if err != nil {
		s.writeScanLoadErr(w, err)
		return
	}
	rows, err := s.store.ListResults(r.Context(), sc.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not load results")
		return
	}
	out := make([]resultResponse, 0, len(rows))
	for _, rw := range rows {
		out = append(out, resultResponse{
			URL: rw.URL, StatusCode: rw.StatusCode, Status: rw.Status,
			Error: rw.Error, Source: rw.Source,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// loadOwnedScan fetches the scan named by the {id} path value and verifies the
// caller owns it, preventing cross-tenant reads.
func (s *Server) loadOwnedScan(r *http.Request) (store.Scan, error) {
	sc, err := s.store.GetScan(r.Context(), r.PathValue("id"))
	if err != nil {
		return store.Scan{}, err
	}
	if sc.UserID != identity(r) {
		return store.Scan{}, store.ErrNotFound // hide existence from non-owners
	}
	return sc, nil
}

func (s *Server) writeScanLoadErr(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "scan not found")
		return
	}
	writeErr(w, http.StatusInternalServerError, "could not load scan")
}
