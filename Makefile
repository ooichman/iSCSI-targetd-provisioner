# --- Environment Variable Validation ---
# These must be set in your shell or passed to make: e.g., 'make build REGISTRY=quay.io/user'
ifndef REGISTRY
$(error REGISTRY is not defined. Please set the REGISTRY environment variable)
endif

# Variables
CONTROLLER_IMG = $(REGISTRY)/iscsi-controller:latest
NODE_IMG       = $(REGISTRY)/iscsi-node:latest
NAMESPACE      = openshift-iscsi-provisioner

# Local Paths
SBIN_DIR = /usr/sbin
ETC_DIR  = /etc
SYSTEMD_DIR = /etc/systemd/system

.PHONY: build push deploy install-server check-env all

all: build push deploy

# Helper to print the current build config
check-env:
	@echo "--- Build Configuration ---"
	@echo "Registry:   $(REGISTRY)"
	@echo "Controller: $(CONTROLLER_IMG)"
	@echo "Node:       $(NODE_IMG)"
	@echo "---------------------------"

# --- Container Build/Push ---
build: check-env
	@echo "Building Controller..."
	cd controller && podman build -t $(CONTROLLER_IMG) .
	@echo "Building Node Plugin..."
	cd node && podman build -t $(NODE_IMG) .

push:
	@echo "Pushing images..."
	podman push $(CONTROLLER_IMG)
	podman push $(NODE_IMG)

deploy:
	@echo "Applying Kustomize to OpenShift..."
	oc apply -k .
	@echo "Granting Privileged SCC..."
	oc adm policy add-scc-to-user privileged -z iscsi-provisioner -n $(NAMESPACE)

# --- Local Storage Server Installation (No sudo) ---
# Note: This target assumes the user executing 'make' has write access to /usr/sbin and /etc
install-server:
	@echo "Installing targetcli-api components..."
	
	# 1. Install executable
	install -m 0755 ./targetcli-api/targetcli-api.py $(SBIN_DIR)/targetcli-api
	
	# 2. Install config (only if missing)
	@if [ ! -f $(ETC_DIR)/targetcli-api.conf ]; then \
		cp ./targetcli-api/targetcli-api.conf $(ETC_DIR)/targetcli-api.conf; \
		echo "Installed: $(ETC_DIR)/targetcli-api.conf"; \
	else \
		echo "Skipping: $(ETC_DIR)/targetcli-api.conf already exists"; \
	fi
	
	# 3. Install systemd service
	cp ./targetcli-api/targetcli-api.service $(SYSTEMD_DIR)/
	
	# 4. Refresh systemd
	systemctl daemon-reload
	systemctl enable targetcli-api --now
	@echo "Installation complete. Checking service status..."
	systemctl status targetcli-api --no-pager
