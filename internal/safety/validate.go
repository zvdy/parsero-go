// Package safety provides the guardrails required to fetch arbitrary
// user-supplied URLs in a multi-tenant service: target normalization, an SSRF
// deny-list, and a guarded HTTP transport that re-validates at dial time.
package safety

import (
	"fmt"
	"net"
	"strings"
)

// blockedHosts are rejected outright regardless of DNS resolution.
var blockedHosts = map[string]bool{
	"localhost":                true,
	"metadata.google.internal": true,
	"metadata":                 true,
}

// NormalizeTarget cleans a user-supplied target into a bare host: it strips any
// scheme, path, query, and port, lowercases the host, and validates it isn't an
// obviously unsafe name. It returns the normalized host or an error.
func NormalizeTarget(raw string) (string, error) {
	t := strings.TrimSpace(raw)
	if t == "" {
		return "", fmt.Errorf("empty target")
	}

	// Strip scheme.
	if i := strings.Index(t, "://"); i >= 0 {
		t = t[i+3:]
	}
	// Strip everything from the first path/query separator.
	if i := strings.IndexAny(t, "/?#"); i >= 0 {
		t = t[:i]
	}
	// Strip userinfo.
	if i := strings.LastIndex(t, "@"); i >= 0 {
		t = t[i+1:]
	}
	// Strip port (handle bracketed IPv6 too).
	if strings.HasPrefix(t, "[") {
		if i := strings.Index(t, "]"); i >= 0 {
			t = t[1:i]
		}
	} else if i := strings.LastIndex(t, ":"); i >= 0 {
		t = t[:i]
	}

	t = strings.ToLower(strings.TrimSpace(t))
	if t == "" {
		return "", fmt.Errorf("empty target after normalization")
	}

	if blockedHosts[t] {
		return "", fmt.Errorf("target %q is not allowed", t)
	}
	if strings.HasSuffix(t, ".local") || strings.HasSuffix(t, ".internal") {
		return "", fmt.Errorf("target %q resolves to an internal namespace", t)
	}

	// If it's a literal IP, validate it immediately against the deny-list.
	if ip := net.ParseIP(t); ip != nil {
		if err := checkIP(ip); err != nil {
			return "", err
		}
	}

	return t, nil
}
