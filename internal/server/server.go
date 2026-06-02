// Package server exposes the parsero REST API and HTMX UI. It is stateless:
// all shared state lives in Postgres (durable) and Redis (queue/cache/throttle),
// so any number of instances can run behind a load balancer.
package server

import (
	"embed"
	"html/template"
	"net/http"

	"github.com/zvdy/parsero-go/internal/cache"
	"github.com/zvdy/parsero-go/internal/config"
	"github.com/zvdy/parsero-go/internal/queue"
	"github.com/zvdy/parsero-go/internal/store"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

// Server bundles the dependencies shared by all handlers.
type Server struct {
	cfg       config.Config
	store     *store.Store
	cache     *cache.Cache
	queue     *queue.Client
	templates *template.Template
	limiter   *rateLimiter
}

// New constructs a Server and parses the embedded templates.
func New(cfg config.Config, st *store.Store, c *cache.Cache, q *queue.Client) (*Server, error) {
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"statusClass": scanStatusClass,
		"percent": func(done, total int) int {
			if total <= 0 {
				return 0
			}
			return done * 100 / total
		},
	}).ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Server{
		cfg:       cfg,
		store:     st,
		cache:     c,
		queue:     q,
		templates: tmpl,
		limiter:   newRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst),
	}, nil
}

// Handler builds the HTTP handler with all routes and middleware applied.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Static assets.
	mux.Handle("GET /static/", http.FileServerFS(staticFS))

	// REST API.
	mux.HandleFunc("POST /api/scans", s.handleCreateScan)
	mux.HandleFunc("GET /api/scans", s.handleListScans)
	mux.HandleFunc("GET /api/scans/{id}", s.handleGetScan)
	mux.HandleFunc("GET /api/scans/{id}/results", s.handleGetResults)
	mux.HandleFunc("GET /api/scans/{id}/events", s.handleEvents)

	// Health.
	mux.HandleFunc("GET /healthz", s.handleHealth)

	// HTMX UI.
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("POST /ui/scans", s.handleUISubmit)
	mux.HandleFunc("GET /scan/{id}", s.handleScanPage)
	mux.HandleFunc("GET /ui/scans/{id}/status", s.handleUIStatus)
	mux.HandleFunc("GET /ui/scans/{id}/results", s.handleUIResults)

	// Middleware chain: logging -> identity -> rate limit.
	return s.withLogging(s.withIdentity(s.withRateLimit(mux)))
}

// handleHealth is a lightweight readiness probe.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Pool().Ping(r.Context()); err != nil {
		http.Error(w, "db unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
