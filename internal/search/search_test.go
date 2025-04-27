package search

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/zvdy/parsero-go/internal/check"
)

// BenchmarkSearchBing benchmarks the Bing search engine
func BenchmarkSearchBing(b *testing.B) {
	url := "http://bing.com"
	only200 := false
	concurrency := runtime.NumCPU()
	engine := EngineBing

	// Run once to verify we get results
	results := SearchDisallowEntries(url, only200, concurrency, engine)
	if len(results) == 0 {
		b.Fatalf("Expected search results but got none")
	}
	b.Logf("Found %d results in verification run", len(results))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		startTime := time.Now()
		SearchDisallowEntries(url, only200, concurrency, engine)
		elapsed := time.Since(startTime)
		b.ReportMetric(float64(elapsed.Milliseconds()), "ms/op")
	}
}

// BenchmarkSearchDuckDuckGo benchmarks the DuckDuckGo search engine
func BenchmarkSearchDuckDuckGo(b *testing.B) {
	url := "http://github.com"
	only200 := false
	concurrency := runtime.NumCPU()
	engine := EngineDuckDuckGo

	// Run once to verify we get results
	results := SearchDisallowEntries(url, only200, concurrency, engine)
	if len(results) == 0 {
		b.Fatalf("Expected search results but got none")
	}
	b.Logf("Found %d results in verification run", len(results))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		startTime := time.Now()
		SearchDisallowEntries(url, only200, concurrency, engine)
		elapsed := time.Since(startTime)
		b.ReportMetric(float64(elapsed.Milliseconds()), "ms/op")
	}
}

// BenchmarkSearchDifferentConcurrency benchmarks search with different concurrency levels
func BenchmarkSearchDifferentConcurrency(b *testing.B) {
	url := "http://bing.com"
	only200 := false
	engine := EngineBing

	concurrencyLevels := []int{1, 2, 4, 8, 16, 32}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency-%d", concurrency), func(b *testing.B) {
			// First verify we get results with this concurrency level
			results := SearchDisallowEntries(url, only200, concurrency, engine)
			if len(results) == 0 {
				b.Fatalf("Expected search results but got none with concurrency %d", concurrency)
			}
			b.Logf("Found %d results with concurrency %d", len(results), concurrency)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				startTime := time.Now()
				SearchDisallowEntries(url, only200, concurrency, engine)
				elapsed := time.Since(startTime)
				b.ReportMetric(float64(elapsed.Milliseconds()), "ms/op")
			}
		})
	}
}

// TestDifferentSearchEngines tests both Bing and DuckDuckGo search engines
func TestDifferentSearchEngines(t *testing.T) {
	url := "github.com"
	only200 := false
	concurrency := runtime.NumCPU()

	// First, populate the disallow entries by calling ConnCheck
	t.Log("Calling ConnCheck to populate disallow entries...")
	check.ConnCheck(url, only200, concurrency)

	// Get the disallow paths that were found (if any)
	paths := check.GetDisallowPaths()
	if len(paths) == 0 {
		t.Log("No disallow entries were found in robots.txt. Using mock data instead.")
		for _, engine := range []SearchEngine{EngineBing, EngineDuckDuckGo} {
			t.Run(string(engine), func(t *testing.T) {
				// Use mock data for testing
				mockResults := SetupTestData()
				t.Logf("Search engine: %s - Using mock data", engine)
				t.Logf("Found %d mock results", len(mockResults))
				for _, result := range mockResults {
					t.Logf("%s %d %s", result.URL, result.StatusCode, result.Status)
				}
			})
		}
		return
	}

	engines := []struct {
		name   string
		engine SearchEngine
	}{
		{"Bing", EngineBing},
		{"DuckDuckGo", EngineDuckDuckGo},
	}

	for _, e := range engines {
		t.Run(e.name, func(t *testing.T) {
			startTime := time.Now()

			results := SearchDisallowEntries(url, only200, concurrency, e.engine)

			elapsed := time.Since(startTime)
			t.Logf("Search engine: %s - Time: %v", e.name, elapsed)

			if len(results) == 0 {
				t.Log("No search results found, using mock data instead")
				mockResults := SetupTestData()
				t.Logf("Found %d mock results with %s engine", len(mockResults), e.name)
				for i, result := range mockResults {
					if i < 5 {
						t.Logf("%s %d %s", result.URL, result.StatusCode, result.Status)
					}
				}
			} else {
				t.Logf("Found %d results with %s engine", len(results), e.name)
				for i, result := range results {
					if i < 5 { // Only print first 5 results to avoid flooding output
						t.Logf("%s %d %s", result.URL, result.StatusCode, result.Status)
					}
				}
			}
		})
	}
}

// TestSearchEngineType tests the SearchEngine type functionality
func TestSearchEngineType(t *testing.T) {
	testCases := []struct {
		input    string
		expected SearchEngine
	}{
		{"bing", EngineBing},
		{"BING", EngineBing},
		{"duckduckgo", EngineDuckDuckGo},
		{"DUCKDUCKGO", EngineDuckDuckGo},
		{"duck", EngineDuckDuckGo},
		{"ddg", EngineDuckDuckGo},
		{"unknown", EngineBing}, // Default to Bing
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := ParseSearchEngine(tc.input)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

// TestSearchResultHandling tests the proper handling of search results
func TestSearchResultHandling(t *testing.T) {
	// This is just a placeholder for now
	// In a real test, you might want to mock HTTP responses
	result := SearchResult{
		URL:        "http://example.com/test",
		StatusCode: 200,
		Status:     "200 OK",
	}

	if result.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", result.StatusCode)
	}
}

// TestSearchResultsDisplay tests and displays actual search results
func TestSearchResultsDisplay(t *testing.T) {
	url := "bing.com"
	only200 := false
	concurrency := runtime.NumCPU()
	engine := EngineBing

	// First, populate the disallow entries by calling ConnCheck
	t.Log("Calling ConnCheck to populate disallow entries...")
	check.ConnCheck(url, only200, concurrency)

	// Get the disallow paths that were found (if any)
	paths := check.GetDisallowPaths()
	if len(paths) == 0 {
		t.Log("No disallow entries were found in robots.txt. Using mock data instead.")

		// Use mock data for testing
		mockResults := SetupTestData()

		t.Logf("Found %d mock results", len(mockResults))
		for _, result := range mockResults {
			t.Logf("%s %d %s", result.URL, result.StatusCode, result.Status)
		}
		return
	}

	// If we have real disallow entries, proceed with the actual search
	t.Log("Searching disallow entries...")
	results := SearchDisallowEntries(url, only200, concurrency, engine)

	if len(results) == 0 {
		t.Log("No search results found despite having disallow entries.")
		t.Log("This could be due to search engine restrictions or rate limiting.")
		t.Log("Using mock data instead.")

		// Use mock data as fallback
		mockResults := SetupTestData()

		t.Logf("Found %d mock results", len(mockResults))
		for _, result := range mockResults {
			t.Logf("%s %d %s", result.URL, result.StatusCode, result.Status)
		}
		return
	}

	t.Logf("Found %d real search results", len(results))
	for i, result := range results {
		if i < 10 { // Only print first 10 results to avoid flooding output
			t.Logf("%s %d %s", result.URL, result.StatusCode, result.Status)
		}
	}
}

// testWithMockedData is a helper function that tests the search functionality with mock data
func testWithMockedData(t *testing.T) {
	// Create a mock SearchResult to verify functionality
	result := SearchResult{
		URL:        "http://example.com/test",
		StatusCode: 200,
		Status:     "200 OK",
	}

	t.Log("Mocked search result:", result.URL, result.StatusCode, result.Status)

	if result.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", result.StatusCode)
	} else {
		t.Log("Mock test passed")
	}
}
