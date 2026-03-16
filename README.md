# iSCSI-targetd provisioner 

Recently I have being asked to provide an Iscsi Storage class for Lab and Test
purposes and I cam across this old and not maintained provisioner So I made a
decission to clone it and change it to work with newer version of OpenShift  

iSCSI-targetd provisioner is an out of tree provisioner for iSCSI storage for
Kubernetes and OpenShift.  The provisioniner uses the API provided by
[targetd](https://github.com/open-iscsi/targetd) to create and export
iSCSI storage on a remote server.

## Prerequisites

iSCSI-targetd provisioner has the following prerequisistes:

1. an iSCSI server managed by `targetd`
2. all the openshift nodes correclty configured to communicate with the iSCSI server
3. sufficient disk space available as LVM2 volume group (thinly provisioned volumes are also supported and can be used to alleviate this requirement)

## How it works

When a pvc request is issued for an iscsi provisioner controlled
storage class the following happens:

1. a new volume in the configured volume group is created, the size of
the volume corresponds to the size requested in the pvc
2. the volume is exported to the first available lun and made
accessible to all the configured initiators.
3. the corresponding pv is created and bound to the pvc.


Each storage class is tied to an iSCSI target and a volume
group. Because a target can manage a maximum of 255 LUNs, each
storage class manages at most 255 pvs. iSCSI-targetd provisioner can manage
multiple storage classes.

## Installing the prerequisites

These instructions should work for RHEL/CentOS 9+ 

For RHEL and Centos make sure you install targetd >= 0.8.6-1 as in 
previous versions there a bug that prevented exposing a volume to more 
than one initiator 

### A note about names

In various places, iSCSI Qualified Names (IQNs) need to be created.
These need to be unique.  So every target must have it's own unique
IQN, and every client (initiator) must have its own IQN.

IF NON-UNIQUE IQNs ARE USED, THEN THERE IS A POTENTIAL FOR DATA LOSS
AND BAD PERFORMANCE!

IQNs have a specific format:

iqn.YEAR-MM.com.example.blah:tag

See the [wikipedia
article](https://en.wikipedia.org/wiki/ISCSI#Addressing) for more
information.

### Configure Storage

Before configuring the iSCSI server, it needs to have storage
configured.  `targetd` uses LVM to provision storage.

If possible, it's best to have a dedicated disk or partition that can
be configured as a volume group.  However, if this is not possible, a
loopback device can be used to simulate a dedicated block device.

#### (Option 1) Create a Volume Group with a dedicated disk or partition

This requires an additional dedicated disk or partition to use for the
volume group.  If that's not possible, see the section on using a
loopback device.

Assuming that the dedicated block device is `/dev/vdb` and that
`targetd` is configured to use `vg-targetd`:

```
pvcreate /dev/sdb
vgcreate vg-targetd /dev/sdb
```

#### (Option 2) Create a Volume Group on a Loopback Device
the volume group should be called `vg-target`, this way you don' have to change any default

here is how you would do it in minishift

```
mkdir /var/lib/minishift && cd /var/lib/minishift
sudo dd if=/dev/zero of=disk.img bs=1G count=2
export LOOP=`sudo losetup -f`
sudo losetup $LOOP disk.img
sudo vgcreate vg-targetd $LOOP
```

#### Optional:  Enable Thin Provisioning

Logical Volumes created in a volume group are thick provisioned by
default, i.e. space is reserved at time of creation.  Optionally, a
LVM can use a thin provisioning pool to create thin provisioned volumes.  

To create a thin provisioning pool, called `pool` this example,
execute the following commands:

```
# This will create a 15GB thin pool in the vg-targetd volume group
lvcreate -L 45G --thinpool pool vg-targetd
```

When configuring `targetd`, the pool_name setting in targetd.yaml will
need to be set to <volume group name>/<thin pool name>.  In this
example, it would be `vg-targetd/pool`.

### Configure the iSCSI server

#### Install targetd and targetcli

Only `targetd` needs to be installed.  However, it's highly recommended
to also install `targetcli` as it provides a simple user interface for
looking at the state of the iSCSI system.

```
dnf install -y targetcli
```

#### Configure target

Enable and start `target.service`.  This will ensure that iSCSI
configuration persists through reboot.

```
systemctl enable --now target
```

#### Configure targetd

The default configuration requires that port 3260/tcp, 3260/udp and
18700/tcp be open on the iSCSI server.

##### Create the Block Backstore
Instead of specifying a file size, you point directly to the LVM path.

```bash
targetcli /backstores/block create name=iscsidev dev=/dev/vg-targetd/pool
```

##### Creating the Target Portal Group (TPG)  
If you don't specify an IQN,  
targetcli will generate one for you.

```bash
targetcli /iscsi create iqn.2026-03.com.example:target01
```

##### Create the LUN
This maps the LVM backstore to your target.

```bash
targetcli /iscsi/iqn.2026-03.com.example:target01/tpg1/luns create /backstores/block/iscsidev
```

Now that we have the LUN we can set the ACL for it :

##### Configure ACL with username and password :

```bash
targetcli /iscsi/iqn.2026-03.com.example:target01/tpg1 set \
   attribute generate_node_acls=0 demo_mode_write_protect=0 authentication=1
```

Now we will attach the client definition to the ACL. This Client IQN will be 
define with-in OpenShift later on this tutorial :

```bash
targetcli /iscsi/iqn.2026-03.com.example:target01/tpg1/acls create iqn.2026-03.com.example:client1
```

Next we will setup the username and the password for the client :

```bash
targetcli /iscsi/iqn.2026-03.com.example:target01/tpg1/acls/iqn.2026-03.com.example:client1 \
   set auth userid=admin password=ciao
```

Now we will save everything and make sure the configuration are premanent

```bash
targetcli saveconfig
```

#### (Optional) Firewalld configuration
If using `firewalld`, 

```
firewall-cmd --add-service=iscsi-target --permanent
firewall-cmd --add-port=18700/tcp --permanent 
firewall-cmd --reload
```

### Configure the nodes (iscsi clients)

Each node requires a unique initiator name.  USE OF DUPLICATE NAMES
MAY CAUSE PERFORMANCE ISSUES AND DATA LOSS.

By default, a random initiator name is generated when the
`iscsi-initiator-utils` package is installed.  This usually unique
enough, but is not guaranteed.  It's also not very descriptive.

#### Configure the OpenShift Node

To set a custom initiator name, we will create a Machine Config 
Resource definition that will create the file `/etc/iscsi/initiatorname.iscsi`
and will make sure the following content is within the file :
`InitiatorName=iqn.2017-04.com.example:node1`

##### (Optional) Configure MCP 

In OpenShift to setup node specific configuration we will need to create
a custom machine config pool (mcp) to mache the node we want to apply the configuration.  
A custom configuration can only be applied to a worker node and we need to make sure  
all the generic worker configuration are still being applied. For that we will create  
a Machine Config Pool to catch both custom and generic label match :

```bash
cat << EOF | oc apply -f -
apiVersion: machineconfiguration.openshift.io/v1
kind: MachineConfigPool
metadata:
    name: master01
spec:
  machineConfigSelector:
    matchExpressions:
      - {key: machineconfiguration.openshift.io/role, operator: In, values: [worker,worker01]}
  maxUnavailable: null
  nodeSelector:
    matchLabels:
      kubernetes.io/hostname: worker01.example.com
EOF
```
In order to make sure the Node as the necessary role to match the machine config pool we  
will label the node with the role we want (in our example it's worker01)

```bash
oc label node worker01.example.com node-role.kubernetes.io/worker01=
```

##### Configure the Machine Config (mc)

Once the MCP is configured we  will generate a base64 string from the file content :

```bash
export CONTENT_BASE64=$(echo 'InitiatorName=iqn.2017-04.com.example:node1' | base64 -w0)
```

The Following mc will create the file we described in the beginning of the scrion  
and we will make sure iscsid is running using the systemd configuration part :

```bash
cat << EOF | oc apply -f -
apiVersion: machineconfiguration.openshift.io/v1
kind: MachineConfig
metadata:
  labels:
    machineconfiguration.openshift.io/role: worker01
  name: 55-master-iscsi-initiator-worker01
spec:
  config:
    ignition:
      version: 3.4.0
    storage:
      files:
      - contents:
          source: data:text/plain;charset=utf-8;base64,${CONTENT_BASE64}
        mode: 420
        overwrite: true
        path: /etc/iscsi/initiatorname.iscsi
    systemd:
      units:
        - enabled: true
          name: iscsid.service
EOF
```


#### Configure Kubernetes Node 
(In regards to Kubernetes It's the same for configuring a normal Node)

### Install the iscsi provisioner pod in Kubernetes

Run the following commands. The secret correspond to username and password you have chosen for targetd (admin is the default for the username).
This set of command will install iSCSI-targetd provisioner in the `default` namespace.

```
export NS=default
kubectl create secret generic targetd-account --from-literal=username=admin --from-literal=password=ciao -n $NS
kubectl apply -f https://raw.githubusercontent.com/kubernetes-incubator/external-storage/master/iscsi/targetd/kubernetes/iscsi-provisioner-d.yaml -n $NS
kubectl apply -f https://raw.githubusercontent.com/kubernetes-incubator/external-storage/master/iscsi/targetd/kubernetes/iscsi-provisioner-pvc.yaml -n $NS
```

### Install the iscsi provisioner pod in Openshift

Run the following commands. The secret correspond to username and password you have chosen for targetd (admin is the default for the username)

```
oc adm new-project openshift-iscsi-provisioner
oc project openshift-iscsi-provisioner
oc create sa iscsi-provisioner
oc adm policy add-cluster-role-to-user cluster-reader system:serviceaccount:openshift-iscsi-provisioner:iscsi-provisioner
oc adm policy add-cluster-role-to-user system:persistent-volume-provisioner system:serviceaccount:openshift-iscsi-provisioner:iscsi-provisioner
#
oc create secret generic targetd-account --from-literal=username=admin --from-literal=password=ciao -n openshift-iscsi-provisioner
oc create -f https://raw.githubusercontent.com/kubernetes-incubator/external-storage/master/iscsi/targetd/openshift/iscsi-provisioner-dc.yaml
```



### Start iscsi provisioner as A container.

Alternatively, you can start a provisioner as a container locally.

```bash
podman run -ti -v /root/.kube:/kube -v /var/run/kubernetes:/var/run/kubernetes --privileged --net=host quay.io/external_storage/iscsi-controller:latest start --kubeconfig=/kube/config --master=http://127.0.0.1:8080 --log-level=debug --targetd-address=192.168.99.100 --targetd-password=ciao --targetd-username=admin
```

### Create a storage class

storage classes should look like the following

```
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: targetd-provisioner
  resourceVersion: "3132739"
parameters:
  chapAuthDiscovery: "true"
  chapAuthSession: "true"
  fsType: xfs
  initiators: iqn.2026-03.com.example:client1
  iqn: iqn.2026-03.com.example:target01
  iscsiInterface: default
  targetPortal: 10.100.0.1:3260
  volumeGroup: vg-targetd
provisioner: iscsi-targetd
reclaimPolicy: Delete
volumeBindingMode: Immediate
```

### Test iscsi provisioner

#### Create a pvc

```
cat << EOF | oc apply -f -
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: test-claim
spec:
  storageClassName: targetd-provisioner
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 100Mi
EOF
```
verify that the pv has been created

```
oc get pvc
NAME         STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS          VOLUMEATTRIBUTESCLASS   AGE
test-claim   Bound    pvc-08822372-8925-4662-a67f-f7056adc5b33   100Mi      RWX            targetd-provisioner   <unset>                 6h10m
```

you may also want to verify that the volume has been created in you volume group

```
targetcli ls
```

deploy a pod that uses the pvc

```
oc create -f https://raw.githubusercontent.com/kubernetes-incubator/external-storage/master/iscsi/targetd/openshift/iscsi-test-pod.yaml
```
