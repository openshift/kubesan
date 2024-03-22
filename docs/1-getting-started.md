# Getting started

## Setting up the cluster and shared block device

This guide assumes you already have a working cluster for Kubernetes;
if you still need to set that up, you might try [CRI-O on
Fedora](https://fedoramagazine.org/kubernetes-with-cri-o-on-fedora-linux-39/)
for bare-metal.

[//]: # (Comment - this would be a good place to add information on
how to set up a Kubevirt or minikube setup)

Additionally, before you deploy Subprovisioner, you need to make sure
every node in your cluster provides the global resources that
Subprovisioner will be using.

Subprovisioner depends on kernel modules for nbd and dm-thin-pool
being loaded on all nodes.  If your kernel does not already have these
built in, you may need to run this on each node:

```console
$ sudo cat <<EOF | sudo tee /etc/modules-load.d/subprovisioner.conf
nbd
dm-thin-pool
EOF
$ systemctl restart systemd-modules-load.service
```

Generally you should enable as many NBD devices on each node as the
maximum number of Subprovisioner volumes you may need to have mounted
on a single node at once.

Second, subprovisioner assumes that you have shared storage visible as
a block device to each node of the cluster, such as a LUN from a SAN.
You need to ensure that this block device can be seen at the same path
in each node.  It may help to use `multipath -ll` or `lsblk -o +uuid`
to determine a stable name, such as /dev/disk/by-id/dm-uuid-mpath-XXXX
when accessing a LUN through multipath.

Other shared storage solutions, such as a shared LV, an NFS file
mounted through loopback, or even /dev/nbdX pointing to a common NBD
server, will likely work, although they are less tested.  However,
shared storage based on host-based mirroring or replication is not
likely to work correctly, since subprovisioner uses sanlock which
works best when all its io is ultimately directed to the same physical
location.

Subprovisioner assumes that it will be the sole owner of the complete
block device; you should not assume that any pre-existing data will be
preserved.  However, in case there may be stale LVM data left over
from previous use of the storage, you may want to wipe the storage
from the control plane before deploying Subprovisioner, either by:

```console
$ sudo pvremove --devices /dev/my-san-lun /dev/my-san-lun
```

or by:

```
$ sudo dd if=/dev/zero of=/dev/my-san-lun bs=1M count=8
```

## Installing Subprovisioner

Adding Subprovisioner to your cluster is straightforward:

```console
$ kubectl create -k https://gitlab.com/subprovisioner/subprovisioner/deploy?ref=v0.1.0
```

Then create a `StorageClass` that uses the Subprovisioner CSI plugin and
specifies the path to the backing device:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: my-san
provisioner: subprovisioner.gitlab.io
parameters:
  backingDevicePath: /dev/my-san-lun
```

And finally you can use that `StorageClass` as normal:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-pvc
spec:
  storageClassName: my-san
  volumeMode: Block
  resources:
    requests:
      storage: 1Ti
  accessModes:
    - ReadWriteOnce
```

You can have several Subprovisioner `StorageClass`es on the same cluster that
are backed by different shared block devices.
