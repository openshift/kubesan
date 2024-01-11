// SPDX-License-Identifier: Apache-2.0

package volumemanager

import (
	"context"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/lvm"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Fails if the volume is attached.
//
// This method is idempotent and may be called from any node.
func (vm *VolumeManager) DeleteVolume(ctx context.Context, vol *Volume) error {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	// ensure LVM VG lockspace is started

	err := lvm.StartVgLockspace(ctx, vol.BackingDevicePath)
	if err != nil {
		return err
	}

	// remove LVM thin LV

	output, err := lvm.IdempotentLvRemove(ctx, "--devices", vol.BackingDevicePath, vol.lvmThinLvRef())
	if err != nil {
		return status.Errorf(codes.Internal, "failed to remove LVM thin LV: %s: %s", err, output)
	}

	// remove LVM thin pool LV

	output, err = lvm.IdempotentLvRemove(ctx, "--devices", vol.BackingDevicePath, vol.lvmThinPoolLvRef())
	if err != nil {
		return status.Errorf(codes.Internal, "failed to remove LVM thin pool LV: %s: %s", err, output)
	}

	// success

	return nil
}
