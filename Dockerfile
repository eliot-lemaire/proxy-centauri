# Stage 1 — build the binary
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy dependency files first so Docker caches this layer.
# If go.mod and go.sum haven't changed, the next build skips the download.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source and compile
COPY . .
RUN go build -o centauri ./cmd/centauri

# Stage 2 — minimal runtime image
FROM alpine:3.19

WORKDIR /app

# Create directories for SQLite metrics store and JSON request logs
RUN mkdir -p /app/data /app/logs

# Copy only the compiled binary from the builder stage
COPY --from=builder /app/centauri .

ENTRYPOINT ["./centauri"]
