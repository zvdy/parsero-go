// internal/search/search.go
package search

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/zvdy/parsero-go/internal/check"
	"github.com/zvdy/parsero-go/pkg/colors"
)

// DefaultConcurrency is the default number of concurrent workers
// It uses the number of available CPU cores
var DefaultConcurrency = runtime.NumCPU()

// SearchEngine represents a search engine type
type SearchEngine string

// Available search engines
const (
	EngineBing   SearchEngine = "bing"
	EngineGoogle SearchEngine = "google"
	// TODO: Add more search engines as needed
)

// SearchResult represents the result of a search query
type SearchResult struct {
	URL        string
	StatusCode int
	Status     string
	Error      error
}

// ParseSearchEngine converts a string to a SearchEngine type
func ParseSearchEngine(engine string) SearchEngine {
	switch strings.ToLower(engine) {
	case "bing":
		return EngineBing
	case "google":
		return EngineGoogle
	default:
		return EngineBing // Default to Bing
	}
}

// String returns the string representation of a SearchEngine
func (e SearchEngine) String() string {
	return string(e)
}

// SearchDisallowEntries searches for disallow entries using the specified search engine
func SearchDisallowEntries(url string, only200 bool, concurrency int, engine SearchEngine) {
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}

	fmt.Printf("\nSearching the Disallows entries using %s...\n\n", engine)

	pathlist := check.GetDisallowPaths()

	if len(pathlist) == 0 {
		fmt.Println(colors.YELLOW + "No disallow entries to search for." + colors.ENDC)
		return
	}

	// Channel for search tasks
	tasks := make(chan string, len(pathlist))
	results := make(chan SearchResult, len(pathlist)*5) // Estimate multiple results per path

	// Launch worker pool for searches
	var wg sync.WaitGroup
	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()

			client := &http.Client{
				Timeout: 15 * time.Second,
				Transport: &http.Transport{
					MaxIdleConnsPerHost: concurrency,
					DisableKeepAlives:   false,
				},
			}

			for p := range tasks {
				disurl := "http://" + url + "/" + p

				// Create search URL based on the selected engine
				var searchURL string
				switch engine {
				case EngineBing:
					searchURL = "http://www.bing.com/search?q=site:" + disurl
				case EngineGoogle:
					searchURL = "https://www.google.com/search?q=site:" + disurl
				default:
					// Default to Bing if unknown engine provided
					searchURL = "http://www.bing.com/search?q=site:" + disurl
				}

				// Throttle a bit to avoid being detected as a bot
				time.Sleep(200 * time.Millisecond)

				req, err := http.NewRequest("GET", searchURL, nil)
				if err != nil {
					continue
				}

				req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
				resp, err := client.Do(req)
				if err != nil {
					continue
				}

				// Process the search results based on the engine
				switch engine {
				case EngineBing:
					processBingResults(resp, url, client, results)
				case EngineGoogle:
					processGoogleResults(resp, url, client, results)
				default:
					processBingResults(resp, url, client, results)
				}
			}
		}()
	}

	// Fill the tasks channel
	go func() {
		for _, p := range pathlist {
			tasks <- p
		}
		close(tasks)
	}()

	// Process results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Display results as they come in
	for result := range results {
		if result.Error != nil {
			continue
		}

		if result.StatusCode == 200 {
			fmt.Println(colors.OKGREEN + " - " + result.URL + " " + result.Status + colors.ENDC)
		} else if !only200 {
			fmt.Println(colors.FAIL + " - " + result.URL + " " + result.Status + colors.ENDC)
		}
	}
}

// For backward compatibility
func SearchBing(url string, only200 bool, concurrency int) {
	SearchDisallowEntries(url, only200, concurrency, EngineBing)
}

// processBingResults processes search results from Bing
func processBingResults(resp *http.Response, baseURL string, client *http.Client, results chan<- SearchResult) {
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return
	}

	// Process each search result
	doc.Find("cite").Each(func(i int, s *goquery.Selection) {
		citeText := s.Text()
		if strings.Contains(citeText, baseURL) {
			// Check the status of this URL
			checkURL(citeText, client, results)
		}
	})
}

// processGoogleResults processes search results from Google
func processGoogleResults(resp *http.Response, baseURL string, client *http.Client, results chan<- SearchResult) {
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return
	}

	// Google formats results differently than Bing
	// Look for URLs in the Google search results
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && strings.Contains(href, "/url?q=") {
			// Extract actual URL from Google's redirect URL
			actualURL := strings.Split(href, "/url?q=")[1]
			if idx := strings.Index(actualURL, "&"); idx != -1 {
				actualURL = actualURL[:idx]
			}

			// Only process URLs from the target domain
			if strings.Contains(actualURL, baseURL) {
				checkURL(actualURL, client, results)
			}
		}
	})
}

// checkURL checks the status of a URL and sends the result to the results channel
func checkURL(url string, client *http.Client, results chan<- SearchResult) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		results <- SearchResult{URL: url, Error: err}
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		results <- SearchResult{URL: url, Error: err}
		return
	}
	defer resp.Body.Close()

	results <- SearchResult{
		URL:        url,
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
	}
}
