# Builder stage: compile the Go application using Go 1.25 on Alpine Linux
FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache ca-certificates

# Copy dependency files and download Go modules for faster builds with caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire source code into the container
COPY . .

# Build the static binary with CGO disabled for maximum portability
RUN CGO_ENABLED=0 GOOS=linux go build -o conduit ./cmd/main.go

# Runtime stage: use minimal Alpine Linux 3.19 image for smaller final image size
FROM alpine:3.19

# Set the working directory for the build process
# Set the working directory for the runtime environment
WORKDIR /app

# Install CA certificates needed for HTTPS connections during build
# Install CA certificates needed for SSH connections and HTTPS requests
RUN apk add --no-cache ca-certificates

# Copy the compiled binary from the builder stage
COPY --from=builder /app/conduit .

# Create a non-root user for security and switch to it
RUN adduser -D -u 1000 conduit
USER conduit

# Set the entrypoint to run conduit with default config file path
ENTRYPOINT ["./conduit"]
CMD ["-config", "/app/config/config.yaml"]