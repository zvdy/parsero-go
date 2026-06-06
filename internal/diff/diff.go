// Package diff compares two scans of the same target to surface security-
// relevant changes — chiefly Disallow paths that have *become reachable*
// (HTTP 200) since the previous scan, which is exactly the regression a
// recurring monitor should alert on.
package diff

import "sort"

// Probe is the minimal view of a probed path the diff needs.
type Probe struct {
	URL        string
	StatusCode int
}

// Result holds the computed differences between a previous and current scan.
type Result struct {
	// NewlyReachable are URLs returning 200 now that were not 200 before
	// (either absent or a non-200 status previously). This is the alertable set.
	NewlyReachable []string
	// NoLongerReachable are URLs that were 200 before but are not 200 now.
	NoLongerReachable []string
}

// HasChanges reports whether anything security-relevant changed.
func (r Result) HasChanges() bool {
	return len(r.NewlyReachable) > 0 || len(r.NoLongerReachable) > 0
}

// Compute diffs the 200-reachable sets of prev and cur. URLs are de-duplicated
// and the result slices are sorted for stable output/notifications.
func Compute(prev, cur []Probe) Result {
	prevOK := reachableSet(prev)
	curOK := reachableSet(cur)

	var res Result
	for url := range curOK {
		if !prevOK[url] {
			res.NewlyReachable = append(res.NewlyReachable, url)
		}
	}
	for url := range prevOK {
		if !curOK[url] {
			res.NoLongerReachable = append(res.NoLongerReachable, url)
		}
	}
	sort.Strings(res.NewlyReachable)
	sort.Strings(res.NoLongerReachable)
	return res
}

func reachableSet(probes []Probe) map[string]bool {
	set := make(map[string]bool, len(probes))
	for _, p := range probes {
		if p.StatusCode == 200 {
			set[p.URL] = true
		}
	}
	return set
}
