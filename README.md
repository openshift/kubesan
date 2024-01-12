# The clustered-csi CSI Plugin

clustered-csi is a [CSI] plugin for [Kubernetes] that enables you to provision
`Block` volumes backed by a single, cluster-wide, shared block device (*e.g.*, a
LUN in a SAN).

## Installation

Ensure that all nodes in the cluster are running [`lvmlockd`] and have a unique
host ID set in [`/etc/lvm/lvmlocal.conf`] under `local/host_id`.

Then install clustered-csi:

```console
$ kubectl create -f deployment.yaml
```

## Setting up a shared block device

The shared block device must be formatted as an [LVM] physical volume, be
available on all nodes in the cluster, and back a shared volume group:

```console
$ vgcreate --shared my-san-vg /dev/my-san
```

Then create a `StorageClass` that uses the clustered-csi CSI plugin and
references the LVM volume group:

```yaml
apiVersion: clustered-csi.gitlab.io/v1
kind: StorageClass
metadata:
  name: my-san
provisioner: clustered-csi.gitlab.io
parameters:
  lvmVolumeGroupName: my-san-vg
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

You can have several clustered-csi `StorageClass`es on the same cluster that are
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
