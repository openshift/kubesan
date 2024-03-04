// SPDX-License-Identifier: Apache-2.0

package blobs

import (
	"context"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/nbd"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/slices"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Adjusts blob attachments such that `blob` has the best I/O performance on `node`.
//
// This may affect the performance of blobs the were (transitively) copied from `blob` or from which `blob` was copied.
//
// Does nothing if the blob isn't attached to any node.
func (bm *BlobManager) OptimizeBlobAttachmentForNode(ctx context.Context, blob *Blob, node string) error {
	poolState, err := bm.getBlobPoolState(ctx, blob)
	if err != nil {
		return err
	}

	err = bm.migratePool(ctx, blob.pool, poolState, node)
	if err != nil {
		return err
	}

	return nil
}

// Works even if the pool has no holders on `toNode`.
//
// Does nothing if the pool has no holders at all.
func (bm *BlobManager) migratePool(ctx context.Context, pool *blobPool, poolState *blobPoolState, toNode string) error {
	if len(poolState.Holders) == 0 {
		return nil
	}

	err := bm.migratePoolDown(ctx, pool, poolState)
	if err != nil {
		return err
	}

	err = bm.migratePoolUp(ctx, pool, poolState, toNode)
	if err != nil {
		return err
	}

	return nil
}

func (bm *BlobManager) migratePoolDown(ctx context.Context, pool *blobPool, poolState *blobPoolState) error {
	nodeWithActiveLvmThinPoolLv := *poolState.ActiveOnNode

	for _, blobName := range poolState.blobsWithHolders() {
		blob := &Blob{
			Name: blobName,
			pool: pool,
		}

		nbdServerId := &nbd.ServerId{
			NodeName: nodeWithActiveLvmThinPoolLv,
			BlobName: blobName,
		}

		for _, node := range poolState.nodesWithHoldersForBlob(blobName) {
			err := bm.runDmMultipathScript(ctx, blob, node, "disconnect")
			if err != nil {
				return err
			}

			if node != nodeWithActiveLvmThinPoolLv {
				err = nbd.DisconnectClient(ctx, bm.clientset, node, nbdServerId)
				if err != nil {
					return err
				}
			}
		}

		err := nbd.StopServer(ctx, bm.clientset, nbdServerId)
		if err != nil {
			return err
		}

		err = bm.runLvmScriptForThinLv(ctx, blob, nodeWithActiveLvmThinPoolLv, "deactivate")
		if err != nil {
			return status.Errorf(codes.Internal, "failed to deactivate LVM thin LV: %s", err)
		}

	}

	// deactivate LVM thin *pool* LV

	err := bm.runLvmScriptForThinPoolLv(ctx, pool, nodeWithActiveLvmThinPoolLv, "deactivate-pool")
	if err != nil {
		return status.Errorf(codes.Internal, "failed to deactivate LVM thin pool LV: %s", err)
	}

	return nil
}

// Assumes that there are holders for the pool.
func (bm *BlobManager) migratePoolUp(
	ctx context.Context, pool *blobPool, poolState *blobPoolState, newNodeWithActiveLvmThinPoolLv string,
) error {
	// activate LVM thin *pool* LV

	err := bm.runLvmScriptForThinPoolLv(ctx, pool, newNodeWithActiveLvmThinPoolLv, "activate-pool")
	if err != nil {
		return status.Errorf(codes.Internal, "failed to activate LVM thin pool LV: %s", err)
	}

	for _, blobName := range poolState.blobsWithHolders() {
		blob := &Blob{
			Name: blobName,
			pool: pool,
		}

		err = bm.runLvmScriptForThinLv(ctx, blob, newNodeWithActiveLvmThinPoolLv, "activate")
		if err != nil {
			return status.Errorf(codes.Internal, "failed to activate LVM thin LV: %s", err)
		}

		if poolState.hasHolderForBlobOnNode(blobName, newNodeWithActiveLvmThinPoolLv) {
			err = bm.runDmMultipathScript(ctx, blob, newNodeWithActiveLvmThinPoolLv, "connect", blob.lvmThinLvPath())
			if err != nil {
				return err
			}
		}

		otherNodesHoldingBlob := poolState.nodesWithHoldersForBlob(blobName)
		slices.Remove(otherNodesHoldingBlob, newNodeWithActiveLvmThinPoolLv)

		if len(otherNodesHoldingBlob) > 0 {
			nbdServerId := &nbd.ServerId{
				NodeName: newNodeWithActiveLvmThinPoolLv,
				BlobName: blobName,
			}

			err = nbd.StartServer(ctx, bm.clientset, nbdServerId, blob.lvmThinLvPath())
			if err != nil {
				return err
			}

			for _, node := range otherNodesHoldingBlob {
				nbdDevicePath, err := nbd.ConnectClient(ctx, bm.clientset, node, nbdServerId)
				if err != nil {
					return err
				}

				err = bm.runDmMultipathScript(ctx, blob, node, "connect", nbdDevicePath)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
