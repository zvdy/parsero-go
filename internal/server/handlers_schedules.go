package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/zvdy/parsero-go/internal/safety"
	"github.com/zvdy/parsero-go/internal/store"
)

type createScheduleRequest struct {
	Target         string `json:"target"`
	Cron           string `json:"cron"`
	Only200        bool   `json:"only200"`
	SearchBing     bool   `json:"search_bing"`
	NotifyWebhook  string `json:"notify_webhook"`
	NotifyOnChange bool   `json:"notify_on_change"`
}

type scheduleResponse struct {
	ID             string `json:"id"`
	Target         string `json:"target"`
	Cron           string `json:"cron"`
	Enabled        bool   `json:"enabled"`
	Only200        bool   `json:"only200"`
	SearchBing     bool   `json:"search_bing"`
	NotifyWebhook  string `json:"notify_webhook,omitempty"`
	NotifyOnChange bool   `json:"notify_on_change"`
	CreatedAt      string `json:"created_at"`
	LastRunAt      string `json:"last_run_at,omitempty"`
}

func toScheduleResponse(sc store.Schedule) scheduleResponse {
	r := scheduleResponse{
		ID: sc.ID, Target: sc.Target, Cron: sc.Cron, Enabled: sc.Enabled,
		Only200: sc.Only200, SearchBing: sc.SearchBing,
		NotifyWebhook: sc.NotifyWebhook, NotifyOnChange: sc.NotifyOnChange,
		CreatedAt: sc.CreatedAt.Format(time.RFC3339),
	}
	if sc.LastRunAt != nil {
		r.LastRunAt = sc.LastRunAt.Format(time.RFC3339)
	}
	return r
}

// cronParser matches what asynq's scheduler accepts (5-field specs + @descriptors).
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// buildSchedule validates a request into a store.Schedule, or returns an HTTP
// status + message on failure.
func (s *Server) buildSchedule(ctx context.Context, userID string, req createScheduleRequest) (store.Schedule, int, string) {
	target, err := safety.NormalizeTarget(req.Target)
	if err != nil {
		return store.Schedule{}, http.StatusBadRequest, err.Error()
	}
	if err := safety.ResolveAndCheck(ctx, target); err != nil {
		return store.Schedule{}, http.StatusBadRequest, "target not allowed: " + err.Error()
	}
	if _, err := cronParser.Parse(req.Cron); err != nil {
		return store.Schedule{}, http.StatusBadRequest, "invalid cron expression"
	}
	if req.NotifyWebhook != "" {
		u, err := url.Parse(req.NotifyWebhook)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return store.Schedule{}, http.StatusBadRequest, "notify_webhook must be a valid http(s) URL"
		}
	}
	return store.Schedule{
		UserID: userID, Target: target,
		OptionsHash: store.OptionsHash(target, req.Only200, req.SearchBing),
		Only200:     req.Only200, SearchBing: req.SearchBing,
		Cron: req.Cron, Enabled: true,
		NotifyWebhook: req.NotifyWebhook, NotifyOnChange: req.NotifyOnChange,
	}, http.StatusOK, ""
}

func (s *Server) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	var req createScheduleRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	sch, code, msg := s.buildSchedule(r.Context(), identity(r), req)
	if msg != "" {
		writeErr(w, code, msg)
		return
	}
	id, err := s.store.CreateSchedule(r.Context(), sch)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not create schedule")
		return
	}
	sch.ID = id
	sch.CreatedAt = time.Now()
	writeJSON(w, http.StatusCreated, toScheduleResponse(sch))
}

func (s *Server) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	schedules, err := s.store.ListSchedulesByUser(r.Context(), identity(r))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not list schedules")
		return
	}
	out := make([]scheduleResponse, 0, len(schedules))
	for _, sc := range schedules {
		out = append(out, toScheduleResponse(sc))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	err := s.store.DeleteSchedule(r.Context(), r.PathValue("id"), identity(r))
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "schedule not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not delete schedule")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleEnableSchedule(w http.ResponseWriter, r *http.Request) {
	s.setEnabled(w, r, true)
}
func (s *Server) handleDisableSchedule(w http.ResponseWriter, r *http.Request) {
	s.setEnabled(w, r, false)
}

func (s *Server) setEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	err := s.store.SetScheduleEnabled(r.Context(), r.PathValue("id"), identity(r), enabled)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "schedule not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not update schedule")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUIScheduleSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderError(w, http.StatusBadRequest, "invalid form")
		return
	}
	req := createScheduleRequest{
		Target:         r.FormValue("target"),
		Cron:           r.FormValue("cron"),
		Only200:        r.FormValue("only200") == "on",
		SearchBing:     r.FormValue("search_bing") == "on",
		NotifyWebhook:  r.FormValue("notify_webhook"),
		NotifyOnChange: r.FormValue("notify_on_change") == "on",
	}
	sch, _, msg := s.buildSchedule(r.Context(), identity(r), req)
	if msg != "" {
		// 200 so HTMX swaps the fragment in (see handleUISubmit).
		s.render(w, "error", map[string]any{"Message": msg})
		return
	}
	if _, err := s.store.CreateSchedule(r.Context(), sch); err != nil {
		s.render(w, "error", map[string]any{"Message": "could not create monitor"})
		return
	}
	s.renderMonitors(w, r)
}

func (s *Server) handleUIScheduleDelete(w http.ResponseWriter, r *http.Request) {
	_ = s.store.DeleteSchedule(r.Context(), r.PathValue("id"), identity(r))
	s.renderMonitors(w, r)
}

func (s *Server) renderMonitors(w http.ResponseWriter, r *http.Request) {
	schedules, _ := s.store.ListSchedulesByUser(r.Context(), identity(r))
	s.render(w, "monitors", map[string]any{"Schedules": schedules})
}
