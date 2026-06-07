package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zvdy/parsero-go/internal/config"
)

// testServer builds a Server with just the fields the middleware needs.
func testServer(rps float64, burst int) *Server {
	return &Server{
		cfg:     config.Config{IdentityHeader: "X-Auth-Request-Email"},
		limiter: newRateLimiter(rps, burst),
	}
}

func TestWithIdentityHeader(t *testing.T) {
	s := testServer(100, 100)
	var got string
	h := s.withIdentity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = identity(r)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Auth-Request-Email", "alice@example.com")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if got != "alice@example.com" {
		t.Errorf("identity = %q, want alice@example.com", got)
	}
}

func TestWithIdentityFallsBackToIP(t *testing.T) {
	s := testServer(100, 100)
	var got string
	h := s.withIdentity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = identity(r)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	h.ServeHTTP(httptest.NewRecorder(), req)

	if got != "203.0.113.7" {
		t.Errorf("identity = %q, want 203.0.113.7", got)
	}
}

func TestRateLimit(t *testing.T) {
	// burst of 2, no refill within the test window.
	s := testServer(0.001, 2)
	h := s.withRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	codes := make([]int, 4)
	for i := range codes {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "198.51.100.9:1234"
		h.ServeHTTP(rec, req)
		codes[i] = rec.Code
	}

	if codes[0] != 200 || codes[1] != 200 {
		t.Errorf("first two requests should pass, got %v", codes)
	}
	if codes[2] != http.StatusTooManyRequests || codes[3] != http.StatusTooManyRequests {
		t.Errorf("requests beyond burst should be 429, got %v", codes)
	}
}

func TestClientIPForwardedFor(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "70.1.2.3, 10.0.0.1")
	if ip := clientIP(req); ip != "70.1.2.3" {
		t.Errorf("clientIP = %q, want 70.1.2.3", ip)
	}
}
