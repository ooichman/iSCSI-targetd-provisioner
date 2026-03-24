# Variables
REGISTRY = quay.io/ooichman
CONTROLLER_IMG = $(REGISTRY)/iscsi-controller:latest
NODE_IMG = $(REGISTRY)/iscsi-node:latest
NAMESPACE = openshift-iscsi-provisioner

.PHONY: build push deploy all

all: build push deploy

build:
	@echo "Building Controller..."
	cd controller && podman build -t $(CONTROLLER_IMG) .
	@echo "Building Node Plugin..."
	cd node && podman build -t $(NODE_IMG) .

push:
	@echo "Pushing images to Quay..."
	podman push $(CONTROLLER_IMG)
	podman push $(NODE_IMG)

deploy:
	@echo "Applying Kustomize to OpenShift..."
	oc apply -k .
	@echo "Granting Privileged SCC to Node Plugin..."
	oc adm policy add-scc-to-user privileged -z iscsi-provisioner -n $(NAMESPACE)

clean:
	oc delete -k .
