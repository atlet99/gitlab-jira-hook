# Build stage v1.24.4-alpine3.22
FROM golang:1.24.5-alpine3.22@sha256:daae04ebad0c21149979cd8e9db38f565ecefd8547cf4a591240dc1972cf1399 AS builder

# Set working directory
WORKDIR /app

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build arguments for version information
ARG VERSION=docker
ARG COMMIT=unknown
ARG DATE
ARG BUILT_BY=docker

# Set build date if not provided
RUN if [ -z "$DATE" ]; then DATE=$(date -u '+%Y-%m-%d_%H:%M:%S'); fi

# Build the application
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X 'github.com/atlet99/gitlab-jira-hook/internal/version.Version=${VERSION}' \
              -X 'github.com/atlet99/gitlab-jira-hook/internal/version.Commit=${COMMIT}' \
              -X 'github.com/atlet99/gitlab-jira-hook/internal/version.Date=${DATE}' \
              -X 'github.com/atlet99/gitlab-jira-hook/internal/version.BuiltBy=${BUILT_BY}'" \
    -o gitlab-jira-hook ./cmd/server

# Final stage
FROM alpine:3.22.1@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/gitlab-jira-hook .

# Copy configuration example
COPY --from=builder /app/config.env.example ./config.env.example

# Change ownership to non-root user
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./gitlab-jira-hook"] 