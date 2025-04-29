package check_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zvdy/parsero-go/internal/check"
)

func TestGetDisallowPaths(t *testing.T) {
	// Initially should be empty
	paths := check.GetDisallowPaths()
	if len(paths) != 0 {
		t.Errorf("Expected empty path list, got %d paths", len(paths))
	}
}

func TestConnCheckParsing(t *testing.T) {
	// Create a test server that serves a robots.txt file
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`
User-agent: *
Disallow: /admin/
Disallow: /private/
Disallow: /secret.html
Allow: /public/
`))
		} else if r.URL.Path == "/admin/" {
			w.WriteHeader(http.StatusForbidden)
		} else if r.URL.Path == "/private/" {
			w.WriteHeader(http.StatusNotFound)
		} else if r.URL.Path == "/secret.html" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Extract the hostname without the protocol
	url := server.URL[7:] // Remove "http://" prefix

	// Call ConnCheck with the test server
	check.ConnCheck(url, false, 1)

	// Verify paths are parsed correctly
	paths := check.GetDisallowPaths()
	if len(paths) != 3 {
		t.Errorf("Expected 3 paths, got %d", len(paths))
	}

	// Check for expected paths
	expectedPaths := map[string]bool{
		"admin/":      true,
		"private/":    true,
		"secret.html": true,
	}

	for _, path := range paths {
		if !expectedPaths[path] {
			t.Errorf("Unexpected path found: %s", path)
		}
	}
}

func TestPrintDate(t *testing.T) {
	// This is mostly a visual test, just ensure no panic
	check.PrintDate("hackthissite.org")
}
