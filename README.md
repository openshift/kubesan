# The Subprovisioner CSI Plugin

Subprovisioner is a [CSI] plugin for [Kubernetes] that enables you to provision
`Block` volumes backed by a single, cluster-wide, shared block device (*e.g.*, a
LUN in a SAN).

## Installation

Ensure that all nodes in the cluster are running [`lvmlockd`] and have a unique
host ID set in [`/etc/lvm/lvmlocal.conf`] under `local/host_id`.

Then install Subprovisioner:

```console
$ kubectl create -f deployment.yaml
```

## Setting up a shared block device

Ensure that the shared, backing block device is available on all nodes in the
cluster at the same path.

Then simply create a `StorageClass` that uses the Subprovisioner CSI plugin and
specifies the path to the backing device:

```yaml
apiVersion: subprovisioner.gitlab.io/v1
kind: StorageClass
metadata:
  name: my-san
provisioner: subprovisioner.gitlab.io
parameters:
  backingDevicePath: /dev/my-san
```

And then you can use that `StorageClass` as normal:

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

You can have several Subprovisioner `StorageClass`es on the same cluster that are
backed by different shared block devices.

## Development

To test, run `tests/run.sh all`. You may need to add yourself to the `libvirt`
group and run `firewall-cmd --zone=libvirt --add-port=10809/tcp` for it to work.

## License

This project is released under the Apache 2.0 license. See [LICENSE](LICENSE).

[CSI]: https://github.com/container-storage-interface/spec
[`/etc/lvm/lvmlocal.conf`]: https://man7.org/linux/man-pages/man5/lvm.conf.5.html
[Kubernetes]: https://kubernetes.io/
[LVM]: https://man7.org/linux/man-pages/man8/lvm.8.html
[`lvmlockd`]: https://man7.org/linux/man-pages/man8/lvmlockd.8.html
