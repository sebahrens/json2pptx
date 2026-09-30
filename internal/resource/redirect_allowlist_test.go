package resource

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

type fakeTransport func(*http.Request) (*http.Response, error)

func (f fakeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

var pngMagic = string([]byte{0x89, 'P', 'N', 'G', 13, 10, 26, 10})

// redirectingClient builds the production safe client, swapping only its
// transport for a deterministic fake: requests to hosts in redirects answer
// 302 to the mapped Location, everything else serves PNG bytes. It records
// every host whose request reached the transport.
func redirectingClient(redirects map[string]string) (*http.Client, func() []string) {
	var mu sync.Mutex
	var seen []string
	client := newSafeHTTPClient(DefaultTimeout)
	client.Transport = fakeTransport(func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		seen = append(seen, req.URL.Hostname())
		mu.Unlock()
		h := make(http.Header)
		if loc, ok := redirects[req.URL.Hostname()]; ok {
			h.Set("Location", loc)
			return &http.Response{StatusCode: http.StatusFound, Header: h, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Header: h, Body: io.NopCloser(strings.NewReader(pngMagic)), Request: req}, nil
	})
	return client, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), seen...)
	}
}

// TestRedirectToUnapprovedDomainBlocked is the go-slide-creator-b7qqg.12
// regression: an allowed host redirecting to an unapproved public host must
// be rejected before the unapproved host is ever requested.
func TestRedirectToUnapprovedDomainBlocked(t *testing.T) {
	client, seen := redirectingClient(map[string]string{
		"allowed.example": "https://unapproved.example/photo.png",
	})
	r, err := NewResolver(ResolverOptions{AllowedDomains: []string{"allowed.example"}, HTTPClient: client})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	p, err := r.ResolveImage("https://allowed.example/photo.png")
	if err == nil {
		t.Fatalf("expected redirect to unapproved domain to fail, got path %q", p)
	}
	if !strings.Contains(err.Error(), "not in the allowed list") {
		t.Errorf("error should name the allow-list, got %v", err)
	}
	for _, h := range seen() {
		if h == "unapproved.example" {
			t.Fatalf("unapproved host was fetched: %v", seen())
		}
	}
}

func TestRedirectToApprovedDomainAllowed(t *testing.T) {
	client, _ := redirectingClient(map[string]string{
		"allowed.example": "https://cdn.allowed.example/photo.png",
	})
	r, err := NewResolver(ResolverOptions{
		AllowedDomains: []string{"allowed.example", "CDN.allowed.example"},
		HTTPClient:     client,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	if _, err := r.ResolveImage("https://allowed.example/photo.png"); err != nil {
		t.Fatalf("approved redirect should succeed: %v", err)
	}
}

// The wrapper must keep the underlying client's bounded-redirect policy.
func TestRedirectAllowListKeepsRedirectLimit(t *testing.T) {
	client, _ := redirectingClient(map[string]string{
		"allowed.example": "https://allowed.example/loop.png",
	})
	r, err := NewResolver(ResolverOptions{AllowedDomains: []string{"allowed.example"}, HTTPClient: client})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	_, err = r.ResolveImage("https://allowed.example/photo.png")
	if err == nil || !strings.Contains(err.Error(), "too many redirects") {
		t.Fatalf("expected redirect-limit error, got %v", err)
	}
}

// The safe dialer remains in force for approved redirect targets: the
// wrapper copies the client, it does not replace the transport.
func TestRedirectAllowListKeepsSafeTransport(t *testing.T) {
	base := newSafeHTTPClient(DefaultTimeout)
	wrapped := withRedirectAllowList(base, map[string]bool{"x.example": true})
	if wrapped.Transport != base.Transport {
		t.Fatal("wrapper must reuse the SSRF-safe transport")
	}
	if base.CheckRedirect == nil {
		t.Fatal("base client lost its CheckRedirect")
	}
}
