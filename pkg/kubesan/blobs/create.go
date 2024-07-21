// SPDX-License-Identifier: Apache-2.0

package blobs

import (
	"context"
	"fmt"
	"strconv"

	"gitlab.com/kubesan/kubesan/pkg/api/v1alpha1"
	"gitlab.com/kubesan/kubesan/pkg/kubesan/util/config"
	"gitlab.com/kubesan/kubesan/pkg/kubesan/util/slices"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Creates an empty blob.
//
// This method is idempotent and may be called from any node.
func (bm *BlobManager) CreateBlobEmpty(ctx context.Context, blob *Blob, k8sStorageClassName string, size int64) error {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	// create CRD

	err := bm.createBlobPoolCrd(ctx, blob.pool.name, &v1alpha1.BlobPoolSpec{Blobs: []string{blob.name}})
	if err != nil {
		return err
	}

	// create LVM thin pool LV and thin LV

	err = bm.runLvmScriptForThinLv(ctx, blob, config.LocalNodeName, "create-empty", strconv.FormatInt(size, 10))
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create LVM thin pool LV and thin LV: %s", err)
	}

	// success

	return nil
}

// Creates a blob that is a copy of the given sourceBlob. Subsequent writes to the latter don't affect the former.
//
// sourceBlob does not need to be attached on any node.
//
// Performance note: Whenever the source and target volumes are simultaneously attached, they cannot each have a "fast"
// attachment to a different node each.
//
// This method is idempotent and may be called from any node.
//
// (Internal behavior note: The two blobs gain the restriction that they can only ever have a "fast" attachment
// simultaneously on the same node.)
func (bm *BlobManager) CreateBlobCopy(ctx context.Context, blobName string, sourceBlob *Blob) (*Blob, error) {
	err := bm.atomicUpdateBlobPoolCrd(ctx, sourceBlob.pool.name, func(spec *v1alpha1.BlobPoolSpec) error {
		spec.Blobs = slices.AppendUnique(spec.Blobs, blobName)
		return nil
	})
	if err != nil {
		return nil, err
	}

	blob := &Blob{
		name: blobName,
		pool: sourceBlob.pool,
	}

	cookie := fmt.Sprintf("copying-to-%s", blob.name)

	nodeName, _, err := bm.AttachBlob(ctx, sourceBlob, nil, cookie)
	if err != nil {
		return nil, err
	}

	err = bm.runLvmScriptForThinLv(ctx, blob, nodeName, "create-snapshot", sourceBlob.lvmThinLvName())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to snapshot LVM LV: %s", err)
	}

	err = bm.DetachBlob(ctx, sourceBlob, nodeName, cookie)
	if err != nil {
		return nil, err
	}

	return blob, nil
}

// Fails if the volume is attached.
//
// This method is idempotent and may be called from any node.
func (bm *BlobManager) DeleteBlob(ctx context.Context, blob *Blob) error {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	// delete LVM thin LV and thin pool LV

	err := bm.runLvmScriptForThinLv(ctx, blob, config.LocalNodeName, "delete")
	if err != nil {
		return status.Errorf(codes.Internal, "failed to delete LVM thin LV and thin pool LV: %s", err)
	}

	var delete bool

	err = bm.atomicUpdateBlobPoolCrd(ctx, blob.pool.name, func(spec *v1alpha1.BlobPoolSpec) error {
		spec.Blobs = slices.Remove(spec.Blobs, blob.name)
		delete = len(spec.Blobs) == 0
		return nil
	})
	if err != nil {
		return err
	}

	if delete {
		err = bm.deleteBlobPoolCrd(ctx, blob.pool.name)
		if err != nil {
			return err
		}
	}

	// success

	return nil
}
