# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Copy go.mod / go.sum and download deps
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build the engine server
RUN CGO_ENABLED=1 go build -o /app/bin/server ./cmd/server/

# Run stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates sqlite-libs tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/server .

# Create data directory
RUN mkdir -p /app/data

EXPOSE 8080

ENV ADDR=:8080
ENV DB_PATH=/app/data/smart-assistant.db

CMD ["./server"]