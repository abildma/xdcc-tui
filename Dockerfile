# Build stage
FROM golang:1.19-alpine AS builder

WORKDIR /app

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .


# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /xdcc ./cmd

# Final stage
FROM alpine:latest

WORKDIR /


# Install CA certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy the binary from builder
COPY --from=builder /xdcc /xdcc

# Set non-root user
RUN adduser -D xdccuser
USER xdccuser

# Run the application
ENTRYPOINT ["/xdcc"]