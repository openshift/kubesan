# The Subprovisioner CSI Plugin

Subprovisioner is a [CSI] plugin for [Kubernetes] that enables you to provision
`Block` volumes backed by a single, cluster-wide, shared block device (*e.g.*, a
LUN in a SAN).

## Installation

```console
$ kubectl create -k https://gitlab.com/subprovisioner/subprovisioner/deploy?ref=main
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
apiVersion: subprovisioner.gitlab.io/v1
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

## Development

To test, run `tests/run.sh all`. You may need to add yourself to the `libvirt`
group for it to work without prompting you for your password repeatedly.

## License

This project is released under the Apache 2.0 license. See [LICENSE](LICENSE).

[CSI]: https://github.com/container-storage-interface/spec
[`/etc/lvm/lvmlocal.conf`]: https://man7.org/linux/man-pages/man5/lvm.conf.5.html
[Kubernetes]: https://kubernetes.io/
[LVM]: https://man7.org/linux/man-pages/man8/lvm.8.html
[`lvmlockd`]: https://man7.org/linux/man-pages/man8/lvmlockd.8.html
