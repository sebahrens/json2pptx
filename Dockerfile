# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.25-alpine AS builder

# Build arguments for version info
ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_TIME=unknown

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy go.mod and go.sum first for better caching
# Must include svggen module files since go.mod uses a replace directive
COPY go.mod go.sum ./
COPY svggen/go.mod svggen/go.sum ./svggen/
RUN go mod download

# Copy source code
COPY . .

# Build the json2pptx binary (includes CLI, HTTP server, and MCP server)
# CGO_ENABLED=0 for static binary, -trimpath for reproducible builds
# -ldflags injects version info into binary
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w \
        -X main.Version=${VERSION} \
        -X main.CommitSHA=${COMMIT_SHA} \
        -X main.BuildTime=${BUILD_TIME}" \
    -o /json2pptx ./cmd/json2pptx

# Runtime stage
FROM alpine:3.21

# Build arguments for labels (must be redeclared in each stage)
ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_TIME=unknown

# OCI Image Labels
LABEL org.opencontainers.image.title="Go Slide Creator"
LABEL org.opencontainers.image.description="AI-powered slide deck generator service"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.revision="${COMMIT_SHA}"
LABEL org.opencontainers.image.created="${BUILD_TIME}"
LABEL org.opencontainers.image.source="https://github.com/sebahrens/json2pptx"
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.vendor="Ahrens"

# Install runtime dependencies
# ca-certificates: for HTTPS connections
# tzdata: for timezone support in logging
# fontconfig + fonts: for chart/SVG text rendering
RUN apk add --no-cache ca-certificates tzdata fontconfig font-dejavu font-noto

# Create non-root user for security
RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

# Create directories for templates and output
RUN mkdir -p /app/templates /app/output && \
    chown -R appuser:appgroup /app

WORKDIR /app

# Copy binary from builder
COPY --from=builder /json2pptx /app/json2pptx

# Switch to non-root user
USER appuser:appgroup

# Default port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/health || exit 1

# Default command: run as HTTP server
ENTRYPOINT ["/app/json2pptx"]
CMD ["serve", "--config", "/app/config.yaml"]
