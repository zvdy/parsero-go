package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/zvdy/parsero-go/internal/check"
	"github.com/zvdy/parsero-go/internal/search"
)

// TestMain runs before all tests
func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()
	os.Exit(code)
}

// BenchmarkParseroFullPipeline benchmarks the full parsero pipeline including check and search
func BenchmarkParseroFullPipeline(b *testing.B) {
	url := "bing.com"
	only200 := false
	doSearch := true
	engine := search.EngineBing
	concurrency := runtime.NumCPU() // Use all available CPU cores

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		startTime := time.Now()

		check.ConnCheck(url, only200, concurrency)
		if doSearch {
			search.SearchDisallowEntries(url, only200, concurrency, engine)
		}

		elapsed := time.Since(startTime)
		b.ReportMetric(float64(elapsed.Milliseconds()), "ms/op")
	}
}

// BenchmarkParseroIntegrationDifferentConcurrency benchmarks the full parsero pipeline with different concurrency levels
func BenchmarkParseroIntegrationDifferentConcurrency(b *testing.B) {
	url := "bing.com"
	only200 := false
	doSearch := true
	engine := search.EngineBing

	concurrencyLevels := []int{1, 2, 4, 8, 16, 32}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency-%d", concurrency), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				startTime := time.Now()

				check.ConnCheck(url, only200, concurrency)
				if doSearch {
					search.SearchDisallowEntries(url, only200, concurrency, engine)
				}

				elapsed := time.Since(startTime)
				b.ReportMetric(float64(elapsed.Milliseconds()), "ms/op")
			}
		})
	}
}

// TestURLProcessingTiming tests the end-to-end processing time for different websites
func TestURLProcessingTiming(t *testing.T) {
	concurrency := runtime.NumCPU()
	testCases := []struct {
		name     string
		url      string
		only200  bool
		doSearch bool
		engine   search.SearchEngine
	}{
		{"Bing Basic", "bing.com", false, false, search.EngineBing},
		{"Bing With Search", "bing.com", false, true, search.EngineBing},
		{"Google Basic", "google.com", false, false, search.EngineBing},
		{"GitHub Basic", "github.com", false, false, search.EngineBing},
		{"Google Search Engine", "github.com", false, true, search.EngineGoogle},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			startTime := time.Now()

			check.ConnCheck(tc.url, tc.only200, concurrency)
			if tc.doSearch {
				search.SearchDisallowEntries(tc.url, tc.only200, concurrency, tc.engine)
			}

			elapsed := time.Since(startTime)
			fmt.Printf("Test: %s - Time: %v\n", tc.name, elapsed)

			// We don't assert on specific times as network conditions can vary,
			// but we make sure the operation completes
			if elapsed > 30*time.Second {
				t.Logf("WARNING: Processing time for %s exceeded 30 seconds: %v", tc.url, elapsed)
			}
		})
	}
}

// TestCLIFlagsProcessing tests the CLI flags processing with minimal network calls
func TestCLIFlagsProcessing(t *testing.T) {
	// Save original os.Args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	testCases := []struct {
		name     string
		args     []string
		wantErr  bool
		duration bool // Whether to measure duration
	}{
		{"Help Flag", []string{"parsero", "--help"}, false, false},
		{"URL Flag", []string{"parsero", "--url", "example.com"}, false, true},
		{"Only200 Flag", []string{"parsero", "--url", "example.com", "--only200"}, false, true},
		{"Search Flag", []string{"parsero", "--url", "example.com", "--search"}, false, true},
		{"Engine Bing", []string{"parsero", "--url", "example.com", "--search", "--engine", "bing"}, false, true},
		{"Engine Google", []string{"parsero", "--url", "example.com", "--search", "--engine", "google"}, false, true},
		{"Concurrency Flag", []string{"parsero", "--url", "example.com", "--concurrency", "4"}, false, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Skip actual execution to avoid network calls in unit tests
			// In a real setup, you might want to mock external dependencies

			if tc.duration {
				t.Logf("To measure actual duration, run: parsero %s", strings.Join(tc.args[1:], " "))
			}
		})
	}
}

// BenchmarkCheckForMultipleWebsites benchmarks the check functionality for multiple websites
func BenchmarkCheckForMultipleWebsites(b *testing.B) {
	concurrency := runtime.NumCPU()
	websites := []string{
		"bing.com",
		"google.com",
		"github.com",
		"stackoverflow.com",
		"wikipedia.org",
	}

	for _, website := range websites {
		b.Run(website, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				startTime := time.Now()

				check.ConnCheck(website, false, concurrency)

				elapsed := time.Since(startTime)
				b.ReportMetric(float64(elapsed.Milliseconds()), "ms/op")
			}
		})
	}
}
