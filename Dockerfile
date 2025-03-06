# ==============================================
# Builder stage
# ==============================================
FROM golang:1.23-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go module files
COPY go.mod go.sum ./

# Download Go modules
RUN go mod download

# Copy the source code
COPY . .

# Copy the Lua script into the container
COPY rateLimiter.lua /app/rateLimiter.lua

# Build the Go application
RUN go build -o main .

# ==============================================
# Runtime stage
# ==============================================
FROM alpine:latest

# Set working directory
WORKDIR /app

# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy the built binary and the Lua script from the builder stage
COPY --from=builder --chown=appuser:appgroup /app/main .
COPY --from=builder --chown=appuser:appgroup /app/rateLimiter.lua .

# Expose the application port
EXPOSE ${PORT}

# Run as a non-root user
USER appuser

# Command to run when the container starts
CMD ["./main"]