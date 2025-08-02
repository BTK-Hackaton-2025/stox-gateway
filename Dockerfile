# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/gateway ./cmd/api-gateway/main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Add ca certificates and timezone data
RUN apk --no-cache add ca-certificates tzdata

# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy the binary from builder
COPY --from=builder /app/gateway .

# Copy config file - will be overridden by volume mount in docker-compose
COPY config.yaml .

# Set ownership
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose the port the API gateway listens on
EXPOSE 8080

# Command to run
CMD ["./gateway"]