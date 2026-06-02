package server

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ctxKey string

const identityKey ctxKey = "identity"

// withIdentity extracts the caller identity from the trusted auth header
// injected by the upstream reverse proxy (oauth2-proxy / forward-auth). When the
// header is absent (e.g. local dev) it falls back to the client IP so the service
// still works, but in production the proxy is responsible for authn.
func (s *Server) withIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(s.cfg.IdentityHeader)
		if id == "" {
			id = clientIP(r)
		}
		ctx := context.WithValue(r.Context(), identityKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// identity returns the caller identity stored by withIdentity.
func identity(r *http.Request) string {
	if v, ok := r.Context().Value(identityKey).(string); ok {
		return v
	}
	return clientIP(r)
}

// clientIP best-effort extracts the client IP, honoring X-Forwarded-For.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first, _, _ := strings.Cut(xff, ",")
		return strings.TrimSpace(first)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// rateLimiter holds a token-bucket limiter per identity. This is per-instance;
// strict global limits would need a Redis token bucket, but per-instance limits
// are a reasonable first guard against abuse.
type rateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	rps      rate.Limit
	burst    int
}

func newRateLimiter(rps float64, burst int) *rateLimiter {
	return &rateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rps:      rate.Limit(rps),
		burst:    burst,
	}
}

func (rl *rateLimiter) get(id string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	l, ok := rl.limiters[id]
	if !ok {
		l = rate.NewLimiter(rl.rps, rl.burst)
		rl.limiters[id] = l
	}
	return l
}

// withRateLimit rejects requests that exceed the per-identity rate.
func (s *Server) withRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.limiter.get(identity(r)).Allow() {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// statusRecorder captures the response status for logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher so SSE streaming keeps working through the
// wrapped ResponseWriter.
func (sr *statusRecorder) Flush() {
	if f, ok := sr.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// withLogging logs each request with method, path, status, and duration.
func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}
