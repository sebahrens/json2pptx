package resource

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// DefaultMaxSize is the default maximum download size (50 MB).
const DefaultMaxSize int64 = 50 * 1024 * 1024

// DefaultTimeout is the default HTTP request timeout.
const DefaultTimeout = 30 * time.Second

// ResolverOptions configures the Resolver.
type ResolverOptions struct {
	// MaxSize is the maximum download size in bytes (default: 50 MB).
	MaxSize int64

	// Timeout is the HTTP request timeout (default: 30s).
	Timeout time.Duration

	// AllowedDomains restricts downloads to these domains. Empty means all public domains.
	AllowedDomains []string

	// HTTPClient overrides the default SSRF-safe client. Used for testing.
	HTTPClient *http.Client
}

// Resolver downloads and caches remote resources (images, SVGs) with SSRF protection.
type Resolver struct {
	client  *http.Client
	cache   *cache
	opts    ResolverOptions
	allowed map[string]bool
}

// NewResolver creates a Resolver with the given options.
// Call Close() when done to clean up cached files.
func NewResolver(opts ResolverOptions) (*Resolver, error) {
	if opts.MaxSize <= 0 {
		opts.MaxSize = DefaultMaxSize
	}
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultTimeout
	}

	c, err := newCache()
	if err != nil {
		return nil, fmt.Errorf("resource resolver: create cache: %w", err)
	}

	allowed := make(map[string]bool, len(opts.AllowedDomains))
	for _, d := range opts.AllowedDomains {
		allowed[strings.ToLower(d)] = true
	}

	client := opts.HTTPClient
	if client == nil {
		client = newSafeHTTPClient(opts.Timeout)
	}

	return &Resolver{
		client:  client,
		cache:   c,
		opts:    opts,
		allowed: allowed,
	}, nil
}

// Close removes all cached files.
func (r *Resolver) Close() {
	r.cache.cleanup()
}

// IsURL returns true if s looks like an http/https URL.
func IsURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// ResolveImage downloads the URL, validates it contains image data, and returns
// the local cached path.
func (r *Resolver) ResolveImage(rawURL string) (string, error) {
	data, ext, err := r.download(rawURL)
	if err != nil {
		if p, ok := handleCached(err); ok {
			return p, nil
		}
		return "", err
	}

	// Validate image magic bytes
	if !isImageContent(data) {
		return "", fmt.Errorf("URL %q: content is not a recognized image format", rawURL)
	}

	if ext == "" {
		ext = guessImageExt(data)
	}

	filename := hashFilename(rawURL, ext)
	return r.cache.put(rawURL, filename, data)
}

// ResolveSVG downloads the URL, validates it contains SVG data, and returns
// the local cached path.
func (r *Resolver) ResolveSVG(rawURL string) (string, error) {
	data, _, err := r.download(rawURL)
	if err != nil {
		if p, ok := handleCached(err); ok {
			return p, nil
		}
		return "", err
	}

	// Validate SVG content
	trimmed := bytes.TrimSpace(data)
	if !bytes.HasPrefix(trimmed, []byte("<svg")) && !bytes.HasPrefix(trimmed, []byte("<?xml")) {
		return "", fmt.Errorf("URL %q: content is not valid SVG", rawURL)
	}

	filename := hashFilename(rawURL, ".svg")
	return r.cache.put(rawURL, filename, data)
}

// download fetches a URL with caching, size limits, and domain restrictions.
// Returns the body bytes and the file extension from the URL path.
func (r *Resolver) download(rawURL string) ([]byte, string, error) {
	// Check cache first
	if p := r.cache.get(rawURL); p != "" {
		// Already downloaded — re-read from cache
		// (caller will validate content type, but it was validated on first download)
		return nil, "", &cachedResult{path: p}
	}

	// Parse and validate URL
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, "", fmt.Errorf("URL %q: only http and https schemes are allowed", rawURL)
	}

	// Domain allowlist check
	if len(r.allowed) > 0 {
		host := strings.ToLower(u.Hostname())
		if !r.allowed[host] {
			return nil, "", fmt.Errorf("URL %q: domain %q is not in the allowed list", rawURL, host)
		}
	}

	resp, err := r.client.Get(rawURL)
	if err != nil {
		return nil, "", fmt.Errorf("URL %q: download failed: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("URL %q: server returned %d", rawURL, resp.StatusCode)
	}

	// Read with size limit
	limited := io.LimitReader(resp.Body, r.opts.MaxSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", fmt.Errorf("URL %q: read error: %w", rawURL, err)
	}
	if int64(len(data)) > r.opts.MaxSize {
		return nil, "", fmt.Errorf("URL %q: exceeds maximum size of %d bytes", rawURL, r.opts.MaxSize)
	}

	ext := strings.ToLower(path.Ext(u.Path))
	return data, ext, nil
}

// cachedResult is a sentinel error that carries the cached local path.
// When download() returns this, the caller can skip validation and use the path.
type cachedResult struct {
	path string
}

func (c *cachedResult) Error() string {
	return "cached: " + c.path
}

// ResolveImage/ResolveSVG handle cachedResult by returning the cached path.
func handleCached(err error) (string, bool) {
	if cr, ok := err.(*cachedResult); ok {
		return cr.path, true
	}
	return "", false
}

// hashFilename generates a deterministic filename from the URL.
func hashFilename(rawURL, ext string) string {
	h := sha256.Sum256([]byte(rawURL))
	if ext == "" {
		ext = ".bin"
	}
	return fmt.Sprintf("%x%s", h[:8], ext)
}

// isImageContent checks the first bytes for known image format magic bytes.
func isImageContent(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	// PNG
	if bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G'}) {
		return true
	}
	// JPEG
	if bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) {
		return true
	}
	// GIF
	if bytes.HasPrefix(data, []byte("GIF8")) {
		return true
	}
	// BMP
	if bytes.HasPrefix(data, []byte("BM")) {
		return true
	}
	// WebP
	if len(data) >= 12 && bytes.Equal(data[0:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) {
		return true
	}
	// TIFF (little-endian and big-endian)
	if bytes.HasPrefix(data, []byte{0x49, 0x49, 0x2A, 0x00}) || bytes.HasPrefix(data, []byte{0x4D, 0x4D, 0x00, 0x2A}) {
		return true
	}
	return false
}

// guessImageExt returns a file extension based on magic bytes.
func guessImageExt(data []byte) string {
	if bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G'}) {
		return ".png"
	}
	if bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) {
		return ".jpg"
	}
	if bytes.HasPrefix(data, []byte("GIF8")) {
		return ".gif"
	}
	if len(data) >= 12 && bytes.Equal(data[0:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) {
		return ".webp"
	}
	return ".bin"
}
