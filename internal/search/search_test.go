package search

import (
	"fmt"
	"runtime"
	"testing"
	"time"
)

// BenchmarkSearchBing benchmarks the Bing search engine
func BenchmarkSearchBing(b *testing.B) {
	url := "bing.com"
	only200 := false
	concurrency := runtime.NumCPU()
	engine := EngineBing

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		startTime := time.Now()
		SearchDisallowEntries(url, only200, concurrency, engine)
		elapsed := time.Since(startTime)
		b.ReportMetric(float64(elapsed.Milliseconds()), "ms/op")
	}
}

// BenchmarkSearchGoogle benchmarks the Google search engine
func BenchmarkSearchGoogle(b *testing.B) {
	url := "github.com"
	only200 := false
	concurrency := runtime.NumCPU()
	engine := EngineGoogle

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
	url := "bing.com"
	only200 := false
	engine := EngineBing

	concurrencyLevels := []int{1, 2, 4, 8, 16, 32}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency-%d", concurrency), func(b *testing.B) {
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

// TestDifferentSearchEngines tests both Bing and Google search engines
func TestDifferentSearchEngines(t *testing.T) {
	url := "github.com"
	only200 := false
	concurrency := runtime.NumCPU()

	engines := []struct {
		name   string
		engine SearchEngine
	}{
		{"Bing", EngineBing},
		{"Google", EngineGoogle},
	}

	for _, e := range engines {
		t.Run(e.name, func(t *testing.T) {
			startTime := time.Now()

			SearchDisallowEntries(url, only200, concurrency, e.engine)

			elapsed := time.Since(startTime)
			fmt.Printf("Search engine: %s - Time: %v\n", e.name, elapsed)
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
		{"google", EngineGoogle},
		{"GOOGLE", EngineGoogle},
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
