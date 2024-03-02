// SPDX-License-Identifier: Apache-2.0

package blobs

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/config"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/jobs"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/lvm"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// Creates an empty blob.
//
// This method is idempotent and may be called from any node.
func (bm *BlobManager) CreateBlobEmpty(ctx context.Context, blob *Blob, k8sStorageClassName string, size int64) error {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	// ensure LVM VG exists

	err := bm.createLvmVg(ctx, k8sStorageClassName, blob.BackingDevicePath)
	if err != nil {
		return err
	}

	// create LVM thin pool LV and thin LV

	err = util.RunCommand(
		"scripts/lvm.sh", "create",
		blob.BackingDevicePath, blob.lvmThinPoolLvName(), blob.lvmThinLvName(), strconv.FormatInt(size, 10),
	)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create LVM thin pool LV and thin LV: %s", err)
	}

	// success

	return nil
}

// Creates a blob that is a copy of the given sourceBlob. Subsequent writes to the latter don't affect the former.
//
// sourceBlob does not need to be attached to any node.
//
// This method is idempotent and may be called from any node.
//
// (Internal behavior note: The two blobs gain the restriction that they can only be directly attached simultaneously to
// the same node. Indirect attachments are not affected.)
func (bm *BlobManager) CreateBlobCopy(ctx context.Context, blobName string, sourceBlob *Blob) (*Blob, error) {
	blob := &Blob{
		Name:                    blobName,
		K8sPersistentVolumeName: sourceBlob.K8sPersistentVolumeName,
		BackingDevicePath:       sourceBlob.BackingDevicePath,
	}

	cookie := fmt.Sprintf("copying-to-%s", blob.Name)

	nodeName, _, err := bm.AttachBlob(ctx, sourceBlob, nil, cookie)
	if err != nil {
		return nil, err
	}

	job := &jobs.Job{
		Name:     fmt.Sprintf("create-lv-%s", util.Hash(nodeName, blob.Name)),
		NodeName: nodeName,
		Command: []string{
			"scripts/lvm.sh", "snapshot",
			blob.BackingDevicePath, sourceBlob.lvmThinPoolLvName(), sourceBlob.lvmThinLvName(),
			blob.lvmThinLvName(),
		},
	}

	err = jobs.CreateAndRunAndDelete(ctx, bm.clientset, job)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to snapshot LVM LV: %s", err)
	}

	err = bm.DetachBlob(ctx, sourceBlob, nodeName, cookie)
	if err != nil {
		return nil, err
	}

	return blob, nil
}

func (bm *BlobManager) createLvmVg(ctx context.Context, storageClassName string, pvPath string) error {
	// TODO: This will hang if the CSI controller plugin creating the VG dies. Fix this, maybe using leases.
	// TODO: Communicate VG creation errors to users through events/status on the SC and PVC.

	storageClasses := bm.clientset.StorageV1().StorageClasses()
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
