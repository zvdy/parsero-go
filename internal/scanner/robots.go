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

var ErrNoRobots = fmt.Errorf("no robots.txt file has been found")

// FetchDisallowPaths returns the Disallow paths from http://{target}/robots.txt
// with the leading slash stripped, honoring ctx and the MaxPaths cap.
func (s *Scanner) FetchDisallowPaths(ctx context.Context, target string) ([]string, error) {
	if s.robotsCache != nil {
		if paths, ok := s.robotsCache.GetRobots(ctx, target); ok {
			return paths, nil
		}
	}

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

	if s.robotsCache != nil {
		s.robotsCache.SetRobots(ctx, target, paths, s.robotsTTL)
	}
	return paths, nil
}

// CheckPaths probes each path with a bounded worker pool; per-path errors are
// returned inside the results.
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
