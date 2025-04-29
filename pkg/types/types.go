// Package types contains shared type definitions and constants used across the application
package types

import "runtime"

// DefaultConcurrency is the default number of concurrent workers
// It uses the number of available CPU cores
var DefaultConcurrency = runtime.NumCPU()

// HalfDefaultConcurrency is half the number of CPU cores, used for search operations
// to prevent rate limiting
var HalfDefaultConcurrency = runtime.NumCPU() / 2

// Result represents the result of a URL check
type Result struct {
	URL        string
	StatusCode int
	Status     string
	Error      error
}
