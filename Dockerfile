# Build stage
FROM docker.io/library/golang:1.26-alpine AS builder

WORKDIR /app

# Install build dependencies for CGO (required by DuckDB) and swag
RUN apk add --no-cache gcc g++ musl-dev && \
    go install github.com/swaggo/swag/cmd/swag@latest

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Generate swagger docs
RUN swag init

# Build the application with CGO enabled
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/simple-kvs-registry .

# Final stage
FROM alpine:latest

WORKDIR /app

# Install CA certificates, timezone data, and libstdc++ (required by DuckDB)
RUN apk --no-cache add ca-certificates tzdata libstdc++

# Copy the binary from the builder stage
COPY --from=builder /app/simple-kvs-registry .

# Copy swagger docs if they exist
COPY --from=builder /app/docs /app/docs

# Default port
EXPOSE 3000

# Command to run the executable
CMD ["./simple-kvs-registry"]
