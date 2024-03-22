# Getting started

## Installing Subprovisioner

```console
$ kubectl create -k https://gitlab.com/subprovisioner/subprovisioner/deploy?ref=v0.1.0
```

## Setting up a shared block device

Ensure that the shared, backing block device is available on all nodes in the
cluster at the same path.

Also ensure that the NBD kernel client is loaded on all nodes. Generally you
should enable as many NBD devices on each node as the maximum number of
Subprovisioner volumes you may need to have mounted on a single node at once.

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
