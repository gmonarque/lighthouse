# Build stage - Frontend
FROM node:20-alpine AS frontend-builder

# Install pnpm
RUN corepack enable && corepack prepare pnpm@latest --activate

WORKDIR /app/web

# Copy frontend files
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY web/ ./
RUN pnpm run build

# Build stage - Backend
FROM golang:1.23-alpine AS backend-builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy built frontend to embed
COPY --from=frontend-builder /app/web/build ./internal/api/static

# Build the binary with FTS5 support
RUN CGO_ENABLED=1 GOOS=linux go build -tags "fts5" -a -ldflags '-linkmode external -extldflags "-static"' -o lighthouse ./cmd/lighthouse

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 lighthouse && \
    adduser -u 1000 -G lighthouse -s /bin/sh -D lighthouse

# Create directories
RUN mkdir -p /app/data /app/config && \
    chown -R lighthouse:lighthouse /app

WORKDIR /app

# Copy binary
COPY --from=backend-builder /app/lighthouse .

# Switch to non-root user
USER lighthouse

# Expose port
EXPOSE 9999

# Volume for persistent data
VOLUME ["/app/data", "/app/config"]

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:9999/health || exit 1

# Run
ENTRYPOINT ["./lighthouse"]
