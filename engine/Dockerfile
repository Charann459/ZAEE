# Build stage
FROM golang:1.21-bookworm AS builder

WORKDIR /app

# We need gcc and build-essential for cgo
RUN apt-get update && apt-get install -y gcc libc6-dev

# Copy source code
COPY . .

# Run go mod tidy to generate go.sum
RUN go mod tidy

# Build the Go app
RUN CGO_ENABLED=1 GOOS=linux go build -o zaee-engine ./cmd/engine

# Final stage
FROM debian:bookworm-slim

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/zaee-engine .

CMD ["./zaee-engine"]
