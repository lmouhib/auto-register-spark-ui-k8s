# Use the official Golang image as the base image
FROM golang:1.22-alpine as builder

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

# Set the working directory inside the container
WORKDIR /root/

# Copy the built Go binary from the builder stage
COPY --from=builder /app/auto-register-k8s-spark-ui .

# Command to run the application
CMD ["./auto-register-k8s-spark-ui"]