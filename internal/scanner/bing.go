package scanner

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/zvdy/parsero-go/pkg/types"
)

// searchBing probes target paths that Bing has indexed. Per-path errors are
// swallowed so a flaky search engine never fails the whole scan.
func (s *Scanner) searchBing(ctx context.Context, target string, paths []string) []types.Result {
	if len(paths) == 0 {
		return nil
	}

	// Smaller pool than path probing to avoid tripping bot detection.
	concurrency := s.opts.Concurrency / 2
	if concurrency < 1 {
		concurrency = 1
	}

	tasks := make(chan string, len(paths))
	out := make(chan types.Result, len(paths)*5)

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range tasks {
				s.bingQuery(ctx, target, p, out)
			}
		}()
	}

	go func() {
		for _, p := range paths {
			tasks <- p
		}
		close(tasks)
	}()

	go func() {
		wg.Wait()
		close(out)
	}()

	var results []types.Result
	for r := range out {
		results = append(results, r)
	}
	return results
}

func (s *Scanner) bingQuery(ctx context.Context, target, path string, out chan<- types.Result) {
	disurl := "http://" + target + "/" + path
	searchURL := "http://www.bing.com/search?q=site:" + disurl

	// Light throttle to look less like a scraper.
	select {
	case <-ctx.Done():
		return
	case <-time.After(200 * time.Millisecond):
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return
	}
	doc.Find("cite").Each(func(i int, sel *goquery.Selection) {
		cite := sel.Text()
		if strings.Contains(cite, target) {
			out <- s.probeBingHit(ctx, cite)
		}
	})
}

func (s *Scanner) probeBingHit(ctx context.Context, url string) types.Result {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return types.Result{URL: url, Error: err, Source: SourceBing}
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return types.Result{URL: url, Error: err, Source: SourceBing}
	}
	defer resp.Body.Close()

	return types.Result{
		URL:        url,
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Source:     SourceBing,
	}
}
