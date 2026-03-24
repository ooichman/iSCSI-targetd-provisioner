# iSCSI Targetd Provisioner for OpenShift
A high-performance, lightweight iSCSI provisioner designed for RHEL 9 and OpenShift 4.18
and Above.  
This project separates the control plane (API management) from the data plane  
(node mounting) to ensure security and stability.

## 🏗 Architecture
Controller: A non-privileged Deployment that communicates with the targetcli Python API to create LUNs.
Node Plugin: A privileged DaemonSet that runs on every worker node. 
It uses the host's native iscsiadm and multipath binaries to mount storage via a scratch container.

Kustomize: Manages environment-specific configurations without duplicating YAML.

## 🚀 Deployment Tutorial
### 1. Prerequisites

A running OpenShift cluster.

RHEL 9 worker nodes with iscsi-initiator-utils and device-mapper-multipath installed.

A storage server running the targetd Python API.

oc CLI and podman installed locally.

### 2. Configure the Environment
Edit the kustomization.yaml file in the root directory. 
Update the configMapGenerator with your storage network details:

```YAML
configMapGenerator:
  - name: iscsi-config
    literals:
      - ISCSI_API_ENDPOINT="http://10.100.0.1:18700/targetd/rpc"
      - ISCSI_TARGET_IQN="iqn.2026-03.com.example:target01"
      - ISCSI_PORTALS="10.100.0.1:3260,10.101.0.1:3260" # Comma separated for multipath
      - ISCSI_MULTIPATH_ENABLED="true"
```

### 3. Build and Push Images
Use the provided Makefile to build the statically linked Go binaries and push them to your registry (e.g., Quay.io).

Bash
#### Update the REGISTRY variable in the Makefile first

```bash
make build
make push
```

### 4. Deploy to OpenShift
Run the following command to create the namespace, RBAC, Service, Deployment, and DaemonSet:

Bash
#### This applies the entire Kustomize stack

```bash
oc apply -k .
```

### 5. Post-Deployment: Security Context
Because the Node Plugin needs to manage host-level ISCSI sessions,  
you must grant the privileged SCC to the service account:

```Bash
oc adm policy add-scc-to-user privileged -z iscsi-provisioner -n openshift-iscsi-provisioner
```

#### 🛠 Usage
Create a StorageClass
The project includes a targetd-provisioner StorageClass.  
Users can request storage by referencing this class in their PVCs.

```YAML
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: targetd-provisioner
provisioner: iscsi-targetd
parameters:
  volumeGroup: "vg-targetd"
  fsType: "xfs"
reclaimPolicy: Delete
volumeBindingMode: WaitForFirstConsumer
```

## Verify Deployment

Check that the controller is running: oc get pods -l app=iscsi-controller  
Check that the node plugins are active on all workers: 

```bash
oc get pods -l app=iscsi-node
```
Test a provisioning request: oc apply -f examples/test-pvc.yaml  

### 📂 Directory Structure

/controller: Go source code and Containerfile for the API manager.
/node: Go source code and Containerfile for the host-mounter.
/base: Kustomize base resources (RBAC, Service, etc).

### 🔒 Security Note
The Node Plugin uses a scratch image. It contains no shell, no libraries, and no OS tools.  
It functions by calling /usr/sbin/iscsiadm directly from the RHEL host's filesystem  
via secure volume mounts. This minimizes the attack surface of your privileged pods.

## 🔍 Troubleshooting
Because the Node Plugin runs in a scratch container, it has no shell (sh or bash).  
All debugging must be done via oc logs or from the RHEL host itself. 
#### 1. Check Pod LogsThe Go binary is designed to pipe all stdout and stderr from the host's  
iscsiadm and multipath commands directly into the pod logs.

```Bash
# View logs for the controller (API issues)
oc logs deployment/iscsi-controller -n openshift-iscsi-provisioner
```

# View logs for a specific node (Mounting/Login issues)
oc logs iscsi-node-xxxxx -n openshift-iscsi-provisioner
#### 2. Common iSCSI FailuresSymptomProbable CauseFixLogin failed: initiator name not foundThe  
RHEL host's /etc/iscsi/initiatorname.iscsi is missing or incorrect.Ensure the host is configured  
and the file is mounted into the pod.iscsiadm: command not foundThe /usr/sbin hostPath mount is  
missing or incorrect.Verify the DaemonSet volume mounts.Connection refusedThe Storage Server API  
or iSCSI target is unreachable.Check ISCSI_API_ENDPOINT and network connectivity from the node to  
the storage IP.

#### 3. Debugging from the RHEL HostIf the pod logs are insufficient, log into the worker node directly.  
Since the pod uses hostNetwork: true, the iSCSI sessions established by the pod are visible on the host.  

```bash
# Check active iSCSI sessions on the RHEL node
iscsiadm -m session -P 1

# Check if multipath has aggregated the devices
multipath -ll

# Check kernel messages for SCSI errors
dmesg | grep -i iscsi
```

#### 4. Restarting the StackIf you update the ConfigMap, Kustomize does not automatically restart the DaemonSet.  
You may need to trigger a manual rollout: 

```bash
oc rollout restart deployment/iscsi-controller -n openshift-iscsi-provisioner
oc rollout restart ds/iscsi-node -n openshift-iscsi-provisioner
```

Pro Tip: The "Ephemeral Debug" ContainerIf you absolutely need a shell inside the node's network and process  
namespace to run manual tests, use an OpenShift ephemeral debug container:

```bash
oc debug node/<node-name>
# Once inside the debug node, you are in a RHEL environment with all tools available.
```

#### 5. 🛠 Testing with Mock API
The controller includes a built-in `/demo` endpoint to verify the pod's internal 
logic and connectivity without touching the production storage server.

##### Run a Mock Provisioning:

```bash
oc exec -it $(oc get pod -l app=iscsi-controller -o name) -- \
  curl -X POST http://localhost:8080/demo/provision \
  -d '{"name": "test-volume"}'
```
