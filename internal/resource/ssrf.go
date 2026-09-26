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
		"0.0.0.0/8",      // "this" network (localhost on Linux)
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"169.254.0.0/16", // link-local
		"100.64.0.0/10",  // CGN / shared address space (RFC6598)
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 unique local
		"fe80::/10",      // IPv6 link-local
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
	// Class-based checks cover addresses the CIDR list misses, notably the
	// unspecified IPv6 address "::" (dialing [::]:port reaches localhost)
	// and multicast in both families.
	if ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return true
	}
	// NAT64 (64:ff9b::/96) and 6to4 (2002::/16) embed an IPv4 address that a
	// translating gateway would forward to; block when that IPv4 is private.
	if embedded := embeddedIPv4(ip); embedded != nil {
		return isPrivateIP(embedded)
	}
	for _, block := range privateRanges {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// nat64Prefix is the well-known NAT64 prefix 64:ff9b::/96 (RFC 6052).
var nat64Prefix = net.IP{0x00, 0x64, 0xff, 0x9b, 0, 0, 0, 0, 0, 0, 0, 0}

// embeddedIPv4 returns the IPv4 address embedded in a NAT64 (64:ff9b::/96)
// or 6to4 (2002::/16) IPv6 address, or nil for any other address.
func embeddedIPv4(ip net.IP) net.IP {
	if ip.To4() != nil || len(ip) != net.IPv6len {
		return nil
	}
	if ip[0] == 0x20 && ip[1] == 0x02 {
		return net.IPv4(ip[2], ip[3], ip[4], ip[5]).To4()
	}
	if ip[:12].Equal(nat64Prefix) {
		return net.IPv4(ip[12], ip[13], ip[14], ip[15]).To4()
	}
	return nil
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
