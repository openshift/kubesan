// SPDX-License-Identifier: Apache-2.0

package blobs

import (
	"context"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Fails if the volume is attached.
//
// This method is idempotent and may be called from any node.
func (bm *BlobManager) DeleteBlob(ctx context.Context, blob *Blob) error {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	// delete LVM thin LV and thin pool LV

	err := util.RunCommand(
		"scripts/lvm.sh", "delete",
		blob.BackingDevicePath, blob.lvmThinPoolLvName(), blob.lvmThinLvName(),
	)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to delete LVM thin LV and thin pool LV: %s", err)
	}

	// success

	return nil
}
