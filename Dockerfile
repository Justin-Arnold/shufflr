# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o shufflr ./cmd/server

# Runtime stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite tzdata

# Create non-root user
RUN addgroup -g 1001 -S shufflr && \
    adduser -u 1001 -S shufflr -G shufflr

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/shufflr .

# Copy templates
COPY --from=builder /app/web ./web

# Create directories for data with proper permissions
RUN mkdir -p /app/data/uploads && \
    chown -R shufflr:shufflr /app

# Switch to non-root user
USER shufflr

# Set environment variables
ENV PORT=8080 \
    DATABASE_PATH=/app/data/shufflr.db \
    UPLOAD_DIR=/app/data/uploads \
    BASE_URL=http://localhost:8080

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./shufflr"]