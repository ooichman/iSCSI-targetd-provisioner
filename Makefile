# Variables
REGISTRY ?= quay.io/two.oes/
IMAGE_NAME = iscsi-targetd-provisioner
TAG ?= latest
GO_VERSION = 1.22

# Binary and Build settings
BINARY = iscsi-targetd-provisioner
GOOS ?= linux
GOARCH ?= amd64

# Help command
.PHONY: help
help:
	@echo "Usage: make [target]"
	@echo "  build   - Build the golang binary"
	@echo "  container - Build the Docker/Podman image"
	@echo "  push    - Push the image to the registry"
	@echo "  clean   - Remove binary and build artifacts"

# 1. Build the Binary
.PHONY: build
build:
	@echo "Building binary for $(GOOS)/$(GOARCH)..."
        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -a -installsuffix cgo \
        -ldflags "-s -w -extldflags '-static'" \
        -o bin/$(BINARY) ./main.go

# 2. Build the Container Image
# Note: Using 'podman' is standard for OpenShift/RHEL environments
.PHONY: container
container: build
	@echo "Building container image $(REGISTRY)/$(IMAGE_NAME):$(TAG)..."
	podman build -t $(REGISTRY)/$(IMAGE_NAME):$(TAG) .

# 3. Push to Registry
.PHONY: push
push:
	@echo "Pushing image..."
	podman push $(REGISTRY)/$(IMAGE_NAME):$(TAG)

# 4. Clean up
.PHONY: clean
clean:
	rm -rf bin/
