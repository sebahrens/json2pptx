package resource

import (
	"net"
	"testing"
)

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip      string
		private bool
	}{
		{"0.0.0.1", true}, // "this" network
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"169.254.0.1", true},
		{"100.64.0.1", true},      // CGN / shared address space
		{"100.127.255.254", true}, // CGN upper bound
		{"::1", true},
		{"::ffff:127.0.0.1", true}, // IPv4-mapped IPv6
		{"::ffff:10.0.0.1", true},  // IPv4-mapped IPv6 private
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"203.0.113.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP %q", tt.ip)
			}
			got := isPrivateIP(ip)
			if got != tt.private {
				t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, got, tt.private)
			}
		})
	}
}

// TestIsPrivateIP_UnspecifiedAndEmbeddedIPv4 guards the SSRF bypasses from
// go-slide-creator-s1uvj.17: the unspecified IPv6 address "::" (connecting to
// [::]:port reaches localhost), multicast, and NAT64 / 6to4 addresses that
// embed a private IPv4 address.
func TestIsPrivateIP_UnspecifiedAndEmbeddedIPv4(t *testing.T) {
	tests := []struct {
		ip      string
		private bool
	}{
		{"::", true},
		{"0.0.0.0", true},
		{"ff02::1", true},          // link-local multicast
		{"ff01::1", true},          // interface-local multicast
		{"ff0e::1", true},          // global multicast
		{"224.0.0.1", true},        // IPv4 multicast
		{"64:ff9b::7f00:1", true},  // NAT64 of 127.0.0.1
		{"64:ff9b::a00:1", true},   // NAT64 of 10.0.0.1
		{"2002:7f00:1::", true},    // 6to4 of 127.0.0.1
		{"2002:c0a8:101::1", true}, // 6to4 of 192.168.1.1
		{"64:ff9b::808:808", false},
		{"2002:808:808::1", false},
		{"2606:4700:4700::1111", false},
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP %q", tt.ip)
			}
			if got := isPrivateIP(ip); got != tt.private {
				t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, got, tt.private)
			}
		})
	}
}
