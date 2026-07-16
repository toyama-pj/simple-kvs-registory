# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install swag for generating swagger docs
RUN go install github.com/swaggo/swag/cmd/swag@latest

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Generate swagger docs
RUN swag init

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/simple-kvs-registry .

# Final stage
FROM alpine:latest

WORKDIR /app

# Install CA certificates for HTTPS requests if needed by the app
RUN apk --no-cache add ca-certificates tzdata

# Copy the binary from the builder stage
COPY --from=builder /app/simple-kvs-registry .

# Copy swagger docs if they exist
COPY --from=builder /app/docs /app/docs

# Default port
EXPOSE 3000

# Command to run the executable
CMD ["./simple-kvs-registry"]
