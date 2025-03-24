# ---------- Stage 1: Build ----------
FROM golang:1.23.7-bullseye AS builder

# Install dependencies needed for CGO (librdkafka)
RUN apt-get update && apt-get install -y librdkafka-dev pkg-config

WORKDIR /app

# Copy go.mod/go.sum and download deps
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire source code
COPY . .

RUN ldd --version

# Build the binary (CGO enabled)
RUN CGO_ENABLED=1 GOOS=linux go build -o /consumer ./cmd/consumer

# ---------- Stage 2: Runtime ----------
FROM debian:bullseye-slim

# Install runtime dependencies (shared libs)
RUN apt-get update && apt-get install -y librdkafka1 ca-certificates && rm -rf /var/lib/apt/lists/*

# Copy binary from builder
COPY --from=builder /consumer /consumer

# Run as non-root user (optional but good practice)
RUN useradd -m appuser
USER appuser

ENTRYPOINT ["/consumer"]
