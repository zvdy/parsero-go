package search

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/zvdy/parsero-go/internal/check"
	"github.com/zvdy/parsero-go/pkg/colors"
	"github.com/zvdy/parsero-go/pkg/types"
)

// SearchDisallowEntries searches for disallow entries using Bing
func SearchDisallowEntries(url string, only200 bool, concurrency int) []types.Result {
	if concurrency <= 0 {
		concurrency = types.HalfDefaultConcurrency
	}

	fmt.Println("Using Bing search engine...")

	pathlist := check.GetDisallowPaths()

	if len(pathlist) == 0 {
		fmt.Println(colors.YELLOW + "No disallow entries to search for." + colors.ENDC)
		return nil
	}

	// Channel for search tasks
	tasks := make(chan string, len(pathlist))
	results := make(chan types.Result, len(pathlist)*5) // Estimate multiple results per path

	// Slice to collect all results
	var allResults []types.Result

	// Launch worker pool for searches
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
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
				searchURL := "http://www.bing.com/search?q=site:" + disurl

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

				processBingResults(resp, url, client, results)
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
func SearchBing(url string, only200 bool, concurrency int) []types.Result {
	return SearchDisallowEntries(url, only200, concurrency)
}

// processBingResults processes search results from Bing
func processBingResults(resp *http.Response, baseURL string, client *http.Client, results chan<- types.Result) {
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

// checkURL checks the status of a URL and sends the result to the results channel
func checkURL(url string, client *http.Client, results chan<- types.Result) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		results <- types.Result{URL: url, Error: err}
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		results <- types.Result{URL: url, Error: err}
		return
	}
	defer resp.Body.Close()

	results <- types.Result{
		URL:        url,
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
	}
}

// SetupTestData sets up test disallow paths for testing purposes
func SetupTestData() []types.Result {
	// For testing purposes
	return nil
}
