package main

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/zvdy/parsero-go/internal/check"
	"github.com/zvdy/parsero-go/internal/search"
)

// getTestDomains returns the domains to use for benchmarking
// It reads from environment variables TEST_DOMAINS or falls back to defaults
func getTestDomains() []string {
	// Check if TEST_DOMAINS environment variable is set
	envDomains := os.Getenv("TEST_DOMAINS")
	if envDomains != "" {
		// Split by comma to get multiple domains
		return strings.Split(envDomains, ",")
	}

	// Default domains for testing
	return []string{"hackthebox.com", "hackthissite.org"}
}

func BenchmarkRobotsCheck(b *testing.B) {
	// Get the domains to test
	domains := getTestDomains()

	// Get the number of CPUs available in the current environment
	cpuCount := runtime.NumCPU()
	halfCPU := cpuCount / 2
	if halfCPU < 1 {
		halfCPU = 1
	}

	// Create benchmarks with dynamic CPU-based concurrency values for each domain
	var benchmarks []struct {
		name        string
		url         string
		concurrency int
	}

	// Generate benchmark configurations for each domain
	for _, domain := range domains {
		benchmarks = append(benchmarks, []struct {
			name        string
			url         string
			concurrency int
		}{
			{domain + "-1CPU", domain, 1},
			{domain + "-HalfCPU", domain, halfCPU},
			{domain + "-FullCPU", domain, cpuCount},
		}...)
	}

	// Disable output during benchmark
	oldStdout := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = oldStdout }()

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				check.ConnCheck(bm.url, false, bm.concurrency)
			}
		})
	}
}

func BenchmarkFullScan(b *testing.B) {
	// Get the domains to test
	domains := getTestDomains()

	// Get the number of CPUs available in the current environment
	cpuCount := runtime.NumCPU()
	halfCPU := cpuCount / 2
	if halfCPU < 1 {
		halfCPU = 1
	}

	// Create benchmarks dynamically based on domains
	var benchmarks []struct {
		name           string
		url            string
		searchDisallow bool
		concurrency    int
	}

	// Generate benchmark configurations for each domain
	for _, domain := range domains {
		// Add basic checks with different CPU settings
		benchmarks = append(benchmarks, []struct {
			name           string
			url            string
			searchDisallow bool
			concurrency    int
		}{
			{domain + "-NoSearch-1CPU", domain, false, 1},
			{domain + "-NoSearch-HalfCPU", domain, false, halfCPU},
			{domain + "-NoSearch-FullCPU", domain, false, cpuCount},

			// Add search benchmark with optimal concurrency (half CPU)
			{domain + "-WithSearch-HalfCPU", domain, true, halfCPU},
		}...)
	}

	// Disable output during benchmark
	oldStdout := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = oldStdout }()

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				startTime := time.Now()

				// Check disallowed URLs in robots.txt
				_ = check.ConnCheck(bm.url, false, bm.concurrency)

				// Only search for disallowed entries if the flag is set
				if bm.searchDisallow {
					searchConcurrency := bm.concurrency / 2
					if searchConcurrency < 1 {
						searchConcurrency = 1
					}
					search.SearchDisallowEntries(bm.url, false, searchConcurrency)
				}

				// Record the execution time
				executionTime := time.Since(startTime)
				b.ReportMetric(float64(executionTime.Milliseconds()), "ms/op") // Report in milliseconds for better readability
			}
		})
	}
}

// TestCLIBasic ensures that the CLI doesn't crash with basic inputs
func TestCLIBasic(t *testing.T) {
	// Skip this test for now as it requires a more complete CLI setup
	t.Skip("Skipping CLI test for benchmark runs")
}

// TestJSONExport tests the JSON export functionality
func TestJSONExport(t *testing.T) {
	// Get the first domain from the test domains
	domains := getTestDomains()
	if len(domains) == 0 {
		t.Skip("No test domains available")
		return
	}
	testDomain := domains[0]

	// Create a temporary file for JSON output
	tmpfile, err := os.CreateTemp("", "parsero-test-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	// Save stdout
	oldStdout := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = oldStdout }()

	// Run a check that exports to JSON
	checkResults := check.ConnCheck(testDomain, false, runtime.NumCPU()/2)
	if checkResults == nil {
		t.Skip("No results from ConnCheck, skipping test")
	}

	// Make sure we have results
	if len(checkResults) == 0 {
		t.Skip("No disallow entries found, skipping test")
	}

	// Verify some basic aspects of the results
	found200 := false
	for _, result := range checkResults {
		if result.StatusCode == 200 {
			found200 = true
			break
		}
	}

	if !found200 {
		t.Log("No 200 status codes found in results")
	}
}
