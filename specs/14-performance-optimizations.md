# Performance Optimizations

Analysis of the Go Slide Creator codebase for performance bottlenecks and optimization opportunities.

## Implementation Status

| Issue | Status | Notes |
|-------|--------|-------|
| C1: Multiple ZIP Passes | ✅ **Implemented** | `singlepass.go` - Single-pass ZIP generation |
| C2: Unbounded Image Loading | ✅ **Implemented** | `singlepass.go:1413-1430` - Uses `io.Copy` streaming |
| C3: Rate Limiter Memory Leak | ✅ **Implemented** | `ratelimit.go:22-32` - Fixed-size ring buffer |
| H1: Template Hash Every Request | ✅ **Implemented** | `cache.go:195-207` - `SetWithModTime` fast validation |
| H2: Regex in Function | ✅ **Implemented** | All regexes are package-level `regexp.MustCompile` |
| H3: Unbounded Cache | ✅ **Implemented** | `cache.go:62-96` - LRU eviction with `maxSize` |
| H4: Chart Temp Files | ✅ **Implemented** | `svg.go:212,285` - `defer os.Remove` cleanup |
| H5: Full File Read Download | ✅ **Implemented** | `download.go:85-96` - Uses `http.ServeContent` |
| M1: Semaphore for Chart Rendering | ✅ **Implemented** | `singlepass.go` - Inline chart rendering during single-pass generation |
| M2: sync.Pool for Buffers | ❌ Not Implemented | Low priority |
| M3: Streaming JSON Parse | ❌ Not Implemented | Low priority |
| M4: LLM Client Goroutine Leak | ⚠️ Partial | Circuit breaker exists; Close() method planned |
| M5: HTTP Transport Settings | ❌ Not Implemented | Uses Go default transport |
| L1-L3: Various | ❌ Not Implemented | Backlog items |
| Prometheus Metrics | ❌ Not Implemented | Planned for future release |
| pprof Endpoints | 📋 Planned | Will be added on protected/separate port |
| LLM Client Close() | 📋 Planned | Implement proper shutdown for graceful termination |

## Executive Summary

This specification documents performance issues identified in the Go Slide Creator service across five key areas: memory usage, CPU performance, I/O operations, concurrency, and API response time. Issues are prioritized by impact and implementation effort.

## Scope

This analysis covers:
- Memory allocation patterns and potential leaks
- CPU-intensive operations and their optimization
- I/O bottlenecks in file and network operations
- Concurrency issues with goroutines and locks
- API response time improvements

---

## Critical Priority Issues

### C1: Multiple ZIP Read/Write Passes in Generation Pipeline

**Location**: `internal/generator/slides.go:176-319`, `internal/generator/text.go:22-124`, `internal/generator/images.go:25-251`

**Current Behavior**:
The PPTX generation pipeline performs three separate ZIP read/write cycles:
1. `addSlidesToPresentation()` - Reads ZIP, copies all files, adds slides, writes new ZIP
2. `populateTextContent()` - Reads entire ZIP again, modifies slides, writes new ZIP
3. `populateImageContent()` - Reads entire ZIP again, modifies slides, writes new ZIP

For a template with 20 slide layouts and 10 slides, this means:
- 3x full ZIP decompression
- 3x copying of all unchanged files
- 3x ZIP recompression
- 3x temporary file creation and rename operations

**Impact**:
- O(3n) file I/O where O(n) is sufficient
- Memory usage spikes from holding full ZIP contents multiple times
- Estimated 60-70% of generation time spent on redundant I/O

**Recommended Fix**:
```go
// Consolidate into single pass
func Generate(req GenerationRequest) (*GenerationResult, error) {
    // Open source ZIP once
    r, err := zip.OpenReader(req.TemplatePath)
    if err != nil {
        return nil, err
    }
    defer r.Close()

    // Create output ZIP once
    tmpPath := req.OutputPath + ".tmp"
    tmpFile, err := os.Create(tmpPath)
    if err != nil {
        return nil, err
    }
    defer os.Remove(tmpPath)

    w := zip.NewWriter(tmpFile)

    // Build slide specifications with all modifications
    slideModifications := prepareAllSlideModifications(req.Slides)

    // Single pass: copy unmodified files, write modified slides
    for _, f := range r.File {
        if isSlideFile(f.Name) {
            // Apply all modifications (text, images) in one pass
            writeModifiedSlide(w, f, slideModifications)
        } else if needsModification(f.Name) {
            writeModifiedFile(w, f, modifications)
        } else {
            copyZipFile(w, f) // Unchanged
        }
    }

    // Add new slides and media files
    addNewSlides(w, req.Slides)
    addMediaFiles(w, mediaFiles)

    w.Close()
    tmpFile.Close()
    os.Rename(tmpPath, req.OutputPath)
}
```

**Expected Improvement**: 50-70% reduction in generation time for typical presentations.

---

### C2: Unbounded Image Loading in Memory

**Location**: `internal/generator/images.go:216-234`

**Current Behavior**:
```go
// Line 216-217
imageData, err := os.ReadFile(imagePath)
// Entire image loaded into memory
```

Images are fully loaded into memory via `os.ReadFile()` before being written to the ZIP. For presentations with multiple large images (e.g., 10MB each), this can cause:
- Memory spikes of 100MB+ for image-heavy presentations
- OOM kills under high concurrency

**Impact**: Memory exhaustion under load with large images.

**Recommended Fix**:
```go
// Stream images directly to ZIP
func addImageToZip(w *zip.Writer, imagePath, zipPath string) error {
    src, err := os.Open(imagePath)
    if err != nil {
        return err
    }
    defer src.Close()

    dst, err := w.Create(zipPath)
    if err != nil {
        return err
    }

    // Stream in 32KB chunks
    _, err = io.Copy(dst, src)
    return err
}
```

**Expected Improvement**: 80-90% reduction in peak memory for image operations.

---

### C3: Rate Limiter Memory Leak

**Location**: `internal/api/ratelimit.go:40-107`

**Current Behavior**:
```go
// Line 63-68: Creates new slice on every request
validRequests := make([]time.Time, 0)
for _, reqTime := range v.requests {
    if reqTime.After(cutoff) {
        validRequests = append(validRequests, reqTime)
    }
}
v.requests = validRequests
```

The rate limiter:
1. Creates a new slice on every `Allow()` call
2. Cleanup runs every minute but uses 2x window threshold
3. Under high traffic, old visitors accumulate until cleanup
4. Slice reallocation causes memory fragmentation

**Impact**: Memory grows linearly with unique IPs; GC pressure increases over time.

**Recommended Fix**:
```go
// Use ring buffer for requests
type visitor struct {
    requests [20]time.Time  // Fixed-size ring buffer for max 20 requests/minute
    head     int
    count    int
    lastSeen time.Time
}

func (rl *RateLimiter) Allow(ip string) (bool, int, time.Time) {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    now := time.Now()
    v, exists := rl.visitors[ip]
    if !exists {
        v = &visitor{lastSeen: now}
        rl.visitors[ip] = v
    }

    v.lastSeen = now

    // Count valid requests in ring buffer (no allocation)
    cutoff := now.Add(-rl.window)
    validCount := 0
    for i := 0; i < v.count; i++ {
        idx := (v.head + i) % len(v.requests)
        if v.requests[idx].After(cutoff) {
            validCount++
        }
    }

    if validCount >= rl.limit {
        return false, 0, v.requests[v.head].Add(rl.window)
    }

    // Add new request to ring buffer
    v.requests[(v.head+v.count)%len(v.requests)] = now
    if v.count < len(v.requests) {
        v.count++
    } else {
        v.head = (v.head + 1) % len(v.requests)
    }

    return true, rl.limit - validCount - 1, now.Add(rl.window)
}
```

**Expected Improvement**: Constant memory per visitor; eliminates slice allocations.

---

## High Priority Issues

### H1: Synchronous Template Analysis on Every Request

**Location**: `internal/api/convert.go:125-132`

**Current Behavior**:
```go
// Line 126-127
templateService := NewTemplateService(cs.templatesDir, cs.cache)
templateAnalysis, err := templateService.getOrAnalyzeTemplate(templatePath)
```

Template analysis involves:
1. Opening ZIP file and calculating SHA256 hash
2. Parsing multiple XML files for layouts
3. Parsing theme XML
4. Layout classification

Even with caching, the hash calculation requires reading the entire template file on every request to check cache validity.

**Impact**: 50-100ms added latency per request for hash calculation on large templates.

**Recommended Fix**:
```go
// Use file modification time for fast invalidation check
type cacheEntry struct {
    analysis  *types.TemplateAnalysis
    modTime   time.Time  // Check this first (fast)
    hash      string     // Only recalculate if modTime changed
    expiresAt time.Time
}

func (c *MemoryCache) IsValid(path string) (bool, error) {
    c.mu.RLock()
    entry, exists := c.entries[path]
    c.mu.RUnlock()

    if !exists || time.Now().After(entry.expiresAt) {
        return false, nil
    }

    // Fast check: file modification time
    info, err := os.Stat(path)
    if err != nil {
        return false, err
    }

    if info.ModTime().Equal(entry.modTime) {
        return true, nil  // No hash calculation needed
    }

    // Slow path: calculate hash only if modTime changed
    hash, err := calculateFileHash(path)
    if err != nil {
        return false, err
    }

    return hash == entry.hash, nil
}
```

**Expected Improvement**: 90% reduction in cache validation overhead.

---

### H2: Regex Compilation on Every Parse Call

**Location**: `internal/parser/slides.go:17-20`, `internal/parser/content.go:19-26`

**Current Behavior**:
```go
// Package-level vars (compiled once - good)
var typeCommentRegex = regexp.MustCompile(`<!--\s*type:\s*(\w+(?:-\w+)?)\s*-->`)

// But also in generator/slides.go:173 (compiled per Generate call)
var slideFileRegex = regexp.MustCompile(`ppt/slides/slide\d+\.xml`)
```

While most regexes are package-level, `slideFileRegex` in `slides.go:173` is inside a function and compiled on every call.

**Impact**: Minor CPU overhead per generation (regex compilation is expensive).

**Recommended Fix**:
```go
// Move to package level
var slideFileRegex = regexp.MustCompile(`ppt/slides/slide\d+\.xml`)

func addSlidesToPresentation(pptxPath string, slides []SlideSpec) error {
    // Use package-level slideFileRegex
}
```

**Expected Improvement**: ~5ms saved per generation call.

---

### H3: Template Cache Has No Size Limit

**Location**: `internal/template/cache.go:17-35`

**Current Behavior**:
```go
type MemoryCache struct {
    mu      sync.RWMutex
    entries map[string]*cacheEntry
    ttl     time.Duration
    // No max size
}
```

The cache grows unbounded with unique templates. In a multi-tenant scenario, memory could grow indefinitely.

**Impact**: Potential OOM in long-running servers with many unique templates.

**Recommended Fix**:
```go
type MemoryCache struct {
    mu       sync.RWMutex
    entries  map[string]*cacheEntry
    ttl      time.Duration
    maxSize  int
    lruList  *list.List  // For LRU eviction
    lruIndex map[string]*list.Element
}

func (c *MemoryCache) Set(path string, analysis *types.TemplateAnalysis) {
    c.mu.Lock()
    defer c.mu.Unlock()

    // Evict if at capacity
    for len(c.entries) >= c.maxSize {
        oldest := c.lruList.Back()
        if oldest == nil {
            break
        }
        key := oldest.Value.(string)
        delete(c.entries, key)
        delete(c.lruIndex, key)
        c.lruList.Remove(oldest)
    }

    c.entries[path] = &cacheEntry{...}
    elem := c.lruList.PushFront(path)
    c.lruIndex[path] = elem
}
```

**Expected Improvement**: Bounded memory usage with configurable cache size.

---

### H4: Chart Rendering Creates Temporary Files Unnecessarily

**Location**: `internal/generator/images.go:327-345`

**Current Behavior**:
```go
// Line 330-344: Chart bytes written to temp file, then read back
tmpFile, err := os.CreateTemp("", "chart-*.png")
// ...
if _, err := tmpFile.Write(imgBytes); err != nil { ... }
tmpFile.Close()
imagePath = tmpFile.Name()
```

Charts are rendered to `[]byte`, written to a temp file, then the file is read back for ZIP embedding.

**Impact**: Unnecessary disk I/O for every chart; temp file cleanup relies on `defer os.Remove`.

**Recommended Fix**:
```go
// Handle byte slices directly
type ImageSource interface {
    WriteTo(w io.Writer) error
}

type FileImageSource struct {
    Path string
}

type ByteImageSource struct {
    Data []byte
}

func addImageToZip(w *zip.Writer, src ImageSource, zipPath string) error {
    dst, err := w.Create(zipPath)
    if err != nil {
        return err
    }
    return src.WriteTo(dst)
}
```

**Expected Improvement**: Eliminates disk I/O for chart embedding.

---

### H5: Full File Read for Download Service

**Location**: `internal/api/download.go:85-97`

**Current Behavior**:
```go
// Line 86: Entire file loaded into memory
data, err := os.ReadFile(filePath)
// Line 94: Then written to response
if _, err := w.Write(data); err != nil { ... }
```

Generated PPTX files (potentially 10-50MB) are fully loaded before serving.

**Impact**: Memory spike proportional to file size on every download.

**Recommended Fix**:
```go
func (ds *DownloadService) DownloadHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // ... validation ...

        file, err := os.Open(filePath)
        if err != nil {
            writeError(w, http.StatusInternalServerError, ...)
            return
        }
        defer file.Close()

        // Set headers
        w.Header().Set("Content-Type",
            "application/vnd.openxmlformats-officedocument.presentationml.presentation")
        w.Header().Set("Content-Disposition", `attachment; filename="presentation.pptx"`)
        w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))

        // Stream file to response
        http.ServeContent(w, r, "presentation.pptx", fileInfo.ModTime(), file)
    }
}
```

**Expected Improvement**: Constant memory usage regardless of file size; supports HTTP range requests.

---

## Medium Priority Issues

### M1: XML Marshaling/Unmarshaling Inefficiency

**Location**: `internal/generator/slides.go:353-381`, `internal/template/layouts.go:42-51`

**Current Behavior**:
```go
// Full XML document unmarshaled into structs
var slide slideXML
if err := xml.Unmarshal(slideData, &slide); err != nil { ... }

// Full document marshaled back
modifiedData, err := xml.MarshalIndent(slide, "", "  ")
```

The standard library `encoding/xml` is used for all XML operations. This involves:
- Full document parsing into intermediate structures
- Reflection-based field mapping
- Pretty-printing with `MarshalIndent`

**Impact**:
- CPU overhead from reflection
- Memory allocation for intermediate structures
- `MarshalIndent` is slower than `Marshal`

**Recommended Fix**:
```go
// Use Marshal (not MarshalIndent) for generated content
modifiedData, err := xml.Marshal(slide)  // 20-30% faster

// For targeted modifications, consider streaming XML
// using xml.Decoder/Encoder for large documents

// For hot paths, consider code-generated marshalers
// using tools like github.com/mailru/easyjson equivalent for XML
```

**Expected Improvement**: 20-30% reduction in XML processing time.

---

### M2: LLM Client Rate Limiter Goroutine Never Cleaned Up

**Location**: `internal/llm/client.go:361-395`

**Current Behavior**:
```go
func newRateLimiter(requestsPerMinute int) *rateLimiter {
    rl := &rateLimiter{...}
    // Goroutine started but never stopped
    go rl.refill()
    return rl
}
```

Each LLM client creates a rate limiter goroutine that runs forever. If clients are created and discarded (e.g., for testing), goroutines accumulate.

**Impact**: Goroutine leak; minor memory impact but poor hygiene.

**Recommended Fix**:
```go
// Add context for lifecycle management
func NewClient(ctx context.Context, config LLMConfig, tracker UsageTracker) *Client {
    client := &Client{...}
    client.rateLimiter = newRateLimiter(ctx, config.RateLimit)
    return client
}

func newRateLimiter(ctx context.Context, requestsPerMinute int) *rateLimiter {
    rl := &rateLimiter{...}
    go rl.refill(ctx)  // Pass context
    return rl
}

func (r *rateLimiter) refill(ctx context.Context) {
    ticker := time.NewTicker(r.refillRate)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return  // Clean exit
        case <-ticker.C:
            select {
            case r.tokens <- struct{}{}:
            default:
            }
        }
    }
}
```

**Expected Improvement**: Proper goroutine cleanup; no leaked goroutines.

---

### M3: Placeholder Search is O(n) Per Content Item

**Location**: `internal/generator/text.go:146-162`, `internal/generator/images.go:263-287`

**Current Behavior**:
```go
// Build map each time
placeholderMap := make(map[string]int)
for i, shape := range slide.CommonSlideData.ShapeTree.Shapes {
    if shape.NonVisualProperties.NvPr.Placeholder != nil {
        phType := shape.NonVisualProperties.NvPr.Placeholder.Type
        placeholderMap[phType] = i
    }
}
```

The placeholder map is rebuilt for every slide. While this is O(n), it's done multiple times in separate functions.

**Impact**: Minor CPU overhead; code duplication.

**Recommended Fix**:
```go
// Extract to reusable function; call once per slide
func buildPlaceholderMap(shapes []shapeXML) map[string]int {
    placeholderMap := make(map[string]int, len(shapes)/2) // Preallocate
    for i, shape := range shapes {
        if shape.NonVisualProperties.NvPr.Placeholder != nil {
            phType := shape.NonVisualProperties.NvPr.Placeholder.Type
            if phType != "" {
                placeholderMap[phType] = i
            }
            if shape.NonVisualProperties.NvPr.Placeholder.Index != nil {
                idx := *shape.NonVisualProperties.NvPr.Placeholder.Index
                placeholderMap[fmt.Sprintf("%d", idx)] = i
            }
        }
    }
    return placeholderMap
}
```

**Expected Improvement**: Reduced code duplication; minor CPU improvement.

---

### M4: String Concatenation in Hot Paths

**Location**: `internal/parser/parser.go:133-148`, `internal/layout/heuristic.go:121-122`

**Current Behavior**:
```go
// Line 133-143 in parser.go
summary := fmt.Sprintf("Presentation: %s\n", presentation.Metadata.Title)
summary += fmt.Sprintf("Template: %s\n", presentation.Metadata.Template)
summary += fmt.Sprintf("Slides: %d\n", slideCount)
// ... more concatenation
```

String concatenation with `+=` creates new strings on each operation.

**Impact**: Memory allocation pressure in frequently called code.

**Recommended Fix**:
```go
func GetSummary(presentation *types.PresentationDefinition) string {
    var b strings.Builder
    b.Grow(256) // Preallocate estimated size

    fmt.Fprintf(&b, "Presentation: %s\n", presentation.Metadata.Title)
    fmt.Fprintf(&b, "Template: %s\n", presentation.Metadata.Template)
    fmt.Fprintf(&b, "Slides: %d\n", len(presentation.Slides))
    // ...

    return b.String()
}
```

**Expected Improvement**: Reduced allocations in string building.

---

### M5: Lack of Connection Pooling for HTTP Client

**Location**: `internal/llm/client.go:120-128`

**Current Behavior**:
```go
return &Client{
    config: config,
    httpClient: &http.Client{
        Timeout: config.Timeout,
    },
    // Default transport used
}
```

The default `http.Transport` settings are used, which have conservative defaults:
- MaxIdleConns: 100
- MaxIdleConnsPerHost: 2
- IdleConnTimeout: 90s

**Impact**: Under load, connections may not be reused efficiently.

**Recommended Fix**:
```go
func NewClient(config LLMConfig, tracker UsageTracker) *Client {
    transport := &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,  // Higher for single API endpoint
        IdleConnTimeout:     120 * time.Second,
        DisableCompression:  false,
        ForceAttemptHTTP2:   true,
    }

    return &Client{
        config: config,
        httpClient: &http.Client{
            Timeout:   config.Timeout,
            Transport: transport,
        },
        // ...
    }
}
```

**Expected Improvement**: Better connection reuse under concurrent LLM calls.

---

## Low Priority Issues

### L1: JSON Encoding Directly to ResponseWriter

**Location**: `internal/api/convert.go:167-198`

**Current Behavior**:
```go
// For base64 response, large encoded string built in memory
encoded := base64.StdEncoding.EncodeToString(data)
resp := ConvertResponseBase64{Data: encoded, ...}
json.NewEncoder(w).Encode(resp)
```

Base64 encoding happens in memory before JSON encoding.

**Impact**: Memory spike for large PPTX files with base64 output.

**Recommended Fix**:
```go
// For base64 output, consider streaming encoding
// or chunked transfer encoding for very large files
```

**Expected Improvement**: Reduced memory for base64 responses.

---

### L2: Error Message Formatting Allocations

**Location**: `internal/api/templates.go:227-243`

**Current Behavior**:
```go
func writeError(w http.ResponseWriter, status int, code, message string, details map[string]interface{}) {
    // Creates new struct on every error
    response := ErrorResponse{
        Success: false,
        Error: ErrorDetail{
            Code:    code,
            Message: message,
            Details: details,
        },
    }
    json.NewEncoder(w).Encode(response)
}
```

**Impact**: Minor allocation overhead on error paths.

**Recommended Fix**: Pre-allocate common error responses or use sync.Pool.

---

### L3: Slide Type Distribution Uses Map

**Location**: `internal/parser/slides.go:176-184`

**Current Behavior**:
```go
func GetSlideTypeDistribution(slides []SlideDefinition) map[SlideType]int {
    distribution := make(map[SlideType]int)
    for _, slide := range slides {
        distribution[slide.Type]++
    }
    return distribution
}
```

**Impact**: Minor; map allocation for small fixed set of types.

**Recommended Fix**: Consider using a struct with fixed fields for the ~7 slide types.

---

## Performance Monitoring Recommendations

### Add Metrics Collection

```go
// Add prometheus metrics
var (
    generationDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "slide_generation_duration_seconds",
            Buckets: []float64{0.1, 0.5, 1, 2, 5, 10},
        },
        []string{"template", "slide_count"},
    )

    zipOperationsDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "zip_operation_duration_seconds",
            Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1},
        },
        []string{"operation"}, // "read", "write", "copy"
    )

    memoryUsage = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "slide_generator_memory_bytes",
        },
        []string{"type"}, // "heap", "stack", "total"
    )
)
```

### Add Profiling Endpoints

```go
import _ "net/http/pprof"

// In main.go, register pprof handlers
mux.HandleFunc("/debug/pprof/", pprof.Index)
mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
mux.HandleFunc("/debug/pprof/heap", pprof.Handler("heap").ServeHTTP)
```

---

## Acceptance Criteria

### AC1: Single-Pass ZIP Generation
- Given a generation request with 10 slides
- When processed
- Then only ONE ZIP read and ONE ZIP write operation occurs

### AC2: Streaming Image Handling
- Given a slide with a 50MB image
- When generating PPTX
- Then peak memory increase is less than 5MB above baseline

### AC3: Rate Limiter Memory Stability
- Given 10,000 unique IPs making requests over 1 hour
- When monitoring memory
- Then rate limiter memory usage stays constant after initial allocation

### AC4: Fast Cache Validation
- Given a cached template analysis
- When validating cache on subsequent request
- Then file hash is NOT recalculated if modification time unchanged

### AC5: Bounded Cache Size
- Given cache configured with max 100 entries
- When 150 unique templates are analyzed
- Then cache contains exactly 100 entries (LRU eviction)

### AC6: Streaming File Downloads
- Given a 50MB PPTX file download
- When serving the file
- Then memory usage increases by less than 1MB

### AC7: No Goroutine Leaks
- Given 100 LLM clients created and garbage collected
- When checking goroutine count
- Then count returns to baseline (no leaked goroutines)

---

## Implementation Priority

| Issue | Impact | Effort | Priority | Status |
|-------|--------|--------|----------|--------|
| C1: Multiple ZIP Passes | Very High | High | Week 1 | ✅ Done |
| C2: Unbounded Image Loading | High | Low | Week 1 | ✅ Done |
| C3: Rate Limiter Memory Leak | Medium | Medium | Week 1 | ✅ Done |
| H1: Template Hash Every Request | Medium | Low | Week 2 | ✅ Done |
| H2: Regex in Function | Low | Very Low | Week 2 | ✅ Done |
| H3: Unbounded Cache | Medium | Medium | Week 2 | ✅ Done |
| H4: Chart Temp Files | Low | Medium | Week 2 | ✅ Done |
| H5: Full File Read Download | Medium | Low | Week 2 | ✅ Done |
| M1: Semaphore for Chart | Medium | Medium | Week 3 | ✅ Done |
| M2-M5: Various | Low-Medium | Low-Medium | Week 3 | ❌ Backlog |
| L1-L3: Various | Low | Low | Backlog | ❌ Backlog |

---

## Testing Requirements

### Load Testing

Create load test scenarios:
1. **Baseline**: 10 requests/minute, simple 5-slide presentations
2. **High Throughput**: 100 requests/minute, simple presentations
3. **Large Files**: 10 requests/minute, 50-slide presentations with images
4. **Sustained Load**: 50 requests/minute for 1 hour

### Memory Profiling

Run with memory profiling enabled:
```bash
go test -memprofile=mem.out -run=BenchmarkGeneration
go tool pprof mem.out
```

### CPU Profiling

Run with CPU profiling:
```bash
go test -cpuprofile=cpu.out -run=BenchmarkGeneration
go tool pprof cpu.out
```

### Benchmark Suite

```go
func BenchmarkGeneration(b *testing.B) {
    // Setup
    for i := 0; i < b.N; i++ {
        generator.Generate(testRequest)
    }
}

func BenchmarkGenerationParallel(b *testing.B) {
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            generator.Generate(testRequest)
        }
    })
}
```
