// Package export provides utilities for exporting parsero results in various formats
package export

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/zvdy/parsero-go/pkg/types"
)

// ScanResult represents the complete result of a parsero scan
type ScanResult struct {
	Timestamp   string         `json:"timestamp"`
	URL         string         `json:"url"`
	Duration    float64        `json:"duration_seconds"`
	Results     []types.Result `json:"results"`
	TotalPaths  int            `json:"total_paths"`
	Status200   int            `json:"status_200"`
	OtherStatus int            `json:"other_status"`
	Errors      int            `json:"errors"`
}

// ToJSON converts a ScanResult to a JSON string
func ToJSON(results ScanResult) (string, error) {
	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error marshalling JSON: %w", err)
	}

	return string(jsonData), nil
}

// SaveToFile saves a ScanResult as a JSON file
func SaveToFile(results ScanResult, filePath string) error {
	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshalling JSON: %w", err)
	}

	err = os.WriteFile(filePath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("error writing to file: %w", err)
	}

	return nil
}

// CreateScanResult creates a new ScanResult from the scan data
func CreateScanResult(url string, duration time.Duration, results []types.Result, only200 bool) ScanResult {
	// If only200 flag is set, filter results to only include 200 status codes
	var filteredResults []types.Result
	if only200 {
		for _, result := range results {
			if result.StatusCode == 200 {
				filteredResults = append(filteredResults, result)
			}
		}
	} else {
		filteredResults = results
	}

	scanResult := ScanResult{
		Timestamp:  time.Now().Format(time.RFC3339),
		URL:        url,
		Duration:   duration.Seconds(),
		Results:    filteredResults,
		TotalPaths: len(filteredResults),
	}

	// Count statuses
	for _, result := range filteredResults {
		if result.Error != nil {
			scanResult.Errors++
		} else if result.StatusCode == 200 {
			scanResult.Status200++
		} else {
			scanResult.OtherStatus++
		}
	}

	return scanResult
}
