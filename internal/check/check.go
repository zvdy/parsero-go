package check

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zvdy/parsero-go/pkg/colors"
	"github.com/zvdy/parsero-go/pkg/types"
)

var pathlist []string

func ConnCheck(url string, only200 bool, concurrency int) []types.Result {
	// Reset pathlist for this run
	pathlist = []string{}

	// Collection for results
	var allResults []types.Result

	// Use a default concurrency if not specified
	if concurrency <= 0 {
		concurrency = types.DefaultConcurrency
	}

	// Set up an optimized client for robots.txt
	robotsClient := &http.Client{
		Timeout: 5 * time.Second, // Shorter timeout for robots.txt
	}

	resp, err := robotsClient.Get("http://" + url + "/robots.txt")
	if err != nil {
		fmt.Println(colors.FAIL + "No robots.txt file has been found." + colors.ENDC)
		return nil
	}
	defer resp.Body.Close()

	// Scan the robots.txt file efficiently
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Disallow: /") {
			path := strings.TrimPrefix(line, "Disallow: /")
			pathlist = append(pathlist, strings.TrimSpace(path))
		}
	}

	if len(pathlist) == 0 {
		fmt.Println(colors.YELLOW + "No Disallow entries found in robots.txt." + colors.ENDC)
		return nil
	}

	fmt.Printf("Found %d Disallow entries. Processing with %d workers...\n", len(pathlist), concurrency)

	// Create a channel for paths to process
	paths := make(chan string, len(pathlist))
	results := make(chan types.Result, len(pathlist))

	// Create an optimized HTTP transport for high-concurrency
	transport := &http.Transport{
		MaxIdleConnsPerHost: concurrency * 2, // Double the number of idle connections
		MaxConnsPerHost:     concurrency * 2, // Double the number of connections
		IdleConnTimeout:     30 * time.Second,
		DisableKeepAlives:   false, // Keep connections alive for reuse
		TLSHandshakeTimeout: 5 * time.Second,
	}

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{
				Timeout:   3 * time.Second, // Reduce timeout to 3 seconds
				Transport: transport,
			}

			for p := range paths {
				disurl := "http://" + url + "/" + p
				req, err := http.NewRequest("GET", disurl, nil)
				if err != nil {
					results <- types.Result{URL: disurl, Error: err}
					continue
				}

				// Set user agent to avoid being blocked
				req.Header.Set("User-Agent", "Mozilla/5.0 Parsero/1.0")
				// Add accept header to reduce response size
				req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9")
				// Only fetch the head if possible
				req.Method = "HEAD"

				resp, err := client.Do(req)
				if err != nil {
					// If HEAD fails, try GET
					if req.Method == "HEAD" {
						req.Method = "GET"
						resp, err = client.Do(req)
						if err != nil {
							results <- types.Result{URL: disurl, Error: err}
							continue
						}
					} else {
						results <- types.Result{URL: disurl, Error: err}
						continue
					}
				}

				results <- types.Result{
					URL:        disurl,
					StatusCode: resp.StatusCode,
					Status:     resp.Status,
				}
				resp.Body.Close() // Ensure we close the body
			}
		}()
	}

	// Fill the paths channel
	go func() {
		for _, p := range pathlist {
			paths <- p
		}
		close(paths)
	}()

	// Collect and process results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Process results as they come in
	for result := range results {
		// Add to our collection
		allResults = append(allResults, result)

		if result.Error != nil {
			// Skip errors silently - same behavior as before
			continue
		}

		if result.StatusCode == 200 {
			fmt.Println(colors.OKGREEN + result.URL + " " + result.Status + colors.ENDC)
		} else if !only200 {
			fmt.Println(colors.FAIL + result.URL + " " + result.Status + colors.ENDC)
		}
	}

	return allResults
}

// GetDisallowPaths returns the list of disallow paths found
// This is useful for the search function to avoid re-parsing robots.txt
func GetDisallowPaths() []string {
	return pathlist
}

func PrintDate(url string) {
	fmt.Println("Starting Parsero v1.0.0 (https://github.com/zvdy/parsero-go) at " + time.Now().Format("01/02/2006 15:04:05"))
	fmt.Println("Parsero scan report for " + url)
}
