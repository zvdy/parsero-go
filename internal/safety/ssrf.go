package safety

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// privateBlocks are CIDR ranges we refuse to connect to, to prevent the scanner
// from being used to reach internal infrastructure (SSRF).
var privateBlocks = func() []*net.IPNet {
	cidrs := []string{
		"10.0.0.0/8",      // RFC1918
		"172.16.0.0/12",   // RFC1918
		"192.168.0.0/16",  // RFC1918
		"127.0.0.0/8",     // loopback
		"169.254.0.0/16",  // link-local (includes cloud metadata 169.254.169.254)
		"0.0.0.0/8",       // "this" network / unspecified
		"100.64.0.0/10",   // carrier-grade NAT
		"192.0.0.0/24",    // IETF protocol assignments
		"192.0.2.0/24",    // TEST-NET-1
		"198.18.0.0/15",   // benchmarking
		"198.51.100.0/24", // TEST-NET-2
		"203.0.113.0/24",  // TEST-NET-3
		"240.0.0.0/4",     // reserved
		"::1/128",         // IPv6 loopback
		"fc00::/7",        // IPv6 unique-local
		"fe80::/10",       // IPv6 link-local
		"::/128",          // IPv6 unspecified
		"ff00::/8",        // IPv6 multicast
	}
	var nets []*net.IPNet
	for _, c := range cidrs {
		_, n, err := net.ParseCIDR(c)
		if err == nil {
			nets = append(nets, n)
		}
	}
	return nets
}()

func checkIP(ip net.IP) error {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return fmt.Errorf("address %s is in a disallowed range", ip)
	}
	for _, b := range privateBlocks {
		if b.Contains(ip) {
			return fmt.Errorf("address %s is in a disallowed range", ip)
		}
	}
	return nil
}

// ResolveAndCheck resolves host and returns an error if any resolved address is
// in a blocked range. A host that resolves to a mix of public and private
// addresses is rejected (fail closed).
func ResolveAndCheck(ctx context.Context, host string) error {
	if ip := net.ParseIP(host); ip != nil {
		return checkIP(ip)
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("cannot resolve %q: %w", host, err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("no addresses for %q", host)
	}
	for _, a := range addrs {
		if err := checkIP(a.IP); err != nil {
			return err
		}
	}
	return nil
}

// GuardedTransport returns an *http.Transport whose DialContext re-resolves and
// re-validates the host at connect time, then dials the validated IP directly.
// This closes the DNS-rebinding TOCTOU gap: validating at request time alone is
// insufficient because DNS can change between check and connect.
func GuardedTransport() *http.Transport {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}

	return &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 16,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("cannot resolve %q: %w", host, err)
			}
			// Validate every candidate and dial the first that passes.
			var lastErr error
			for _, ip := range ips {
				if err := checkIP(ip.IP); err != nil {
					lastErr = err
					continue
				}
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
				if err != nil {
					lastErr = err
					continue
				}
				return conn, nil
			}
			if lastErr == nil {
				lastErr = fmt.Errorf("no dialable address for %q", host)
			}
			return nil, lastErr
		},
	}
}

// GuardedClient builds an http.Client using GuardedTransport with the given
// overall timeout and a CheckRedirect that re-validates each redirect hop's
// host, so a redirect can't be used to bounce into a private range.
func GuardedClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: GuardedTransport(),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return ResolveAndCheck(req.Context(), req.URL.Hostname())
		},
	}
}
