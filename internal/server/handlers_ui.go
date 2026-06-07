package server

import (
	"net/http"

	"github.com/zvdy/parsero-go/internal/store"
)

type uiResult struct {
	URL    string
	Code   int
	Status string
	Error  string
	Source string
	OK     bool
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	scans, _ := s.store.ListScansByUser(r.Context(), identity(r), 20)
	schedules, _ := s.store.ListSchedulesByUser(r.Context(), identity(r))
	s.render(w, "index", map[string]any{
		"Identity":    identity(r),
		"BingEnabled": s.cfg.BingEnabled,
		"Scans":       scans,
		"Schedules":   schedules,
	})
}

func (s *Server) handleUISubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderError(w, http.StatusBadRequest, "invalid form")
		return
	}
	req := createScanRequest{
		Target:     r.FormValue("target"),
		Only200:    r.FormValue("only200") == "on",
		SearchBing: r.FormValue("search_bing") == "on",
	}
	sc, _, _, msg := s.submitScan(r.Context(), identity(r), req)
	if msg != "" {
		// 200 so HTMX swaps the fragment in — it ignores non-2xx bodies.
		s.render(w, "error", map[string]any{"Message": msg})
		return
	}
	w.Header().Set("HX-Redirect", "/scan/"+sc.ID)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleScanPage(w http.ResponseWriter, r *http.Request) {
	sc, err := s.loadOwnedScan(r)
	if err != nil {
		s.renderError(w, http.StatusNotFound, "scan not found")
		return
	}
	s.render(w, "scan", map[string]any{
		"Identity": identity(r),
		"Scan":     sc,
	})
}

func (s *Server) handleUIStatus(w http.ResponseWriter, r *http.Request) {
	sc, err := s.loadOwnedScan(r)
	if err != nil {
		s.renderError(w, http.StatusNotFound, "scan not found")
		return
	}
	done, total, _ := s.cache.GetProgress(r.Context(), sc.ID)
	// Tell HTMX to stop polling once terminal.
	if sc.Status == "done" || sc.Status == "failed" {
		w.Header().Set("HX-Trigger", "scan-finished")
	}
	s.render(w, "status", map[string]any{
		"Scan": sc, "Done": done, "Total": total,
	})
}

func (s *Server) handleUIResults(w http.ResponseWriter, r *http.Request) {
	sc, err := s.loadOwnedScan(r)
	if err != nil {
		s.renderError(w, http.StatusNotFound, "scan not found")
		return
	}
	rows, _ := s.store.ListResults(r.Context(), sc.ID)
	results := make([]uiResult, 0, len(rows))
	for _, rw := range rows {
		results = append(results, uiResult{
			URL: rw.URL, Code: rw.StatusCode, Status: rw.Status,
			Error: rw.Error, Source: rw.Source, OK: rw.StatusCode == 200,
		})
	}
	s.render(w, "results_table", map[string]any{
		"Scan": sc, "Results": results,
	})
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) renderError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	_ = s.templates.ExecuteTemplate(w, "error", map[string]any{"Message": msg})
}

func scanStatusClass(sc store.Scan) string {
	switch sc.Status {
	case "done":
		return "badge-done"
	case "failed":
		return "badge-failed"
	case "running":
		return "badge-running"
	default:
		return "badge-queued"
	}
}
