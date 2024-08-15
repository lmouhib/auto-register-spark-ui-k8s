# Use the official Golang image as the base image
FROM golang:1.22-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the go.mod, go.sum and main.go files
COPY go.mod go.sum main.go ./

# Copy the controllers directory
COPY controllers ./controllers 

# Download the Go module dependencies
RUN go mod download

# Build the Go application
RUN go build -o auto-register-k8s-spark-ui .

# Use a minimal base image to run the application
FROM alpine:latest

# Install CA certificatesdocker run -it auto-register-k8s-spark-ui sh
RUN apk --no-cache add ca-certificates

# Create a user and group
RUN addgroup -g 3000 -S auto-register-ui && \
    adduser -u 1000 -S auto-register-ui -G auto-register-ui

# Set the working directory
WORKDIR /app

COPY LICENSE LICENSE

# Copy the built Go binary from the builder stage
COPY --from=builder /app/auto-register-k8s-spark-ui /app/

# Change ownership of the application files
RUN chown -R auto-register-ui:auto-register-ui /app

# Switch to the non-root user
USER auto-register-ui:auto-register-ui

# Command to run the application
CMD ["./auto-register-k8s-spark-ui"]