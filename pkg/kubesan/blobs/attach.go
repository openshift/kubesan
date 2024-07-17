// SPDX-License-Identifier: Apache-2.0

package blobs

import (
	"context"
	"fmt"
	"log"

	"gitlab.com/kubesan/kubesan/pkg/kubesan/util/config"
	"gitlab.com/kubesan/kubesan/pkg/kubesan/util/nbd"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (bm *BlobManager) AttachBlob(
	ctx context.Context, blob *Blob, node *string, cookie string,
) (actualNode string, path string, err error) {
	poolSpec, err := bm.getBlobPoolCrd(ctx, blob.pool.name)
	if err != nil {
		return
	}

	if node != nil {
		actualNode = *node
	} else if poolSpec.ActiveOnNode != nil {
		actualNode = *poolSpec.ActiveOnNode
	} else {
		actualNode = config.LocalNodeName
	}

	log.Println(fmt.Sprintf("Attaching blob %s on current node %s", blob, actualNode))
	path, err = bm.attachBlob(ctx, blob, poolSpec, actualNode, cookie)
	if err != nil {
		log.Println(fmt.Sprintf("Failed to attach blob %s on current node %s: %v", blob, actualNode, err))
	}
	return
}

// Ensure that the given blob is attached on the given node (or any node if `node` is nil).
//
// If `node` is nil, this will select a node where the blob gets a "fast" attachment.
//
// Cookies are "namespaced" to the (blob, node) pair.
//
// This method is idempotent and may be called from any node.
func (bm *BlobManager) attachBlob(
	ctx context.Context,
	blob *Blob,
	poolSpec *blobPoolCrdSpec,
	actualNode string,
	cookie string,
) (path string, err error) {
	var activeOnNode string

	if poolSpec.ActiveOnNode == nil {
		activeOnNode = actualNode

		// activate LVM thin *pool* LV

		err = bm.runLvmScriptForThinPoolLv(ctx, blob.pool, actualNode, "activate-pool")
		if err != nil {
			err = status.Errorf(codes.Internal, "failed to activate LVM thin pool LV: %s", err)
			return
		}
	} else {
		activeOnNode = *poolSpec.ActiveOnNode
	}

	path = blob.dmMultipathVolumePath()

	if poolSpec.hasHolder(blob.name, actualNode, cookie) {
		// nothing to do
		return
	}

	if !poolSpec.hasHolderForBlobOnNode(blob.name, actualNode) {
		var lvmOrNbdPath string

		if activeOnNode == actualNode {
			// activate LVM thin LV

			err = bm.runLvmScriptForThinLv(ctx, blob, actualNode, "activate")
			if err != nil {
				err = status.Errorf(codes.Internal, "failed to activate LVM thin LV: %s", err)
				return
			}

			lvmOrNbdPath = blob.lvmThinLvPath()
		} else {
			// start NBD server

			nbdServerId := &nbd.ServerId{
				NodeName: activeOnNode,
				BlobName: blob.name,
			}

			err = nbd.StartServer(ctx, bm.clientset, nbdServerId, blob.lvmThinLvPath())
			if err != nil {
				return
			}

			// connect NBD client

			lvmOrNbdPath, err = nbd.ConnectClient(ctx, bm.clientset, actualNode, nbdServerId)
			if err != nil {
				return
			}
		}

		// create dm-multipath volume

		err = bm.runDmMultipathScript(ctx, blob, actualNode, "create", lvmOrNbdPath)
		if err != nil {
			return
		}
	}

	// TODO: For now we assume that the state hasn't changed since we checked it at the beginning of this method.

	err = bm.atomicUpdateBlobPoolCrd(ctx, blob.pool.name, func(poolSpec *blobPoolCrdSpec) error {
		poolSpec.ActiveOnNode = &activeOnNode
		poolSpec.addHolder(blob.name, actualNode, cookie)
		return nil
	})
	if err != nil {
		return
	}

	return
}

// The reverse of AttachBlob().
//
// Each call to AttachBlob() must be paired with a call to DetachBlob() with the same `cookie`.
//
// This method is idempotent and may be called from any node.
func (bm *BlobManager) DetachBlob(ctx context.Context, blob *Blob, node string, cookie string) error {
	poolSpec, err := bm.getBlobPoolCrd(ctx, blob.pool.name)
	if err != nil {
		return err
	}

	if !poolSpec.hasHolder(blob.name, node, cookie) {
		// nothing to do
		return nil
	}

	poolSpec.removeHolder(blob.name, node, cookie)

	if !poolSpec.hasHolderForBlobOnNode(blob.name, node) {
		// remove dm-multipath volume

		err = bm.runDmMultipathScript(ctx, blob, node, "remove")
		if err != nil {
			return err
		}

		nbdServerId := &nbd.ServerId{
			NodeName: *poolSpec.ActiveOnNode,
			BlobName: blob.name,
		}

		if node != *poolSpec.ActiveOnNode {
			// disconnect NBD client

			err = nbd.DisconnectClient(ctx, bm.clientset, node, nbdServerId)
			if err != nil {
				return err
			}
		}

		if !poolSpec.hasHolderForBlob(blob.name) {
			// no one else is using the NBD server (if any), stop it

			err = nbd.StopServer(ctx, bm.clientset, nbdServerId)
			if err != nil {
				return err
			}

			// deactivate LVM thin LV

			err = bm.runLvmScriptForThinLv(ctx, blob, *poolSpec.ActiveOnNode, "deactivate")
			if err != nil {
				return status.Errorf(codes.Internal, "failed to deactivate LVM thin LV: %s", err)
			}
		}

		if !poolSpec.hasHolderOnNode(*poolSpec.ActiveOnNode) {
			// `node` where the LVM thin *pool* LV is active no longer needs access to the pool

			if len(poolSpec.Holders) > 0 {
				// migrate to any node where the pool is already attached

				err = bm.migratePool(ctx, blob.pool, poolSpec, poolSpec.Holders[0].Node)
				if err != nil {
					return err
				}
			} else {
				// deactivate LVM thin *pool* LV

				err := bm.runLvmScriptForThinPoolLv(ctx, blob.pool, *poolSpec.ActiveOnNode, "deactivate-pool")
				if err != nil {
					return status.Errorf(codes.Internal, "failed to deactivate LVM thin pool LV: %s", err)
				}
			}
		}

	}

	// TODO: For now we assume that the state hasn't changed since we checked it at the beginning of this method.

	err = bm.atomicUpdateBlobPoolCrd(ctx, blob.pool.name, func(poolSpec *blobPoolCrdSpec) error {
		poolSpec.removeHolder(blob.name, node, cookie)
		if len(poolSpec.Holders) == 0 {
			poolSpec.ActiveOnNode = nil
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}
