package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zvdy/parsero-go/internal/scanner"
	"github.com/zvdy/parsero-go/pkg/export"
)

// newTestServer serves a small robots.txt plus canned path statuses, so tests
// stay deterministic and offline.
func newTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("User-agent: *\nDisallow: /admin/\nDisallow: /private/\nDisallow: /open/\n"))
		case "/open/":
			w.WriteHeader(http.StatusOK)
		case "/admin/":
			w.WriteHeader(http.StatusForbidden)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestScannerRun(t *testing.T) {
	srv := newTestServer()
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
}

func TestJSONExport(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	target := strings.TrimPrefix(srv.URL, "http://")

	s := scanner.New(srv.Client(), scanner.Options{Concurrency: 2})
	start := time.Now()
	results, _, err := s.Run(context.Background(), target)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	sr := export.CreateScanResult(target, time.Since(start), results, false)
	if sr.TotalPaths != 3 {
		t.Errorf("expected 3 total paths, got %d", sr.TotalPaths)
	}
	if sr.Status200 != 1 {
		t.Errorf("expected 1 status-200 result, got %d", sr.Status200)
	}

	js, err := export.ToJSON(sr)
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	if !strings.Contains(js, "total_paths") {
		t.Errorf("JSON missing expected field: %s", js)
	}
}

func TestPrintResultsNoPanic(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	target := strings.TrimPrefix(srv.URL, "http://")

	s := scanner.New(srv.Client(), scanner.Options{Concurrency: 1})
	results, _, _ := s.Run(context.Background(), target)
	// Should not panic with either flag value.
	printResults(results, false)
	printResults(results, true)
	printDate(target)
}
