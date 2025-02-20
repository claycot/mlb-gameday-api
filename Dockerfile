# Build stage
FROM golang:1.22.4-alpine AS builder
# Install git if you need it for fetching dependencies
# RUN apk add --no-cache git
RUN apk add --no-cache tzdata

WORKDIR /app

# Copy go mod and sum files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
# CGO is disabled to produce a fully static binary for Alpine
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/main.go

# Final stage
FROM alpine:latest

# Add certificates (optional, but useful if your app makes HTTPS calls)
# RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/main .

# Expose the port your web server is listening on
EXPOSE 8080

# Command to run the binary
CMD ["./main"]