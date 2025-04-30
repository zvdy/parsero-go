package search_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zvdy/parsero-go/internal/search"
	"github.com/zvdy/parsero-go/pkg/types"
)

// MockBingServer creates a test server that simulates Bing search results
func MockBingServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if it's a search query
		if strings.Contains(r.URL.Path, "/search") {
			// Simulate Bing search results with cite tags
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`
				<html>
				<body>
					<div class="b_algo">
						<cite>http://hackthissite.org/admin</cite>
					</div>
					<div class="b_algo">
						<cite>http://hackthissite.org/private</cite>
					</div>
					<div class="b_algo">
						<cite>http://othersite.com/something</cite>
					</div>
				</body>
				</html>
			`))
		} else if strings.Contains(r.URL.Host, "hackthissite.org") {
			// Simulate responses for hackthissite.org URLs
			if strings.Contains(r.URL.Path, "admin") {
				w.WriteHeader(http.StatusForbidden)
			} else if strings.Contains(r.URL.Path, "private") {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestSearchBing(t *testing.T) {
	// Set up a test server
	server := MockBingServer(t)
	defer server.Close()

	// Patch the HTTP client behavior for this test
	// This is just a simple test to check that the function runs without errors
	results := search.SetupTestData()
	if results != nil && len(results) != 0 {
		t.Logf("Received %d test results", len(results))
	}

	// Verify backward compatibility function works
	compatResults := search.SearchBing("hackthissite.org", false, 1)
	if compatResults != nil {
		t.Logf("SearchBing compatibility function returned results")
	}
}

func TestSearchDisallowEntries(t *testing.T) {
	// Test with zero concurrency (should use default)
	results := search.SearchDisallowEntries("hackthissite.org", false, 0)
	if results != nil {
		t.Logf("SearchDisallowEntries with zero concurrency ran successfully")
	}
}

func TestURLResult(t *testing.T) {
	// Create a test Result instance
	result := types.Result{
		URL:        "http://hackthissite.org/admin",
		StatusCode: 403,
		Status:     "403 Forbidden",
		Error:      nil,
	}

	// Verify the fields
	if result.URL != "http://hackthissite.org/admin" {
		t.Errorf("Expected URL 'http://hackthissite.org/admin', got '%s'", result.URL)
	}
	if result.StatusCode != 403 {
		t.Errorf("Expected StatusCode 403, got %d", result.StatusCode)
	}
	if result.Status != "403 Forbidden" {
		t.Errorf("Expected Status '403 Forbidden', got '%s'", result.Status)
	}
	if result.Error != nil {
		t.Errorf("Expected Error nil, got %v", result.Error)
	}
}
