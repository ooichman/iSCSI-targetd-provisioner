# Stage 1: Build the static binary
FROM golang:1.23-alpine AS builder

# Install git and certificates (needed for the build and the final image)
RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Copy and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
# -ldflags "-s -w" removes debug info to keep the binary tiny
# -extldflags "-static" ensures NO dynamic linking at all
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -a -installsuffix cgo \
    -ldflags "-s -w -extldflags '-static'" \
    -o iscsi-provisioner ./main.go
# Stage 2: Final Runtime Image (The "Scratch" stage)
FROM scratch

# 1. Important: Copy CA certificates so the binary can trust the OpenShift API
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# 2. Copy the statically linked binary
COPY --from=builder /app/iscsi-provisioner /iscsi-provisioner

# 3. OpenShift Compatibility:
# Even in scratch, we should specify a non-root User ID. 
# OpenShift's SCC will assign a random UID, but 1001 is a common convention.
USER 1001

ENTRYPOINT ["/iscsi-provisioner"]
