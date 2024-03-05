// SPDX-License-Identifier: Apache-2.0

package blobs

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/config"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/k8s"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/lvm"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BlobManager struct {
	clientset *k8s.Clientset
}

func NewBlobManager(clientset *k8s.Clientset) *BlobManager {
	return &BlobManager{clientset: clientset}
}

// This method may be called from any node, and fails if the blob does not exist.
func (bm *BlobManager) GetBlobSize(ctx context.Context, blob *Blob) (int64, error) {
	output, err := lvm.Command(
		ctx,
		"lvs",
		"--devices", blob.pool.backingDevicePath,
		"--options", "lv_size",
		"--units", "b",
		"--nosuffix",
		"--noheadings",
		fmt.Sprintf("%s/%s", config.LvmVgName, blob.lvmThinLvName()),
	)
	if err != nil {
		return -1, status.Errorf(codes.Internal, "failed to get size of LVM LV: %s: %s", err, output)
	}

	sizeStr := strings.TrimSpace(output)

	size, err := strconv.ParseInt(sizeStr, 0, 64)
	if err != nil {
		return -1, status.Errorf(codes.Internal, "failed to get size of LVM LV: %s: %s", err, sizeStr)
	}

	return size, nil
}

func (bm *BlobManager) getK8sPvForBlob(ctx context.Context, blob *Blob) (*corev1.PersistentVolume, error) {
	return bm.clientset.CoreV1().PersistentVolumes().Get(ctx, blob.pool.k8sPersistentVolumeName, metav1.GetOptions{})
}

func (bm *BlobManager) atomicUpdateK8sPvForBlobPool(
	ctx context.Context,
	pool *blobPool,
	f func(*corev1.PersistentVolume) error,
) error {
	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: pool.k8sPersistentVolumeName}}

	err := k8s.AtomicUpdate(ctx, bm.clientset.CoreV1().RESTClient(), "persistentvolumes", pv, f)
	if err != nil {
		err = status.Errorf(codes.Internal, "failed to update PersistentVolume: %s", err)
	}

	return nil
}
