# Build Stage
FROM golang:1.26.2-alpine AS builder

WORKDIR /app

# Install git
RUN apk add --no-cache git

# Copy dependency files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build application
RUN CGO_ENABLED=0 GOOS=linux go build -o pmt ./cmd/api

# Runtime Stage
FROM alpine:latest

WORKDIR /app

# Install certificates
RUN apk add --no-cache ca-certificates

# Copy binary from builder
COPY --from=builder /app/pmt .

# Copy environment file (optional)
COPY env.sh env.sh

# Expose application port
EXPOSE 6369

# Run application
CMD ["./pmt"]