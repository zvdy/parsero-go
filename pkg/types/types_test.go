package types_test

import (
	"runtime"
	"testing"

	"github.com/zvdy/parsero-go/pkg/types"
)

func TestConcurrencyValues(t *testing.T) {
	cpuCount := runtime.NumCPU()

	// Check that DefaultConcurrency is set to CPU count
	if types.DefaultConcurrency != cpuCount {
		t.Errorf("DefaultConcurrency should be %d, got %d", cpuCount, types.DefaultConcurrency)
	}

	// Check that HalfDefaultConcurrency is set to half the CPU count
	expectedHalf := cpuCount / 2
	if types.HalfDefaultConcurrency != expectedHalf {
		t.Errorf("HalfDefaultConcurrency should be %d, got %d", expectedHalf, types.HalfDefaultConcurrency)
	}
}

func TestResultStruct(t *testing.T) {
	// Create a test Result instance
	result := types.Result{
		URL:        "http://hackthissite.org",
		StatusCode: 200,
		Status:     "200 OK",
		Error:      nil,
	}

	// Verify the fields
	if result.URL != "http://hackthissite.org" {
		t.Errorf("Expected URL 'http://hackthissite.org', got '%s'", result.URL)
	}
	if result.StatusCode != 200 {
		t.Errorf("Expected StatusCode 200, got %d", result.StatusCode)
	}
	if result.Status != "200 OK" {
		t.Errorf("Expected Status '200 OK', got '%s'", result.Status)
	}
	if result.Error != nil {
		t.Errorf("Expected Error nil, got %v", result.Error)
	}
}
