// SPDX-License-Identifier: Apache-2.0

package blobs

import (
	"context"
	"fmt"
	"strings"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/lvm"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Fails if the volume is attached.
//
// This method is idempotent and may be called from any node.
func (bm *BlobManager) DeleteBlob(ctx context.Context, blob *Blob) error {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	// ensure LVM VG lockspace is started

	err := lvm.StartVgLockspace(ctx, blob.BackingDevicePath)
	if err != nil {
		return err
	}

	// remove LVM thin LV

	output, err := lvm.IdempotentLvRemove(ctx, "--devices", blob.BackingDevicePath, blob.lvmThinLvRef())
	if err != nil {
		return status.Errorf(codes.Internal, "failed to remove LVM thin LV: %s: %s", err, output)
	}

	// remove LVM thin pool LV if there are no more thin LVs

	output, err = lvm.IdempotentLvRemove(ctx, "--devices", blob.BackingDevicePath, blob.lvmThinPoolLvRef())
	if err != nil {
		poolHasDependentLvsMsg := fmt.Sprintf("removing pool %s will remove", blob.lvmThinPoolLvRef())
		if !strings.Contains(strings.ToLower(output), poolHasDependentLvsMsg) {
			return status.Errorf(codes.Internal, "failed to remove LVM thin pool LV: %s: %s", err, output)
		}
	}

	// success

	return nil
}
