# Development

Information relevant for KubeSAN developers.

## Code design

The CSI plugin implementation is split into two main layers: (1) the "blob
manager" layer and (2) the actual CSI gRPC server implementation layer. The blob
manager layer knows nothing about CSI or gRPC, while the CSI layer relies on the
former and knows nothing about LVM or NBD.

This leads to a nice separation of concerns, with the blob manager layer
managing all storage on the backing shared volume group (including everything
LVM-, device mapper-, and NBD-related), and the CSI layer implementing
higher-level functionality like copying data between blobs (volumes/snapshots)
to implement volume cloning and creating and managing file systems inside blobs
to provision `Filesystem` volumes for users.

### The blob manager layer

The blob manager layer is fully responsible for managing storage on the backing
shared volume group. It exposes functionality to provision "blobs" (essentially
disks) and to attach those blobs to nodes as needed, dealing with all
complexities like managing NBD connections and keeping track of whether a blob
is still needed on a given node and so on. It implements its functionality using
a mix of device mapper, LVM, and NBD.

"Blobs" are named as such instead of simply "volumes" to avoid confusion with
the concept of Kubernetes volumes, as blobs are used to store the contents of
both Kubernetes volumes and volume snapshots.

This layer fully resides in the `pkg/kubesan/blobs` package and provides
a couple types:

- `Blob`: An opaque blob identifier. Create one with `NewBlob(blobName string,
  backingVolumeGroup string)`.
- `BlobManager`: Exposes all functionality around blobs. Create one with
  `NewBlobManager()`.

Here's a quick summary of `BlobManager`'s API:

- Methods to provision new blobs and create copies of existing blobs:
  - `CreateBlobEmpty(blob *Blob, k8sStorageClassName string, size int64)`
  - `CreateBlobCopy(blobName string, sourceBlob *Blob)`
  - `DeleteBlob(blob *Blob)`
- Methods to make blobs available as block devices on a given node:
  - `AttachBlob(blob *Blob, node *string, cookie string)`
  - `DetachBlob(blob *Blob, node string, cookie string)`
  - `OptimizeBlobAttachmentForNode(blob *Blob, node string)`
- Other methods:
  - `GetBlobSize(blob *Blob)`

All of these methods can be called in the context of any node, even if they are
supposed to operate on other nodes.

#### How blobs are stored

An LVM shared Volume Group (VG) must be pre-created on a shared
backing device; KubeSAN assumes that the LVM global lock will
already be accessible to each node in the cluster (things should work
whether the global lock shares the same VG as passed to the
StorageClass, or is stored in a separate VG to make external
management of global lock handover easier). Whenever a new empty blob
is created (`CreateBlobEmpty()`), a thin pool Logical Volume (LV) is
created in the VG, and a thin LV is created in the thin pool. The
blob's contents reside in that last thin LV.

Whenever a blob is created by copying another blob (`CreateBlobCopy()`), a thin
LV snapshotting the source blob's thin LV is created in the latter's pool. This
means that we can have more than one blob residing in the same thin pool LV.

#### "Fast" attachments

LVM thin pools can only be active on one node at a time. To allow attaching the
same blob (or blobs residing in the same thin pool) on several nodes
simultaneously, `BlobManager` launches an NBD server on the node where the thin
pool is active, and other nodes wanting to attach the same blob connect to that
server using the kernel NBD client.

This implies that one of the nodes can access the blob with superior
performance. We say that the blob has a "fast" attachment on that node.

#### Blob pool migration

When an LVM thin pool no longer needs to be accessed on the node where it is
active, but other nodes are still accessing it over NBD, a "blob pool migration"
is triggered. This procedure consists of deactivating the thin pool and
activating it on one of the nodes that is currently accessing it, increasing the
latter's I/O performance.

Although currently unused, the implementation can also perform this migration
while the thin pool is in use on the node where it is active, see
`BlobManager.OptimizeBlobAttachmentForNode()`.

The diagrams below illustrate the before, midway, and after of a migration:

- **Before** migration:

       +---------- NODE A -----------+     +---------- NODE B -----------+
       |                             |     |                             |
       |       Pod                   |     |                    Pod      |
       |        |                    |     |                     |       |
       |        ^                    |     |                     ^       |
       |  dm-multipath               |     |               dm-multipath  |
       |        |                    |     |                     |       |
       |        ^                    |     |                     ^       |
       |    dm-linear   NBD server <---------- nbd.ko <---- dm-linear    |
       |          |       |          |     |                             |
       |          ^       ^          |     |                             |
       |         LVM thin LV         |     |                             |
       |              |              |     |                             |
       |              ^              |     |                             |
       |       LVM thin pool LV      |     |                             |
       |              |              |     |                             |
       +--------------|--------------+     +-----------------------------+
                      |                                   |
                      |         =================         |
                      +-------> === shared VG === <-------+
                                =================
                                        |
                                        ^
                                   ===========
                                   === SAN ===
                                   ===========

- **Halfway** through the migration:

       +---------- NODE A -----------+     +---------- NODE B -----------+
       |                             |     |                             |
       |       Pod                   |     |                    Pod      |
       |        |                    |     |                     |       |
       |        ^                    |     |                     ^       |
       |  dm-multipath               |     |               dm-multipath  |
       |        | (disabled path)    |     |     (disabled path) |       |
       |        ^                    |     |                     ^       |
       |    dm-error                 |     |                 dm-error    |
       |                             |     |                             |
       |                             |     |                             |
       |                             |     |                             |
       |                             |     |                             |
       |                             |     |                             |
       |                             |     |                             |
       |                             |     |                             |
       +-----------------------------+     +-----------------------------+
                      |                                   |
                      |         =================         |
                      +-------> === shared VG === <-------+
                                =================
                                        |
                                        ^
                                   ===========
                                   === SAN ===
                                   ===========

- **After** migration:

       +---------- NODE A -----------+     +---------- NODE B -----------+
       |                             |     |                             |
       |       Pod                   |     |                    Pod      |
       |        |                    |     |                     |       |
       |        ^                    |     |                     ^       |
       |  dm-multipath               |     |               dm-multipath  |
       |        |                    |     |                     |       |
       |        ^                    |     |                     ^       |
       |    dm-linear ----> nbd.ko ----------> NBD server   dm-linear    |
       |                             |     |          |       |          |
       |                             |     |          ^       ^          |
       |                             |     |         LVM thin LV         |
       |                             |     |              |              |
       |                             |     |              ^              |
       |                             |     |       LVM thin pool LV      |
       |                             |     |              |              |
       +-----------------------------+     +--------------|--------------+
                      |                                   |
                      |         =================         |
                      +-------> === shared VG === <-------+
                                =================
                                        |
                                        ^
                                   ===========
                                   === SAN ===
                                   ===========

In more detail, the migration process involves the following steps:

  - For every blob residing in the thin pool and that is attached to some node:
    - For every such node:
      - Pause the corresponding dm-multipath target on all nodes by disabling
         its path (using `dmsetup message`) so that ongoing and incoming I/O is
         queued up.
      - Replace the dm-linear target by a dm-error target so that the underlying
         LVM thin LV or NBD client device is released.
      - Disconnect the underlying NBD client device (if applicable).
    - On the node where the thin pool is currently active:
      - Stop the blob's NBD server if it is running.
      - Deactivate the blob's thin LV.
  - On the node where the thin pool is currently active:
    - Deactivate the thin pool LV.
  - On the node where we now want to activate the thin pool:
    - Activate the thin pool LV.
  - (... and now essentially do the reverse of the steps above ...)

The dm-linear/dm-error layer exists because we cannot dynamically add or remove
paths from a dm-multipath target, only enable and disable existing paths.

### The CSI layer

The CSI layer implements the actual CSI gRPCs like `CreateVolume` and
`NodeStageVolume`. It ends up being relatively simple since most of the
complexity resides in the blob manager layer.

Higher-level functionality that the blob manager layer doesn't implement itself,
like creating independent volume clones and supporting `Filesystem` volumes, is
implemented in this layer.

Cloning volumes and provisioning volumes from snapshots is implemented by
creating a new empty blob for the new volume and copying over the data from the
source blob using a Kubernetes `Job` that runs `dd`.

## Running tests against the working tree

To test, run `tests/run.sh all`. You may need to add yourself to the `libvirt`
group for it to work without prompting you for your password repeatedly.

Run `tests/run.sh` without arguments to see available options. The
`--pause-on-failure` flag is particularly handy during development. With it,
when a test fails, the test harness launches a shell through which you can
inspect the test cluster. The shell also provides several helper commands for
tasks like retrieving CSI component logs and ssh'ing into cluster nodes.

## Sandbox mode for `tests/run.sh`

The test harness also provides a sandbox mode that simply sets up a test cluster
with KubeSAN installed and a corresponding `StorageClass` configured and
gives you a shell to interact with it, providing the same helpers as the
`--pause-on-failure` option. This is useful for experimentation and to do demos.
