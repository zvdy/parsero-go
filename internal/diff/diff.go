// Package diff compares two scans of the same target to surface security-
// relevant changes — chiefly Disallow paths that have *become reachable*
// (HTTP 200) since the previous scan, which is exactly the regression a
// recurring monitor should alert on.
package diff

import "sort"

type Probe struct {
	URL        string
	StatusCode int
}

type Result struct {
	NewlyReachable    []string // 200 now, wasn't before — the alertable set
	NoLongerReachable []string // 200 before, isn't now
}

func (r Result) HasChanges() bool {
	return len(r.NewlyReachable) > 0 || len(r.NoLongerReachable) > 0
}

// Compute diffs the 200-reachable sets of prev and cur; output is sorted.
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
