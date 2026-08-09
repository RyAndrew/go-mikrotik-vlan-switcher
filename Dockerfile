# Stage 1: Build
FROM golang:1.26 AS builder
WORKDIR /app
# Copy module files first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Build static binary (disables CGO for portability)
RUN CGO_ENABLED=0 GOOS=linux go build -o /go-mikrotik-vlan-switcher

# Stage 2: Run
FROM alpine:latest
WORKDIR /root/
# Copy only the binary from the builder stage
COPY --from=builder /go-mikrotik-vlan-switcher .
EXPOSE 7071
CMD ["./go-mikrotik-vlan-switcher"]
