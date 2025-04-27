package search

import (
	"fmt"
	"net/http"
	"net/url"
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
	EngineBing       SearchEngine = "bing"
	EngineDuckDuckGo SearchEngine = "duckduckgo"
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
	case "duckduckgo", "duck", "ddg":
		return EngineDuckDuckGo
	default:
		return EngineBing // Default to Bing
	}
}

// String returns the string representation of a SearchEngine
func (e SearchEngine) String() string {
	return string(e)
}

// SearchDisallowEntries searches for disallow entries using the specified search engine
func SearchDisallowEntries(url string, only200 bool, concurrency int, engine SearchEngine) []SearchResult {
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}

	fmt.Printf("\nSearching the Disallows entries using %s...\n\n", engine)

	pathlist := check.GetDisallowPaths()

	if len(pathlist) == 0 {
		fmt.Println(colors.YELLOW + "No disallow entries to search for." + colors.ENDC)
		return nil
	}

	// Channel for search tasks
	tasks := make(chan string, len(pathlist))
	results := make(chan SearchResult, len(pathlist)*5) // Estimate multiple results per path

	// Slice to collect all results
	var allResults []SearchResult

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
				case EngineDuckDuckGo:
					searchURL = "https://html.duckduckgo.com/html/?q=site:" + disurl
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
				case EngineDuckDuckGo:
					processDuckDuckGoResults(resp, url, client, results)
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

	// Display results as they come in and collect them
	for result := range results {
		if result.Error != nil {
			continue
		}

		// Add to our collection
		allResults = append(allResults, result)

		if result.StatusCode == 200 {
			fmt.Println(colors.OKGREEN + " - " + result.URL + " " + result.Status + colors.ENDC)
		} else if !only200 {
			fmt.Println(colors.FAIL + " - " + result.URL + " " + result.Status + colors.ENDC)
		}
	}

	return allResults
}

// For backward compatibility
func SearchBing(url string, only200 bool, concurrency int) []SearchResult {
	return SearchDisallowEntries(url, only200, concurrency, EngineBing)
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

// processDuckDuckGoResults processes search results from DuckDuckGo
func processDuckDuckGoResults(resp *http.Response, baseURL string, client *http.Client, results chan<- SearchResult) {
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		fmt.Println(colors.FAIL + "Error parsing DuckDuckGo search results: " + err.Error() + colors.ENDC)
		return
	}

	// Track if we found any results
	foundResults := false

	// Method 1: Look for result URLs in DuckDuckGo HTML results
	// The HTML version of DuckDuckGo uses this structure
	doc.Find("a.result__url").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && strings.Contains(href, baseURL) {
			foundResults = true
			// Make sure it's a valid URL with proper protocol
			if !strings.HasPrefix(href, "http") {
				href = "http://" + href
			}
			checkURL(href, client, results)
		}
	})

	// Method 2: Parse links from results
	doc.Find("a.result__a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			// Extract target URL from DuckDuckGo redirects
			if strings.HasPrefix(href, "/d.js?q=") {
				href = href[6:] // Remove the /d.js?q= prefix
				if idx := strings.Index(href, "&"); idx != -1 {
					href = href[:idx]
				}
				// URL decode the href
				href, _ = url.QueryUnescape(href)

				if strings.Contains(href, baseURL) {
					foundResults = true
					checkURL(href, client, results)
				}
			}
		}
	})

	// Method 3: Find URLs in the result snippets
	doc.Find("div.result__snippet").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		words := strings.Fields(text)

		for _, word := range words {
			if (strings.HasPrefix(word, "http://") || strings.HasPrefix(word, "https://")) && strings.Contains(word, baseURL) {
				foundResults = true
				checkURL(word, client, results)
			} else if strings.HasPrefix(word, baseURL) {
				foundResults = true
				checkURL("http://"+word, client, results)
			}
		}
	})

	// Method 4: Look for any other elements with URLs that might contain our base URL
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && strings.Contains(href, baseURL) {
			foundResults = true

			// Make sure it's a valid URL
			if !strings.HasPrefix(href, "http") {
				href = "http://" + href
			}

			checkURL(href, client, results)
		}
	})

	// If we didn't find any results, notify in logs
	if !foundResults {
		fmt.Println(colors.YELLOW + "No results found in DuckDuckGo search. The page structure may have changed or no matching URLs exist." + colors.ENDC)
	}
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

// SetupTestData sets up test disallow paths for testing purposes
// This allows tests to run independently without relying on external websites
func SetupTestData() []SearchResult {
	// Create mock search results
	mockResults := []SearchResult{
		{URL: "http://example.com/private", StatusCode: 200, Status: "200 OK"},
		{URL: "http://example.com/admin", StatusCode: 403, Status: "403 Forbidden"},
		{URL: "http://example.com/dashboard", StatusCode: 401, Status: "401 Unauthorized"},
		{URL: "http://example.com/login", StatusCode: 200, Status: "200 OK"},
		{URL: "http://example.com/internal", StatusCode: 404, Status: "404 Not Found"},
	}

	return mockResults
}
