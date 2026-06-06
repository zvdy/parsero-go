package server

import (
	"net/http"

	"github.com/zvdy/parsero-go/internal/store"
)

// uiResult is the view model for a row in the results table.
type uiResult struct {
	URL    string
	Code   int
	Status string
	Error  string
	Source string
	OK     bool // true when status code is 200 (green styling)
}

// handleIndex renders the landing page with the user's recent scans.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	scans, _ := s.store.ListScansByUser(r.Context(), identity(r), 20)
	s.render(w, "index", map[string]any{
		"Identity":    identity(r),
		"BingEnabled": s.cfg.BingEnabled,
		"Scans":       scans,
	})
}

// handleUISubmit handles the HTMX form post. On success it asks HTMX to redirect
// the browser to the scan page; on error it returns an inline error fragment.
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
		// Render the inline error fragment with 200 so HTMX swaps it in (it
		// ignores non-2xx bodies by default). The REST API still returns proper
		// status codes; this is purely a UI affordance.
		s.render(w, "error", map[string]any{"Message": msg})
		return
	}
	// HTMX redirect to the live scan page.
	w.Header().Set("HX-Redirect", "/scan/"+sc.ID)
	w.WriteHeader(http.StatusOK)
}

// handleScanPage renders the live scan view.
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

// handleUIStatus returns the status partial (polling fallback for SSE).
func (s *Server) handleUIStatus(w http.ResponseWriter, r *http.Request) {
	sc, err := s.loadOwnedScan(r)
	if err != nil {
		s.renderError(w, http.StatusNotFound, "scan not found")
		return
	}
	done, total, _ := s.cache.GetProgress(r.Context(), sc.ID)
	// When finished, tell HTMX to stop polling by sending the terminal header.
	if sc.Status == "done" || sc.Status == "failed" {
		w.Header().Set("HX-Trigger", "scan-finished")
	}
	s.render(w, "status", map[string]any{
		"Scan": sc, "Done": done, "Total": total,
	})
}

// handleUIResults returns the results table partial.
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

// render executes a named template, writing a 500 on failure.
func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// renderError writes the error partial with the given status code.
func (s *Server) renderError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	_ = s.templates.ExecuteTemplate(w, "error", map[string]any{"Message": msg})
}

// scanStatusClass maps a scan status to a CSS badge class (used by templates).
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
