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

// searchBing discovers indexed disallow paths via Bing and probes the ones that
// belong to target. It takes the path list as an argument (no global state) and
// returns results tagged with SourceBing. Errors are swallowed per-path so a
// flaky search engine never fails the whole scan.
func (s *Scanner) searchBing(ctx context.Context, target string, paths []string) []types.Result {
	if len(paths) == 0 {
		return nil
	}

	// Use a smaller pool for search to avoid tripping bot detection.
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

// bingQuery runs a single `site:` Bing query and probes matching cite links.
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

// probeBingHit fetches a URL discovered via Bing and reports its status.
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
