// SPDX-License-Identifier: Apache-2.0

package volumemanager

import (
	"context"
	"fmt"
	"time"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/config"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/lvm"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// This method is idempotent and may be called from any node.
func (vm *VolumeManager) CreateVolume(ctx context.Context, vol *Volume, k8sStorageClassName string, size int64) error {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	// ensure LVM VG exists

	err := vm.createLvmVg(ctx, k8sStorageClassName, vol.BackingDevicePath)
	if err != nil {
		return err
	}

	// ensure LVM VG lockspace is started

	err = lvm.StartVgLockspace(ctx, vol.BackingDevicePath)
	if err != nil {
		return err
	}

	// create LVM thin pool LV

	sizeString := fmt.Sprintf("%db", size)

	output, err := lvm.IdempotentLvCreate(
		ctx,
		"--devices", vol.BackingDevicePath,
		"--activate", "n",
		"--type", "thin-pool",
		"--name", vol.lvmThinPoolLvName(),
		"--size", sizeString,
		config.LvmVgName,
	)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create LVM thin pool LV: %s: %s", err, output)
	}

	// create LVM thin LV

	output, err = lvm.IdempotentLvCreate(
		ctx,
		"--devices", vol.BackingDevicePath,
		"--type", "thin",
		"--name", vol.lvmThinLvName(),
		"--thinpool", vol.lvmThinPoolLvName(),
		"--virtualsize", sizeString,
		config.LvmVgName,
	)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create LVM thin LV: %s: %s", err, output)
	}

	// deactivate LVM thin LV (`--activate n` has no effect on `lvcreate --type thin`)

	output, err = lvm.Command(
		ctx,
		"lvchange",
		"--devices", vol.BackingDevicePath,
		"--activate", "n",
		vol.lvmThinLvRef(),
	)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to deactivate LVM thin LV: %s: %s", err, output)
	}

	// success

	return nil
}

func (vm *VolumeManager) createLvmVg(ctx context.Context, storageClassName string, pvPath string) error {
	// TODO: This will hang if the CSI controller plugin creating the VG dies. Fix this, maybe using leases.
	// TODO: Communicate VG creation errors to users through events/status on the SC and PVC.

	storageClasses := vm.clientset.StorageV1().StorageClasses()
	stateAnnotation := fmt.Sprintf("%s/vg-state", config.Domain)

	// check VG state

	var sc *storagev1.StorageClass
	var shouldCreateVg bool

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error
		sc, err = storageClasses.Get(ctx, storageClassName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		switch state, _ := sc.Annotations[stateAnnotation]; state {
		case "", "creation-failed":
			// VG wasn't created and isn't being created, try to create it ourselves

			if sc.Annotations == nil {
				sc.Annotations = map[string]string{}
			}
			sc.Annotations[stateAnnotation] = "creating"

			sc, err = storageClasses.Update(ctx, sc, metav1.UpdateOptions{})
			if err != nil {
				return err
			}

			shouldCreateVg = true

		case "creating", "created":
			shouldCreateVg = false
		}

		return nil
	})
	if err != nil {
		return err
	}

	// create VG or wait until it is created

	if shouldCreateVg {
		_, err = lvm.Command(ctx, "vgcreate", "--lock-type", "sanlock", config.LvmVgName, pvPath)

		if err == nil {
			sc.Annotations[stateAnnotation] = "created"
		} else {
			sc.Annotations[stateAnnotation] = "creation-failed"
		}

		// don't use ctx so that we don't fail to update the annotation after successfully creating the VG
		// TODO: This fails if the SC was modified meanwhile, fix this.
		_, err = storageClasses.Update(context.Background(), sc, metav1.UpdateOptions{})
		return err
	} else {
		// TODO: Watch instead of polling.
		for {
			sc, err := storageClasses.Get(ctx, storageClassName, metav1.GetOptions{})

			if err != nil {
				return err
			} else if ctx.Err() != nil {
				return ctx.Err()
			} else if sc.Annotations[stateAnnotation] == "created" {
				return nil
			}

			time.Sleep(1 * time.Second)
		}
	}
}
