// SPDX-License-Identifier: Apache-2.0

package blobs

import (
	"context"
	"encoding/json"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/config"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/nbd"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/slices"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type blobPoolState struct {
	// The node on which the LVM thin *pool* LV backing this blob pool is active, if any.
	ActiveOnNode *string `json:"activeOnNode"`

	Holders []blobPoolHolder `json:"holders"`
}

type blobPoolHolder struct {
	Blob   string `json:"blob"`
	Node   string `json:"node"`
	Cookie string `json:"cookie"`
}

func newBlobPoolState() *blobPoolState {
	return &blobPoolState{ActiveOnNode: nil, Holders: []blobPoolHolder{}}
}

func (bps *blobPoolState) blobsWithHolders() []string {
	var blobs []string
	for _, holder := range bps.Holders {
		blobs = slices.AppendUnique(blobs, holder.Blob)
	}
	return blobs
}

func (bps *blobPoolState) nodesWithHoldersForBlob(blob string) []string {
	var nodes []string
	for _, holder := range bps.Holders {
		if holder.Blob == blob {
			nodes = slices.AppendUnique(nodes, holder.Node)
		}
	}
	return nodes
}

func (bps *blobPoolState) hasHolderForBlob(blob string) bool {
	return slices.Any(bps.Holders, func(h blobPoolHolder) bool { return h.Blob == blob })
}

func (bps *blobPoolState) hasHolderOnNode(node string) bool {
	return slices.Any(bps.Holders, func(h blobPoolHolder) bool { return h.Node == node })
}

func (bps *blobPoolState) hasHolderForBlobOnNode(blob string, node string) bool {
	return slices.Any(bps.Holders, func(h blobPoolHolder) bool { return h.Blob == blob && h.Node == node })
}

func (bps *blobPoolState) addHolder(blob *Blob, node string, cookie string) {
	holder := blobPoolHolder{Blob: blob.Name, Node: node, Cookie: cookie}
	bps.Holders = slices.AppendUnique(bps.Holders, holder)
}

func (bps *blobPoolState) removeHolder(blob *Blob, node string, cookie string) {
	holder := blobPoolHolder{Blob: blob.Name, Node: node, Cookie: cookie}
	bps.Holders = slices.Remove(bps.Holders, holder)
}

func blobPoolStateFromK8sMeta(obj metav1.ObjectMetaAccessor) (*blobPoolState, error) {
	if obj.GetObjectMeta().GetAnnotations() == nil {
		obj.GetObjectMeta().SetAnnotations(map[string]string{})
	}

	annotation, _ := obj.GetObjectMeta().GetAnnotations()[config.Domain+"/blob-pool-state"]
	poolState := newBlobPoolState()

	if annotation != "" {
		err := json.Unmarshal([]byte(annotation), poolState)
		if err != nil {
			return nil, err
		}
	}

	return poolState, nil
}

func (poolState *blobPoolState) setBlobPoolStateOnK8sMeta(obj metav1.ObjectMetaAccessor) error {
	if poolState.Holders == nil {
		poolState.Holders = []blobPoolHolder{}
	}

	annotation, err := json.Marshal(poolState)
	if err != nil {
		return err
	}

	if obj.GetObjectMeta().GetAnnotations() == nil {
		obj.GetObjectMeta().SetAnnotations(map[string]string{})
	}

	obj.GetObjectMeta().GetAnnotations()[config.Domain+"/blob-pool-state"] = string(annotation)

	return nil
}

func (bm *BlobManager) getBlobPoolState(ctx context.Context, blob *Blob) (*blobPoolState, error) {
	pv, err := bm.getK8sPvForBlob(ctx, blob)
	if err != nil {
		return nil, err
	}

	poolState, err := blobPoolStateFromK8sMeta(pv)
	if err != nil {
		return nil, err
	}

	return poolState, nil
}

// Ensure that the given blob is attached on the given node (or any node if `node` is nil).
//
// If `node` is nil, this will select a node where the blob gets a "fast" attachment.
//
// Cookies are "namespaced" to the (blob, node) pair.
//
// This method is idempotent and may be called from any node.
func (bm *BlobManager) AttachBlob(
	ctx context.Context, blob *Blob, node *string, cookie string,
) (actualNode string, path string, err error) {
	poolState, err := bm.getBlobPoolState(ctx, blob)
	if err != nil {
		return
	}

	if node != nil {
		actualNode = *node
	} else if poolState.ActiveOnNode != nil {
		actualNode = *poolState.ActiveOnNode
	} else {
		actualNode = config.LocalNodeName
	}

	var activeOnNode string

	if poolState.ActiveOnNode == nil {
		activeOnNode = actualNode

		// activate LVM thin *pool* LV

		err = bm.runLvmScriptForThinPoolLv(ctx, blob.pool, actualNode, "activate-pool")
		if err != nil {
			err = status.Errorf(codes.Internal, "failed to activate LVM thin pool LV: %s", err)
			return
		}
	} else {
		activeOnNode = *poolState.ActiveOnNode
	}

	if !poolState.hasHolderForBlobOnNode(blob.Name, actualNode) {
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
				BlobName: blob.Name,
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

	path = blob.dmMultipathVolumePath()

	// TODO: For now we assume that the state hasn't changed since we checked it at the beginning of this method.

	err = bm.atomicUpdateK8sPvForBlob(ctx, blob, func(pv *corev1.PersistentVolume) error {
		poolState, err := blobPoolStateFromK8sMeta(pv)
		if err != nil {
			return err
		}

		poolState.ActiveOnNode = &activeOnNode
		poolState.addHolder(blob, actualNode, cookie)

		err = poolState.setBlobPoolStateOnK8sMeta(pv)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return
	}

	return
}

// Like AttachBlob(), but works before the blob's corresponding Kubernetes PersistentVolume exists.
//
// This method is idempotent and may be called from any node.
func (bm *BlobManager) AttachBlobUnmanaged(
	ctx context.Context, blob *Blob, node *string,
) (actualNode string, path string, err error) {
	if node != nil {
		actualNode = *node
	} else {
		actualNode = config.LocalNodeName
	}

	// activate LVM thin *pool* LV

	err = bm.runLvmScriptForThinPoolLv(ctx, blob.pool, actualNode, "activate-pool")
	if err != nil {
		err = status.Errorf(codes.Internal, "failed to activate LVM thin pool LV: %s", err)
		return
	}

	// activate LVM thin LV

	err = bm.runLvmScriptForThinLv(ctx, blob, actualNode, "activate")
	if err != nil {
		err = status.Errorf(codes.Internal, "failed to activate LVM thin LV: %s", err)
		return
	}

	path = blob.lvmThinLvPath()

	return
}

// The reverse of AttachBlob().
//
// Each call to AttachBlob() must be paired with a call to DetachBlob() with the same `cookie`.
//
// This method is idempotent and may be called from any node.
func (bm *BlobManager) DetachBlob(ctx context.Context, blob *Blob, node string, cookie string) error {
	poolState, err := bm.getBlobPoolState(ctx, blob)
	if err != nil {
		return err
	}

	if poolState.ActiveOnNode == nil {
		return nil
	}

	poolState.removeHolder(blob, node, cookie)

	if !poolState.hasHolderForBlobOnNode(blob.Name, node) {
		// remove dm-multipath volume

		err = bm.runDmMultipathScript(ctx, blob, node, "remove")
		if err != nil {
			return err
		}

		nbdServerId := &nbd.ServerId{
			NodeName: *poolState.ActiveOnNode,
			BlobName: blob.Name,
		}

		if node != *poolState.ActiveOnNode {
			// disconnect NBD client

			err = nbd.DisconnectClient(ctx, bm.clientset, node, nbdServerId)
			if err != nil {
				return err
			}
		}

		if !poolState.hasHolderForBlob(blob.Name) {
			// no one else is using the NBD server (if any), stop it

			err = nbd.StopServer(ctx, bm.clientset, nbdServerId)
			if err != nil {
				return err
			}

			// deactivate LVM thin LV

			err = bm.runLvmScriptForThinLv(ctx, blob, *poolState.ActiveOnNode, "deactivate")
			if err != nil {
				return status.Errorf(codes.Internal, "failed to deactivate LVM thin LV: %s", err)
			}
		}

		if !poolState.hasHolderOnNode(*poolState.ActiveOnNode) {
			// `node` where the LVM thin *pool* LV is active no longer needs access to the pool

			if len(poolState.Holders) > 0 {
				// migrate to any node where the pool is already attached

				err = bm.migratePool(ctx, blob.pool, poolState, poolState.Holders[0].Node)
				if err != nil {
					return err
				}
			} else {
				// deactivate LVM thin *pool* LV

				err := bm.runLvmScriptForThinPoolLv(ctx, blob.pool, *poolState.ActiveOnNode, "deactivate-pool")
				if err != nil {
					return status.Errorf(codes.Internal, "failed to deactivate LVM thin pool LV: %s", err)
				}
			}
		}

	}

	// TODO: For now we assume that the state hasn't changed since we checked it at the beginning of this method.

	err = bm.atomicUpdateK8sPvForBlob(ctx, blob, func(pv *corev1.PersistentVolume) error {
		poolState, err := blobPoolStateFromK8sMeta(pv)
		if err != nil {
			return err
		}

		poolState.removeHolder(blob, node, cookie)
		if len(poolState.Holders) == 0 {
			poolState.ActiveOnNode = nil
		}

		err = poolState.setBlobPoolStateOnK8sMeta(pv)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// Like DetachBlob(), but works before the blob's corresponding Kubernetes PersistentVolume exists.
//
// This method is idempotent and may be called from any node.
func (bm *BlobManager) DetachBlobUnmanaged(ctx context.Context, blob *Blob, node string) error {
	// deactivate LVM thin LV

	err := bm.runLvmScriptForThinLv(ctx, blob, node, "deactivate")
	if err != nil {
		return status.Errorf(codes.Internal, "failed to deactivate LVM thin LV: %s", err)
	}

	// deactivate LVM thin *pool* LV

	err = bm.runLvmScriptForThinPoolLv(ctx, blob.pool, node, "deactivate-pool")
	if err != nil {
		return status.Errorf(codes.Internal, "failed to deactivate LVM thin pool LV: %s", err)
	}

	return nil
}
