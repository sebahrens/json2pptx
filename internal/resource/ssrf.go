// Package resource provides URL resolution with SSRF protection and caching.
package resource

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// privateRanges contains CIDR blocks for private/loopback addresses.
var privateRanges []*net.IPNet

func init() {
	for _, cidr := range []string{
		"0.0.0.0/8",          // "this" network (localhost on Linux)
		"127.0.0.0/8",        // IPv4 loopback
		"10.0.0.0/8",         // RFC1918
		"172.16.0.0/12",      // RFC1918
		"192.168.0.0/16",     // RFC1918
		"169.254.0.0/16",     // link-local
		"100.64.0.0/10",      // CGN / shared address space (RFC6598)
		"::1/128",            // IPv6 loopback
		"fc00::/7",           // IPv6 unique local
		"fe80::/10",          // IPv6 link-local
	} {
		_, block, _ := net.ParseCIDR(cidr)
		privateRanges = append(privateRanges, block)
	}
}

// isPrivateIP returns true if the IP falls within a private/loopback range.
// IPv4-mapped IPv6 addresses (e.g. ::ffff:127.0.0.1) are normalized to IPv4
// before checking, preventing bypass via IPv6 encoding of private IPv4 addresses.
func isPrivateIP(ip net.IP) bool {
	// Normalize IPv4-mapped IPv6 (::ffff:x.x.x.x) to plain IPv4 so that
	// IPv4 CIDR ranges match regardless of representation.
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	for _, block := range privateRanges {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// safeDialContext returns a net.Dialer.DialContext wrapper that resolves DNS
// and rejects connections to private/loopback IP addresses.
func safeDialContext(timeout time.Duration) func(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: timeout}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid address %q: %w", addr, err)
		}

		// Resolve DNS before connecting
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("DNS lookup failed for %q: %w", host, err)
		}

		for _, ipAddr := range ips {
			if isPrivateIP(ipAddr.IP) {
				return nil, fmt.Errorf("SSRF blocked: %q resolves to private IP %s", host, ipAddr.IP)
			}
		}

		// Connect to the first resolved IP
		if len(ips) == 0 {
			return nil, fmt.Errorf("DNS lookup returned no addresses for %q", host)
		}

		target := net.JoinHostPort(ips[0].IP.String(), port)
		return dialer.DialContext(ctx, network, target)
	}
}

// newSafeHTTPClient creates an http.Client that blocks requests to private IPs.
func newSafeHTTPClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{
		DialContext: safeDialContext(timeout),
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		// Don't follow redirects to private IPs — the safe dialer checks each connection
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
}
