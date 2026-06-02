package scanner

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/zvdy/parsero-go/pkg/types"
)

// ErrNoRobots is returned by FetchDisallowPaths when robots.txt can't be fetched.
var ErrNoRobots = fmt.Errorf("no robots.txt file has been found")

// FetchDisallowPaths fetches http://{target}/robots.txt and returns the list of
// Disallow paths (with the leading slash stripped, matching the original tool).
// The list is returned rather than stored in a global, so concurrent scans never
// interfere. Honors ctx and the MaxPaths cap.
func (s *Scanner) FetchDisallowPaths(ctx context.Context, target string) ([]string, error) {
	url := "http://" + target + "/robots.txt"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 Parsero/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, ErrNoRobots
	}
	defer resp.Body.Close()

	var paths []string
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "Disallow: /") {
			path := strings.TrimSpace(strings.TrimPrefix(line, "Disallow: /"))
			paths = append(paths, path)
			if s.opts.MaxPaths > 0 && len(paths) >= s.opts.MaxPaths {
				break
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return paths, nil
}

// CheckPaths probes each disallow path on target using a bounded worker pool,
// trying HEAD first and falling back to GET. It never prints; per-path errors
// are returned inside the results. Results are tagged with SourceRobots.
func (s *Scanner) CheckPaths(ctx context.Context, target string, paths []string) []types.Result {
	if len(paths) == 0 {
		return nil
	}

	work := make(chan string, len(paths))
	out := make(chan types.Result, len(paths))

	var wg sync.WaitGroup
	for i := 0; i < s.opts.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range work {
				out <- s.probe(ctx, target, p)
			}
		}()
	}

	go func() {
		for _, p := range paths {
			work <- p
		}
		close(work)
	}()

	go func() {
		wg.Wait()
		close(out)
	}()

	results := make([]types.Result, 0, len(paths))
	done := 0
	for r := range out {
		results = append(results, r)
		done++
		if s.progress != nil {
			s.progress(done, len(paths))
		}
	}
	return results
}

// probe issues a HEAD (falling back to GET) for a single disallow path.
func (s *Scanner) probe(ctx context.Context, target, path string) types.Result {
	disurl := "http://" + target + "/" + path

	reqCtx := ctx
	if s.opts.RequestTimeout > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, s.opts.RequestTimeout)
		defer cancel()
	}

	doReq := func(method string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(reqCtx, method, disurl, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 Parsero/1.0")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9")
		return s.client.Do(req)
	}

	resp, err := doReq(http.MethodHead)
	if err != nil {
		// HEAD can be rejected by some servers; fall back to GET.
		resp, err = doReq(http.MethodGet)
		if err != nil {
			return types.Result{URL: disurl, Error: err, Source: SourceRobots}
		}
	}
	defer resp.Body.Close()

	return types.Result{
		URL:        disurl,
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Source:     SourceRobots,
	}
}
