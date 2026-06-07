package scanner_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zvdy/parsero-go/internal/scanner"
)

// newRobotsServer serves a robots.txt with three disallow entries and canned
// statuses for each path.
func newRobotsServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("User-agent: *\nDisallow: /admin/\nDisallow: /private/\nDisallow: /secret.html\nAllow: /public/\n"))
		case "/admin/":
			w.WriteHeader(http.StatusForbidden)
		case "/private/":
			w.WriteHeader(http.StatusNotFound)
		case "/secret.html":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestFetchDisallowPaths(t *testing.T) {
	srv := newRobotsServer()
	defer srv.Close()
	target := strings.TrimPrefix(srv.URL, "http://")

	s := scanner.New(srv.Client(), scanner.Options{Concurrency: 1})
	paths, err := s.FetchDisallowPaths(context.Background(), target)
	if err != nil {
		t.Fatalf("FetchDisallowPaths: %v", err)
	}
	if len(paths) != 3 {
		t.Fatalf("expected 3 paths, got %d: %v", len(paths), paths)
	}
	want := map[string]bool{"admin/": true, "private/": true, "secret.html": true}
	for _, p := range paths {
		if !want[p] {
			t.Errorf("unexpected path: %q", p)
		}
	}
}

func TestFetchDisallowPathsMaxCap(t *testing.T) {
	srv := newRobotsServer()
	defer srv.Close()
	target := strings.TrimPrefix(srv.URL, "http://")

	s := scanner.New(srv.Client(), scanner.Options{Concurrency: 1, MaxPaths: 2})
	paths, err := s.FetchDisallowPaths(context.Background(), target)
	if err != nil {
		t.Fatalf("FetchDisallowPaths: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("MaxPaths cap not honored: got %d paths", len(paths))
	}
}

func TestRunReturnsResults(t *testing.T) {
	srv := newRobotsServer()
	defer srv.Close()
	target := strings.TrimPrefix(srv.URL, "http://")

	s := scanner.New(srv.Client(), scanner.Options{Concurrency: 2})
	results, disallow, err := s.Run(context.Background(), target)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(disallow) != 3 {
		t.Fatalf("expected 3 disallow paths, got %d", len(disallow))
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	codes := map[int]int{}
	for _, r := range results {
		if r.Error != nil {
			t.Errorf("unexpected error for %s: %v", r.URL, r.Error)
		}
		if r.Source != scanner.SourceRobots {
			t.Errorf("expected source %q, got %q", scanner.SourceRobots, r.Source)
		}
		codes[r.StatusCode]++
	}
	if codes[200] != 1 || codes[403] != 1 || codes[404] != 1 {
		t.Errorf("unexpected status distribution: %v", codes)
	}
}

func TestRunNoRobots(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	target := strings.TrimPrefix(srv.URL, "http://")

	// 404 robots.txt still returns a (empty) body, so no fatal error but no paths.
	s := scanner.New(srv.Client(), scanner.Options{Concurrency: 1})
	results, disallow, err := s.Run(context.Background(), target)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(disallow) != 0 || len(results) != 0 {
		t.Fatalf("expected no paths/results, got %d/%d", len(disallow), len(results))
	}
}

func TestProgressCallback(t *testing.T) {
	srv := newRobotsServer()
	defer srv.Close()
	target := strings.TrimPrefix(srv.URL, "http://")

	s := scanner.New(srv.Client(), scanner.Options{Concurrency: 1})
	paths, _ := s.FetchDisallowPaths(context.Background(), target)

	var lastDone, lastTotal int
	s.OnProgress(func(done, total int) { lastDone, lastTotal = done, total })
	s.CheckPaths(context.Background(), target, paths)

	if lastTotal != len(paths) || lastDone != len(paths) {
		t.Errorf("progress ended at %d/%d, want %d/%d", lastDone, lastTotal, len(paths), len(paths))
	}
}
