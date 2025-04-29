package export_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/zvdy/parsero-go/pkg/export"
	"github.com/zvdy/parsero-go/pkg/types"
)

func TestToJSON(t *testing.T) {
	// Create a test result
	results := []types.Result{
		{
			URL:        "http://hackthissite.org/admin",
			StatusCode: 403,
			Status:     "403 Forbidden",
			Error:      nil,
		},
		{
			URL:        "http://hackthissite.org/private",
			StatusCode: 200,
			Status:     "200 OK",
			Error:      nil,
		},
	}

	// Create a scan result (with only200=false to include all results)
	scanResult := export.CreateScanResult("hackthissite.org", 1*time.Second, results, false)

	// Convert to JSON
	jsonStr, err := export.ToJSON(scanResult)
	if err != nil {
		t.Fatalf("Failed to convert to JSON: %v", err)
	}

	// Validate JSON by unmarshaling it back
	var parsedResult map[string]interface{}
	err = json.Unmarshal([]byte(jsonStr), &parsedResult)
	if err != nil {
		t.Fatalf("Generated invalid JSON: %v", err)
	}

	// Check some fields
	if parsedResult["url"] != "hackthissite.org" {
		t.Errorf("Expected URL 'hackthissite.org', got '%v'", parsedResult["url"])
	}
	if parsedResult["duration_seconds"].(float64) != 1.0 {
		t.Errorf("Expected duration 1.0, got %v", parsedResult["duration_seconds"])
	}
	if parsedResult["total_paths"].(float64) != 2 {
		t.Errorf("Expected total_paths 2, got %v", parsedResult["total_paths"])
	}
	if parsedResult["status_200"].(float64) != 1 {
		t.Errorf("Expected status_200 1, got %v", parsedResult["status_200"])
	}
	if parsedResult["other_status"].(float64) != 1 {
		t.Errorf("Expected other_status 1, got %v", parsedResult["other_status"])
	}
}

func TestSaveToFile(t *testing.T) {
	// Create a test result
	results := []types.Result{
		{
			URL:        "http://hackthissite.org/test",
			StatusCode: 200,
			Status:     "200 OK",
			Error:      nil,
		},
	}

	// Create a scan result
	scanResult := export.CreateScanResult("hackthissite.org", 1*time.Second, results, false)

	// Create a temporary file
	tempFile := os.TempDir() + "/parsero_test.json"
	err := export.SaveToFile(scanResult, tempFile)
	if err != nil {
		t.Fatalf("Failed to save to file: %v", err)
	}

	// Check if file exists
	_, err = os.Stat(tempFile)
	if os.IsNotExist(err) {
		t.Fatalf("Expected file to be created: %v", tempFile)
	}

	// Read the file back
	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read saved file: %v", err)
	}

	// Validate JSON by unmarshaling it back
	var parsedResult map[string]interface{}
	err = json.Unmarshal(content, &parsedResult)
	if err != nil {
		t.Fatalf("Generated invalid JSON in file: %v", err)
	}

	// Clean up
	os.Remove(tempFile)
}

func TestCreateScanResult(t *testing.T) {
	// Create test results with different status codes and errors
	results := []types.Result{
		{
			URL:        "http://hackthissite.org/admin",
			StatusCode: 403,
			Status:     "403 Forbidden",
			Error:      nil,
		},
		{
			URL:        "http://hackthissite.org/private",
			StatusCode: 200,
			Status:     "200 OK",
			Error:      nil,
		},
		{
			URL:        "http://hackthissite.org/error",
			StatusCode: 0,
			Status:     "",
			Error:      os.ErrNotExist,
		},
	}

	// Test with only200=false (include all results)
	scanResult := export.CreateScanResult("hackthissite.org", 2*time.Second, results, false)

	// Check values
	if scanResult.URL != "hackthissite.org" {
		t.Errorf("Expected URL 'hackthissite.org', got '%s'", scanResult.URL)
	}
	if scanResult.Duration != 2.0 {
		t.Errorf("Expected duration 2.0, got %f", scanResult.Duration)
	}
	if scanResult.TotalPaths != 3 {
		t.Errorf("Expected total_paths 3, got %d", scanResult.TotalPaths)
	}
	if scanResult.Status200 != 1 {
		t.Errorf("Expected status_200 1, got %d", scanResult.Status200)
	}
	if scanResult.OtherStatus != 1 {
		t.Errorf("Expected other_status 1, got %d", scanResult.OtherStatus)
	}
	if scanResult.Errors != 1 {
		t.Errorf("Expected errors 1, got %d", scanResult.Errors)
	}

	// Test with only200=true (only include status code 200 results)
	scanResult = export.CreateScanResult("hackthissite.org", 2*time.Second, results, true)

	// Check values with only200=true
	if scanResult.URL != "hackthissite.org" {
		t.Errorf("Expected URL 'hackthissite.org', got '%s'", scanResult.URL)
	}
	if scanResult.TotalPaths != 1 {
		t.Errorf("Expected total_paths 1, got %d", scanResult.TotalPaths)
	}
	if scanResult.Status200 != 1 {
		t.Errorf("Expected status_200 1, got %d", scanResult.Status200)
	}
	if scanResult.OtherStatus != 0 {
		t.Errorf("Expected other_status 0, got %d", scanResult.OtherStatus)
	}
	if scanResult.Errors != 0 {
		t.Errorf("Expected errors 0, got %d", scanResult.Errors)
	}

	// Check that only status 200 results are included
	if len(scanResult.Results) != 1 || scanResult.Results[0].StatusCode != 200 {
		t.Errorf("With only200=true, expected only status 200 results, got %d results", len(scanResult.Results))
	}
}
